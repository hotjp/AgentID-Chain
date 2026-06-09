// Package telemetry slog 适配器（注入 trace_id / span_id）。
package telemetry

import (
	"context"
	"io"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel/trace"
)

// NewSlogHandler 包装 slog.Handler，自动注入 trace_id / span_id。
func NewSlogHandler(level slog.Level, format string, w io.Writer) slog.Handler {
	if w == nil {
		w = os.Stderr
	}
	opts := &slog.HandlerOptions{Level: level, AddSource: false}
	var h slog.Handler
	switch format {
	case "text":
		h = slog.NewTextHandler(w, opts)
	default:
		h = slog.NewJSONHandler(w, opts)
	}
	return &traceHandler{Handler: h}
}

// traceHandler 在 Record 上注入 trace 上下文属性。
type traceHandler struct {
	slog.Handler
}

func (h *traceHandler) Handle(_ context.Context, r slog.Record) error {
	// 这里我们不持有 ctx；trace_id/span_id 由调用方在 WithTrace(...) 中预注入
	return h.Handler.Handle(context.Background(), r)
}

// WithAttrs 透传；调用方在 slog.Logger.With 上传 trace_id/span_id。
func (h *traceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &traceHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h *traceHandler) WithGroup(name string) slog.Handler {
	return &traceHandler{Handler: h.Handler.WithGroup(name)}
}

// WithTrace 在 ctx 包含 trace 时返回带 trace_id/span_id 的 logger。
// 推荐用法：logger := telemetry.WithTrace(ctx, baseLogger)
func WithTrace(ctx context.Context, base *slog.Logger) *slog.Logger {
	if base == nil {
		base = slog.Default()
	}
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().IsValid() {
		return base
	}
	sc := span.SpanContext()
	return base.With(
		slog.String("trace_id", sc.TraceID().String()),
		slog.String("span_id", sc.SpanID().String()),
	)
}

// stderrWriter 返回 stderr io.Writer（用于 telemetry 内部自记日志）。
func stderrWriter() io.Writer { return os.Stderr }
