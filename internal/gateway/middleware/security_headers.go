// Package middleware: 安全响应头中间件（P17.8）。
//
// 注入 OWASP 推荐的安全响应头：
//   - Content-Security-Policy     限制资源加载源
//   - X-Content-Type-Options      禁止 MIME 嗅探
//   - X-Frame-Options             禁止 iframe 嵌入（点击劫持防护）
//   - Referrer-Policy             控制 Referer 泄露
//   - Permissions-Policy          关闭不需要的浏览器特性
//   - X-Permitted-Cross-Domain-Policies 限制跨域策略
//   - Cross-Origin-*-Policy       跨源隔离
//   - Cache-Control               敏感端点禁止缓存
package middleware

import "net/http"

// SecurityHeadersConfig 安全响应头配置。
type SecurityHeadersConfig struct {
	// Enabled 是否启用。
	Enabled bool
	// CSP Content-Security-Policy 值。
	// 默认严格：default-src 'self'，禁用 inline/eval。
	CSP string
	// XFrameOptions DENY | SAMEORIGIN | 留空不发送。
	XFrameOptions string
	// ReferrerPolicy strict-origin-when-cross-origin 等。
	ReferrerPolicy string
	// PermissionsPolicy 关闭不需要的特性。
	PermissionsPolicy string
	// COOP cross-origin 跨源隔离。
	COOP string
	// COEP require-corp 跨源嵌入限制。
	COEP string
	// CORP same-origin 跨源资源限制。
	CORP string
	// NoCachePaths 这些路径额外加 Cache-Control: no-store。
	NoCachePaths []string
	// HSTSMaxAge HSTS 过期秒数（0 = 不发送 HSTS；推荐 31536000 = 1 年）。
	HSTSMaxAge int
	// HSTSIncludeSubdomains 是否包含子域名。
	HSTSIncludeSubdomains bool
	// HSTSPreload 是否加入浏览器预加载列表。
	HSTSPreload bool
}

// DefaultSecurityHeadersConfig 生产推荐配置。
func DefaultSecurityHeadersConfig() SecurityHeadersConfig {
	return SecurityHeadersConfig{
		Enabled: true,
		// API 网关场景：默认拒所有，仅允许 self + 显式 https 内联
		CSP:               "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'",
		XFrameOptions:     "DENY",
		ReferrerPolicy:    "strict-origin-when-cross-origin",
		PermissionsPolicy: "accelerometer=(), camera=(), geolocation=(), gyroscope=(), magnetometer=(), microphone=(), payment=(), usb=()",
		COOP:              "same-origin",
		COEP:              "require-corp",
		CORP:              "same-origin",
		NoCachePaths: []string{
			"/api/",
			"/v1/",
			"/aap/",
			"/auth/",
		},
		HSTSMaxAge:            31536000,
		HSTSIncludeSubdomains: true,
		HSTSPreload:           true,
	}
}

// SecurityHeaders 返回安全响应头中间件。
func SecurityHeaders(cfg SecurityHeadersConfig) HandlerFunc {
	if !cfg.Enabled {
		return func(next http.Handler) http.Handler { return next }
	}
	noCache := make(map[string]struct{}, len(cfg.NoCachePaths))
	for _, p := range cfg.NoCachePaths {
		noCache[p] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			// 1. CSP
			if cfg.CSP != "" {
				h.Set("Content-Security-Policy", cfg.CSP)
			}
			// 2. 禁止 MIME 嗅探（所有响应都加）
			h.Set("X-Content-Type-Options", "nosniff")
			// 3. 点击劫持防护
			if cfg.XFrameOptions != "" {
				h.Set("X-Frame-Options", cfg.XFrameOptions)
			}
			// 4. Referer 控制
			if cfg.ReferrerPolicy != "" {
				h.Set("Referrer-Policy", cfg.ReferrerPolicy)
			}
			// 5. 浏览器特性白名单
			if cfg.PermissionsPolicy != "" {
				h.Set("Permissions-Policy", cfg.PermissionsPolicy)
			}
			// 6. Flash/PDF 跨域策略
			h.Set("X-Permitted-Cross-Domain-Policies", "none")
			// 7. 跨源隔离
			if cfg.COOP != "" {
				h.Set("Cross-Origin-Opener-Policy", cfg.COOP)
			}
			if cfg.COEP != "" {
				h.Set("Cross-Origin-Embedder-Policy", cfg.COEP)
			}
			if cfg.CORP != "" {
				h.Set("Cross-Origin-Resource-Policy", cfg.CORP)
			}
			// 8. HSTS（与 TLS 中间件可叠加；这里单独允许配置）
			if cfg.HSTSMaxAge > 0 {
				h.Set("Strict-Transport-Security", buildHSTS(TLSConfig{
					HSTSMaxAge:            cfg.HSTSMaxAge,
					HSTSIncludeSubdomains: cfg.HSTSIncludeSubdomains,
					HSTSPreload:           cfg.HSTSPreload,
				}))
			}
			// 9. 敏感路径禁止缓存
			for prefix := range noCache {
				if hasPathPrefix(r.URL.Path, prefix) {
					h.Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
					h.Set("Pragma", "no-cache")
					h.Set("Expires", "0")
					break
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func hasPathPrefix(path, prefix string) bool {
	if len(path) < len(prefix) {
		return false
	}
	return path[:len(prefix)] == prefix
}
