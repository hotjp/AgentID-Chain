// Package router: 路由注册（P7.10）。
//
// 把所有业务 endpoint 注册到 *http.ServeMux。中间件由 gateway.Server 装配。
//
// 注册的路由：
//   /live, /readyz, /healthz        — 探活（P7.11）
//   /metrics                        — Prometheus 暴露（P7.12）
//   /debug/pprof/                   — pprof（P7.13）
//   /api/v2/agents/register         — 注册 agent（P7.14）
//   /api/v2/agents/{uuid}           — 查询 agent
//   /api/v2/agents/{uuid}/upgrade   — 升级 level
//   /api/v2/agents/{uuid}/check     — 权限校验
//   /api/v2/captcha/moltcaptcha/*   — 验证码
package router

import (
	"net/http"
	"net/http/pprof"

	"github.com/agentid-chain/agentid-chain/internal/gateway/handler"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Router 持有 mux + 业务 handler 引用。
type Router struct {
	mux    *http.ServeMux
	api    *handler.APIHandler
	health *handler.HealthHandler
}

// New 构造 router 并注册所有路由。
func New(api *handler.APIHandler, health *handler.HealthHandler) *Router {
	mux := http.NewServeMux()
	r := &Router{mux: mux, api: api, health: health}
	r.register()
	return r
}

// Mux 返回底层 mux。
func (r *Router) Mux() *http.ServeMux { return r.mux }

// register 一次性注册所有路由。
func (r *Router) register() {
	// ----- 探活 -----
	r.mux.HandleFunc("/live", r.health.Live)
	r.mux.HandleFunc("/readyz", r.health.Ready)
	r.mux.HandleFunc("/ready", r.health.Ready)
	r.mux.HandleFunc("/healthz", r.health.Healthz)

	// ----- Prometheus -----
	r.mux.Handle("/metrics", promhttp.Handler())

	// ----- pprof -----
	r.mux.HandleFunc("/debug/pprof/", pprof.Index)
	r.mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	r.mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	r.mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	r.mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	// ----- API v2 -----
	if r.api != nil {
		r.mux.HandleFunc("/api/v2/agents/register", r.api.Register)
		r.mux.HandleFunc("/api/v2/agents/", r.api.AgentByPath)
		if r.api.CaptchaChallenge != nil {
			r.mux.HandleFunc("/api/v2/captcha/moltcaptcha/challenge", r.api.CaptchaChallenge)
		}
		if r.api.CaptchaVerify != nil {
			r.mux.HandleFunc("/api/v2/captcha/moltcaptcha/verify", r.api.CaptchaVerify)
		}
	}
}
