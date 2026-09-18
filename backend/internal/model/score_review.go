package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ReviewHistoryItem 复核流转留痕条目（申请/受理/驳回全程不可变记录）。
type ReviewHistoryItem struct {
	Action         string    `bson:"action" json:"action"` // submit / approve / reject
	OperatorID     string    `bson:"operator_id" json:"operator_id"`
	OperatorName   string    `bson:"operator_name" json:"operator_name"`
	OperatorRole   string    `bson:"operator_role" json:"operator_role"`
	Opinion        string    `bson:"opinion" json:"opinion"` // 学生申请理由或教师处理意见
	FromStatus     string    `bson:"from_status" json:"from_status"`
	ToStatus       string    `bson:"to_status" json:"to_status"`
	OriginalScore  float64   `bson:"original_score" json:"original_score"`
	CorrectedScore float64   `bson:"corrected_score" json:"corrected_score"`
	OccurredAt     time.Time `bson:"occurred_at" json:"occurred_at"`
}

// ScoreReview 成绩复核申请实体，集合 score_reviews。
// 状态枚举：pending / approved / rejected（状态机见 constants/enums.go）。
// 业务规则：每份答卷（exam_record）至多一条复核记录（无论待处理/已处理）；
// 仅批改完成后 48 小时内可申请；待处理期间原成绩锁定，教师只能通过受理并更正来改变总分/及格状态。
type ScoreReview struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	RecordID    primitive.ObjectID `bson:"record_id" json:"record_id"`
	ExamID      primitive.ObjectID `bson:"exam_id" json:"exam_id"`
	ExamTitle   string             `bson:"exam_title" json:"exam_title"`
	StudentID   primitive.ObjectID `bson:"student_id" json:"student_id"`
	StudentName string             `bson:"student_name" json:"student_name"`
	Status      string             `bson:"status" json:"status"` // pending / approved / rejected
	Reason      string             `bson:"reason" json:"reason"` // 学生申请理由（必填）

	// 申请时刻的成绩快照（原成绩，任何重复提交/越权操作都不得修改）
	OriginalScore  float64 `bson:"original_score" json:"original_score"`
	OriginalPassed bool    `bson:"original_passed" json:"original_passed"`

	// 教师受理后的更正结果
	ScoreCorrected  bool               `bson:"score_corrected" json:"score_corrected"`
	CorrectedScore  float64            `bson:"corrected_score" json:"corrected_score"`
	CorrectedPassed bool               `bson:"corrected_passed" json:"corrected_passed"`
	TeacherOpinion  string             `bson:"teacher_opinion" json:"teacher_opinion"` // 教师处理意见（受理/驳回均必填）
	TeacherID       primitive.ObjectID `bson:"teacher_id,omitempty" json:"teacher_id,omitempty"`
	TeacherName     string             `bson:"teacher_name" json:"teacher_name"`

	// 全程留痕：申请与处理的完整操作链
	History     []ReviewHistoryItem `bson:"history" json:"history"`
	Deadline    time.Time           `bson:"deadline" json:"deadline"` // 申请截止时间（批改完成 +48h）
	ProcessedAt *time.Time          `bson:"processed_at,omitempty" json:"processed_at"`
	CreatedAt   time.Time           `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time           `bson:"updated_at" json:"updated_at"`
}
