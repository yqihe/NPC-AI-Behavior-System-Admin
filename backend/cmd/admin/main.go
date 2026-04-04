package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/npc-admin/backend/internal/cache"
	"github.com/npc-admin/backend/internal/handler"
	"github.com/npc-admin/backend/internal/service"
	"github.com/npc-admin/backend/internal/store"
)

func main() {
	// 配置 slog
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))

	// 读取环境变量
	mongoURI := envOrDefault("MONGO_URI", "mongodb://localhost:27017")
	redisAddr := envOrDefault("REDIS_ADDR", "localhost:6379")
	listenAddr := envOrDefault("LISTEN_ADDR", ":9821")
	mongoDatabase := envOrDefault("MONGO_DATABASE", "npc_ai")

	ctx := context.Background()

	// 初始化 MongoDB
	mongoStore, err := store.NewMongoStore(ctx, mongoURI, mongoDatabase)
	if err != nil {
		slog.Error("main.mongo_init", "err", err)
		os.Exit(1)
	}

	// 初始化 Redis
	redisCache, err := cache.NewRedisCache(ctx, redisAddr)
	if err != nil {
		slog.Error("main.redis_init", "err", err)
		os.Exit(1)
	}

	// 创建 service
	eventTypeSvc := service.NewEventTypeService(mongoStore, redisCache)
	npcTypeSvc := service.NewNpcTypeService(mongoStore, redisCache)
	fsmConfigSvc := service.NewFsmConfigService(mongoStore, redisCache)
	btTreeSvc := service.NewBtTreeService(mongoStore, redisCache)

	// 创建 handler
	eventTypeH := handler.NewEventTypeHandler(eventTypeSvc)
	npcTypeH := handler.NewNpcTypeHandler(npcTypeSvc)
	fsmConfigH := handler.NewFsmConfigHandler(fsmConfigSvc)
	btTreeH := handler.NewBtTreeHandler(btTreeSvc)

	// 创建配置导出 handler（供游戏服务端拉取全量配置）
	configExportH := handler.NewConfigExportHandler(mongoStore)

	// 注册路由
	router := handler.NewRouter(eventTypeH, npcTypeH, fsmConfigH, btTreeH, configExportH)

	// 启动 HTTP server
	server := &http.Server{
		Addr:         listenAddr,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Graceful shutdown
	go func() {
		slog.Info("main.server_starting", "addr", listenAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("main.server_error", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("main.server_shutting_down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("main.shutdown_error", "err", err)
	}
	slog.Info("main.server_stopped")
}

func envOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
