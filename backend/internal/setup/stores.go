package setup

import (
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/yqihe/npc-ai-admin/backend/internal/config"
	storemysql "github.com/yqihe/npc-ai-admin/backend/internal/store/mysql"
)

// Stores 聚合 MySQL 连接 + 所有 store
type Stores struct {
	DB              *sqlx.DB
	Field           *storemysql.FieldStore
	FieldRef        *storemysql.FieldRefStore
	Dict            *storemysql.DictionaryStore
	Template        *storemysql.TemplateStore
	EventType       *storemysql.EventTypeStore
	EventTypeSchema *storemysql.EventTypeSchemaStore
	SchemaRef       *storemysql.SchemaRefStore
	FsmConfig       *storemysql.FsmConfigStore
	FsmStateDict    *storemysql.FsmStateDictStore
	BtTree          *storemysql.BtTreeStore
	BtNodeType      *storemysql.BtNodeTypeStore
}

// NewStores 连接 MySQL + 一次性初始化所有 store
func NewStores(cfg *config.MySQLConfig) (*Stores, error) {
	db, err := sqlx.Connect("mysql", cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("connect mysql: %w", err)
	}
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	return &Stores{
		DB:              db,
		Field:           storemysql.NewFieldStore(db),
		FieldRef:        storemysql.NewFieldRefStore(db),
		Dict:            storemysql.NewDictionaryStore(db),
		Template:        storemysql.NewTemplateStore(db),
		EventType:       storemysql.NewEventTypeStore(db),
		EventTypeSchema: storemysql.NewEventTypeSchemaStore(db),
		SchemaRef:       storemysql.NewSchemaRefStore(db),
		FsmConfig:       storemysql.NewFsmConfigStore(db),
		FsmStateDict:    storemysql.NewFsmStateDictStore(db),
		BtTree:          storemysql.NewBtTreeStore(db),
		BtNodeType:      storemysql.NewBtNodeTypeStore(db),
	}, nil
}

// Close 关闭 MySQL 连接
func (s *Stores) Close() error { return s.DB.Close() }
