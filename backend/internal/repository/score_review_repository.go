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

// ScoreReviewRepository 成绩复核仓储接口。
type ScoreReviewRepository interface {
	Create(ctx context.Context, r *model.ScoreReview) error
	FindByID(ctx context.Context, id primitive.ObjectID) (*model.ScoreReview, error)
	FindByRecordID(ctx context.Context, recordID primitive.ObjectID) (*model.ScoreReview, error)
	// FindByRecordIDs 批量按答卷查询复核（列表/报告拼装复核状态复用）。
	FindByRecordIDs(ctx context.Context, recordIDs []primitive.ObjectID) ([]*model.ScoreReview, error)
	List(ctx context.Context, filter bson.M, page, pageSize int64) ([]*model.ScoreReview, int64, error)
	ListAll(ctx context.Context, filter bson.M) ([]*model.ScoreReview, error)
	// ClaimPending 原子地把待处理申请置为处理中快照：仅 status=pending 可被认领，
	// 返回 ErrConflict 表示已有他人处理（并发越权/重复处理的数据库层兜底）。
	ClaimPending(ctx context.Context, id primitive.ObjectID, handlerID primitive.ObjectID, handlerName, comment, decision string, now time.Time) (*model.ScoreReview, error)
	// FinishApproved 受理成功后写入更正结果；FinishRejected 驳回定稿；Reopen 在更正落库失败时恢复为 pending。
	FinishApproved(ctx context.Context, id primitive.ObjectID, upd bson.M) error
	FinishRejected(ctx context.Context, id primitive.ObjectID) error
	Reopen(ctx context.Context, id primitive.ObjectID) error
}

// MongoScoreReviewRepository MongoDB 成绩复核仓储实现。
type MongoScoreReviewRepository struct {
	coll *mongo.Collection
}

// NewMongoScoreReviewRepository 构造成绩复核仓储。
func NewMongoScoreReviewRepository(db *mongo.Database) *MongoScoreReviewRepository {
	return &MongoScoreReviewRepository{coll: db.Collection("score_reviews")}
}

func (r *MongoScoreReviewRepository) Create(ctx context.Context, rv *model.ScoreReview) error {
	_, err := r.coll.InsertOne(ctx, rv)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return fmt.Errorf("create score review: %w", ErrConflict)
		}
		return fmt.Errorf("create score review: %w", err)
	}
	return nil
}

func (r *MongoScoreReviewRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*model.ScoreReview, error) {
	var rv model.ScoreReview
	if err := r.coll.FindOne(ctx, bson.M{"_id": id}).Decode(&rv); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, fmt.Errorf("find score review by id: %w", ErrNotFound)
		}
		return nil, fmt.Errorf("find score review by id: %w", err)
	}
	return &rv, nil
}

func (r *MongoScoreReviewRepository) FindByRecordID(ctx context.Context, recordID primitive.ObjectID) (*model.ScoreReview, error) {
	var rv model.ScoreReview
	if err := r.coll.FindOne(ctx, bson.M{"record_id": recordID}).Decode(&rv); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, fmt.Errorf("find score review by record: %w", ErrNotFound)
		}
		return nil, fmt.Errorf("find score review by record: %w", err)
	}
	return &rv, nil
}

func (r *MongoScoreReviewRepository) FindByRecordIDs(ctx context.Context, recordIDs []primitive.ObjectID) ([]*model.ScoreReview, error) {
	if len(recordIDs) == 0 {
		return nil, nil
	}
	cur, err := r.coll.Find(ctx, bson.M{"record_id": bson.M{"$in": recordIDs}})
	if err != nil {
		return nil, fmt.Errorf("find score reviews by records: %w", err)
	}
	defer func() { _ = cur.Close(ctx) }()
	var out []*model.ScoreReview
	if err := cur.All(ctx, &out); err != nil {
		return nil, fmt.Errorf("decode score reviews by records: %w", err)
	}
	return out, nil
}

func (r *MongoScoreReviewRepository) List(ctx context.Context, filter bson.M, page, pageSize int64) ([]*model.ScoreReview, int64, error) {
	total, err := r.coll.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("count score reviews: %w", err)
	}
	opts := options.Find().
		SetSkip((page - 1) * pageSize).
		SetLimit(pageSize).
		SetSort(bson.M{"created_at": -1})
	cur, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("list score reviews: %w", err)
	}
	defer func() { _ = cur.Close(ctx) }()
	var list []*model.ScoreReview
	if err := cur.All(ctx, &list); err != nil {
		return nil, 0, fmt.Errorf("decode score reviews: %w", err)
	}
	return list, total, nil
}

func (r *MongoScoreReviewRepository) ListAll(ctx context.Context, filter bson.M) ([]*model.ScoreReview, error) {
	cur, err := r.coll.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("list all score reviews: %w", err)
	}
	defer func() { _ = cur.Close(ctx) }()
	var list []*model.ScoreReview
	if err := cur.All(ctx, &list); err != nil {
		return nil, fmt.Errorf("decode all score reviews: %w", err)
	}
	return list, nil
}

// ClaimPending 使用条件更新原子认领待处理申请，避免并发重复/越权处理。
func (r *MongoScoreReviewRepository) ClaimPending(ctx context.Context, id, handlerID primitive.ObjectID, handlerName, comment, decision string, now time.Time) (*model.ScoreReview, error) {
	filter := bson.M{"_id": id, "status": modelReviewStatusPending()}
	update := bson.M{"$set": bson.M{
		"handled_by":   handlerID,
		"handler_name": handlerName,
		"decision":     decision,
		"comment":      comment,
		"handled_at":   now,
		"updated_at":   now,
	}}
	res, err := r.coll.UpdateOne(ctx, filter, update)
	if err != nil {
		return nil, fmt.Errorf("claim pending score review: %w", err)
	}
	if res.MatchedCount == 0 {
		return nil, fmt.Errorf("claim pending score review: %w", ErrConflict)
	}
	return r.FindByID(ctx, id)
}

// FinishApproved 认领成功后，原子地把申请置为 approved 并写入更正结果。
func (r *MongoScoreReviewRepository) FinishApproved(ctx context.Context, id primitive.ObjectID, upd bson.M) error {
	set := bson.M{"status": "approved"}
	for k, v := range upd {
		set[k] = v
	}
	res, err := r.coll.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": set})
	if err != nil {
		return fmt.Errorf("finish approved score review: %w", err)
	}
	if res.MatchedCount == 0 {
		return fmt.Errorf("finish approved score review: %w", ErrNotFound)
	}
	return nil
}

// FinishRejected 认领后把申请置为 rejected（意见在认领时已写入）。
func (r *MongoScoreReviewRepository) FinishRejected(ctx context.Context, id primitive.ObjectID) error {
	res, err := r.coll.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"status": "rejected"}})
	if err != nil {
		return fmt.Errorf("finish rejected score review: %w", err)
	}
	if res.MatchedCount == 0 {
		return fmt.Errorf("finish rejected score review: %w", ErrNotFound)
	}
	return nil
}

// Reopen 受理更正答卷失败时回滚：清空处理留痕并恢复为 pending，释放给后续处理。
func (r *MongoScoreReviewRepository) Reopen(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.coll.UpdateOne(ctx, bson.M{"_id": id}, bson.M{
		"$set": bson.M{"status": "pending"},
		"$unset": bson.M{
			"handled_by": "", "handler_name": "", "decision": "",
			"comment": "", "handled_at": "",
		},
	})
	if err != nil {
		return fmt.Errorf("reopen score review: %w", err)
	}
	return nil
}

// modelReviewStatusPending 避免 repository 直接依赖 constants（与 exam_record_repository 同样的硬编码约定）。
func modelReviewStatusPending() string {
	return "pending"
}
