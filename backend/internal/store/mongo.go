package store

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/npc-admin/backend/internal/model"
)

const dbTimeout = 5 * time.Second

// Collections 是运营平台管理的 4 个 collection 名称。
var Collections = []string{"event_types", "npc_types", "fsm_configs", "bt_trees"}

// MongoStore 实现 Store 接口，操作 MongoDB。
type MongoStore struct {
	client *mongo.Client
	db     *mongo.Database
}

// NewMongoStore 连接 MongoDB 并确保 4 个 collection 的 name unique index。
func NewMongoStore(ctx context.Context, uri string, database string) (*MongoStore, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("store.mongo_connect: %w", err)
	}

	// 验证连接
	pingCtx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()
	if err := client.Ping(pingCtx, nil); err != nil {
		return nil, fmt.Errorf("store.mongo_ping: %w", err)
	}

	db := client.Database(database)
	s := &MongoStore{client: client, db: db}

	// 确保 unique index
	if err := s.ensureIndexes(ctx); err != nil {
		return nil, fmt.Errorf("store.ensure_indexes: %w", err)
	}

	slog.Info("store.mongo_connected", "uri", uri, "database", database)
	return s, nil
}

// Close 断开 MongoDB 连接，用于优雅关闭。
func (s *MongoStore) Close(ctx context.Context) error {
	return s.client.Disconnect(ctx)
}

func (s *MongoStore) ensureIndexes(ctx context.Context) error {
	for _, coll := range Collections {
		indexCtx, cancel := context.WithTimeout(ctx, dbTimeout)
		indexModel := mongo.IndexModel{
			Keys:    bson.D{{Key: "name", Value: 1}},
			Options: options.Index().SetUnique(true),
		}
		_, err := s.db.Collection(coll).Indexes().CreateOne(indexCtx, indexModel)
		cancel()
		if err != nil {
			return fmt.Errorf("collection %s: %w", coll, err)
		}
		slog.Info("store.index_ensured", "collection", coll)
	}
	return nil
}

func (s *MongoStore) List(ctx context.Context, collection string) ([]model.Document, error) {
	opCtx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	// 只返回 name 和 config，排除 _id
	projection := bson.M{"_id": 0, "name": 1, "config": 1}
	cursor, err := s.db.Collection(collection).Find(opCtx, bson.M{}, options.Find().SetProjection(projection))
	if err != nil {
		slog.Error("store.list_error", "collection", collection, "err", err)
		return nil, fmt.Errorf("store.list: %w", err)
	}
	defer cursor.Close(context.Background())

	var bsonDocs []model.BsonDocument
	if err := cursor.All(opCtx, &bsonDocs); err != nil {
		slog.Error("store.list_decode", "collection", collection, "err", err)
		return nil, fmt.Errorf("store.list_decode: %w", err)
	}

	// 转换为 Document，保证返回空 slice 不是 nil
	docs := make([]model.Document, 0, len(bsonDocs))
	for _, bdoc := range bsonDocs {
		doc, err := model.FromBsonDocument(bdoc)
		if err != nil {
			slog.Error("store.list_convert", "collection", collection, "name", bdoc.Name, "err", err)
			return nil, fmt.Errorf("store.list_convert %s: %w", bdoc.Name, err)
		}
		docs = append(docs, doc)
	}

	slog.Debug("store.list", "collection", collection, "count", len(docs))
	return docs, nil
}

func (s *MongoStore) Get(ctx context.Context, collection string, name string) (model.Document, error) {
	opCtx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	projection := bson.M{"_id": 0, "name": 1, "config": 1}
	var bdoc model.BsonDocument
	err := s.db.Collection(collection).FindOne(opCtx, bson.M{"name": name}, options.FindOne().SetProjection(projection)).Decode(&bdoc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return model.Document{}, ErrNotFound
		}
		slog.Error("store.get_error", "collection", collection, "name", name, "err", err)
		return model.Document{}, fmt.Errorf("store.get: %w", err)
	}

	doc, err := model.FromBsonDocument(bdoc)
	if err != nil {
		slog.Error("store.get_convert", "collection", collection, "name", name, "err", err)
		return model.Document{}, fmt.Errorf("store.get_convert: %w", err)
	}

	slog.Debug("store.get", "collection", collection, "name", name)
	return doc, nil
}

func (s *MongoStore) Create(ctx context.Context, collection string, doc model.Document) error {
	opCtx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	bdoc, err := model.ToBsonDocument(doc)
	if err != nil {
		return fmt.Errorf("store.create_convert: %w", err)
	}

	_, err = s.db.Collection(collection).InsertOne(opCtx, bdoc)
	if err != nil {
		if isDuplicateKeyError(err) {
			return ErrDuplicate
		}
		slog.Error("store.create_error", "collection", collection, "name", doc.Name, "err", err)
		return fmt.Errorf("store.create: %w", err)
	}

	slog.Info("store.created", "collection", collection, "name", doc.Name)
	return nil
}

func (s *MongoStore) Update(ctx context.Context, collection string, name string, doc model.Document) error {
	opCtx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	bdoc, err := model.ToBsonDocument(doc)
	if err != nil {
		return fmt.Errorf("store.update_convert: %w", err)
	}

	result, err := s.db.Collection(collection).ReplaceOne(opCtx, bson.M{"name": name}, bdoc)
	if err != nil {
		if isDuplicateKeyError(err) {
			return ErrDuplicate
		}
		slog.Error("store.update_error", "collection", collection, "name", name, "err", err)
		return fmt.Errorf("store.update: %w", err)
	}
	if result.MatchedCount == 0 {
		return ErrNotFound
	}

	slog.Info("store.updated", "collection", collection, "name", name)
	return nil
}

func (s *MongoStore) Delete(ctx context.Context, collection string, name string) error {
	opCtx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	result, err := s.db.Collection(collection).DeleteOne(opCtx, bson.M{"name": name})
	if err != nil {
		slog.Error("store.delete_error", "collection", collection, "name", name, "err", err)
		return fmt.Errorf("store.delete: %w", err)
	}
	if result.DeletedCount == 0 {
		return ErrNotFound
	}

	slog.Info("store.deleted", "collection", collection, "name", name)
	return nil
}

// isDuplicateKeyError 检查是否为 MongoDB duplicate key error (code 11000)。
func isDuplicateKeyError(err error) bool {
	// mongo-driver v2 的 WriteException 包含 WriteErrors
	var we mongo.WriteException
	if ok := errors.As(err, &we); ok {
		for _, we := range we.WriteErrors {
			if we.Code == 11000 {
				return true
			}
		}
	}
	// 兜底：错误信息中包含 duplicate key（防止 driver 版本差异）
	return strings.Contains(err.Error(), "duplicate key")
}
