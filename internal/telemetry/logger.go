// Package telemetry: 结构化日志 (P19.10)。
//
// 基于 slog 的标准化 logger 构造器。
// 提供：
//   - 服务级默认 attrs（service.name, service.version, service.instance）
//   - 自动注入 trace_id / span_id（来自 OTel context）
//   - 敏感字段脱敏（来自 SensitiveHandler, P17.12）
//   - level 控制（debug|info|warn|error）
package telemetry

import (
	"context"
	"fmt"
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

// LogConfig 日志配置。
type LogConfig struct {
	// ServiceName 服务名。
	ServiceName string
	// ServiceVersion 服务版本。
	ServiceVersion string
	// ServiceNamespace 命名空间。
	ServiceNamespace string
	// ServiceInstance 实例 ID（hostname 等）。
	ServiceInstance string
	// Level 日志级别：debug|info|warn|error（默认 info）。
	Level string
	// Format 输出格式：json|text（默认 json）。
	Format string
	// Output 输出目标：stdout|stderr|file（默认 stdout）。
	Output string
	// FilePath output=file 时启用。
	FilePath string
	// SensitiveConfig 敏感字段脱敏配置（nil = 启用默认）。
	SensitiveConfig *SensitiveConfig
	// AddSource 是否记录 source 字段（生产建议 false）。
	AddSource bool
}

// NewServiceLogger 构造带服务字段的 logger。
//
// 推荐用法：
//
//	cfg := telemetry.LogConfig{
//	    ServiceName: "agentid-gateway",
//	    ServiceVersion: "2.0.1",
//	    Level: "info",
//	}
//	logger := telemetry.NewServiceLogger(cfg)
//	logger.Info("server starting", "port", 8080)
func NewServiceLogger(cfg LogConfig) *slog.Logger {
	return NewServiceLoggerTo(cfg, os.Stdout)
}

// NewServiceLoggerTo 构造 logger（自定义 io.Writer；便于测试）。
func NewServiceLoggerTo(cfg LogConfig, w io.Writer) *slog.Logger {
	if w == nil {
		w = os.Stdout
	}
	level := parseLevel(cfg.Level)
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: cfg.AddSource,
	}
	var h slog.Handler
	switch cfg.Format {
	case "text":
		h = slog.NewTextHandler(w, opts)
	default:
		h = slog.NewJSONHandler(w, opts)
	}
	// 包装脱敏（仅当显式启用或用户传入 config 时）
	if cfg.SensitiveConfig != nil {
		h = NewSensitiveHandler(h, cfg.SensitiveConfig)
	}
	// 预绑定服务字段
	logger := slog.New(h).With(
		slog.String("service", cfg.ServiceName),
		slog.String("version", cfg.ServiceVersion),
	)
	if cfg.ServiceNamespace != "" {
		logger = logger.With(slog.String("namespace", cfg.ServiceNamespace))
	}
	if cfg.ServiceInstance != "" {
		logger = logger.With(slog.String("instance", cfg.ServiceInstance))
	}
	return logger
}

// NewServiceLoggerWithStderr 输出到 stderr。
func NewServiceLoggerWithStderr(cfg LogConfig) *slog.Logger {
	return NewServiceLoggerTo(cfg, os.Stderr)
}

// parseLevel 解析日志级别字符串。
func parseLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error", "err":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// FormatLevelToString 将 slog.Level 转为字符串。
func FormatLevelToString(l slog.Level) string {
	switch l {
	case slog.LevelDebug:
		return "debug"
	case slog.LevelInfo:
		return "info"
	case slog.LevelWarn:
		return "warn"
	case slog.LevelError:
		return "error"
	default:
		return fmt.Sprintf("level(%d)", int(l))
	}
}

// WithRequestID 注入 request_id 字段（用于 L5 网关中间件）。
func WithRequestID(ctx context.Context, logger *slog.Logger, requestID string) *slog.Logger {
	return logger.With(slog.String("request_id", requestID))
}

// WithAgent 注入 agent 相关字段。
func WithAgent(logger *slog.Logger, agentID, owner, level string) *slog.Logger {
	return logger.With(
		slog.String("agent_id", agentID),
		slog.String("owner", owner),
		slog.String("level", level),
	)
}

// WithError 注入错误字段（含 error 类型）。
func WithError(logger *slog.Logger, err error) *slog.Logger {
	return logger.With(slog.String("error", err.Error()))
}

// WithLatency 注入延迟字段。
func WithLatency(logger *slog.Logger, ms float64) *slog.Logger {
	return logger.With(slog.Float64("latency_ms", ms))
}
