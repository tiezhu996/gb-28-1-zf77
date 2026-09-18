package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
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

// ExamRecordService 考试记录服务：开始考试、提交/自动提交、阅卷、成绩分析、成绩复核联动。
type ExamRecordService struct {
	repo   repository.ExamRecordRepository
	exam   *ExamService // 复用试卷服务（校验考试窗口、及格线快照）
	logger *slog.Logger
}

// NewExamRecordService 构造考试记录服务。
func NewExamRecordService(repo repository.ExamRecordRepository, exam *ExamService, logger *slog.Logger) *ExamRecordService {
	return &ExamRecordService{repo: repo, exam: exam, logger: logger}
}

// passScoreOf 及格线：优先答卷快照，回退试卷当前配置。
func passScoreOf(rec *model.ExamRecord, exam *model.Exam) float64 {
	if rec.PassScore > 0 {
		return rec.PassScore
	}
	return exam.PassScore
}

// gradingCompleted 判断答卷是否已批改完成（复核申请的前置条件）。
func gradingCompleted(rec *model.ExamRecord) bool {
	return rec.Status == constants.RecordStatusGraded
}

// reviewWindowBase 复核窗口起点：优先 GradedAt，历史数据回退 SubmittedAt。
func reviewWindowBase(rec *model.ExamRecord) *time.Time {
	if rec.GradedAt != nil {
		return rec.GradedAt
	}
	return rec.SubmittedAt
}

// StartExam 学生开始考试：校验时间窗口、生成随机题序/选项快照。
func (s *ExamRecordService) StartExam(ctx context.Context, examID, studentID primitive.ObjectID, studentName string) (*model.ExamRecord, error) {
	exam, err := s.exam.GetByID(ctx, examID)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if exam.Status != constants.ExamStatusPublished && exam.Status != constants.ExamStatusOngoing {
		return nil, util.NewAppError(constants.CodeExamStatusErr, fmt.Sprintf(constants.MsgExamStatusInvalid, exam.Status, exam.Status, "start"))
	}
	if now.Before(exam.StartAt) || now.After(exam.EndAt) {
		return nil, util.NewAppError(constants.CodeExamNotInWindow, constants.MsgExamNotInWindow)
	}
	if len(exam.Questions) == 0 {
		return nil, util.NewAppError(constants.CodeExamNoQuestions, constants.MsgExamNoQuestions)
	}
	// 已有进行中的记录则直接返回（幂等）
	if existing, err := s.repo.FindActiveByExamAndStudent(ctx, examID, studentID); err == nil {
		return existing, nil
	}

	questions := make([]model.AttemptQuestion, 0, len(exam.Questions))
	// 随机题序：拷贝题目列表后打乱
	ordered := make([]model.ExamQuestion, len(exam.Questions))
	copy(ordered, exam.Questions)
	if exam.ShuffleQuestion {
		util.Shuffle(ordered)
	}
	for _, eq := range ordered {
		q, err := s.exam.question.GetByID(ctx, eq.QuestionID)
		if err != nil {
			return nil, fmt.Errorf("exam record service start load question: %w", err)
		}
		opts := make([]model.AttemptOption, 0, len(q.Options))
		for _, o := range q.Options {
			opts = append(opts, model.AttemptOption{Key: o.Key, Text: o.Text})
		}
		if exam.ShuffleOption {
			util.Shuffle(opts)
		}
		questions = append(questions, model.AttemptQuestion{
			QuestionID:      q.ID,
			Type:            q.Type,
			Subject:         q.Subject,
			KnowledgePoints: q.KnowledgePoints,
			Content:         q.Content,
			Options:         opts,
			Score:           eq.Score,
			CorrectAnswer:   q.Answer,
			Result:          constants.AnswerResultUnmarked,
		})
	}

	rec := &model.ExamRecord{
		ID:          primitive.NewObjectID(),
		ExamID:      exam.ID,
		ExamTitle:   exam.Title,
		StudentID:   studentID,
		StudentName: studentName,
		Questions:   questions,
		Status:      constants.RecordStatusInProgress,
		PassScore:   exam.PassScore, // 及格线快照：复核更正及格状态以此为准
		StartedAt:   now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.repo.Create(ctx, rec); err != nil {
		return nil, fmt.Errorf("exam record service start: %w", err)
	}
	s.logger.Info(constants.LogRecordStarted, "record_id", rec.ID.Hex(), "exam_id", exam.ID.Hex(), "student", studentName)
	return rec, nil
}

// gradeQuestion 客观题自动判分（Submit 与 AutoSubmit 复用）。
func gradeQuestion(q *model.AttemptQuestion, userAnswer string) {
	answer := strings.TrimSpace(strings.ToUpper(userAnswer))
	correct := strings.TrimSpace(strings.ToUpper(q.CorrectAnswer))
	switch q.Type {
	case constants.QuestionTypeSingle, constants.QuestionTypeJudge:
		if answer != "" && answer == correct {
			q.Result = constants.AnswerResultCorrect
			q.GotScore = q.Score
		} else {
			q.Result = constants.AnswerResultWrong
			q.GotScore = 0
		}
	case constants.QuestionTypeMultiple:
		if answer == "" {
			q.Result = constants.AnswerResultWrong
			q.GotScore = 0
			return
		}
		userKeys := splitSorted(answer)
		correctKeys := splitSorted(correct)
		if equalStrings(userKeys, correctKeys) {
			q.Result = constants.AnswerResultCorrect
			q.GotScore = q.Score
		} else {
			q.Result = constants.AnswerResultWrong
			q.GotScore = 0
		}
	default: // fill / short 主观题由教师批改
		q.Result = constants.AnswerResultUnmarked
		q.GotScore = 0
	}
}

// gradeObjective 对整份答卷客观题自动评分，返回客观题总分（Submit/AutoSubmit 复用）。
func gradeObjective(questions []model.AttemptQuestion) float64 {
	var total float64
	for i := range questions {
		gradeQuestion(&questions[i], questions[i].UserAnswer)
		if questions[i].Result == constants.AnswerResultCorrect || questions[i].Result == constants.AnswerResultPartial {
			total += questions[i].GotScore
		}
	}
	return total
}

// Submit 提交答卷（自动提交共用 finish 逻辑）。
func (s *ExamRecordService) Submit(ctx context.Context, recordID primitive.ObjectID, answers []dto.AnswerInput, cheatCount int, cheatEvents []dto.CheatEventInput, auto bool) (*model.ExamRecord, error) {
	rec, err := s.repo.FindByID(ctx, recordID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(constants.CodeRecordNotFound, fmt.Sprintf(constants.MsgRecordNotFound, recordID.Hex()))
		}
		return nil, fmt.Errorf("exam record service submit find: %w", err)
	}
	if rec.Status != constants.RecordStatusInProgress {
		return nil, util.NewAppError(constants.CodeRecordAlreadyDone, fmt.Sprintf(constants.MsgRecordStatusInvalid, rec.Status))
	}
	answerMap := make(map[string]string, len(answers))
	for _, a := range answers {
		answerMap[a.QuestionID] = a.Answer
	}
	for i := range rec.Questions {
		if ans, ok := answerMap[rec.Questions[i].QuestionID.Hex()]; ok {
			rec.Questions[i].UserAnswer = ans
		}
	}
	rec.ObjectiveScore = gradeObjective(rec.Questions)
	rec.CheatCount = cheatCount
	for _, ce := range cheatEvents {
		rec.CheatEvents = append(rec.CheatEvents, model.CheatEvent{
			Type:       ce.Type,
			Detail:     ce.Detail,
			OccurredAt: time.Now(),
		})
	}
	now := time.Now()
	rec.SubmittedAt = &now
	rec.Status = constants.RecordStatusSubmitted
	rec.AutoSubmitted = auto
	rec.UpdatedAt = now
	if err := s.repo.Update(ctx, rec); err != nil {
		return nil, fmt.Errorf("exam record service submit: %w", err)
	}
	if auto {
		s.logger.Info(constants.LogRecordAutoSubmit, "record_id", rec.ID.Hex(), "exam_id", rec.ExamID.Hex(), "student", rec.StudentName)
	} else {
		s.logger.Info(constants.LogRecordSubmitted, "record_id", rec.ID.Hex(), "status", rec.Status, "objective_score", rec.ObjectiveScore, "student", rec.StudentName)
	}
	return rec, nil
}

// Grade 教师批改主观题（填空题/简答题）。
func (s *ExamRecordService) Grade(ctx context.Context, recordID primitive.ObjectID, grades []dto.GradeItem, teacher string) (*model.ExamRecord, error) {
	rec, err := s.repo.FindByID(ctx, recordID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(constants.CodeRecordNotFound, fmt.Sprintf(constants.MsgRecordNotFound, recordID.Hex()))
		}
		return nil, fmt.Errorf("exam record service grade find: %w", err)
	}
	if rec.Status != constants.RecordStatusSubmitted && rec.Status != constants.RecordStatusGraded {
		return nil, util.NewAppError(constants.CodeRecordStatusErr, fmt.Sprintf(constants.MsgRecordStatusInvalid, rec.Status))
	}
	// 复核闭环：答卷一旦存在复核申请（待处理或已处理），普通批改通道锁定，
	// 分数只能通过复核受理动作更正，任何重复批改/越权操作都不能改变（复核后的）原成绩。
	if rec.ReviewStatus != "" || rec.ReviewID != primitive.NilObjectID {
		return nil, util.NewAppError(constants.CodeReviewRecordLocked, fmt.Sprintf(constants.MsgReviewRecordLocked, recordID.Hex()))
	}
	gradeMap := make(map[string]dto.GradeItem, len(grades))
	for _, g := range grades {
		gradeMap[g.QuestionID] = g
	}
	var subjectiveTotal float64
	for i := range rec.Questions {
		q := &rec.Questions[i]
		if constants.IsObjectiveQuestion(q.Type) {
			continue
		}
		if g, ok := gradeMap[q.QuestionID.Hex()]; ok {
			q.SubjectiveScore = g.Score
			q.Comment = g.Comment
			q.Marked = true
			q.GotScore = g.Score
			q.Result = constants.AnswerResultPartial
			subjectiveTotal += g.Score
		}
	}
	rec.SubjectiveScore = subjectiveTotal
	rec.FinalScore = rec.ObjectiveScore + subjectiveTotal
	rec.Status = constants.RecordStatusGraded
	now := time.Now()
	if rec.GradedAt == nil {
		rec.GradedAt = &now // 批改完成时间：复核 48h 窗口起点
	}
	rec.UpdatedAt = now
	if err := s.repo.Update(ctx, rec); err != nil {
		return nil, fmt.Errorf("exam record service grade: %w", err)
	}
	s.logger.Info(constants.LogRecordGraded, "record_id", rec.ID.Hex(), "result", rec.Status, "final_score", rec.FinalScore, "teacher", teacher)
	return rec, nil
}

// ListByStudent 学生查询自己的考试记录。
func (s *ExamRecordService) ListByStudent(ctx context.Context, studentID primitive.ObjectID, filter bson.M, page, pageSize int64) ([]*model.ExamRecord, int64, error) {
	filter["student_id"] = studentID
	list, total, err := s.repo.List(ctx, filter, page, pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("exam record service list by student: %w", err)
	}
	return list, total, nil
}

// ListByExam 教师按试卷查询考试记录。
func (s *ExamRecordService) ListByExam(ctx context.Context, examID primitive.ObjectID, filter bson.M, page, pageSize int64) ([]*model.ExamRecord, int64, error) {
	filter["exam_id"] = examID
	list, total, err := s.repo.List(ctx, filter, page, pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("exam record service list by exam: %w", err)
	}
	return list, total, nil
}

// GetByID 查询单个考试记录。
func (s *ExamRecordService) GetByID(ctx context.Context, id primitive.ObjectID) (*model.ExamRecord, error) {
	rec, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(constants.CodeRecordNotFound, fmt.Sprintf(constants.MsgRecordNotFound, id.Hex()))
		}
		return nil, fmt.Errorf("exam record service get: %w", err)
	}
	return rec, nil
}

// GetForViewer 查询答卷并做越权校验：学生只能查看本人答卷，教师/管理员可查看全部。
// 防止学生通过篡改 recordId 查看他人答卷或对他人成绩发起复核。
func (s *ExamRecordService) GetForViewer(ctx context.Context, id, viewerID primitive.ObjectID, role string) (*model.ExamRecord, error) {
	rec, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if role == constants.RoleStudent && rec.StudentID != viewerID {
		s.logger.Warn("越权访问答卷被拒绝", "record_id", id.Hex(), "viewer", viewerID.Hex(), "role", role)
		return nil, util.NewAppError(constants.CodeForbidden, constants.MsgForbidden)
	}
	return rec, nil
}

// Report 生成成绩分析报告（平均分/最高/最低/及格率/分数段/每题正确率）。
func (s *ExamRecordService) Report(ctx context.Context, examID primitive.ObjectID) (*dto.ExamReport, error) {
	exam, err := s.exam.GetByID(ctx, examID)
	if err != nil {
		return nil, err
	}
	recs, err := s.repo.ListAll(ctx, bson.M{"exam_id": examID, "status": bson.M{"$in": []string{constants.RecordStatusSubmitted, constants.RecordStatusGraded}}})
	if err != nil {
		return nil, fmt.Errorf("exam record service report: %w", err)
	}
	report := &dto.ExamReport{
		ExamID:          examID.Hex(),
		ExamTitle:       exam.Title,
		ScoreBands:      map[string]int{"0-59": 0, "60-69": 0, "70-79": 0, "80-89": 0, "90-100": 0},
		QuestionReports: make([]dto.ExamReportItem, 0, len(exam.Questions)),
	}
	if len(recs) == 0 {
		report.TotalStudents = 0
		return report, nil
	}
	var sum float64
	var maxScore float64
	minScore := -1.0
	passCount := 0
	for _, r := range recs {
		// 复核受理更正后 FinalScore 已是最终分，统计天然以复核后的分数为准
		score := r.FinalScore
		if score <= 0 && r.Status == constants.RecordStatusSubmitted {
			score = r.ObjectiveScore
		}
		if score > 0 {
			sum += score
		}
		if score > maxScore {
			maxScore = score
		}
		if minScore < 0 || score < minScore {
			minScore = score
		}
		// 及格状态只统计批改完成的答卷；复核更正后以更正后的最终分与快照及格线判定
		if r.Status == constants.RecordStatusGraded && passScoreOf(r, exam) > 0 && score >= passScoreOf(r, exam) {
			passCount++
		}
		band := scoreBand(score)
		report.ScoreBands[band]++

		// 成绩复核闭环统计
		switch r.ReviewStatus {
		case constants.ReviewStatusPending:
			report.ReviewPendingCount++
		case constants.ReviewStatusApproved:
			report.ReviewApprovedCount++
			if r.ScoreCorrected {
				report.ReviewCorrectedCount++
			}
		case constants.ReviewStatusRejected:
			report.ReviewRejectedCount++
		}
	}
	report.TotalStudents = len(recs)
	report.AverageScore = round2(sum / float64(len(recs)))
	report.MaxScore = round2(maxScore)
	report.MinScore = round2(minScore)
	report.PassRate = round2(float64(passCount) / float64(len(recs)) * 100)

	// 每题正确率：聚合所有已提交/已批改记录
	questionStats := make(map[string]*dto.ExamReportItem)
	for _, r := range recs {
		for _, q := range r.Questions {
			item, ok := questionStats[q.QuestionID.Hex()]
			if !ok {
				item = &dto.ExamReportItem{
					QuestionID: q.QuestionID.Hex(),
					Content:    q.Content,
					Type:       q.Type,
				}
				questionStats[q.QuestionID.Hex()] = item
			}
			item.AnswerCount++
			if q.Result == constants.AnswerResultCorrect || (q.Result == constants.AnswerResultPartial && q.GotScore > 0) {
				item.CorrectCount++
			}
		}
	}
	for _, q := range exam.Questions {
		if item, ok := questionStats[q.QuestionID.Hex()]; ok {
			if item.AnswerCount > 0 {
				item.Accuracy = round2(float64(item.CorrectCount) / float64(item.AnswerCount) * 100)
			}
			report.QuestionReports = append(report.QuestionReports, *item)
		}
	}
	return report, nil
}

// AutoSubmitExpired 定时清理：自动提交所有已超时的进行中记录（前端定时器也会自动提交）。
func (s *ExamRecordService) AutoSubmitExpired(ctx context.Context, now time.Time) (int, error) {
	recs, err := s.repo.ListAll(ctx, bson.M{"status": constants.RecordStatusInProgress})
	if err != nil {
		return 0, fmt.Errorf("exam record service auto submit expired: %w", err)
	}
	count := 0
	for _, r := range recs {
		exam, err := s.exam.GetByID(ctx, r.ExamID)
		if err != nil {
			continue
		}
		deadline := r.StartedAt.Add(time.Duration(exam.DurationMin) * time.Minute)
		if now.After(deadline) {
			if _, err := s.Submit(ctx, r.ID, nil, r.CheatCount, nil, true); err == nil {
				count++
			}
		}
	}
	return count, nil
}

func splitSorted(s string) []string {
	parts := strings.Split(s, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(strings.ToUpper(parts[i]))
	}
	sort.Strings(parts)
	return parts
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func scoreBand(score float64) string {
	switch {
	case score < 60:
		return "0-59"
	case score < 70:
		return "60-69"
	case score < 80:
		return "70-79"
	case score < 90:
		return "80-89"
	default:
		return "90-100"
	}
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
