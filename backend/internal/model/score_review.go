package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ScoreReview 成绩复核申请实体，集合 score_reviews。
// 状态枚举（ScoreReviewStatus）：pending / approved / rejected（状态机见 constants/enums.go）。
// 业务规则：
//  1. 仅批改完成（graded）后 48 小时内，学生本人可发起一次复核并说明理由；
//  2. 同一答卷只允许一条复核申请（record_id 唯一索引兜底），逾期或已有申请直接拒绝；
//  3. 教师可驳回或受理（受理时更正总分/及格状态），两种处理都必须填写意见；
//  4. 申请与处理全程留痕（Reason/Decision + 时间戳/操作人），不修改原始批改明细。
type ScoreReview struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	RecordID    primitive.ObjectID `bson:"record_id" json:"record_id"`
	ExamID      primitive.ObjectID `bson:"exam_id" json:"exam_id"`
	ExamTitle   string             `bson:"exam_title" json:"exam_title"`
	StudentID   primitive.ObjectID `bson:"student_id" json:"student_id"`
	StudentName string             `bson:"student_name" json:"student_name"`
	Reason      string             `bson:"reason" json:"reason"` // 学生申请理由（必填）
	Status      string             `bson:"status" json:"status"` // pending/approved/rejected

	// 受理时的成绩快照与教师更正内容（驳回时为空）
	OriginalScore  float64 `bson:"original_score,omitempty" json:"original_score,omitempty"`
	CorrectedScore float64 `bson:"corrected_score,omitempty" json:"corrected_score,omitempty"`
	// PassedOverride 教师对及格状态的显式更正；nil 表示按更正后总分与及格线自动判定。
	PassedOverride  *bool `bson:"passed_override,omitempty" json:"passed_override,omitempty"`
	OriginalPassed  bool  `bson:"original_passed,omitempty" json:"original_passed,omitempty"`
	CorrectedPassed bool  `bson:"corrected_passed,omitempty" json:"corrected_passed,omitempty"`

	// 处理留痕（驳回/受理均必填意见）
	Decision    string             `bson:"decision,omitempty" json:"decision,omitempty"` // approve/reject
	Comment     string             `bson:"comment,omitempty" json:"comment,omitempty"`   // 教师处理意见
	HandledBy   primitive.ObjectID `bson:"handled_by,omitempty" json:"handled_by,omitempty"`
	HandlerName string             `bson:"handler_name,omitempty" json:"handler_name,omitempty"`
	HandledAt   *time.Time         `bson:"handled_at,omitempty" json:"handled_at,omitempty"`

	CreatedAt time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time `bson:"updated_at" json:"updated_at"`
}
