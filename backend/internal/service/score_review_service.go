package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/onlineexam/onlineexam/internal/constants"
	"github.com/onlineexam/onlineexam/internal/dto"
	"github.com/onlineexam/onlineexam/internal/model"
	"github.com/onlineexam/onlineexam/internal/repository"
	"github.com/onlineexam/onlineexam/internal/util"
)

// ScoreReviewService 成绩复核服务：学生 48h 内发起一次复核、教师受理更正或驳回，全程留痕。
// 依赖方向：service → repository / 其他 service（record 复用答卷加载与试卷信息）。
type ScoreReviewService struct {
	repo    repository.ScoreReviewRepository
	record  repository.ExamRecordRepository // 原子挂载/写回复核结果，保证并发下原成绩不被越权改动
	examRec *ExamRecordService              // 复用答卷查询（AppError 包装）与试卷校验
	audit   *AuditService                   // service 层审计埋点（与中间件 HTTP 审计互补，全程留痕）
	logger  *slog.Logger
}

// NewScoreReviewService 构造成绩复核服务。
func NewScoreReviewService(
	repo repository.ScoreReviewRepository,
	record repository.ExamRecordRepository,
	examRec *ExamRecordService,
	audit *AuditService,
	logger *slog.Logger,
) *ScoreReviewService {
	return &ScoreReviewService{repo: repo, record: record, examRec: examRec, audit: audit, logger: logger}
}

// Operator 复核操作人（由 handler 从 JWT claims 组装）。
type Operator struct {
	ID   primitive.ObjectID
	Name string
	Role string
}

// Create 学生发起成绩复核。
// 规则：仅本人答卷；答卷须已批改完成；批改完成 48h 内；每份答卷至多一条复核（含已处理）。
func (s *ScoreReviewService) Create(ctx context.Context, recordID primitive.ObjectID, reason string, op Operator) (*model.ScoreReview, error) {
	if strings.TrimSpace(reason) == "" {
		return nil, util.NewAppError(constants.CodeBadRequest, constants.MsgReviewReasonEmpty)
	}

	rec, err := s.examRec.GetByID(ctx, recordID)
	if err != nil {
		return nil, err
	}
	// 越权：学生只能对本人成绩发起复核
	if op.Role == constants.RoleStudent && rec.StudentID != op.ID {
		s.logger.Warn("越权发起复核被拒绝", "record_id", recordID.Hex(), "operator", op.ID.Hex(), "owner", rec.StudentID.Hex())
		return nil, util.NewAppError(constants.CodeForbidden, fmt.Sprintf(constants.MsgReviewForbidden, op.Role))
	}

	// 每份答卷只允许一条复核申请（无论待处理还是已处理）
	if existing, err := s.repo.FindByRecord(ctx, recordID); err == nil {
		if existing.Status == constants.ReviewStatusPending {
			return nil, util.NewAppError(constants.CodeReviewPendingExists, fmt.Sprintf(constants.MsgReviewPendingExists, recordID.Hex()))
		}
		return nil, util.NewAppError(constants.CodeReviewExists, fmt.Sprintf(constants.MsgReviewExists, recordID.Hex()))
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, fmt.Errorf("score review service create find existing: %w", err)
	}

	// 必须批改完成才能复核
	if !gradingCompleted(rec) {
		return nil, util.NewAppError(constants.CodeReviewNotGraded, fmt.Sprintf(constants.MsgReviewNotGraded, recordID.Hex(), rec.Status))
	}
	// 48h 窗口校验
	base := reviewWindowBase(rec)
	if base == nil {
		return nil, util.NewAppError(constants.CodeReviewNotGraded, fmt.Sprintf(constants.MsgReviewNotGraded, recordID.Hex(), rec.Status))
	}
	deadline := base.Add(constants.ReviewApplyWindow)
	if now := time.Now(); !now.Before(deadline) {
		s.logger.Info(constants.LogReviewBlocked, "record_id", recordID.Hex(), "student", op.Name, "reason", "window_closed")
		return nil, util.NewAppError(constants.CodeReviewWindowClosed, fmt.Sprintf(constants.MsgReviewWindowClosed, recordID.Hex()))
	}

	// 及格线快照（回退到试卷当前配置）
	exam, examErr := s.examRec.exam.GetByID(ctx, rec.ExamID)
	passScore := rec.PassScore
	if examErr == nil {
		passScore = passScoreOf(rec, exam)
	}
	originalPassed := passScore > 0 && rec.FinalScore >= passScore

	now := time.Now()
	rev := &model.ScoreReview{
		ID:             primitive.NewObjectID(),
		RecordID:       rec.ID,
		ExamID:         rec.ExamID,
		ExamTitle:      rec.ExamTitle,
		StudentID:      rec.StudentID,
		StudentName:    rec.StudentName,
		Status:         constants.ReviewStatusPending,
		Reason:         strings.TrimSpace(reason),
		OriginalScore:  rec.FinalScore,
		OriginalPassed: originalPassed,
		History: []model.ReviewHistoryItem{{
			Action:        constants.ReviewActionSubmit,
			OperatorID:    op.ID.Hex(),
			OperatorName:  op.Name,
			OperatorRole:  op.Role,
			Opinion:       strings.TrimSpace(reason),
			FromStatus:    "",
			ToStatus:      constants.ReviewStatusPending,
			OriginalScore: rec.FinalScore,
			OccurredAt:    now,
		}},
		Deadline:  deadline,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.repo.Create(ctx, rev); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			return nil, util.NewAppError(constants.CodeReviewPendingExists, fmt.Sprintf(constants.MsgReviewPendingExists, recordID.Hex()))
		}
		return nil, fmt.Errorf("score review service create: %w", err)
	}

	// 原子挂载到答卷：仅当答卷尚无复核时成功；失败则回滚已插入的复核单
	if err := s.record.AttachPendingReview(ctx, rec.ID, rev.ID); err != nil {
		if delErr := s.repo.DeleteByID(ctx, rev.ID); delErr != nil {
			s.logger.Error("复核单回滚失败，请人工核对", "review_id", rev.ID.Hex(), "error", delErr.Error())
		}
		if errors.Is(err, repository.ErrConflict) {
			return nil, util.NewAppError(constants.CodeReviewPendingExists, fmt.Sprintf(constants.MsgReviewPendingExists, recordID.Hex()))
		}
		return nil, fmt.Errorf("score review service attach: %w", err)
	}

	s.writeAudit(ctx, op, constants.AuditActionReviewSubmit, rev, "发起复核："+rev.Reason)
	s.logger.Info(constants.LogReviewSubmitted, "review_id", rev.ID.Hex(), "record_id", rec.ID.Hex(), "student", op.Name)
	return rev, nil
}

// Process 教师处理复核：action=approve 受理（可更正总分），action=reject 驳回。两种处理都必须填写意见。
func (s *ScoreReviewService) Process(ctx context.Context, reviewID primitive.ObjectID, req dto.HandleScoreReviewRequest, op Operator) (*model.ScoreReview, error) {
	if strings.TrimSpace(req.Opinion) == "" {
		return nil, util.NewAppError(constants.CodeReviewOpinionEmpty, fmt.Sprintf(constants.MsgReviewOpinionEmpty, op.Role))
	}
	rev, err := s.repo.FindByID(ctx, reviewID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(constants.CodeReviewNotFound, fmt.Sprintf(constants.MsgReviewNotFound, reviewID.Hex(), op.Role))
		}
		return nil, fmt.Errorf("score review service process find: %w", err)
	}
	if rev.Status != constants.ReviewStatusPending {
		return nil, util.NewAppError(constants.CodeReviewStatusErr, fmt.Sprintf(constants.MsgReviewStatusInvalid, reviewID.Hex(), rev.Status, op.Role))
	}

	rec, err := s.examRec.GetByID(ctx, rev.RecordID)
	if err != nil {
		return nil, err
	}
	// 试卷可能被删除/加载失败：总分以答卷题目分值快照为准，及格线以答卷快照为准。
	total := sumQuestionScores(rec)
	pass := rec.PassScore
	if exam, examErr := s.examRec.exam.GetByID(ctx, rev.ExamID); examErr == nil && exam != nil {
		if total == 0 {
			total = exam.TotalScore
		}
		if pass == 0 {
			pass = exam.PassScore
		}
	}

	now := time.Now()
	toStatus := constants.ReviewStatusPending
	var correctedPtr *float64
	switch req.Action {
	case "approve":
		toStatus = constants.ReviewStatusApproved
		if req.CorrectedScore != nil {
			cs := *req.CorrectedScore
			if cs < 0 || (total > 0 && cs > total) {
				return nil, util.NewAppError(constants.CodeReviewScoreOutOfRange, fmt.Sprintf(constants.MsgReviewScoreRange, cs, total))
			}
			correctedPtr = &cs
		}
	case "reject":
		toStatus = constants.ReviewStatusRejected
		if req.CorrectedScore != nil {
			// 驳回不允许改分：显式拒绝，避免越权改分通道
			return nil, util.NewAppError(constants.CodeBadRequest, fmt.Sprintf(constants.MsgReviewStatusInvalid, reviewID.Hex(), "reject_with_score", op.Role))
		}
	default:
		return nil, util.NewAppError(constants.CodeBadRequest, fmt.Sprintf(constants.MsgValidationFailed, "action"))
	}

	// 计算受理后的更正结果
	finalScore := rev.OriginalScore
	scoreCorrected := false
	correctedPassed := rev.OriginalPassed
	if toStatus == constants.ReviewStatusApproved && correctedPtr != nil {
		finalScore = *correctedPtr
		scoreCorrected = finalScore != rev.OriginalScore
		correctedPassed = pass > 0 && finalScore >= pass
	}

	// 1) 原子写回答卷（条件：同一 pending 复核单）。原成绩在此步之前完全不变。
	if err := s.record.ApplyReviewResult(ctx, rec.ID, rev.ID, toStatus, correctedPtr); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			return nil, util.NewAppError(constants.CodeReviewStatusErr, fmt.Sprintf(constants.MsgReviewStatusInvalid, reviewID.Hex(), "changed", op.Role))
		}
		return nil, fmt.Errorf("score review service apply record: %w", err)
	}

	// 2) 原子迁移复核单状态（条件：_id + pending），附带留痕。并发/重复处理只会一方成功。
	history := append(append([]model.ReviewHistoryItem{}, rev.History...), model.ReviewHistoryItem{
		Action:         mapAction(req.Action),
		OperatorID:     op.ID.Hex(),
		OperatorName:   op.Name,
		OperatorRole:   op.Role,
		Opinion:        strings.TrimSpace(req.Opinion),
		FromStatus:     constants.ReviewStatusPending,
		ToStatus:       toStatus,
		OriginalScore:  rev.OriginalScore,
		CorrectedScore: finalScore,
		OccurredAt:     now,
	})
	set := bson.M{
		"status":           toStatus,
		"teacher_opinion":  strings.TrimSpace(req.Opinion),
		"teacher_id":       op.ID,
		"teacher_name":     op.Name,
		"score_corrected":  scoreCorrected,
		"corrected_score":  finalScore,
		"corrected_passed": correctedPassed,
		"history":          history,
		"processed_at":     now,
		"updated_at":       now,
	}
	matched, err := s.repo.ApplyPendingResult(ctx, rev.ID, bson.M{"$set": set})
	if err != nil {
		// 极端情况：答卷已改但复核单迁移失败。原成绩已按受理结果更正，记录错误日志供对账。
		s.logger.Error("复核单状态迁移失败，答卷已更新，需人工对账", "review_id", rev.ID.Hex(), "record_id", rec.ID.Hex(), "error", err.Error())
		return nil, fmt.Errorf("score review service apply review: %w", err)
	}
	if !matched {
		s.logger.Error("复核单已被并发处理，但答卷写回成功，需人工对账", "review_id", rev.ID.Hex(), "record_id", rec.ID.Hex())
		return nil, util.NewAppError(constants.CodeReviewStatusErr, fmt.Sprintf(constants.MsgReviewStatusInvalid, reviewID.Hex(), "changed", op.Role))
	}

	actionConst := constants.AuditActionReviewApprove
	logTpl := constants.LogReviewApproved
	if toStatus == constants.ReviewStatusRejected {
		actionConst = constants.AuditActionReviewReject
		logTpl = constants.LogReviewRejected
	}
	s.writeAudit(ctx, op, actionConst, rev, strings.TrimSpace(req.Opinion))
	s.logger.Info(logTpl,
		"review_id", rev.ID.Hex(), "record_id", rec.ID.Hex(),
		"from", constants.ReviewStatusPending, "to", toStatus,
		"original_score", rev.OriginalScore, "corrected_score", finalScore, "teacher", op.Name)

	return s.repo.FindByID(ctx, rev.ID)
}

// GetForViewer 查看复核详情：学生仅限本人，教师/管理员可查看全部。
func (s *ScoreReviewService) GetForViewer(ctx context.Context, id primitive.ObjectID, op Operator) (*model.ScoreReview, error) {
	rev, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(constants.CodeReviewNotFound, fmt.Sprintf(constants.MsgReviewNotFound, id.Hex(), op.Role))
		}
		return nil, fmt.Errorf("score review service get: %w", err)
	}
	if op.Role == constants.RoleStudent && rev.StudentID != op.ID {
		s.logger.Warn("越权查看复核被拒绝", "review_id", id.Hex(), "operator", op.ID.Hex())
		return nil, util.NewAppError(constants.CodeForbidden, fmt.Sprintf(constants.MsgReviewForbidden, op.Role))
	}
	return rev, nil
}

// GetByRecordForViewer 按答卷查看复核（学生本人 / 教师管理员）。
func (s *ScoreReviewService) GetByRecordForViewer(ctx context.Context, recordID primitive.ObjectID, op Operator) (*model.ScoreReview, error) {
	rev, err := s.repo.FindByRecord(ctx, recordID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(constants.CodeReviewNotFound, fmt.Sprintf(constants.MsgReviewNotFound, recordID.Hex(), op.Role))
		}
		return nil, fmt.Errorf("score review service get by record: %w", err)
	}
	if op.Role == constants.RoleStudent && rev.StudentID != op.ID {
		return nil, util.NewAppError(constants.CodeForbidden, fmt.Sprintf(constants.MsgReviewForbidden, op.Role))
	}
	return rev, nil
}

// ListByStudent 学生查询自己的复核申请。
func (s *ScoreReviewService) ListByStudent(ctx context.Context, studentID primitive.ObjectID, status string, page, pageSize int64) ([]*model.ScoreReview, int64, error) {
	filter := bson.M{"student_id": studentID}
	if status != "" {
		filter["status"] = status
	}
	list, total, err := s.repo.List(ctx, filter, page, pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("score review service list by student: %w", err)
	}
	return list, total, nil
}

// ListForTeacher 教师/管理员查询复核（可按状态、试卷过滤）。
func (s *ScoreReviewService) ListForTeacher(ctx context.Context, filter bson.M, page, pageSize int64) ([]*model.ScoreReview, int64, error) {
	list, total, err := s.repo.List(ctx, filter, page, pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("score review service list for teacher: %w", err)
	}
	return list, total, nil
}

// writeAudit service 层审计埋点：申请/受理/驳回全程落 audit_logs（含意见与分数变更）。
func (s *ScoreReviewService) writeAudit(ctx context.Context, op Operator, action string, rev *model.ScoreReview, detail string) {
	entry := &model.AuditLog{
		ID:         primitive.NewObjectID(),
		UserID:     op.ID,
		Username:   op.Name,
		Role:       op.Role,
		Module:     "score-reviews",
		Action:     action,
		Method:     "SERVICE",
		Path:       fmt.Sprintf("/api/v1/score-reviews/%s", rev.ID.Hex()),
		StatusCode: 200,
		Detail:     fmt.Sprintf("record_id=%s status=%s %s", rev.RecordID.Hex(), rev.Status, detail),
		CreatedAt:  time.Now(),
	}
	if err := s.audit.Create(ctx, entry); err != nil {
		s.logger.Warn("复核审计埋点写入失败", "review_id", rev.ID.Hex(), "error", err.Error())
	}
}

func mapAction(a string) string {
	switch a {
	case "approve":
		return constants.ReviewActionApprove
	case "reject":
		return constants.ReviewActionReject
	default:
		return a
	}
}

func sumQuestionScores(rec *model.ExamRecord) float64 {
	var total float64
	for i := range rec.Questions {
		total += rec.Questions[i].Score
	}
	return total
}
