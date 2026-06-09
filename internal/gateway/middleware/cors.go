// Package middleware: CORS 中间件（P7.6）。
//
// 注入标准 CORS 响应头；可配置 origin（默认 "*"）。
// 仅处理 preflight (OPTIONS) + 透传响应头给下游。
package middleware

import "net/http"

// CORSConfig CORS 配置。
type CORSConfig struct {
	// AllowOrigins 允许的 origin 列表（"*" = 全部；逗号分隔）。
	AllowOrigins string
	// AllowMethods 允许的 HTTP 方法。
	AllowMethods string
	// AllowHeaders 允许的请求头。
	AllowHeaders string
	// ExposeHeaders 暴露给 JS 的响应头。
	ExposeHeaders string
	// MaxAge 秒。
	MaxAge int
	// AllowCredentials 是否允许 credentials。
	AllowCredentials bool
}

// DefaultCORSConfig 保守默认。
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS,PATCH",
		AllowHeaders: "Content-Type,Authorization,X-Request-ID,X-API-Key,X-AAP-Proof",
		ExposeHeaders: "X-Request-ID",
		MaxAge:        86400,
	}
}

// CORS 返回 CORS 中间件。
func CORS(cfg CORSConfig) HandlerFunc {
	if cfg.AllowOrigins == "" {
		cfg = DefaultCORSConfig()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", cfg.AllowOrigins)
			w.Header().Set("Access-Control-Allow-Methods", cfg.AllowMethods)
			w.Header().Set("Access-Control-Allow-Headers", cfg.AllowHeaders)
			w.Header().Set("Access-Control-Expose-Headers", cfg.ExposeHeaders)
			w.Header().Set("Access-Control-Max-Age", itoa(cfg.MaxAge))
			if cfg.AllowCredentials {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
			// preflight
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func itoa(n int) string {
	// 避免 strconv 导入冲突
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
