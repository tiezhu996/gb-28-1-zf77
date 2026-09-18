package dto

import (
	"time"

	"github.com/onlineexam/onlineexam/internal/constants"
	"github.com/onlineexam/onlineexam/internal/model"
)

// CreateReviewRequest 学生发起成绩复核请求。
type CreateReviewRequest struct {
	Reason string `json:"reason" binding:"required,min=2,max=500"` // 申请理由必填
}

// ReviewDecisionRequest 教师处理复核请求。
// Action=approve 受理并更正：corrected_score 必填；passed_override 为空则按及格线自动判定。
// Action=reject 驳回：仅需 comment。两种处理 comment 均必填。
type ReviewDecisionRequest struct {
	Action         string  `json:"action" binding:"required,oneof=approve reject"`
	Comment        string  `json:"comment" binding:"required,min=1,max=500"` // 处理意见必填（全程留痕）
	CorrectedScore float64 `json:"corrected_score" binding:"min=0"`          // 更正后总分（受理时使用）
	PassedOverride *bool   `json:"passed_override,omitempty"`                // 显式更正及格状态；nil 自动判定
}

// ReviewResponse 成绩复核响应。
type ReviewResponse struct {
	ID              string     `json:"id"`
	RecordID        string     `json:"record_id"`
	ExamID          string     `json:"exam_id"`
	ExamTitle       string     `json:"exam_title"`
	StudentID       string     `json:"student_id"`
	StudentName     string     `json:"student_name"`
	Reason          string     `json:"reason"`
	Status          string     `json:"status"`
	StatusText      string     `json:"status_text"`
	OriginalScore   float64    `json:"original_score"`
	CorrectedScore  float64    `json:"corrected_score"`
	OriginalPassed  bool       `json:"original_passed"`
	CorrectedPassed bool       `json:"corrected_passed"`
	PassedOverride  *bool      `json:"passed_override,omitempty"`
	Decision        string     `json:"decision,omitempty"`
	Comment         string     `json:"comment,omitempty"`
	HandlerName     string     `json:"handler_name,omitempty"`
	HandledAt       *time.Time `json:"handled_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

// ToReviewResponse 模型转响应。
func ToReviewResponse(rv *model.ScoreReview) ReviewResponse {
	return ReviewResponse{
		ID:              rv.ID.Hex(),
		RecordID:        rv.RecordID.Hex(),
		ExamID:          rv.ExamID.Hex(),
		ExamTitle:       rv.ExamTitle,
		StudentID:       rv.StudentID.Hex(),
		StudentName:     rv.StudentName,
		Reason:          rv.Reason,
		Status:          rv.Status,
		StatusText:      reviewStatusText(rv.Status),
		OriginalScore:   rv.OriginalScore,
		CorrectedScore:  rv.CorrectedScore,
		OriginalPassed:  rv.OriginalPassed,
		CorrectedPassed: rv.CorrectedPassed,
		PassedOverride:  rv.PassedOverride,
		Decision:        rv.Decision,
		Comment:         rv.Comment,
		HandlerName:     rv.HandlerName,
		HandledAt:       rv.HandledAt,
		CreatedAt:       rv.CreatedAt,
	}
}

// AdjustedRecord 成绩分析中“经复核更正”的答卷条目。
type AdjustedRecord struct {
	RecordID        string  `json:"record_id"`
	StudentName     string  `json:"student_name"`
	OriginalScore   float64 `json:"original_score"`
	CorrectedScore  float64 `json:"corrected_score"`
	CorrectedPassed bool    `json:"corrected_passed"`
	Comment         string  `json:"comment"`
}

// ReviewStats 成绩分析页的复核统计。
type ReviewStats struct {
	TotalCount      int              `json:"total_count"`
	PendingCount    int              `json:"pending_count"`
	ApprovedCount   int              `json:"approved_count"`
	RejectedCount   int              `json:"rejected_count"`
	AdjustedRecords []AdjustedRecord `json:"adjusted_records"`
}

func reviewStatusText(s string) string {
	switch s {
	case constants.ReviewStatusPending:
		return "待处理"
	case constants.ReviewStatusApproved:
		return "已受理"
	case constants.ReviewStatusRejected:
		return "已驳回"
	default:
		return s
	}
}
