package repository

import (
	"context"
	"errors"
	"fmt"

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
	FindByRecord(ctx context.Context, recordID primitive.ObjectID) (*model.ScoreReview, error)
	List(ctx context.Context, filter bson.M, page, pageSize int64) ([]*model.ScoreReview, int64, error)
	ListAll(ctx context.Context, filter bson.M) ([]*model.ScoreReview, error)
	// ApplyPendingResult 原子地把指定复核单从 pending 迁移到目标状态并写入处理结果。
	// 通过条件过滤（_id + status=pending）保证并发处理/重复提交只有一方成功：
	// MatchedCount==0 时返回 ErrConflict（状态已被改动）。
	ApplyPendingResult(ctx context.Context, id primitive.ObjectID, update bson.M) (bool, error)
	// DeleteByID 回滚用：申请写回答卷失败时删除已插入的复核单。
	DeleteByID(ctx context.Context, id primitive.ObjectID) error
}

// MongoScoreReviewRepository MongoDB 成绩复核仓储实现。
type MongoScoreReviewRepository struct {
	coll *mongo.Collection
}

// NewMongoScoreReviewRepository 构造成绩复核仓储。
func NewMongoScoreReviewRepository(db *mongo.Database) *MongoScoreReviewRepository {
	return &MongoScoreReviewRepository{coll: db.Collection("score_reviews")}
}

func (r *MongoScoreReviewRepository) Create(ctx context.Context, rev *model.ScoreReview) error {
	_, err := r.coll.InsertOne(ctx, rev)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return fmt.Errorf("create score review: %w", ErrConflict)
		}
		return fmt.Errorf("create score review: %w", err)
	}
	return nil
}

func (r *MongoScoreReviewRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*model.ScoreReview, error) {
	var rev model.ScoreReview
	if err := r.coll.FindOne(ctx, bson.M{"_id": id}).Decode(&rev); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, fmt.Errorf("find score review by id: %w", ErrNotFound)
		}
		return nil, fmt.Errorf("find score review by id: %w", err)
	}
	return &rev, nil
}

func (r *MongoScoreReviewRepository) FindByRecord(ctx context.Context, recordID primitive.ObjectID) (*model.ScoreReview, error) {
	var rev model.ScoreReview
	if err := r.coll.FindOne(ctx, bson.M{"record_id": recordID}).Decode(&rev); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, fmt.Errorf("find score review by record: %w", ErrNotFound)
		}
		return nil, fmt.Errorf("find score review by record: %w", err)
	}
	return &rev, nil
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

func (r *MongoScoreReviewRepository) ApplyPendingResult(ctx context.Context, id primitive.ObjectID, update bson.M) (bool, error) {
	res, err := r.coll.UpdateOne(ctx,
		bson.M{"_id": id, "status": reviewStatusPending()},
		update,
	)
	if err != nil {
		return false, fmt.Errorf("apply score review result: %w", err)
	}
	return res.MatchedCount > 0, nil
}

func (r *MongoScoreReviewRepository) DeleteByID(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.coll.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("delete score review: %w", err)
	}
	return nil
}
