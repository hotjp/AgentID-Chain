// Package middleware: UA 拦截中间件（P7.7）。
//
// 拦截恶意 UA 模式：sqlmap、curl/7.0（无 UA 测试工具）、空 UA（API 客户端必须
// 携带 UA 头）。AllowList 模式：若配置了 AllowList，仅放行匹配的 UA。
package middleware

import (
	"net/http"
	"strings"
)

// UAConfig UA 拦截配置。
type UAConfig struct {
	// BlockPatterns 黑名单（子串匹配，不区分大小写）。
	BlockPatterns []string
	// AllowList 白名单（命中即放行；为空 = 不启用）。
	AllowList []string
	// BlockEmpty 拦截空 UA（默认 true）。
	BlockEmpty bool
}

// DefaultUAConfig 保守默认：拦截常见扫描器 + 空 UA。
func DefaultUAConfig() UAConfig {
	return UAConfig{
		BlockPatterns: []string{"sqlmap", "nikto", "masscan", "nmap", "zgrab", "nessus"},
		BlockEmpty:    true,
	}
}

// UABlock 返回 UA 拦截中间件。
func UABlock(cfg UAConfig) HandlerFunc {
	if cfg.BlockEmpty == false && len(cfg.BlockPatterns) == 0 && len(cfg.AllowList) == 0 {
		cfg = DefaultUAConfig()
	}
	block := make([]string, len(cfg.BlockPatterns))
	for i, p := range cfg.BlockPatterns {
		block[i] = strings.ToLower(p)
	}
	allow := make([]string, len(cfg.AllowList))
	for i, p := range cfg.AllowList {
		allow[i] = strings.ToLower(p)
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ua := r.UserAgent()
			uaLower := strings.ToLower(ua)
			// 探活路径豁免
			if isProbePath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			// 白名单优先
			if len(allow) > 0 {
				matched := false
				for _, p := range allow {
					if strings.Contains(uaLower, p) {
						matched = true
						break
					}
				}
				if !matched {
					http.Error(w, "forbidden user agent", http.StatusForbidden)
					return
				}
			}
			// 空 UA
			if cfg.BlockEmpty && ua == "" {
				http.Error(w, "empty user agent", http.StatusForbidden)
				return
			}
			// 黑名单
			for _, p := range block {
				if strings.Contains(uaLower, p) {
					http.Error(w, "blocked user agent", http.StatusForbidden)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func isProbePath(p string) bool {
	switch p {
	case "/live", "/ready", "/readyz", "/healthz", "/metrics":
		return true
	}
	return false
}
