// Package main serve 子命令实现 + 依赖注入。
//
// serve 是真正的业务入口；按顺序完成：
//
//  1. config.Load       — 读配置（YAML + env + flag）
//  2. logger init       — slog JSON handler
//  3. telemetry.Init    — 启动 OTel + Prometheus
//  4. cache.NewRedis    — 连 Redis（健康检查）
//  5. storage/authz/service/gateway — P3-P5 接入
//  6. signal handling   — SIGINT/SIGTERM 优雅退出
//  7. telemetry.Shutdown
//
// 任何步骤失败立刻退出（fail-fast）；日志全部走 slog。
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/agentid-chain/agentid-chain/internal/cache"
	"github.com/agentid-chain/agentid-chain/internal/config"
	"github.com/agentid-chain/agentid-chain/internal/telemetry"
)

// runServe serveCmd 的 RunE 实现。
func runServe(_ /*cmd*/ interface{}, _ /*args*/ []string) error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// ---------- 1. config ----------
	cfgPath, _ := rootCmd.PersistentFlags().GetString("config")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// ---------- 2. logger ----------
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: parseLogLevel(cfg.Log.Level),
	}))
	slog.SetDefault(logger)
	logger.Info("starting",
		slog.String("service", cfg.Service.Name),
		slog.String("version", cfg.Service.Version),
		slog.String("role", cfg.Service.Role),
		slog.String("http_addr", cfg.Service.HTTPAddr),
	)

	// ---------- 3. telemetry ----------
	tel, err := telemetry.Init(ctx, telemetry.Config{
		ServiceName:  cfg.Service.Name,
		OTELEndpoint: cfg.Telemetry.OTELEndpoint,
		MetricsAddr:  cfg.Telemetry.MetricsAddr,
		PProfAddr:    cfg.Telemetry.PProfAddr,
		SampleRate:   cfg.Telemetry.SampleRate,
	})
	if err != nil {
		return fmt.Errorf("init telemetry: %w", err)
	}
	defer func() {
		shutdownCtx, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		if err := tel.Shutdown(shutdownCtx); err != nil {
			logger.Error("telemetry shutdown", slog.String("err", err.Error()))
		}
	}()
	logger.Info("telemetry ready",
		slog.String("metrics_addr", cfg.Telemetry.MetricsAddr),
		slog.String("otel_endpoint", cfg.Telemetry.OTELEndpoint),
	)

	// ---------- 4. cache ----------
	cch, err := cache.NewRedis(cache.RedisConfig{
		Addr:     cfg.Storage.Redis.Addr,
		Password: cfg.Storage.Redis.Password,
		DB:       cfg.Storage.Redis.DB,
		Timeout:  cfg.Storage.Redis.Timeout,
	})
	if err != nil {
		// 缓存不可用不致命（写入失败时降级到 DB 直查）
		logger.Warn("redis unavailable, running without cache", slog.String("err", err.Error()))
		cch = nil
	} else {
		defer cch.Close()
		logger.Info("redis ready", slog.String("addr", cfg.Storage.Redis.Addr))
	}

	// ---------- 5. pending services (P3-P5 占位) ----------
	return runPlaceholderHTTPServer(ctx, cfg, logger, cch)
}

// runPlaceholderHTTPServer 占位 HTTP server（仅 /live + /readyz）。
//
// 待 P5 gateway 实装后替换为 connect-go handler 注册流程。
func runPlaceholderHTTPServer(
	ctx context.Context,
	cfg *config.Config,
	logger *slog.Logger,
	_ cache.Cache,
) error {
	logger.Info("pending services (P2.12 stub)",
		slog.String("role", cfg.Service.Role),
		slog.String("pending",
			"storage (P3) / authz (P3) / service (P4) / gateway (P5); see LRA task_004_12"),
	)

	mux := http.NewServeMux()
	mux.HandleFunc("/live", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	srv := &http.Server{
		Addr:              cfg.Service.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// 异步 listen
	go func() {
		logger.Info("placeholder http server listening", slog.String("addr", cfg.Service.HTTPAddr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("placeholder server error", slog.String("err", err.Error()))
		}
	}()

	// 等待 signal
	<-ctx.Done()
	logger.Info("shutdown signal received, draining...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("http shutdown", slog.String("err", err.Error()))
	}
	return nil
}

func parseLogLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
