// Package middleware: APIKey 鉴权中间件（P7.9）。
//
// 静态 API Key 校验：客户端在 X-API-Key 头携带服务端预共享的 key。
// 用于内部 admin / 测试客户端；生产应配合 AAP / OAuth2 使用。
package middleware

import (
	"crypto/subtle"
	"net/http"
)

// APIKeyConfig API Key 配置。
type APIKeyConfig struct {
	// Keys 允许的 key 集合（任意一个匹配即通过）。
	Keys []string
	// Header 请求头名（默认 "X-API-Key"）。
	Header string
	// SkipPaths 跳过鉴权的路径（如 /live /healthz）。
	SkipPaths []string
	// Realm WWW-Authenticate realm（默认 "agentid"）。
	Realm string
}

// APIKey 返回 APIKey 中间件。
//
// 实现细节：用 crypto/subtle 做常量时间比较，避免时序侧信道。
func APIKey(cfg APIKeyConfig) HandlerFunc {
	if cfg.Header == "" {
		cfg.Header = "X-API-Key"
	}
	if cfg.Realm == "" {
		cfg.Realm = "agentid"
	}
	skip := make(map[string]bool, len(cfg.SkipPaths))
	for _, p := range cfg.SkipPaths {
		skip[p] = true
	}
	keys := make([][]byte, len(cfg.Keys))
	for i, k := range cfg.Keys {
		keys[i] = []byte(k)
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if skip[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}
			got := r.Header.Get(cfg.Header)
			if got == "" {
				unauthorized(w, cfg.Realm, "missing API key")
				return
			}
			matched := false
			for _, k := range keys {
				if subtle.ConstantTimeCompare([]byte(got), k) == 1 {
					matched = true
					break
				}
			}
			if !matched {
				unauthorized(w, cfg.Realm, "invalid API key")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func unauthorized(w http.ResponseWriter, realm, msg string) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="`+realm+`"`)
	http.Error(w, msg, http.StatusUnauthorized)
}
