package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/yqihe/npc-ai-admin/backend/internal/cache"
	"github.com/yqihe/npc-ai-admin/backend/internal/handler"
	"github.com/yqihe/npc-ai-admin/backend/internal/service"
	"github.com/yqihe/npc-ai-admin/backend/internal/store/mysql"
)

func main() {
	// 日志
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})))

	// MySQL
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		dsn = "root:root@tcp(127.0.0.1:3306)/npc_ai_admin?charset=utf8mb4&parseTime=true&loc=Local"
	}

	db, err := sqlx.Connect("mysql", dsn)
	if err != nil {
		slog.Error("启动.连接MySQL失败", "error", err)
		os.Exit(1)
	}
	db.SetMaxOpenConns(50)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Store
	fieldStore := mysql.NewFieldStore(db)
	dictStore := mysql.NewDictionaryStore(db)

	// Cache: 启动时加载 dictionaries 到内存
	dictCache := cache.NewDictCache(dictStore)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := dictCache.Load(ctx); err != nil {
		slog.Error("启动.加载字典缓存失败", "error", err)
		os.Exit(1)
	}
	cancel()

	// Service
	fieldService := service.NewFieldService(fieldStore, dictCache)

	// Handler
	fieldHandler := handler.NewFieldHandler(fieldService)

	// Gin
	r := gin.Default()
	v1 := r.Group("/api/v1")
	fieldHandler.RegisterRoutes(v1)

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Server
	port := os.Getenv("PORT")
	if port == "" {
		port = "9821"
	}
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	// Graceful Shutdown
	go func() {
		slog.Info("启动.HTTP服务", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("启动.HTTP服务失败", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("关闭.开始优雅关闭")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("关闭.HTTP服务失败", "error", err)
	}
	if err := db.Close(); err != nil {
		slog.Error("关闭.MySQL连接失败", "error", err)
	}

	slog.Info("关闭.完成")
}
