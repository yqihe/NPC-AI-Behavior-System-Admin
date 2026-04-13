package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	goredis "github.com/redis/go-redis/v9"
	"github.com/yqihe/npc-ai-admin/backend/internal/cache"
	"github.com/yqihe/npc-ai-admin/backend/internal/config"
	"github.com/yqihe/npc-ai-admin/backend/internal/handler"
	"github.com/yqihe/npc-ai-admin/backend/internal/router"
	"github.com/yqihe/npc-ai-admin/backend/internal/service"
	storemysql "github.com/yqihe/npc-ai-admin/backend/internal/store/mysql"
	storeredis "github.com/yqihe/npc-ai-admin/backend/internal/store/redis"
)

func main() {
	configPath := flag.String("config", "config.yaml", "配置文件路径")
	flag.Parse()

	// 日志
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})))

	// 配置
	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("启动.加载配置失败", "error", err, "path", *configPath)
		os.Exit(1)
	}

	// MySQL
	db, err := sqlx.Connect("mysql", cfg.MySQL.DSN)
	if err != nil {
		slog.Error("启动.连接MySQL失败", "error", err)
		os.Exit(1)
	}
	db.SetMaxOpenConns(cfg.MySQL.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MySQL.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.MySQL.ConnMaxLifetime)

	// Redis
	rdb := goredis.NewClient(&goredis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	if err := rdb.Ping(ctx).Err(); err != nil {
		slog.Warn("启动.Redis连接失败，缓存将降级", "error", err)
	} else {
		slog.Info("启动.Redis连接成功", "addr", cfg.Redis.Addr)
	}
	cancel()

	// Store
	fieldStore := storemysql.NewFieldStore(db)
	fieldRefStore := storemysql.NewFieldRefStore(db)
	dictStore := storemysql.NewDictionaryStore(db)
	templateStore := storemysql.NewTemplateStore(db)
	eventTypeStore := storemysql.NewEventTypeStore(db)
	eventTypeSchemaStore := storemysql.NewEventTypeSchemaStore(db)
	fieldCache := storeredis.NewFieldCache(rdb)
	templateCache := storeredis.NewTemplateCache(rdb)
	eventTypeCache := storeredis.NewEventTypeCache(rdb)
	fsmConfigStore := storemysql.NewFsmConfigStore(db)
	fsmConfigCache := storeredis.NewFsmConfigCache(rdb)

	// DictCache（内存缓存，启动时从 MySQL 加载）
	dictCache := cache.NewDictCache(dictStore)
	ctx2, cancel2 := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	if err := dictCache.Load(ctx2); err != nil {
		slog.Error("启动.加载字典缓存失败", "error", err)
		os.Exit(1)
	}
	cancel2()

	// EventTypeSchemaCache（内存缓存，启动时从 MySQL 加载）
	eventTypeSchemaCache := cache.NewEventTypeSchemaCache(eventTypeSchemaStore)
	ctx3, cancel3 := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	if err := eventTypeSchemaCache.Load(ctx3); err != nil {
		slog.Error("启动.加载事件类型Schema缓存失败", "error", err)
		os.Exit(1)
	}
	cancel3()

	// Service（按"分层职责"硬规则：service 间无横向依赖）
	fieldService := service.NewFieldService(fieldStore, fieldRefStore, fieldCache, dictCache, &cfg.Pagination)
	templateService := service.NewTemplateService(templateStore, templateCache, &cfg.Pagination)
	eventTypeService := service.NewEventTypeService(eventTypeStore, eventTypeCache, eventTypeSchemaCache, &cfg.Pagination, &cfg.EventType)
	eventTypeSchemaService := service.NewEventTypeSchemaService(eventTypeSchemaStore, eventTypeSchemaCache, &cfg.EventTypeSchema)
	fsmConfigService := service.NewFsmConfigService(fsmConfigStore, fsmConfigCache, &cfg.Pagination, &cfg.FsmConfig)

	// Handler（跨模块编排在 handler 层）
	fieldHandler := handler.NewFieldHandler(fieldService, templateService, &cfg.Validation)
	templateHandler := handler.NewTemplateHandler(db, templateService, fieldService, &cfg.Validation)
	dictHandler := handler.NewDictionaryHandler(dictCache)
	eventTypeHandler := handler.NewEventTypeHandler(eventTypeService, eventTypeSchemaService, &cfg.EventType)
	eventTypeSchemaHandler := handler.NewEventTypeSchemaHandler(eventTypeSchemaService, &cfg.EventTypeSchema)
	fsmConfigHandler := handler.NewFsmConfigHandler(fsmConfigService, &cfg.FsmConfig)
	exportHandler := handler.NewExportHandler(eventTypeService, fsmConfigService)

	// Router
	r := gin.Default()
	router.Setup(r, fieldHandler, dictHandler, templateHandler, eventTypeHandler, eventTypeSchemaHandler, fsmConfigHandler, exportHandler)

	// Server
	srv := &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: r,
	}

	go func() {
		slog.Info("启动.HTTP服务", "port", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("启动.HTTP服务失败", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("关闭.开始优雅关闭")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("关闭.HTTP服务失败", "error", err)
	}
	if err := rdb.Close(); err != nil {
		slog.Error("关闭.Redis连接失败", "error", err)
	}
	if err := db.Close(); err != nil {
		slog.Error("关闭.MySQL连接失败", "error", err)
	}

	slog.Info("关闭.完成")
}
