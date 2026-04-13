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
	"github.com/yqihe/npc-ai-admin/backend/internal/config"
	"github.com/yqihe/npc-ai-admin/backend/internal/router"
	"github.com/yqihe/npc-ai-admin/backend/internal/setup"
)

func main() {
	configPath := flag.String("config", "config.yaml", "配置文件路径")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})))

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("启动.加载配置失败", "error", err, "path", *configPath)
		os.Exit(1)
	}

	// 基础设施 + 分层初始化
	st, err := setup.NewStores(&cfg.MySQL)
	if err != nil {
		slog.Error("启动.连接MySQL失败", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	rc := setup.NewCaches(ctx, &cfg.Redis)
	cancel()

	mcCtx, mcCancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	mc, err := setup.NewMemCaches(mcCtx, st)
	mcCancel()
	if err != nil {
		slog.Error("启动.加载内存缓存失败", "error", err)
		os.Exit(1)
	}

	svc := setup.NewServices(st, rc, mc, cfg)
	h := setup.NewHandlers(st, svc, mc, cfg)

	// Router + Server
	r := gin.Default()
	router.Setup(r, h)

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
	if err := rc.Close(); err != nil {
		slog.Error("关闭.Redis连接失败", "error", err)
	}
	if err := st.Close(); err != nil {
		slog.Error("关闭.MySQL连接失败", "error", err)
	}

	slog.Info("关闭.完成")
}
