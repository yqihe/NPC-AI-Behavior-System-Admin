package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const dbTimeout = 5 * time.Second

// importMapping 定义一个导入映射：源子目录 → MongoDB 集合名。
type importMapping struct {
	subDir     string // schemas 目录下的子目录（如 "components"）
	collection string // MongoDB 集合名
}

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	schemasDir := flag.String("schemas-dir", "", "服务端 schema 文件目录路径（如 ../../NPC-AI-Behavior-System-Server/configs/schemas）")
	mongoURI := flag.String("mongo-uri", envOrDefault("MONGO_URI", "mongodb://localhost:27017"), "MongoDB 连接地址")
	mongoDatabase := flag.String("mongo-database", envOrDefault("MONGO_DATABASE", "npc_ai"), "MongoDB 数据库名")
	flag.Parse()

	if *schemasDir == "" {
		fmt.Fprintln(os.Stderr, "错误：必须指定 --schemas-dir 参数")
		flag.Usage()
		os.Exit(1)
	}

	ctx := context.Background()

	// 连接 MongoDB
	client, err := mongo.Connect(options.Client().ApplyURI(*mongoURI))
	if err != nil {
		slog.Error("seed.mongo_connect", "err", err)
		os.Exit(1)
	}
	defer client.Disconnect(context.Background())

	pingCtx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()
	if err := client.Ping(pingCtx, nil); err != nil {
		slog.Error("seed.mongo_ping", "err", err)
		os.Exit(1)
	}

	db := client.Database(*mongoDatabase)
	slog.Info("seed.connected", "database", *mongoDatabase)

	// 导入目录映射
	mappings := []importMapping{
		{subDir: "components", collection: "component_schemas"},
		{subDir: "presets", collection: "npc_presets"},
		{subDir: "node_types", collection: "node_type_schemas"},
		{subDir: "condition_types", collection: "condition_type_schemas"},
	}

	totalImported := 0

	for _, m := range mappings {
		dir := filepath.Join(*schemasDir, m.subDir)
		count, err := importDirectory(ctx, db, dir, m.collection)
		if err != nil {
			slog.Error("seed.import_failed", "dir", m.subDir, "collection", m.collection, "err", err)
			os.Exit(1)
		}
		totalImported += count
	}

	// 导入独立 schema 文件（单个文件 → component_schemas，name 带下划线前缀）
	standaloneSchemas := []struct {
		file string
		name string
	}{
		{"region.json", "_region"},
		{"event_type.json", "_event_type"},
	}
	for _, ss := range standaloneSchemas {
		filePath := filepath.Join(*schemasDir, ss.file)
		if _, err := os.Stat(filePath); err == nil {
			if err := importFile(ctx, db, filePath, "component_schemas", ss.name); err != nil {
				slog.Error("seed.import_standalone_failed", "file", ss.file, "err", err)
				os.Exit(1)
			}
			totalImported++
			slog.Info("seed.imported", "name", ss.name, "collection", "component_schemas")
		}
	}

	slog.Info("seed.done", "total", totalImported)
}

// importDirectory 扫描目录下所有 .json 文件并 upsert 到指定集合。
func importDirectory(ctx context.Context, db *mongo.Database, dir string, collection string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Warn("seed.dir_not_found", "dir", dir)
			return 0, nil
		}
		return 0, fmt.Errorf("读取目录 %s: %w", dir, err)
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".json")
		filePath := filepath.Join(dir, entry.Name())

		if err := importFile(ctx, db, filePath, collection, name); err != nil {
			return count, fmt.Errorf("导入 %s: %w", filePath, err)
		}

		count++
		slog.Info("seed.imported", "name", name, "collection", collection)
	}

	slog.Info("seed.dir_done", "dir", dir, "collection", collection, "count", count)
	return count, nil
}

// importFile 读取单个 JSON 文件并 upsert 到 MongoDB。
// 文档格式：{name: <name>, config: <整个 JSON 内容>}
func importFile(ctx context.Context, db *mongo.Database, filePath string, collection string, name string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("读取文件: %w", err)
	}

	// 验证是合法 JSON
	var jsonObj any
	if err := json.Unmarshal(data, &jsonObj); err != nil {
		return fmt.Errorf("JSON 解析失败: %w", err)
	}

	// 将 JSON 转为 BSON raw（保持原始结构）
	var configRaw bson.Raw
	if err := bson.UnmarshalExtJSON(data, false, &configRaw); err != nil {
		return fmt.Errorf("JSON→BSON 转换失败: %w", err)
	}

	// 构造文档
	doc := bson.D{
		{Key: "name", Value: name},
		{Key: "config", Value: configRaw},
	}

	// Upsert（幂等）
	opCtx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	filter := bson.M{"name": name}
	_, err = db.Collection(collection).ReplaceOne(opCtx, filter, doc, options.Replace().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("MongoDB upsert: %w", err)
	}

	return nil
}

func envOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
