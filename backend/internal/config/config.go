package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 全局配置
type Config struct {
	Server          ServerConfig          `yaml:"server"`
	MySQL           MySQLConfig           `yaml:"mysql"`
	Redis           RedisConfig           `yaml:"redis"`
	Pagination      PaginationConfig      `yaml:"pagination"`
	Validation      ValidationConfig      `yaml:"validation"`
	EventType       EventTypeConfig       `yaml:"event_type"`
	EventTypeSchema EventTypeSchemaConfig `yaml:"event_type_schema"`
	FsmConfig       FsmConfigConfig       `yaml:"fsm_config"`
	FsmStateDict    FsmStateDictConfig    `yaml:"fsm_state_dict"`
	BtTree          BtTreeConfig          `yaml:"bt_tree"`
	BtNodeType      BtNodeTypeConfig      `yaml:"bt_node_type"`
}

// ServerConfig HTTP 服务配置
type ServerConfig struct {
	Port            string        `yaml:"port"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}

// MySQLConfig MySQL 连接配置
type MySQLConfig struct {
	DSN             string        `yaml:"dsn"`
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
}

// LogValue 实现 slog.LogValuer，日志输出时脱敏 DSN 密码段。
// DSN 格式 user:password@tcp(...)/db，password 部分替换为 ***。
func (c MySQLConfig) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("dsn", maskDSN(c.DSN)),
		slog.Int("max_open_conns", c.MaxOpenConns),
		slog.Int("max_idle_conns", c.MaxIdleConns),
		slog.Duration("conn_max_lifetime", c.ConnMaxLifetime),
	)
}

// RedisConfig Redis 连接配置
type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

// LogValue 实现 slog.LogValuer，日志输出时脱敏 Password。
func (c RedisConfig) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("addr", c.Addr),
		slog.String("password", maskSecret(c.Password)),
		slog.Int("db", c.DB),
	)
}

// maskDSN 脱敏 MySQL DSN 的密码段。原格式 user:password@tcp(host:port)/db → user:***@tcp(host:port)/db
func maskDSN(dsn string) string {
	atIdx := strings.LastIndex(dsn, "@")
	if atIdx <= 0 {
		return dsn
	}
	colonIdx := strings.Index(dsn[:atIdx], ":")
	if colonIdx <= 0 {
		return dsn
	}
	return dsn[:colonIdx+1] + "***" + dsn[atIdx:]
}

// maskSecret 空串返回空串，非空统一返回 ***。
func maskSecret(s string) string {
	if s == "" {
		return ""
	}
	return "***"
}

// PaginationConfig 分页配置
type PaginationConfig struct {
	DefaultPage     int `yaml:"default_page"`
	DefaultPageSize int `yaml:"default_page_size"`
	MaxPageSize     int `yaml:"max_page_size"`
}

// ValidationConfig 校验配置
type ValidationConfig struct {
	FieldNameMaxLength     int `yaml:"field_name_max_length"`
	FieldLabelMaxLength    int `yaml:"field_label_max_length"`
	TemplateNameMaxLength  int `yaml:"template_name_max_length"`
	DescriptionMaxLength   int `yaml:"description_max_length"`
}

// EventTypeConfig 事件类型配置
type EventTypeConfig struct {
	NameMaxLength        int           `yaml:"name_max_length"`
	DisplayNameMaxLength int           `yaml:"display_name_max_length"`
	CacheDetailTTL       time.Duration `yaml:"cache_detail_ttl"`
	CacheListTTL         time.Duration `yaml:"cache_list_ttl"`
	CacheLockTTL         time.Duration `yaml:"cache_lock_ttl"`
}

// EventTypeSchemaConfig 事件类型扩展字段 Schema 配置
type EventTypeSchemaConfig struct {
	FieldNameMaxLength  int `yaml:"field_name_max_length"`
	FieldLabelMaxLength int `yaml:"field_label_max_length"`
	MaxSchemas          int `yaml:"max_schemas"`
}

// FsmConfigConfig 状态机管理配置
type FsmConfigConfig struct {
	NameMaxLength        int           `yaml:"name_max_length"`
	DisplayNameMaxLength int           `yaml:"display_name_max_length"`
	MaxStates            int           `yaml:"max_states"`
	MaxTransitions       int           `yaml:"max_transitions"`
	ConditionMaxDepth    int           `yaml:"condition_max_depth"`
	CacheDetailTTL       time.Duration `yaml:"cache_detail_ttl"`
	CacheListTTL         time.Duration `yaml:"cache_list_ttl"`
	CacheLockTTL         time.Duration `yaml:"cache_lock_ttl"`
}

// FsmStateDictConfig 状态字典管理配置
type FsmStateDictConfig struct {
	NameMaxLength        int `yaml:"name_max_length"`
	DisplayNameMaxLength int `yaml:"display_name_max_length"`
	CategoryMaxLength    int `yaml:"category_max_length"`
	DescriptionMaxLength int `yaml:"description_max_length"`
}

// BtTreeConfig 行为树管理配置
type BtTreeConfig struct {
	NameMaxLength        int           `yaml:"name_max_length"`
	DisplayNameMaxLength int           `yaml:"display_name_max_length"`
	CacheDetailTTL       time.Duration `yaml:"cache_detail_ttl"`
	CacheListTTL         time.Duration `yaml:"cache_list_ttl"`
	CacheLockTTL         time.Duration `yaml:"cache_lock_ttl"`
}

// BtNodeTypeConfig 节点类型管理配置
type BtNodeTypeConfig struct {
	NameMaxLength  int           `yaml:"name_max_length"`
	LabelMaxLength int           `yaml:"label_max_length"`
	CacheDetailTTL time.Duration `yaml:"cache_detail_ttl"`
	CacheListTTL   time.Duration `yaml:"cache_list_ttl"`
	CacheLockTTL   time.Duration `yaml:"cache_lock_ttl"`
}

// Load 从文件加载配置
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// 环境变量覆盖（Docker 部署用）
	if v := os.Getenv("MYSQL_DSN"); v != "" {
		cfg.MySQL.DSN = v
	}
	if v := os.Getenv("REDIS_ADDR"); v != "" {
		cfg.Redis.Addr = v
	}
	if v := os.Getenv("REDIS_PASSWORD"); v != "" {
		cfg.Redis.Password = v
	}
	if v := os.Getenv("PORT"); v != "" {
		cfg.Server.Port = v
	}

	return cfg, nil
}
