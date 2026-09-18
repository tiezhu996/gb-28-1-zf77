package service

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/onlineexam/onlineexam/internal/constants"
	"github.com/onlineexam/onlineexam/internal/dto"
	"github.com/onlineexam/onlineexam/internal/model"
	"github.com/onlineexam/onlineexam/internal/repository"
)

// fakeReviewRepo 内存版成绩复核仓储（与 Mongo 实现同一并发语义：仅 pending 可认领）。
type fakeReviewRepo struct {
	reviews map[string]*model.ScoreReview
}

func newFakeReviewRepo() *fakeReviewRepo {
	return &fakeReviewRepo{reviews: make(map[string]*model.ScoreReview)}
}

func (f *fakeReviewRepo) Create(_ context.Context, rv *model.ScoreReview) error {
	for _, ex := range f.reviews {
		if ex.RecordID == rv.RecordID {
			return repository.ErrConflict
		}
	}
	f.reviews[rv.ID.Hex()] = rv
	return nil
}
func (f *fakeReviewRepo) FindByID(_ context.Context, id primitive.ObjectID) (*model.ScoreReview, error) {
	if rv, ok := f.reviews[id.Hex()]; ok {
		return rv, nil
	}
	return nil, repository.ErrNotFound
}
func (f *fakeReviewRepo) FindByRecordID(_ context.Context, recordID primitive.ObjectID) (*model.ScoreReview, error) {
	for _, rv := range f.reviews {
		if rv.RecordID == recordID {
			return rv, nil
		}
	}
	return nil, repository.ErrNotFound
}
func (f *fakeReviewRepo) FindByRecordIDs(_ context.Context, ids []primitive.ObjectID) ([]*model.ScoreReview, error) {
	want := map[string]bool{}
	for _, id := range ids {
		want[id.Hex()] = true
	}
	var out []*model.ScoreReview
	for _, rv := range f.reviews {
		if want[rv.RecordID.Hex()] {
			out = append(out, rv)
		}
	}
	return out, nil
}
func (f *fakeReviewRepo) List(_ context.Context, filter bson.M, _, _ int64) ([]*model.ScoreReview, int64, error) {
	var out []*model.ScoreReview
	for _, rv := range f.reviews {
		if st, ok := filter["status"].(string); ok && rv.Status != st {
			continue
		}
		out = append(out, rv)
	}
	return out, int64(len(out)), nil
}
func (f *fakeReviewRepo) ListAll(_ context.Context, filter bson.M) ([]*model.ScoreReview, error) {
	var out []*model.ScoreReview
	for _, rv := range f.reviews {
		if eid, ok := filter["exam_id"].(primitive.ObjectID); ok && rv.ExamID != eid {
			continue
		}
		out = append(out, rv)
	}
	return out, nil
}
func (f *fakeReviewRepo) ClaimPending(_ context.Context, id, handlerID primitive.ObjectID, handlerName, comment, decision string, now time.Time) (*model.ScoreReview, error) {
	rv, ok := f.reviews[id.Hex()]
	if !ok {
		return nil, repository.ErrNotFound
	}
	if rv.Status != constants.ReviewStatusPending {
		return nil, repository.ErrConflict
	}
	rv.Status = constants.ReviewStatusPending // 认领不改终态，等 Finish 定稿
	rv.HandledBy = handlerID
	rv.HandlerName = handlerName
	rv.Comment = comment
	rv.Decision = decision
	rv.HandledAt = &now
	return rv, nil
}
func (f *fakeReviewRepo) FinishApproved(_ context.Context, id primitive.ObjectID, upd bson.M) error {
	rv, ok := f.reviews[id.Hex()]
	if !ok {
		return repository.ErrNotFound
	}
	rv.Status = constants.ReviewStatusApproved
	if v, ok := upd["original_score"].(float64); ok {
		rv.OriginalScore = v
	}
	if v, ok := upd["corrected_score"].(float64); ok {
		rv.CorrectedScore = v
	}
	if v, ok := upd["original_passed"].(bool); ok {
		rv.OriginalPassed = v
	}
	if v, ok := upd["corrected_passed"].(bool); ok {
		rv.CorrectedPassed = v
	}
	return nil
}
func (f *fakeReviewRepo) FinishRejected(_ context.Context, id primitive.ObjectID) error {
	rv, ok := f.reviews[id.Hex()]
	if !ok {
		return repository.ErrNotFound
	}
	rv.Status = constants.ReviewStatusRejected
	return nil
}
func (f *fakeReviewRepo) Reopen(_ context.Context, id primitive.ObjectID) error {
	rv, ok := f.reviews[id.Hex()]
	if !ok {
		return repository.ErrNotFound
	}
	rv.Status = constants.ReviewStatusPending
	rv.HandledBy = primitive.NilObjectID
	rv.HandlerName = ""
	rv.Comment = ""
	rv.Decision = ""
	rv.HandledAt = nil
	return nil
}

func newTestReviewSvc() (*ScoreReviewService, *ExamRecordService, *fakeReviewRepo, *fakeRecordRepo, primitive.ObjectID, primitive.ObjectID, *model.ExamRecord) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	questionRepo := newFakeQuestionRepo()
	questionSvc := NewQuestionService(questionRepo, logger)
	examRepo := newFakeExamRepo()
	examSvc := NewExamService(examRepo, questionSvc, logger)
	recordRepo := newFakeRecordRepo()
	recordSvc := NewExamRecordService(recordRepo, examSvc, logger)
	reviewRepo := newFakeReviewRepo()
	reviewSvc := NewScoreReviewService(reviewRepo, recordRepo, examSvc, logger)

	teacher := primitive.NewObjectID()
	student := primitive.NewObjectID()
	q, _ := questionSvc.Create(context.Background(), &dto.CreateQuestionRequest{
		Type: "single", Subject: "数学", KnowledgePoints: []string{"代数"},
		Difficulty: "easy", Content: "1+1=?", Answer: "B", Score: 100,
		Options: []dto.OptionInput{{Key: "A", Text: "1"}, {Key: "B", Text: "2"}},
	}, teacher)
	now := time.Now()
	exam, _ := examSvc.Create(context.Background(), &dto.CreateExamRequest{
		Title: "复核测试卷", Subject: "数学", DurationMin: 30, PassScore: 60,
		StartAt: now.Add(-time.Hour), EndAt: now.Add(time.Hour),
		Questions: []dto.ExamQuestionInput{{QuestionID: q.ID.Hex()}},
	}, teacher)
	_, _ = examSvc.Publish(context.Background(), exam.ID, "t@example.com")
	rec, _ := recordSvc.StartExam(context.Background(), exam.ID, student, "李同学")
	// 答错 → 0 分不及格
	_, _ = recordSvc.Submit(context.Background(), rec.ID, []dto.AnswerInput{{QuestionID: q.ID.Hex(), Answer: "A"}}, 0, nil, false)
	graded, _ := recordSvc.Grade(context.Background(), rec.ID, nil, "t@example.com")
	return reviewSvc, recordSvc, reviewRepo, recordRepo, teacher, student, graded
}

// TestReviewLifecycle 覆盖申请→受理更正→统计→终态拒绝的完整闭环。
func TestReviewLifecycle(t *testing.T) {
	reviewSvc, recordSvc, _, recRepo, teacher, student, rec := newTestReviewSvc()

	// 学生窗口内申请
	rv, err := reviewSvc.Create(context.Background(), rec.ID, student, "李同学", constants.RoleStudent, "客观题判分有误")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if rv.Status != constants.ReviewStatusPending {
		t.Fatalf("status = %s, want pending", rv.Status)
	}

	// 同一答卷重复申请必须被拒，且不改变原成绩
	if _, err := reviewSvc.Create(context.Background(), rec.ID, student, "李同学", constants.RoleStudent, "再次申请"); err == nil {
		t.Fatal("重复复核申请应被拒绝")
	}
	if got := recRepo.records[rec.ID.Hex()].FinalScore; got != 0 {
		t.Fatalf("重复申请后原成绩被改变 = %f", got)
	}

	// 受理并更正为 80 分
	approved, updated, err := reviewSvc.Decide(context.Background(), rv.ID, teacher, "王老师", constants.RoleTeacher,
		dto.ReviewDecisionRequest{Action: constants.ReviewActionApprove, Comment: "复核发现标准答案配置错误，更正为80分", CorrectedScore: 80})
	if err != nil {
		t.Fatalf("Decide(approve) error = %v", err)
	}
	if approved.Status != constants.ReviewStatusApproved {
		t.Fatalf("status = %s, want approved", approved.Status)
	}
	if updated.FinalScore != 80 {
		t.Fatalf("final score = %f, want 80", updated.FinalScore)
	}
	if !EffectivePassed(updated) {
		t.Fatal("更正 80 分（及格线 60）后应为及格")
	}
	if updated.Adjustment == nil || updated.Adjustment.OriginalScore != 0 || updated.Adjustment.CorrectedScore != 80 {
		t.Fatal("复核更正留痕不完整")
	}

	// 已终态的申请不可再次处理（重复/越权处理不能改变成绩）
	if _, _, err := reviewSvc.Decide(context.Background(), rv.ID, teacher, "王老师", constants.RoleTeacher,
		dto.ReviewDecisionRequest{Action: constants.ReviewActionApprove, Comment: "二次处理", CorrectedScore: 90}); err == nil {
		t.Fatal("已处理申请应禁止再次处理")
	}
	if got := recRepo.records[rec.ID.Hex()].FinalScore; got != 80 {
		t.Fatalf("二次处理改变了成绩 = %f, want 80", got)
	}

	// 报告统计采用更正后的最终分数
	report, err := recordSvc.Report(context.Background(), rec.ExamID)
	if err != nil {
		t.Fatalf("Report() error = %v", err)
	}
	if report.AverageScore != 80 {
		t.Fatalf("average = %f, want 80（采用复核更正后的分数）", report.AverageScore)
	}
	if report.PassRate != 100 {
		t.Fatalf("pass rate = %f, want 100", report.PassRate)
	}

	stats, err := reviewSvc.ReviewStats(context.Background(), rec.ExamID)
	if err != nil {
		t.Fatalf("ReviewStats() error = %v", err)
	}
	if stats.ApprovedCount != 1 || stats.PendingCount != 0 || stats.RejectedCount != 0 {
		t.Fatalf("stats = %+v", stats)
	}
}

// TestReviewRejectKeepsScore 驳回不改变原成绩，但必须留痕意见。
func TestReviewRejectKeepsScore(t *testing.T) {
	reviewSvc, _, _, recRepo, teacher, student, rec := newTestReviewSvc()
	rv, _ := reviewSvc.Create(context.Background(), rec.ID, student, "李同学", constants.RoleStudent, "觉得给分低")

	// 不填意见必须拒绝
	if _, _, err := reviewSvc.Decide(context.Background(), rv.ID, teacher, "王老师", constants.RoleTeacher,
		dto.ReviewDecisionRequest{Action: constants.ReviewActionReject, Comment: "  "}); err == nil {
		t.Fatal("驳回/受理不填意见应被拒绝")
	}

	rejected, updated, err := reviewSvc.Decide(context.Background(), rv.ID, teacher, "王老师", constants.RoleTeacher,
		dto.ReviewDecisionRequest{Action: constants.ReviewActionReject, Comment: "评分标准无误，维持原判"})
	if err != nil {
		t.Fatalf("Decide(reject) error = %v", err)
	}
	if rejected.Status != constants.ReviewStatusRejected {
		t.Fatalf("status = %s, want rejected", rejected.Status)
	}
	if rejected.Comment != "评分标准无误，维持原判" || rejected.HandlerName != "王老师" || rejected.HandledAt == nil {
		t.Fatal("驳回留痕不完整")
	}
	if updated.FinalScore != 0 || updated.Adjustment != nil {
		t.Fatal("驳回不应改变原成绩或写入更正记录")
	}
	if recRepo.records[rec.ID.Hex()].FinalScore != 0 {
		t.Fatal("驳回后成绩被改变")
	}
}

// TestReviewRejectCases 逾期、非本人、非 graded 与空理由都必须直接拒绝。
func TestReviewRejectCases(t *testing.T) {
	reviewSvc, _, _, _, _, student, rec := newTestReviewSvc()

	// 非本人申请 → 越权拒绝
	other := primitive.NewObjectID()
	if _, err := reviewSvc.Create(context.Background(), rec.ID, other, "张同学", constants.RoleStudent, "不是我的卷子"); err == nil {
		t.Fatal("非本人答卷发起复核应被拒绝")
	}

	// 空理由
	if _, err := reviewSvc.Create(context.Background(), rec.ID, student, "李同学", constants.RoleStudent, " "); err == nil {
		t.Fatal("空理由应被拒绝")
	}

	// 逾期：把批改时间改到 49 小时前
	past := time.Now().Add(-49 * time.Hour)
	rec.GradedAt = &past
	if _, err := reviewSvc.Create(context.Background(), rec.ID, student, "李同学", constants.RoleStudent, "超过48小时"); err == nil {
		t.Fatal("超过 48 小时窗口应拒绝申请")
	}
}

// TestReviewApproveValidation 受理更正分超出试卷总分时拒绝，且不改变原成绩。
func TestReviewApproveValidation(t *testing.T) {
	reviewSvc, _, _, recRepo, teacher, student, rec := newTestReviewSvc()
	rv, _ := reviewSvc.Create(context.Background(), rec.ID, student, "李同学", constants.RoleStudent, "分数不对")

	if _, _, err := reviewSvc.Decide(context.Background(), rv.ID, teacher, "王老师", constants.RoleTeacher,
		dto.ReviewDecisionRequest{Action: constants.ReviewActionApprove, Comment: "更正", CorrectedScore: 150}); err == nil {
		t.Fatal("更正分超过试卷总分应被拒绝")
	}
	if recRepo.records[rec.ID.Hex()].Adjustment != nil {
		t.Fatal("非法更正不应写入答卷")
	}
	// 申请应仍为待处理，可继续合法处理
	if got, _ := reviewSvc.GetByID(context.Background(), rv.ID); got.Status != constants.ReviewStatusPending {
		t.Fatalf("非法更正后申请状态 = %s, want pending", got.Status)
	}
}

// TestReviewRegradeLocked 复核受理更正后，重新批改不得覆盖最终成绩。
func TestReviewRegradeLocked(t *testing.T) {
	reviewSvc, recordSvc, _, recRepo, teacher, student, rec := newTestReviewSvc()
	rv, _ := reviewSvc.Create(context.Background(), rec.ID, student, "李同学", constants.RoleStudent, "分数不对")
	_, updated, err := reviewSvc.Decide(context.Background(), rv.ID, teacher, "王老师", constants.RoleTeacher,
		dto.ReviewDecisionRequest{Action: constants.ReviewActionApprove, Comment: "更正为70", CorrectedScore: 70})
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	if updated.FinalScore != 70 {
		t.Fatalf("final = %f, want 70", updated.FinalScore)
	}
	if _, err := recordSvc.Grade(context.Background(), rec.ID, nil, "t@example.com"); err == nil {
		t.Fatal("复核受理后再次批改应被拒绝")
	}
	if got := recRepo.records[rec.ID.Hex()].FinalScore; got != 70 {
		t.Fatalf("被拒绝的批改改变了成绩 = %f, want 70", got)
	}
}

// TestReviewPassedOverride 教师可显式更正及格状态。
func TestReviewPassedOverride(t *testing.T) {
	reviewSvc, _, _, _, teacher, student, rec := newTestReviewSvc()
	rv, _ := reviewSvc.Create(context.Background(), rec.ID, student, "李同学", constants.RoleStudent, "分数不对")
	no := false
	approved, updated, err := reviewSvc.Decide(context.Background(), rv.ID, teacher, "王老师", constants.RoleTeacher,
		dto.ReviewDecisionRequest{Action: constants.ReviewActionApprove, Comment: "总分更正但仍判定不及格", CorrectedScore: 90, PassedOverride: &no})
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	if approved.CorrectedPassed != false {
		t.Fatal("及格状态覆盖未生效")
	}
	if EffectivePassed(updated) {
		t.Fatal("90 分但教师显式判定不及格时应不及格")
	}
}
