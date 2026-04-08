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
	"github.com/yqihe/npc-ai-admin/backend/internal/cache"
	"github.com/yqihe/npc-ai-admin/backend/internal/config"
	"github.com/yqihe/npc-ai-admin/backend/internal/handler"
	"github.com/yqihe/npc-ai-admin/backend/internal/router"
	"github.com/yqihe/npc-ai-admin/backend/internal/service"
	storemysql "github.com/yqihe/npc-ai-admin/backend/internal/store/mysql"
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

	// Store
	fieldStore := storemysql.NewFieldStore(db)
	fieldRefStore := storemysql.NewFieldRefStore(db)
	dictStore := storemysql.NewDictionaryStore(db)

	// Cache
	dictCache := cache.NewDictCache(dictStore)
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	if err := dictCache.Load(ctx); err != nil {
		slog.Error("启动.加载字典缓存失败", "error", err)
		os.Exit(1)
	}
	cancel()

	// Service
	fieldService := service.NewFieldService(fieldStore, fieldRefStore, dictCache, &cfg.Pagination)

	// Handler
	fieldHandler := handler.NewFieldHandler(fieldService, &cfg.Validation)
	dictHandler := handler.NewDictionaryHandler(dictCache)

	// Router
	r := gin.Default()
	router.Setup(r, fieldHandler, dictHandler)

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
	if err := db.Close(); err != nil {
		slog.Error("关闭.MySQL连接失败", "error", err)
	}

	slog.Info("关闭.完成")
}
