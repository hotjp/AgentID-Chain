// Package telemetry: 日志脱敏 Handler 别名文件 (P19.11 deliverable path)。
//
// 实际实现位于 sensitive_handler.go；本文件保留任务交付物路径的入口。
//
// 推荐用法：
//
//	base := slog.NewJSONHandler(os.Stdout, nil)
//	logger := slog.New(telemetry.NewSensitiveHandler(base, nil))
package telemetry

import "log/slog"

// SensitiveCfg 是 SensitiveConfig 的语义化别名（向后兼容）。
type SensitiveCfg = SensitiveConfig

// NewRedactHandler 是 NewSensitiveHandler 的语义化别名。
func NewRedactHandler(inner slog.Handler, cfg *SensitiveConfig) *SensitiveHandler {
	return NewSensitiveHandler(inner, cfg)
}
