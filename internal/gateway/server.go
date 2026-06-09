// Package gateway: HTTP server bootstrap (P7.1)。
//
// 设计：基于 net/http + stdlib mux，启用 connect-go 仅作 Interceptor 兼容层
// （middleware 风格统一）。后续若需要 gRPC 双模，可在此处挂 connect-go handler。
//
// 启动流程：
//  1. 构造 Server（注入 logger / 中间件链 / router / 业务 handler）
//  2. Listen :8080
//  3. Shutdown（graceful）
//
// 中间件顺序由 server 包内组装（按 docs §3.3 决策顺序），handler 内不感知。
package gateway

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/agentid-chain/agentid-chain/internal/gateway/middleware"
	"github.com/agentid-chain/agentid-chain/internal/gateway/router"
)

// ServerConfig 启动参数。
type ServerConfig struct {
	Addr            string        // 监听地址（默认 ":8080"）
	ReadTimeout     time.Duration // 读超时（默认 5s）
	WriteTimeout    time.Duration // 写超时（默认 30s）
	ShutdownTimeout time.Duration // 优雅关闭超时（默认 10s）
}

// Server L5 网关入口。
type Server struct {
	cfg    ServerConfig
	logger *slog.Logger
	mw     *middleware.Chain
	mux    *http.ServeMux
	http   *http.Server
}

// NewServer 构造 server。
//
// mux: 由 router.Router 注入（所有路由已注册）。
// mw:  由调用方构造中间件链（顺序由 server 包决定）。
func NewServer(cfg ServerConfig, logger *slog.Logger, mw *middleware.Chain, mux *http.ServeMux) *Server {
	if cfg.Addr == "" {
		cfg.Addr = ":8080"
	}
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = 5 * time.Second
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = 30 * time.Second
	}
	if cfg.ShutdownTimeout == 0 {
		cfg.ShutdownTimeout = 10 * time.Second
	}
	if logger == nil {
		logger = slog.Default()
	}
	if mw == nil {
		mw = middleware.NewChain()
	}
	if mux == nil {
		mux = http.NewServeMux()
	}
	return &Server{
		cfg:    cfg,
		logger: logger,
		mw:     mw,
		mux:    mux,
		http: &http.Server{
			Addr:              cfg.Addr,
			Handler:           mw.Wrap(mux),
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       cfg.ReadTimeout,
			WriteTimeout:      cfg.WriteTimeout,
		},
	}
}

// Mux 返回内部 mux（供 router.Router 注册路由）。
func (s *Server) Mux() *http.ServeMux { return s.mux }

// Start 启动 server（阻塞直到 ctx 取消或 server 失败）。
//
// 设计：
//   - ListenAndServe 在独立 goroutine 中跑
//   - 主 goroutine 阻塞在 ctx.Done()
//   - ctx 取消 → 触发 Shutdown（graceful）
func (s *Server) Start(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		s.logger.Info("http server listening", slog.String("addr", s.cfg.Addr))
		if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		s.logger.Info("shutdown signal received, draining...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.cfg.ShutdownTimeout)
		defer cancel()
		if err := s.http.Shutdown(shutdownCtx); err != nil {
			s.logger.Error("http shutdown", slog.String("err", err.Error()))
			return err
		}
		// 等待 ListenAndServe 退出
		<-errCh
		return nil
	}
}

// Handler 返回包装后的 http.Handler（含中间件链），便于测试用 httptest。
func (s *Server) Handler() http.Handler { return s.http.Handler }

// Router 注入的路由构造（router 包提供）。当前 import 用于编译期校验。
var _ = router.New
