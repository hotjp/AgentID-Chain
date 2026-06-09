// Package middleware: TLS 中间件（P17.7 — 强制 HTTPS）。
//
// 职责：
//  1. HTTP → HTTPS 永久重定向（308）
//  2. 设置 HSTS 头（Strict-Transport-Security）
//  3. 代理感知（X-Forwarded-Proto）
//  4. 例外路径：health/metrics/pprof 可走明文
//
// 设计：纯中间件；与证书加载解耦（证书由 server 包的 TLSConfig 处理）。
package middleware

import (
	"net/http"
	"strings"
)

// TLSConfig TLS 强制配置。
type TLSConfig struct {
	// Enabled 是否启用 TLS 强制（开发环境可关闭）。
	Enabled bool
	// Port HTTPS 端口（用于构造重定向 URL，默认 443）。
	Port int
	// TrustProxy 是否信任 X-Forwarded-Proto 头（前置 LB/ingress 时启用）。
	TrustProxy bool
	// HSTSMaxAge HSTS 过期秒数（0 = 不发送 HSTS；推荐 31536000 = 1 年）。
	HSTSMaxAge int
	// HSTSIncludeSubdomains 是否包含子域名。
	HSTSIncludeSubdomains bool
	// HSTSPreload 是否加入浏览器预加载列表。
	HSTSPreload bool
	// ExemptPaths 不强制 TLS 的路径前缀（health、metrics、pprof 等）。
	ExemptPaths []string
}

// DefaultTLSConfig 生产环境推荐配置。
func DefaultTLSConfig() TLSConfig {
	return TLSConfig{
		Enabled:               true,
		Port:                  443,
		TrustProxy:            false,
		HSTSMaxAge:            31536000, // 1 年
		HSTSIncludeSubdomains: true,
		HSTSPreload:           true,
		ExemptPaths: []string{
			"/live",
			"/ready",
			"/healthz",
			"/health",
			"/metrics",
			"/debug/",
		},
	}
}

// TLS 返回 TLS 强制中间件。
func TLS(cfg TLSConfig) HandlerFunc {
	if cfg.Port == 0 {
		cfg.Port = 443
	}
	if cfg.HSTSMaxAge == 0 && cfg.Enabled {
		// 默认 HSTS 1 年
		cfg.HSTSMaxAge = 31536000
	}
	exempt := make(map[string]struct{}, len(cfg.ExemptPaths))
	for _, p := range cfg.ExemptPaths {
		exempt[p] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !cfg.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			// 1. 检测是否已 HTTPS（直连或代理）
			scheme := "http"
			if r.TLS != nil {
				scheme = "https"
			} else if cfg.TrustProxy {
				if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
					scheme = strings.ToLower(proto)
				}
			}

			// 2. 例外路径放行
			if _, ok := exempt[r.URL.Path]; ok {
				setHSTS(w, cfg)
				next.ServeHTTP(w, r)
				return
			}

			// 3. HTTP → HTTPS 永久重定向
			if scheme != "https" {
				host := r.Host
				// 去掉已有端口，加 HTTPS 端口
				if h, _, err := splitHostPort(host); err == nil {
					host = h
				}
				target := "https://" + host
				if cfg.Port != 443 {
					target += ":" + itoa(cfg.Port)
				}
				target += r.URL.RequestURI()
				w.Header().Set("Location", target)
				w.Header().Set("Strict-Transport-Security", buildHSTS(cfg))
				w.WriteHeader(http.StatusPermanentRedirect)
				return
			}

			// 4. 已 HTTPS：注入 HSTS
			setHSTS(w, cfg)
			next.ServeHTTP(w, r)
		})
	}
}

func setHSTS(w http.ResponseWriter, cfg TLSConfig) {
	if cfg.HSTSMaxAge > 0 {
		w.Header().Set("Strict-Transport-Security", buildHSTS(cfg))
	}
}

func buildHSTS(cfg TLSConfig) string {
	v := "max-age=" + itoa(cfg.HSTSMaxAge)
	if cfg.HSTSIncludeSubdomains {
		v += "; includeSubDomains"
	}
	if cfg.HSTSPreload {
		v += "; preload"
	}
	return v
}

// splitHostPort 拆 host:port（无端口时 host 原样返回）。
func splitHostPort(addr string) (host string, port string, err error) {
	// IPv6 形式：[::1] 或 [::1]:8080
	if strings.HasPrefix(addr, "[") {
		end := strings.Index(addr, "]")
		if end < 0 {
			return addr, "", nil
		}
		host = addr[:end+1]
		if end+1 < len(addr) && addr[end+1] == ':' {
			port = addr[end+2:]
		}
		return host, port, nil
	}
	// 普通形式（含多个冒号视为纯 IPv6 不带端口，net.SplitHostPort 才是权威；这里保守处理）
	if strings.Count(addr, ":") > 1 {
		return addr, "", nil
	}
	idx := strings.LastIndex(addr, ":")
	if idx < 0 {
		return addr, "", nil
	}
	return addr[:idx], addr[idx+1:], nil
}
