// Package telemetry: trace_id 注入日志 别名文件 (P19.12 deliverable path)。
//
// 实现位于 logger.go (WithTrace) 和 propagation.go；本文件保留向后兼容入口。
package telemetry

import (
	"context"
	"log/slog"
)

// LogWithTrace 返回带 trace_id / span_id 的 logger。
//
// 推荐用法：logger := telemetry.LogWithTrace(ctx, baseLogger)
//
// 这是 WithTrace 的语义化别名。
func LogWithTrace(ctx context.Context, base *slog.Logger) *slog.Logger {
	return WithTrace(ctx, base)
}
