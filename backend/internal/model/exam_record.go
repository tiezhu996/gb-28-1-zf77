package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AttemptOption 考试快照中的选项（已随机打乱）。
type AttemptOption struct {
	Key  string `bson:"key" json:"key"`
	Text string `bson:"text" json:"text"`
}

// AttemptQuestion 考试快照中的题目。
type AttemptQuestion struct {
	QuestionID      primitive.ObjectID `bson:"question_id" json:"question_id"`
	Type            string             `bson:"type" json:"type"`
	Subject         string             `bson:"subject" json:"subject"`
	KnowledgePoints []string           `bson:"knowledge_points" json:"knowledge_points"`
	Content         string             `bson:"content" json:"content"`
	Options         []AttemptOption    `bson:"options,omitempty" json:"options"`
	Score           float64            `bson:"score" json:"score"`
	CorrectAnswer   string             `bson:"correct_answer" json:"correct_answer"` // 客观题标准答案快照
	UserAnswer      string             `bson:"user_answer" json:"user_answer"`       // 学生作答
	Result          string             `bson:"result" json:"result"`                 // correct/wrong/partial/unmarked
	GotScore        float64            `bson:"got_score" json:"got_score"`           // 客观题自动得分
	SubjectiveScore float64            `bson:"subjective_score,omitempty" json:"subjective_score"`
	Comment         string             `bson:"comment,omitempty" json:"comment"`
	Marked          bool               `bson:"marked" json:"marked"` // 教师是否已批改
}

// CheatEvent 切屏/防作弊事件。
type CheatEvent struct {
	Type       string    `bson:"type" json:"type"` // switch_tab / copy_paste / blur
	Detail     string    `bson:"detail" json:"detail"`
	OccurredAt time.Time `bson:"occurred_at" json:"occurred_at"`
}

// ScoreAdjustment 成绩复核受理后的更正留痕（不可变，复核受理时写入答卷）。
type ScoreAdjustment struct {
	ReviewID        primitive.ObjectID `bson:"review_id" json:"review_id"`
	OperatorID      primitive.ObjectID `bson:"operator_id" json:"operator_id"`
	OperatorName    string             `bson:"operator_name" json:"operator_name"`
	OriginalScore   float64            `bson:"original_score" json:"original_score"`     // 更正前总分
	CorrectedScore  float64            `bson:"corrected_score" json:"corrected_score"`   // 更正后总分
	OriginalPassed  bool               `bson:"original_passed" json:"original_passed"`   // 更正前及格状态
	CorrectedPassed bool               `bson:"corrected_passed" json:"corrected_passed"` // 更正后及格状态
	PassedOverride  *bool              `bson:"passed_override,omitempty" json:"passed_override,omitempty"`
	Comment         string             `bson:"comment" json:"comment"` // 教师处理意见
	CreatedAt       time.Time          `bson:"created_at" json:"created_at"`
}

// ExamRecord 考试记录/答卷实体，集合 exam_records。
// 状态枚举：in_progress / submitted / graded；结果枚举：correct / wrong / partial / unmarked。
type ExamRecord struct {
	ID              primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ExamID          primitive.ObjectID `bson:"exam_id" json:"exam_id"`
	ExamTitle       string             `bson:"exam_title" json:"exam_title"`
	StudentID       primitive.ObjectID `bson:"student_id" json:"student_id"`
	StudentName     string             `bson:"student_name" json:"student_name"`
	Questions       []AttemptQuestion  `bson:"questions" json:"questions"`
	Status          string             `bson:"status" json:"status"`
	StartedAt       time.Time          `bson:"started_at" json:"started_at"`
	SubmittedAt     *time.Time         `bson:"submitted_at,omitempty" json:"submitted_at"`
	ObjectiveScore  float64            `bson:"objective_score" json:"objective_score"` // 客观题自动得分
	SubjectiveScore float64            `bson:"subjective_score" json:"subjective_score"`
	FinalScore      float64            `bson:"final_score" json:"final_score"`       // 最终总分（复核受理后为更正后的总分）
	PassScore       float64            `bson:"pass_score" json:"pass_score"`         // 批改时快照的及格分（冗余自试卷）
	GradedAt        *time.Time         `bson:"graded_at,omitempty" json:"graded_at"` // 首次批改完成时间（复核 48h 窗口锚点）
	// 复核更正信息：受理后写入，用于成绩页/批改页/成绩分析展示“最终分数”与留痕。
	// 原批改结果保留在 objective_score/subjective_score/questions 中不被覆盖。
	Adjustment    *ScoreAdjustment `bson:"adjustment,omitempty" json:"adjustment,omitempty"`
	CheatCount    int              `bson:"cheat_count" json:"cheat_count"`
	CheatEvents   []CheatEvent     `bson:"cheat_events,omitempty" json:"cheat_events"`
	AutoSubmitted bool             `bson:"auto_submitted" json:"auto_submitted"`
	CreatedAt     time.Time        `bson:"created_at" json:"created_at"`
	UpdatedAt     time.Time        `bson:"updated_at" json:"updated_at"`
}
