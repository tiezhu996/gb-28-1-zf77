package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/onlineexam/onlineexam/internal/model"
)

// ExamRecordRepository 考试记录仓储接口。
type ExamRecordRepository interface {
	Create(ctx context.Context, r *model.ExamRecord) error
	Update(ctx context.Context, r *model.ExamRecord) error
	FindByID(ctx context.Context, id primitive.ObjectID) (*model.ExamRecord, error)
	FindActiveByExamAndStudent(ctx context.Context, examID, studentID primitive.ObjectID) (*model.ExamRecord, error)
	List(ctx context.Context, filter bson.M, page, pageSize int64) ([]*model.ExamRecord, int64, error)
	ListAll(ctx context.Context, filter bson.M) ([]*model.ExamRecord, error)
	CountByExamAndStatus(ctx context.Context, examID primitive.ObjectID, statuses []string) (int64, error)

	// AttachPendingReview 原子地把待处理复核单挂到答卷上（仅当答卷尚无任何复核单）。
	// 条件过滤 review_status 不存在/为 none 杜绝并发重复申请；MatchedCount==0 返回 ErrConflict。
	AttachPendingReview(ctx context.Context, recordID, reviewID primitive.ObjectID) error
	// ApplyReviewResult 原子地写回复核处理结果（仅当答卷当前挂着该 pending 复核单）。
	// 审批（approved）时更新最终分、及格快照与更正标记；驳回（rejected）时只更新复核状态。
	ApplyReviewResult(ctx context.Context, recordID, reviewID primitive.ObjectID, reviewStatus string, correctedScore *float64) error
}

// reviewStatusNone / reviewStatusPending 与 constants 中复核枚举保持同步
// （repository 不直接依赖 constants，沿用 modelStatusInProgress 的硬编码约定）。
func reviewStatusNone() string    { return "none" }
func reviewStatusPending() string { return "pending" }

// MongoExamRecordRepository MongoDB 考试记录仓储实现。
type MongoExamRecordRepository struct {
	coll *mongo.Collection
}

// NewMongoExamRecordRepository 构造考试记录仓储。
func NewMongoExamRecordRepository(db *mongo.Database) *MongoExamRecordRepository {
	return &MongoExamRecordRepository{coll: db.Collection("exam_records")}
}

func (r *MongoExamRecordRepository) Create(ctx context.Context, rec *model.ExamRecord) error {
	_, err := r.coll.InsertOne(ctx, rec)
	if err != nil {
		return fmt.Errorf("create exam record: %w", err)
	}
	return nil
}

func (r *MongoExamRecordRepository) Update(ctx context.Context, rec *model.ExamRecord) error {
	res, err := r.coll.ReplaceOne(ctx, bson.M{"_id": rec.ID}, rec)
	if err != nil {
		return fmt.Errorf("update exam record: %w", err)
	}
	if res.MatchedCount == 0 {
		return fmt.Errorf("update exam record: %w", ErrNotFound)
	}
	return nil
}

func (r *MongoExamRecordRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*model.ExamRecord, error) {
	var rec model.ExamRecord
	if err := r.coll.FindOne(ctx, bson.M{"_id": id}).Decode(&rec); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, fmt.Errorf("find exam record by id: %w", ErrNotFound)
		}
		return nil, fmt.Errorf("find exam record by id: %w", err)
	}
	return &rec, nil
}

func (r *MongoExamRecordRepository) FindActiveByExamAndStudent(ctx context.Context, examID, studentID primitive.ObjectID) (*model.ExamRecord, error) {
	var rec model.ExamRecord
	err := r.coll.FindOne(ctx, bson.M{
		"exam_id":    examID,
		"student_id": studentID,
		"status":     modelStatusInProgress(),
	}).Decode(&rec)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, fmt.Errorf("find active exam record: %w", ErrNotFound)
		}
		return nil, fmt.Errorf("find active exam record: %w", err)
	}
	return &rec, nil
}

func (r *MongoExamRecordRepository) List(ctx context.Context, filter bson.M, page, pageSize int64) ([]*model.ExamRecord, int64, error) {
	total, err := r.coll.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("count exam records: %w", err)
	}
	opts := options.Find().
		SetSkip((page - 1) * pageSize).
		SetLimit(pageSize).
		SetSort(bson.M{"started_at": -1})
	cur, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("list exam records: %w", err)
	}
	defer func() { _ = cur.Close(ctx) }()
	var recs []*model.ExamRecord
	if err := cur.All(ctx, &recs); err != nil {
		return nil, 0, fmt.Errorf("decode exam records: %w", err)
	}
	return recs, total, nil
}

func (r *MongoExamRecordRepository) ListAll(ctx context.Context, filter bson.M) ([]*model.ExamRecord, error) {
	cur, err := r.coll.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("list all exam records: %w", err)
	}
	defer func() { _ = cur.Close(ctx) }()
	var recs []*model.ExamRecord
	if err := cur.All(ctx, &recs); err != nil {
		return nil, fmt.Errorf("decode all exam records: %w", err)
	}
	return recs, nil
}

func (r *MongoExamRecordRepository) CountByExamAndStatus(ctx context.Context, examID primitive.ObjectID, statuses []string) (int64, error) {
	filter := bson.M{"exam_id": examID}
	if len(statuses) > 0 {
		filter["status"] = bson.M{"$in": statuses}
	}
	count, err := r.coll.CountDocuments(ctx, filter)
	if err != nil {
		return 0, fmt.Errorf("count exam records by status: %w", err)
	}
	return count, nil
}

// modelStatusInProgress 避免 repository 直接依赖 constants 的循环，这里硬编码状态值。
// 注意：该状态枚举同时在 constants/enums.go、service 状态机、formatters、前端 constants 中出现。
func modelStatusInProgress() string {
	return "in_progress"
}

// AttachPendingReview 条件更新：仅当答卷不存在任何复核关联时挂上 pending 复核单。
// 使用 $or 兼容历史数据（review_status 字段缺失）与显式 none 两种形态。
func (r *MongoExamRecordRepository) AttachPendingReview(ctx context.Context, recordID, reviewID primitive.ObjectID) error {
	res, err := r.coll.UpdateOne(ctx,
		bson.M{
			"_id": recordID,
			"$or": bson.A{
				bson.M{"review_id": primitive.NilObjectID},
				bson.M{"review_id": bson.M{"$exists": false}},
				bson.M{"review_status": ""},
				bson.M{"review_status": reviewStatusNone()},
			},
		},
		bson.M{
			"$set": bson.M{
				"review_id":     reviewID,
				"review_status": reviewStatusPending(),
				"updated_at":    time.Now(),
			},
		},
	)
	if err != nil {
		return fmt.Errorf("attach pending review: %w", err)
	}
	if res.MatchedCount == 0 {
		return fmt.Errorf("attach pending review: %w", ErrConflict)
	}
	return nil
}

// ApplyReviewResult 条件更新：仅当答卷挂着同一个 pending 复核单时写回处理结果。
// 任何越权/重复处理（复核单不匹配或状态已迁移）都会 MatchedCount==0 → ErrConflict，
// 原成绩不会被改变。
func (r *MongoExamRecordRepository) ApplyReviewResult(ctx context.Context, recordID, reviewID primitive.ObjectID, reviewStatus string, correctedScore *float64) error {
	set := bson.M{
		"review_status": reviewStatus,
		"updated_at":    time.Now(),
	}
	if correctedScore != nil {
		set["final_score"] = *correctedScore
		set["score_corrected"] = true
	}
	res, err := r.coll.UpdateOne(ctx,
		bson.M{
			"_id":           recordID,
			"review_id":     reviewID,
			"review_status": reviewStatusPending(),
		},
		bson.M{"$set": set},
	)
	if err != nil {
		return fmt.Errorf("apply review result: %w", err)
	}
	if res.MatchedCount == 0 {
		return fmt.Errorf("apply review result: %w", ErrConflict)
	}
	return nil
}
