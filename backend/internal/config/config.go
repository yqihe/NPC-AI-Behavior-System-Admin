package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 全局配置
type Config struct {
	Server     ServerConfig     `yaml:"server"`
	MySQL      MySQLConfig      `yaml:"mysql"`
	Redis      RedisConfig      `yaml:"redis"`
	Pagination PaginationConfig `yaml:"pagination"`
	Validation ValidationConfig `yaml:"validation"`
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

// RedisConfig Redis 连接配置
type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

// PaginationConfig 分页配置
type PaginationConfig struct {
	DefaultPage     int `yaml:"default_page"`
	DefaultPageSize int `yaml:"default_page_size"`
	MaxPageSize     int `yaml:"max_page_size"`
}

// ValidationConfig 校验配置
type ValidationConfig struct {
	FieldNameMaxLength  int `yaml:"field_name_max_length"`
	FieldLabelMaxLength int `yaml:"field_label_max_length"`
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
