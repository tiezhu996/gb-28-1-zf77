package dto

import (
	"time"

	"github.com/onlineexam/onlineexam/internal/constants"
	"github.com/onlineexam/onlineexam/internal/model"
)

// StartExamRequest 开始考试请求（无业务字段，预留防作弊配置）。
type StartExamRequest struct {
	ExamID string `json:"exam_id" binding:"required"`
}

// AnswerInput 单题作答输入。
type AnswerInput struct {
	QuestionID string `json:"question_id" binding:"required"`
	Answer     string `json:"answer" binding:"max=2000"`
}

// SubmitRecordRequest 提交答卷请求。
type SubmitRecordRequest struct {
	Answers     []AnswerInput     `json:"answers" binding:"required,min=1"`
	CheatCount  int               `json:"cheat_count" binding:"omitempty,min=0,max=1000"`
	CheatEvents []CheatEventInput `json:"cheat_events"`
}

// CheatEventInput 防作弊事件输入。
type CheatEventInput struct {
	Type   string `json:"type" binding:"omitempty,oneof=switch_tab copy_paste blur"`
	Detail string `json:"detail" binding:"omitempty,max=500"`
}

// GradeRequest 主观题批改请求。
type GradeRequest struct {
	Grades []GradeItem `json:"grades" binding:"required,min=1"`
}

// GradeItem 单题批改。
type GradeItem struct {
	QuestionID string  `json:"question_id" binding:"required"`
	Score      float64 `json:"score" binding:"min=0,max=100"`
	Comment    string  `json:"comment" binding:"omitempty,max=500"`
}

// RecordQuery 考试记录查询参数。
type RecordQuery struct {
	ExamID    string `form:"exam_id"`
	Status    string `form:"status"`
	StudentID string `form:"student_id"`
	Page      int64  `form:"page"`
	PageSize  int64  `form:"page_size"`
}

// ExamReportItem 成绩分析单题正确率。
type ExamReportItem struct {
	QuestionID   string  `json:"question_id"`
	Content      string  `json:"content"`
	Type         string  `json:"type"`
	AnswerCount  int     `json:"answer_count"`
	CorrectCount int     `json:"correct_count"`
	Accuracy     float64 `json:"accuracy"`
}

// ExamReport 成绩分析报告。
type ExamReport struct {
	ExamID          string           `json:"exam_id"`
	ExamTitle       string           `json:"exam_title"`
	TotalStudents   int              `json:"total_students"`
	AverageScore    float64          `json:"average_score"`
	MaxScore        float64          `json:"max_score"`
	MinScore        float64          `json:"min_score"`
	PassRate        float64          `json:"pass_rate"`
	ScoreBands      map[string]int   `json:"score_bands"` // 分数段直方图
	QuestionReports []ExamReportItem `json:"question_reports"`
	// 成绩复核闭环统计（以复核更正后的最终分为准）
	ReviewPendingCount   int `json:"review_pending_count"`
	ReviewApprovedCount  int `json:"review_approved_count"`
	ReviewRejectedCount  int `json:"review_rejected_count"`
	ReviewCorrectedCount int `json:"review_corrected_count"` // 受理且实际改分的份数
}

// RecordResponse 考试记录响应。
type RecordResponse struct {
	ID               string                  `json:"id"`
	ExamID           string                  `json:"exam_id"`
	ExamTitle        string                  `json:"exam_title"`
	StudentID        string                  `json:"student_id"`
	StudentName      string                  `json:"student_name"`
	Status           string                  `json:"status"`
	StartedAt        time.Time               `json:"started_at"`
	SubmittedAt      *time.Time              `json:"submitted_at"`
	GradedAt         *time.Time              `json:"graded_at"`
	ObjectiveScore   float64                 `json:"objective_score"`
	SubjectiveScore  float64                 `json:"subjective_score"`
	FinalScore       float64                 `json:"final_score"`
	PassScore        float64                 `json:"pass_score"`
	IsPassed         bool                    `json:"is_passed"`       // 最终及格状态（复核更正后以更正结果为准）
	ScoreCorrected   bool                    `json:"score_corrected"` // 最终分是否经复核更正
	CheatCount       int                     `json:"cheat_count"`
	AutoSubmitted    bool                    `json:"auto_submitted"`
	Questions        []model.AttemptQuestion `json:"questions"`
	ReviewID         string                  `json:"review_id"`
	ReviewStatus     string                  `json:"review_status"` // none/pending/approved/rejected
	ReviewDeadline   *time.Time              `json:"review_deadline"`
	ReviewWindowOpen bool                    `json:"review_window_open"` // 批改完成 48h 内且无复核记录
	CreatedAt        time.Time               `json:"created_at"`
}

// ToRecordResponse 模型转响应。复核窗口规则与 service 保持一致：批改完成（graded）起 48h。
func ToRecordResponse(r *model.ExamRecord) RecordResponse {
	resp := RecordResponse{
		ID:              r.ID.Hex(),
		ExamID:          r.ExamID.Hex(),
		ExamTitle:       r.ExamTitle,
		StudentID:       r.StudentID.Hex(),
		StudentName:     r.StudentName,
		Status:          r.Status,
		StartedAt:       r.StartedAt,
		SubmittedAt:     r.SubmittedAt,
		GradedAt:        r.GradedAt,
		ObjectiveScore:  r.ObjectiveScore,
		SubjectiveScore: r.SubjectiveScore,
		FinalScore:      r.FinalScore,
		PassScore:       r.PassScore,
		IsPassed:        r.PassScore > 0 && r.FinalScore >= r.PassScore,
		ScoreCorrected:  r.ScoreCorrected,
		CheatCount:      r.CheatCount,
		AutoSubmitted:   r.AutoSubmitted,
		Questions:       r.Questions,
		ReviewID:        r.ReviewID.Hex(),
		ReviewStatus:    reviewStatusOf(r),
		CreatedAt:       r.CreatedAt,
	}
	if base := reviewWindowBase(r); base != nil {
		deadline := base.Add(constants.ReviewApplyWindow)
		resp.ReviewDeadline = &deadline
		resp.ReviewWindowOpen = r.ReviewStatus == "" && time.Now().Before(deadline)
	}
	return resp
}

// reviewStatusOf 答卷复核状态，空值归一化为 none。
func reviewStatusOf(r *model.ExamRecord) string {
	if r.ReviewStatus == "" {
		return constants.ReviewStatusNone
	}
	return r.ReviewStatus
}

// reviewWindowBase 复核窗口起点：优先 GradedAt，兼容历史数据回退 SubmittedAt。
// 仅已批改完成（graded）的答卷开放窗口。
func reviewWindowBase(r *model.ExamRecord) *time.Time {
	if r.Status != constants.RecordStatusGraded {
		return nil
	}
	if r.GradedAt != nil {
		return r.GradedAt
	}
	return r.SubmittedAt
}
