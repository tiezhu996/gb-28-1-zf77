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

// ScoreReviewService 成绩复核服务：学生窗口内申请、教师驳回/受理更正，全程留痕。
// 写操作顺序遵循“先原子认领、再更正答卷、失败回滚”，任何重复提交或越权处理都不能改变原成绩。
type ScoreReviewService struct {
	reviewRepo repository.ScoreReviewRepository
	recordRepo repository.ExamRecordRepository
	examSvc    *ExamService
	logger     *slog.Logger
}

// NewScoreReviewService 构造成绩复核服务。
func NewScoreReviewService(reviewRepo repository.ScoreReviewRepository, recordRepo repository.ExamRecordRepository, examSvc *ExamService, logger *slog.Logger) *ScoreReviewService {
	return &ScoreReviewService{reviewRepo: reviewRepo, recordRepo: recordRepo, examSvc: examSvc, logger: logger}
}

// Create 学生在批改完成后 48 小时内对本人答卷发起一次复核。
// 拒绝条件（直接拒绝，不落任何成绩变更）：非 graded、超过 48h、非本人、已存在申请。
func (s *ScoreReviewService) Create(ctx context.Context, recordID, studentID primitive.ObjectID, studentName, role, reason string) (*model.ScoreReview, error) {
	reason = trimSpace(reason)
	if reason == "" {
		return nil, util.NewAppError(constants.CodeValidationFailed, constants.MsgReviewReasonEmpty)
	}
	rec, err := s.recordRepo.FindByID(ctx, recordID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(constants.CodeRecordNotFound, fmt.Sprintf(constants.MsgRecordNotFound, recordID.Hex()))
		}
		return nil, fmt.Errorf("score review service create find record: %w", err)
	}
	// 越权：仅本人可对自己的答卷发起复核
	if rec.StudentID != studentID {
		s.logger.Warn(constants.LogReviewDenied, "record_id", recordID.Hex(), "student", studentName, "reason", "not owner")
		return nil, util.NewAppError(constants.CodeReviewForbidden, fmt.Sprintf(constants.MsgReviewForbidden, role, recordID.Hex()))
	}
	// 仅已批改完成的答卷可复核
	if rec.Status != constants.RecordStatusGraded || rec.GradedAt == nil {
		return nil, util.NewAppError(constants.CodeRecordStatusErr, fmt.Sprintf(constants.MsgReviewNotGraded, role, recordID.Hex()))
	}
	// 窗口期：批改完成后 48 小时内
	now := time.Now()
	if now.Sub(*rec.GradedAt) > constants.ReviewApplyWindow {
		s.logger.Warn(constants.LogReviewDenied, "record_id", recordID.Hex(), "student", studentName, "reason", "window closed")
		return nil, util.NewAppError(constants.CodeReviewWindowClosed,
			fmt.Sprintf(constants.MsgReviewWindowClosed, recordID.Hex(), util.FormatDateTime(*rec.GradedAt)))
	}
	// 同一答卷只允许一条复核申请（待处理或历史申请都拒绝；唯一索引兜底并发）
	if existing, err := s.reviewRepo.FindByRecordID(ctx, recordID); err == nil {
		return nil, util.NewAppError(constants.CodeReviewDuplicate,
			fmt.Sprintf(constants.MsgReviewDuplicate, recordID.Hex(), existing.Status))
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, fmt.Errorf("score review service create find existing: %w", err)
	}

	rv := &model.ScoreReview{
		ID:          primitive.NewObjectID(),
		RecordID:    rec.ID,
		ExamID:      rec.ExamID,
		ExamTitle:   rec.ExamTitle,
		StudentID:   rec.StudentID,
		StudentName: rec.StudentName,
		Reason:      reason,
		Status:      constants.ReviewStatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.reviewRepo.Create(ctx, rv); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			// 并发重复提交被 record_id 唯一索引拦截：原成绩不受影响
			return nil, util.NewAppError(constants.CodeReviewDuplicate,
				fmt.Sprintf(constants.MsgReviewDuplicate, recordID.Hex(), constants.ReviewStatusPending))
		}
		return nil, fmt.Errorf("score review service create: %w", err)
	}
	s.logger.Info(constants.LogReviewCreated, "record_id", recordID.Hex(), "review_id", rv.ID.Hex(),
		"status", rv.Status, "student", studentName, "reason_len", len(reason))
	return rv, nil
}

// Decide 教师处理复核：action=reject 驳回；action=approve 受理并更正总分/及格状态。
// 两种处理都必须填写意见；仅 pending 可处理（原子认领防止并发重复/越权改变成绩）。
func (s *ScoreReviewService) Decide(ctx context.Context, reviewID, operatorID primitive.ObjectID, operatorName, role string, req dto.ReviewDecisionRequest) (*model.ScoreReview, *model.ExamRecord, error) {
	comment := trimSpace(req.Comment)
	if comment == "" {
		return nil, nil, util.NewAppError(constants.CodeValidationFailed, constants.MsgReviewCommentEmpty)
	}
	if !constants.IsValidReviewAction(req.Action) {
		return nil, nil, util.NewAppError(constants.CodeBadRequest,
			fmt.Sprintf("成绩复核模块：处理动作字段 action=%s 非法（仅 approve/reject）", req.Action))
	}
	rv, err := s.reviewRepo.FindByID(ctx, reviewID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil, util.NewAppError(constants.CodeReviewNotFound, fmt.Sprintf(constants.MsgReviewNotFound, reviewID.Hex()))
		}
		return nil, nil, fmt.Errorf("score review service decide find: %w", err)
	}
	if rv.Status != constants.ReviewStatusPending {
		return nil, nil, util.NewAppError(constants.CodeReviewNotPending,
			fmt.Sprintf(constants.MsgReviewNotPending, reviewID.Hex(), rv.Status))
	}
	rec, err := s.recordRepo.FindByID(ctx, rv.RecordID)
	if err != nil {
		return nil, nil, fmt.Errorf("score review service decide find record: %w", err)
	}

	// 受理时提前校验更正分范围，避免认领后才失败（任何非法输入都不得改变原成绩）
	var correctedScore, originalScore float64
	var originalPassed, correctedPassed bool
	var override *bool
	totalScore := rec.PassScore // 兜底；以试卷总分为准
	if exam, examErr := s.examSvc.GetByID(ctx, rec.ExamID); examErr == nil {
		totalScore = exam.TotalScore
	}
	if req.Action == constants.ReviewActionApprove {
		correctedScore = req.CorrectedScore
		if correctedScore < 0 || (totalScore > 0 && correctedScore > totalScore) {
			return nil, nil, util.NewAppError(constants.CodeBadRequest,
				fmt.Sprintf(constants.MsgReviewScoreRange, correctedScore, totalScore))
		}
		originalScore = EffectiveScore(rec)
		originalPassed = EffectivePassed(rec)
		override = req.PassedOverride
		correctedPassed = originalPassed
		if override != nil {
			correctedPassed = *override
		} else if rec.PassScore > 0 {
			correctedPassed = correctedScore >= rec.PassScore
		}
	}

	now := time.Now()
	// 第一步：原子认领（仅 pending 可认领），并发的第二个处理请求在此被拒，成绩不会被二次修改
	claimed, err := s.reviewRepo.ClaimPending(ctx, reviewID, operatorID, operatorName, comment, req.Action, now)
	if err != nil {
		if errors.Is(err, repository.ErrConflict) {
			return nil, nil, util.NewAppError(constants.CodeReviewNotPending,
				fmt.Sprintf(constants.MsgReviewNotPending, reviewID.Hex(), constants.ReviewStatusPending))
		}
		return nil, nil, fmt.Errorf("score review service decide claim: %w", err)
	}

	// 驳回：定稿即可，答卷原成绩保持不变
	if req.Action == constants.ReviewActionReject {
		if err := s.reviewRepo.FinishRejected(ctx, reviewID); err != nil {
			return nil, nil, fmt.Errorf("score review service decide reject: %w", err)
		}
		claimed.Status = constants.ReviewStatusRejected
		s.logger.Info(constants.LogReviewRejected, "review_id", reviewID.Hex(), "record_id", rv.RecordID.Hex(),
			"status", claimed.Status, "operator", operatorName)
		s.logger.Info(constants.LogReviewProcessed, "review_id", reviewID.Hex(), "action", req.Action,
			"operator", operatorName, "comment_len", len(comment))
		return claimed, rec, nil
	}

	// 受理：更正答卷的最终总分/及格状态，同时保留原始批改明细，全程留痕
	rec.FinalScore = correctedScore
	rec.Adjustment = &model.ScoreAdjustment{
		ReviewID:        rv.ID,
		OperatorID:      operatorID,
		OperatorName:    operatorName,
		OriginalScore:   originalScore,
		CorrectedScore:  correctedScore,
		OriginalPassed:  originalPassed,
		CorrectedPassed: correctedPassed,
		PassedOverride:  override,
		Comment:         comment,
		CreatedAt:       now,
	}
	rec.UpdatedAt = now
	if err := s.recordRepo.Update(ctx, rec); err != nil {
		// 更正答卷失败：回滚认领，恢复为 pending，原成绩未被改变
		_ = s.reviewRepo.Reopen(ctx, reviewID)
		return nil, nil, fmt.Errorf("score review service approve update record: %w", err)
	}

	if err := s.reviewRepo.FinishApproved(ctx, reviewID, bson.M{
		"original_score":   originalScore,
		"corrected_score":  correctedScore,
		"original_passed":  originalPassed,
		"corrected_passed": correctedPassed,
		"passed_override":  override,
		"updated_at":       now,
	}); err != nil {
		// 复核定稿失败：回滚答卷更正，保持“复核状态与最终分数”一致
		rec.FinalScore = originalScore
		rec.Adjustment = nil
		_ = s.recordRepo.Update(ctx, rec)
		_ = s.reviewRepo.Reopen(ctx, reviewID)
		return nil, nil, fmt.Errorf("score review service approve finish: %w", err)
	}
	claimed.Status = constants.ReviewStatusApproved
	claimed.OriginalScore = originalScore
	claimed.CorrectedScore = correctedScore
	claimed.OriginalPassed = originalPassed
	claimed.CorrectedPassed = correctedPassed
	claimed.PassedOverride = override

	s.logger.Info(constants.LogReviewApproved, "review_id", reviewID.Hex(), "record_id", rv.RecordID.Hex(),
		"status", claimed.Status, "score_from", originalScore, "score_to", correctedScore,
		"passed", correctedPassed, "operator", operatorName)
	s.logger.Info(constants.LogReviewProcessed, "review_id", reviewID.Hex(), "action", req.Action,
		"operator", operatorName, "comment_len", len(comment))
	return claimed, rec, nil
}

// GetByID 查询单条复核（越权校验由 handler 基于角色与归属完成）。
func (s *ScoreReviewService) GetByID(ctx context.Context, id primitive.ObjectID) (*model.ScoreReview, error) {
	rv, err := s.reviewRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(constants.CodeReviewNotFound, fmt.Sprintf(constants.MsgReviewNotFound, id.Hex()))
		}
		return nil, fmt.Errorf("score review service get: %w", err)
	}
	return rv, nil
}

// GetByRecordID 查询某答卷的复核（学生成绩页/教师批改页复用）。
func (s *ScoreReviewService) GetByRecordID(ctx context.Context, recordID primitive.ObjectID) (*model.ScoreReview, error) {
	rv, err := s.reviewRepo.FindByRecordID(ctx, recordID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(constants.CodeReviewNotFound, fmt.Sprintf(constants.MsgReviewNotFound, recordID.Hex()))
		}
		return nil, fmt.Errorf("score review service get by record: %w", err)
	}
	return rv, nil
}

// ListByStudent 学生查询本人的复核申请。
func (s *ScoreReviewService) ListByStudent(ctx context.Context, studentID primitive.ObjectID, page, pageSize int64) ([]*model.ScoreReview, int64, error) {
	list, total, err := s.reviewRepo.List(ctx, bson.M{"student_id": studentID}, page, pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("score review service list by student: %w", err)
	}
	return list, total, nil
}

// ListPending 教师查询待处理复核（默认仅 pending，可传 status 查全部）。
func (s *ScoreReviewService) List(ctx context.Context, status string, page, pageSize int64) ([]*model.ScoreReview, int64, error) {
	filter := bson.M{}
	if status != "" {
		if !constants.IsValidReviewStatus(status) {
			return nil, 0, util.NewAppError(constants.CodeBadRequest,
				fmt.Sprintf("成绩复核模块：状态字段 status=%s 非法", status))
		}
		filter["status"] = status
	}
	list, total, err := s.reviewRepo.List(ctx, filter, page, pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("score review service list: %w", err)
	}
	return list, total, nil
}

// MapByRecordID 批量返回 record_id -> 复核（答卷列表拼装复核状态复用）。
func (s *ScoreReviewService) MapByRecordID(ctx context.Context, recordIDs []primitive.ObjectID) (map[string]*model.ScoreReview, error) {
	list, err := s.reviewRepo.FindByRecordIDs(ctx, recordIDs)
	if err != nil {
		return nil, fmt.Errorf("score review service map by record: %w", err)
	}
	out := make(map[string]*model.ScoreReview, len(list))
	for _, rv := range list {
		out[rv.RecordID.Hex()] = rv
	}
	return out, nil
}

// ReviewStats 成绩分析复核统计：待处理/已受理/已驳回数量、受理更正的答卷清单。
func (s *ScoreReviewService) ReviewStats(ctx context.Context, examID primitive.ObjectID) (dto.ReviewStats, error) {
	all, err := s.reviewRepo.ListAll(ctx, bson.M{"exam_id": examID})
	if err != nil {
		return dto.ReviewStats{}, fmt.Errorf("score review service stats: %w", err)
	}
	stats := dto.ReviewStats{AdjustedRecords: []dto.AdjustedRecord{}}
	recordIDs := make([]primitive.ObjectID, 0, len(all))
	for _, rv := range all {
		switch rv.Status {
		case constants.ReviewStatusPending:
			stats.PendingCount++
		case constants.ReviewStatusApproved:
			stats.ApprovedCount++
			recordIDs = append(recordIDs, rv.RecordID)
		case constants.ReviewStatusRejected:
			stats.RejectedCount++
		}
	}
	stats.TotalCount = len(all)
	if len(recordIDs) > 0 {
		for _, rv := range all {
			if rv.Status != constants.ReviewStatusApproved {
				continue
			}
			stats.AdjustedRecords = append(stats.AdjustedRecords, dto.AdjustedRecord{
				RecordID:        rv.RecordID.Hex(),
				StudentName:     rv.StudentName,
				OriginalScore:   rv.OriginalScore,
				CorrectedScore:  rv.CorrectedScore,
				CorrectedPassed: rv.CorrectedPassed,
				Comment:         rv.Comment,
			})
		}
	}
	return stats, nil
}

func trimSpace(s string) string {
	return strings.TrimSpace(s)
}
