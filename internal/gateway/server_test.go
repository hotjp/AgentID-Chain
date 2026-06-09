package gateway

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agentid-chain/agentid-chain/internal/gateway/middleware"
)

func newListener(addr string) (net.Listener, error) {
	return net.Listen("tcp", addr)
}

func newTestServer() *Server {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mux := http.NewServeMux()
	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("pong"))
	})
	chain := middleware.NewChain(
		middleware.Recover(logger),
		middleware.RequestID(),
	)
	return NewServer(ServerConfig{Addr: ":0"}, logger, chain, mux)
}

func TestServer_Handler(t *testing.T) {
	s := newTestServer()
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/ping", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d", rec.Code)
	}
	if rec.Body.String() != "pong" {
		t.Errorf("body = %q", rec.Body.String())
	}
}

func TestServer_StartShutdown(t *testing.T) {
	s := newTestServer()
	// 用随机端口
	ln, err := newListener("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	s.cfg.Addr = addr

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- s.Start(ctx) }()
	// 等待 server up
	time.Sleep(100 * time.Millisecond)
	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("start err = %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Error("shutdown timeout")
	}
}

func TestServer_DefaultConfig(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s := NewServer(ServerConfig{}, logger, nil, nil)
	if s.cfg.Addr != ":8080" {
		t.Errorf("default addr = %q", s.cfg.Addr)
	}
	if s.cfg.ReadTimeout == 0 {
		t.Error("default read timeout = 0")
	}
	if s.cfg.WriteTimeout == 0 {
		t.Error("default write timeout = 0")
	}
	if s.cfg.ShutdownTimeout == 0 {
		t.Error("default shutdown timeout = 0")
	}
}

func TestServer_Mux(t *testing.T) {
	s := newTestServer()
	if s.Mux() == nil {
		t.Error("nil mux")
	}
}
