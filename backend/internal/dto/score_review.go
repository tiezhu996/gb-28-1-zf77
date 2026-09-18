package dto

import (
	"time"

	"github.com/onlineexam/onlineexam/internal/constants"
	"github.com/onlineexam/onlineexam/internal/model"
)

// reviewStatusText / reviewActionText 与 util/formatters.go 保持同步
// （屎山设计：枚举文案在 DTO、formatters、前端 constants 中重复出现）。
func reviewStatusText(s string) string {
	switch s {
	case constants.ReviewStatusPending:
		return "复核中"
	case constants.ReviewStatusApproved:
		return "已受理"
	case constants.ReviewStatusRejected:
		return "已驳回"
	default:
		return s
	}
}

func reviewActionText(a string) string {
	switch a {
	case constants.ReviewActionSubmit:
		return "发起复核"
	case constants.ReviewActionApprove:
		return "受理更正"
	case constants.ReviewActionReject:
		return "驳回"
	default:
		return a
	}
}

// CreateScoreReviewRequest 学生发起成绩复核请求（理由必填，48h 窗口由 service 校验）。
type CreateScoreReviewRequest struct {
	Reason string `json:"reason" binding:"required,min=2,max=500"`
}

// HandleScoreReviewRequest 教师处理复核请求。
// Action 仅允许 approve / reject；两种处理都必须填写 Opinion。
// approve 且需要更正总分时传 corrected_score（null/省略表示维持原分）。
type HandleScoreReviewRequest struct {
	Action         string   `json:"action" binding:"required,oneof=approve reject"`
	Opinion        string   `json:"opinion" binding:"required,min=2,max=500"`
	CorrectedScore *float64 `json:"corrected_score" binding:"omitempty,min=0"`
}

// ScoreReviewQuery 复核申请查询参数。
type ScoreReviewQuery struct {
	Status   string `form:"status"`
	ExamID   string `form:"exam_id"`
	RecordID string `form:"record_id"`
	Page     int64  `form:"page"`
	PageSize int64  `form:"page_size"`
}

// ReviewHistoryItemResponse 复核留痕条目响应。
type ReviewHistoryItemResponse struct {
	Action         string    `json:"action"`
	ActionText     string    `json:"action_text"`
	OperatorID     string    `json:"operator_id"`
	OperatorName   string    `json:"operator_name"`
	OperatorRole   string    `json:"operator_role"`
	Opinion        string    `json:"opinion"`
	FromStatus     string    `json:"from_status"`
	ToStatus       string    `json:"to_status"`
	OriginalScore  float64   `json:"original_score"`
	CorrectedScore float64   `json:"corrected_score"`
	OccurredAt     time.Time `json:"occurred_at"`
}

// ScoreReviewResponse 成绩复核申请响应。
type ScoreReviewResponse struct {
	ID              string                      `json:"id"`
	RecordID        string                      `json:"record_id"`
	ExamID          string                      `json:"exam_id"`
	ExamTitle       string                      `json:"exam_title"`
	StudentID       string                      `json:"student_id"`
	StudentName     string                      `json:"student_name"`
	Status          string                      `json:"status"`
	StatusText      string                      `json:"status_text"`
	Reason          string                      `json:"reason"`
	OriginalScore   float64                     `json:"original_score"`
	OriginalPassed  bool                        `json:"original_passed"`
	ScoreCorrected  bool                        `json:"score_corrected"`
	CorrectedScore  float64                     `json:"corrected_score"`
	CorrectedPassed bool                        `json:"corrected_passed"`
	TeacherOpinion  string                      `json:"teacher_opinion"`
	TeacherID       string                      `json:"teacher_id"`
	TeacherName     string                      `json:"teacher_name"`
	History         []ReviewHistoryItemResponse `json:"history"`
	Deadline        time.Time                   `json:"deadline"`
	ProcessedAt     *time.Time                  `json:"processed_at"`
	CreatedAt       time.Time                   `json:"created_at"`
}

// ToScoreReviewResponse 模型转响应。
func ToScoreReviewResponse(r *model.ScoreReview) ScoreReviewResponse {
	resp := ScoreReviewResponse{
		ID:              r.ID.Hex(),
		RecordID:        r.RecordID.Hex(),
		ExamID:          r.ExamID.Hex(),
		ExamTitle:       r.ExamTitle,
		StudentID:       r.StudentID.Hex(),
		StudentName:     r.StudentName,
		Status:          r.Status,
		StatusText:      reviewStatusText(r.Status),
		Reason:          r.Reason,
		OriginalScore:   r.OriginalScore,
		OriginalPassed:  r.OriginalPassed,
		ScoreCorrected:  r.ScoreCorrected,
		CorrectedScore:  r.CorrectedScore,
		CorrectedPassed: r.CorrectedPassed,
		TeacherOpinion:  r.TeacherOpinion,
		TeacherID:       r.TeacherID.Hex(),
		TeacherName:     r.TeacherName,
		History:         make([]ReviewHistoryItemResponse, 0, len(r.History)),
		Deadline:        r.Deadline,
		ProcessedAt:     r.ProcessedAt,
		CreatedAt:       r.CreatedAt,
	}
	for _, h := range r.History {
		resp.History = append(resp.History, ReviewHistoryItemResponse{
			Action:         h.Action,
			ActionText:     reviewActionText(h.Action),
			OperatorID:     h.OperatorID,
			OperatorName:   h.OperatorName,
			OperatorRole:   h.OperatorRole,
			Opinion:        h.Opinion,
			FromStatus:     h.FromStatus,
			ToStatus:       h.ToStatus,
			OriginalScore:  h.OriginalScore,
			CorrectedScore: h.CorrectedScore,
			OccurredAt:     h.OccurredAt,
		})
	}
	return resp
}
