// Package handler: 探活与 pprof 处理器（P7.11, P7.13）。
package handler

import (
	"net/http"
)

// HealthHandler 探活处理。
type HealthHandler struct {
	// ReadyFn 返回当前是否 ready（依赖 L4/L1 health check）。
	// nil = 永远 ready。
	ReadyFn func() error
}

// NewHealthHandler 构造。
func NewHealthHandler(readyFn func() error) *HealthHandler {
	return &HealthHandler{ReadyFn: readyFn}
}

// Live 存活探针：永远 200。
func (h *HealthHandler) Live(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// Ready 就绪探针：ReadyFn nil → 200；否则根据 fn 决定。
func (h *HealthHandler) Ready(w http.ResponseWriter, _ *http.Request) {
	if h.ReadyFn == nil {
		h.Live(w, nil)
		return
	}
	if err := h.ReadyFn(); err != nil {
		http.Error(w, "not ready: "+err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ready"))
}

// Healthz 综合健康检查（区别于 ready：可包含更详细的 deps 状态）。
func (h *HealthHandler) Healthz(w http.ResponseWriter, _ *http.Request) {
	if h.ReadyFn == nil {
		h.Live(w, nil)
		return
	}
	if err := h.ReadyFn(); err != nil {
		http.Error(w, "unhealthy: "+err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("healthy"))
}
