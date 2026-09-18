package service

import (
	"context"
	"errors"
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
	"github.com/onlineexam/onlineexam/internal/util"
)

// fakeReviewRepo 内存版成绩复核仓储。
type fakeReviewRepo struct {
	reviews map[string]*model.ScoreReview
}

func newFakeReviewRepo() *fakeReviewRepo {
	return &fakeReviewRepo{reviews: make(map[string]*model.ScoreReview)}
}

func (f *fakeReviewRepo) Create(_ context.Context, r *model.ScoreReview) error {
	if _, exists := f.reviews[r.RecordID.Hex()+""]; exists {
		// 简化：record_id 唯一约束由 FindByRecord 体现
	}
	f.reviews[r.ID.Hex()] = r
	return nil
}
func (f *fakeReviewRepo) FindByID(_ context.Context, id primitive.ObjectID) (*model.ScoreReview, error) {
	if r, ok := f.reviews[id.Hex()]; ok {
		return r, nil
	}
	return nil, repository.ErrNotFound
}
func (f *fakeReviewRepo) FindByRecord(_ context.Context, recordID primitive.ObjectID) (*model.ScoreReview, error) {
	for _, r := range f.reviews {
		if r.RecordID == recordID {
			return r, nil
		}
	}
	return nil, repository.ErrNotFound
}
func (f *fakeReviewRepo) List(_ context.Context, filter bson.M, _, _ int64) ([]*model.ScoreReview, int64, error) {
	var out []*model.ScoreReview
	for _, r := range f.reviews {
		if st, ok := filter["status"].(string); ok && r.Status != st {
			continue
		}
		out = append(out, r)
	}
	return out, int64(len(out)), nil
}
func (f *fakeReviewRepo) ListAll(_ context.Context, _ bson.M) ([]*model.ScoreReview, error) {
	var out []*model.ScoreReview
	for _, r := range f.reviews {
		out = append(out, r)
	}
	return out, nil
}
func (f *fakeReviewRepo) ApplyPendingResult(_ context.Context, id primitive.ObjectID, update bson.M) (bool, error) {
	r, ok := f.reviews[id.Hex()]
	if !ok || r.Status != constants.ReviewStatusPending {
		return false, nil
	}
	if set, ok := update["$set"].(bson.M); ok {
		if v, ok := set["status"].(string); ok {
			r.Status = v
		}
		if v, ok := set["teacher_opinion"].(string); ok {
			r.TeacherOpinion = v
		}
		if v, ok := set["history"].([]model.ReviewHistoryItem); ok {
			r.History = v
		}
		if v, ok := set["score_corrected"].(bool); ok {
			r.ScoreCorrected = v
		}
		if v, ok := set["corrected_score"].(float64); ok {
			r.CorrectedScore = v
		}
		if v, ok := set["corrected_passed"].(bool); ok {
			r.CorrectedPassed = v
		}
		if v, ok := set["processed_at"].(time.Time); ok {
			r.ProcessedAt = &v
		}
		if v, ok := set["teacher_id"].(primitive.ObjectID); ok {
			r.TeacherID = v
		}
		if v, ok := set["teacher_name"].(string); ok {
			r.TeacherName = v
		}
	}
	return true, nil
}
func (f *fakeReviewRepo) DeleteByID(_ context.Context, id primitive.ObjectID) error {
	delete(f.reviews, id.Hex())
	return nil
}

// fakeAuditRepo 内存版审计日志仓储。
type fakeAuditRepo struct {
	logs []*model.AuditLog
}

func (f *fakeAuditRepo) Create(_ context.Context, a *model.AuditLog) error {
	f.logs = append(f.logs, a)
	return nil
}
func (f *fakeAuditRepo) List(_ context.Context, _ bson.M, _, _ int64) ([]*model.AuditLog, int64, error) {
	return f.logs, int64(len(f.logs)), nil
}

// prepareReviewedRecord 构造一份已批改完成的答卷（客观题 5 分 + 主观题 4 分 = 9 分，及格 60）。
func prepareReviewedRecord(t *testing.T, svc *ExamRecordService) (*model.Exam, *model.Question, *model.Question, *model.ExamRecord, primitive.ObjectID, primitive.ObjectID) {
	t.Helper()
	teacher := primitive.NewObjectID()
	student := primitive.NewObjectID()
	ctx := context.Background()

	singleReq := &dto.CreateQuestionRequest{
		Type: "single", Subject: "数学", KnowledgePoints: []string{"代数"},
		Difficulty: "easy", Content: "1+1=?", Answer: "B", Score: 5,
		Options: []dto.OptionInput{{Key: "A", Text: "1"}, {Key: "B", Text: "2"}},
	}
	q1, _ := svc.exam.question.Create(ctx, singleReq, teacher)
	fillReq := &dto.CreateQuestionRequest{
		Type: "fill", Subject: "数学", KnowledgePoints: []string{"代数"},
		Difficulty: "medium", Content: "圆周率前两位是？", Answer: "3.14", Score: 5,
	}
	q2, _ := svc.exam.question.Create(ctx, fillReq, teacher)

	now := time.Now()
	exam, err := svc.exam.Create(ctx, &dto.CreateExamRequest{
		Title: "复核测试卷", Subject: "数学", DurationMin: 30, PassScore: 5,
		StartAt: now.Add(-time.Hour), EndAt: now.Add(time.Hour),
		Questions: []dto.ExamQuestionInput{{QuestionID: q1.ID.Hex()}, {QuestionID: q2.ID.Hex()}},
	}, teacher)
	if err != nil {
		t.Fatalf("create exam: %v", err)
	}
	_, _ = svc.exam.Publish(ctx, exam.ID, "t@example.com")

	rec, err := svc.StartExam(ctx, exam.ID, student, "李同学")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	_, err = svc.Submit(ctx, rec.ID, []dto.AnswerInput{{QuestionID: q1.ID.Hex(), Answer: "B"}}, 0, nil, false)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	graded, err := svc.Grade(ctx, rec.ID, []dto.GradeItem{{QuestionID: q2.ID.Hex(), Score: 4, Comment: "部分正确"}}, "t@example.com")
	if err != nil {
		t.Fatalf("grade: %v", err)
	}
	return exam, q1, q2, graded, teacher, student
}

func newTestReviewSvc() (*ScoreReviewService, *ExamRecordService) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	recordSvc := newTestRecordSvc()
	reviewRepo := newFakeReviewRepo()
	auditSvc := NewAuditService(&fakeAuditRepo{}, logger)
	return NewScoreReviewService(reviewRepo, recordSvc.repo, recordSvc, auditSvc, logger), recordSvc
}

func TestReviewCreateWithinWindow(t *testing.T) {
	reviewSvc, recordSvc := newTestReviewSvc()
	_, _, _, rec, _, student := prepareReviewedRecord(t, recordSvc)

	op := Operator{ID: student, Name: "李同学", Role: constants.RoleStudent}
	rev, err := reviewSvc.Create(context.Background(), rec.ID, "主观题给分偏低，我的答案包含关键步骤", op)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if rev.Status != constants.ReviewStatusPending {
		t.Fatalf("status = %s, want pending", rev.Status)
	}
	if rev.OriginalScore != 9 {
		t.Fatalf("original score = %f, want 9", rev.OriginalScore)
	}
	// 原成绩应被快照，且答卷挂上待处理复核
	updated, _ := recordSvc.GetByID(context.Background(), rec.ID)
	if updated.ReviewStatus != constants.ReviewStatusPending || updated.FinalScore != 9 {
		t.Fatalf("record review linkage wrong: status=%s score=%f", updated.ReviewStatus, updated.FinalScore)
	}
	if len(rev.History) != 1 || rev.History[0].Action != constants.ReviewActionSubmit {
		t.Fatalf("history = %+v", rev.History)
	}
}

func TestReviewDuplicateRejected(t *testing.T) {
	reviewSvc, recordSvc := newTestReviewSvc()
	_, _, _, rec, _, student := prepareReviewedRecord(t, recordSvc)

	op := Operator{ID: student, Name: "李同学", Role: constants.RoleStudent}
	if _, err := reviewSvc.Create(context.Background(), rec.ID, "第一次申请", op); err != nil {
		t.Fatalf("first create: %v", err)
	}
	// 待处理期间重复提交必须拒绝
	if _, err := reviewSvc.Create(context.Background(), rec.ID, "重复申请", op); err == nil {
		t.Fatal("待处理时重复申请应失败")
	}

	// 教师驳回后再次申请仍必须拒绝（每份答卷只允许一次）
	teacherOp := Operator{ID: primitive.NewObjectID(), Name: "王老师", Role: constants.RoleTeacher}
	rev, _ := reviewSvc.repo.FindByRecord(context.Background(), rec.ID)
	if _, err := reviewSvc.Process(context.Background(), rev.ID, dto.HandleScoreReviewRequest{
		Action: "reject", Opinion: "批改无误，维持原分",
	}, teacherOp); err != nil {
		t.Fatalf("reject: %v", err)
	}
	if _, err := reviewSvc.Create(context.Background(), rec.ID, "驳回后再申请", op); err == nil {
		t.Fatal("已处理后再次申请应失败")
	}
}

func TestReviewApproveCorrectsScore(t *testing.T) {
	reviewSvc, recordSvc := newTestReviewSvc()
	exam, _, _, rec, teacher, student := prepareReviewedRecord(t, recordSvc)

	studentOp := Operator{ID: student, Name: "李同学", Role: constants.RoleStudent}
	rev, err := reviewSvc.Create(context.Background(), rec.ID, "主观题应得满分", studentOp)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	teacherOp := Operator{ID: teacher, Name: "王老师", Role: constants.RoleTeacher}
	newScore := 10.0
	done, err := reviewSvc.Process(context.Background(), rev.ID, dto.HandleScoreReviewRequest{
		Action: "approve", Opinion: "复核后主观题补 1 分", CorrectedScore: &newScore,
	}, teacherOp)
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if done.Status != constants.ReviewStatusApproved {
		t.Fatalf("status = %s", done.Status)
	}
	if !done.ScoreCorrected || done.CorrectedScore != 10 {
		t.Fatalf("corrected = %v score = %f", done.ScoreCorrected, done.CorrectedScore)
	}
	// 答卷最终分与及格状态已更新（及格线 5）
	updated, _ := recordSvc.GetByID(context.Background(), rec.ID)
	if updated.FinalScore != 10 || !updated.ScoreCorrected {
		t.Fatalf("record final = %f corrected=%v", updated.FinalScore, updated.ScoreCorrected)
	}
	resp := dto.ToRecordResponse(updated)
	if !resp.IsPassed || resp.ReviewStatus != constants.ReviewStatusApproved {
		t.Fatalf("response passed=%v review=%s", resp.IsPassed, resp.ReviewStatus)
	}
	_ = exam
	// 留痕：申请 + 受理两条
	if len(done.History) != 2 {
		t.Fatalf("history len = %d, want 2", len(done.History))
	}
}

func TestReviewRejectKeepsScore(t *testing.T) {
	reviewSvc, recordSvc := newTestReviewSvc()
	_, _, _, rec, teacher, student := prepareReviewedRecord(t, recordSvc)

	studentOp := Operator{ID: student, Name: "李同学", Role: constants.RoleStudent}
	rev, _ := reviewSvc.Create(context.Background(), rec.ID, "分数不对", studentOp)

	teacherOp := Operator{ID: teacher, Name: "王老师", Role: constants.RoleTeacher}
	done, err := reviewSvc.Process(context.Background(), rev.ID, dto.HandleScoreReviewRequest{
		Action: "reject", Opinion: "评分标准执行正确，驳回",
	}, teacherOp)
	if err != nil {
		t.Fatalf("reject: %v", err)
	}
	if done.Status != constants.ReviewStatusRejected {
		t.Fatalf("status = %s", done.Status)
	}
	updated, _ := recordSvc.GetByID(context.Background(), rec.ID)
	if updated.FinalScore != 9 || updated.ScoreCorrected {
		t.Fatalf("驳回不应改变原成绩: score=%f corrected=%v", updated.FinalScore, updated.ScoreCorrected)
	}
}

func TestReviewProcessWithoutOpinionRejected(t *testing.T) {
	reviewSvc, recordSvc := newTestReviewSvc()
	_, _, _, rec, teacher, student := prepareReviewedRecord(t, recordSvc)

	studentOp := Operator{ID: student, Name: "李同学", Role: constants.RoleStudent}
	rev, _ := reviewSvc.Create(context.Background(), rec.ID, "分数不对", studentOp)
	teacherOp := Operator{ID: teacher, Name: "王老师", Role: constants.RoleTeacher}

	if _, err := reviewSvc.Process(context.Background(), rev.ID, dto.HandleScoreReviewRequest{
		Action: "approve", Opinion: "  ",
	}, teacherOp); err == nil {
		t.Fatal("受理不填意见应失败")
	}
	if _, err := reviewSvc.Process(context.Background(), rev.ID, dto.HandleScoreReviewRequest{
		Action: "reject", Opinion: "",
	}, teacherOp); err == nil {
		t.Fatal("驳回不填意见应失败")
	}
}

func TestReviewDoubleProcessRejected(t *testing.T) {
	reviewSvc, recordSvc := newTestReviewSvc()
	_, _, _, rec, _, student := prepareReviewedRecord(t, recordSvc)

	studentOp := Operator{ID: student, Name: "李同学", Role: constants.RoleStudent}
	rev, _ := reviewSvc.Create(context.Background(), rec.ID, "分数不对", studentOp)
	t1 := Operator{ID: primitive.NewObjectID(), Name: "王老师", Role: constants.RoleTeacher}
	if _, err := reviewSvc.Process(context.Background(), rev.ID, dto.HandleScoreReviewRequest{
		Action: "reject", Opinion: "第一次处理",
	}, t1); err != nil {
		t.Fatalf("first process: %v", err)
	}
	// 已处理的复核单不能再次处理（越权/重复处理不能改变成绩）
	if _, err := reviewSvc.Process(context.Background(), rev.ID, dto.HandleScoreReviewRequest{
		Action: "approve", Opinion: "第二次处理",
	}, t1); err == nil {
		t.Fatal("重复处理应失败")
	}
}

func TestReviewUnauthorizedAndLocked(t *testing.T) {
	reviewSvc, recordSvc := newTestReviewSvc()
	_, _, _, rec, teacher, student := prepareReviewedRecord(t, recordSvc)
	ctx := context.Background()

	// 学生 A 不能对学生 B 的答卷发起复核
	other := Operator{ID: primitive.NewObjectID(), Name: "张同学", Role: constants.RoleStudent}
	if _, err := reviewSvc.Create(ctx, rec.ID, "越权申请", other); err == nil {
		t.Fatal("非本人答卷申请复核应失败")
	}
	// 学生不能处理复核
	studentOp := Operator{ID: student, Name: "李同学", Role: constants.RoleStudent}
	rev, err := reviewSvc.Create(ctx, rec.ID, "本人申请", studentOp)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// 复核存在期间，普通批改通道锁定，不能再通过 grade 改分
	if _, err := recordSvc.Grade(ctx, rec.ID, []dto.GradeItem{
		{QuestionID: rec.Questions[1].QuestionID.Hex(), Score: 2, Comment: "试图改分"},
	}, "teacher@example.com"); err == nil {
		t.Fatal("复核期间普通批改应被锁定")
	}
	updated, _ := recordSvc.GetByID(ctx, rec.ID)
	if updated.FinalScore != 9 {
		t.Fatalf("锁定失败，成绩被改动: %f", updated.FinalScore)
	}
	_ = teacher

	// 受理时更正分超过试卷总分应拒绝
	teacherOp := Operator{ID: teacher, Name: "王老师", Role: constants.RoleTeacher}
	bad := 999.0
	if _, err := reviewSvc.Process(ctx, rev.ID, dto.HandleScoreReviewRequest{
		Action: "approve", Opinion: "想改成 999", CorrectedScore: &bad,
	}, teacherOp); err == nil {
		t.Fatal("更正分超出试卷总分应失败")
	}
}

func TestReviewExpiredWindowRejected(t *testing.T) {
	reviewSvc, recordSvc := newTestReviewSvc()
	_, _, _, rec, _, student := prepareReviewedRecord(t, recordSvc)
	ctx := context.Background()

	// 把批改完成时间拨到 49 小时前：复核窗口已关闭
	expired := time.Now().Add(-(constants.ReviewApplyWindow + time.Hour))
	rec.GradedAt = &expired
	rec.SubmittedAt = &expired
	if err := recordSvc.repo.Update(ctx, rec); err != nil {
		t.Fatalf("update: %v", err)
	}

	op := Operator{ID: student, Name: "李同学", Role: constants.RoleStudent}
	_, err := reviewSvc.Create(ctx, rec.ID, "超过48小时才申请", op)
	if err == nil {
		t.Fatal("超过 48 小时窗口应拒绝复核")
	}
	var appErr *util.AppError
	if !errors.As(err, &appErr) || appErr.Code != constants.CodeReviewWindowClosed {
		t.Fatalf("期望 CodeReviewWindowClosed(%d)，得到 %v", constants.CodeReviewWindowClosed, err)
	}
	// 原成绩不应变化
	updated, _ := recordSvc.GetByID(ctx, rec.ID)
	if updated.FinalScore != 9 || updated.ReviewStatus != "" {
		t.Fatalf("逾期拒绝不应产生复核单或改动成绩: score=%f review=%q", updated.FinalScore, updated.ReviewStatus)
	}
}

func TestReviewStudentCannotViewOthers(t *testing.T) {
	reviewSvc, recordSvc := newTestReviewSvc()
	_, _, _, rec, _, student := prepareReviewedRecord(t, recordSvc)

	owner := Operator{ID: student, Name: "李同学", Role: constants.RoleStudent}
	rev, err := reviewSvc.Create(context.Background(), rec.ID, "本人申请", owner)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	other := Operator{ID: primitive.NewObjectID(), Name: "张同学", Role: constants.RoleStudent}
	if _, err := reviewSvc.GetForViewer(context.Background(), rev.ID, other); err == nil {
		t.Fatal("学生不应能查看他人复核单")
	}
}
