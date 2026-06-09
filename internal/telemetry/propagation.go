// Package telemetry: W3C TraceContext 透传 (P19.1)。
//
// 实现 W3C Trace Context 标准的 traceparent / tracestate 头注入与提取。
// 兼容 OpenTelemetry API。
//
// 参考：https://www.w3.org/TR/trace-context/
package telemetry

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"go.opentelemetry.io/otel/trace"
)

const (
	// W3C 标准 header 名（小写）
	TraceparentHeader = "traceparent"
	TracestateHeader  = "tracestate"

	// 版本（当前仅支持 00）
	TraceparentVersion = "00"
)

// Traceparent 格式：<version>-<trace-id>-<parent-id>-<trace-flags>
// 示例：00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01
var traceparentRegex = regexp.MustCompile(
	`^(?P<version>[0-9a-f]{2})-(?P<trace_id>[0-9a-f]{32})-(?P<parent_id>[0-9a-f]{16})-(?P<flags>[0-9a-f]{2})$`,
)

// Propagator W3C TraceContext Propagator。
type Propagator struct{}

// NewPropagator 构造 W3C Propagator。
func NewPropagator() *Propagator { return &Propagator{} }

// Inject 注入 trace context 到 header。
//
// 参数：
//   - sc: 当前 span 的 SpanContext
//   - header: 目标 http.Header
func (p *Propagator) Inject(sc trace.SpanContext, header http.Header) {
	if !sc.IsValid() {
		return
	}
	// traceparent
	traceparent := fmt.Sprintf("%s-%s-%s-%s",
		TraceparentVersion,
		sc.TraceID(),
		sc.SpanID(),
		formatFlags(sc.TraceFlags()),
	)
	header.Set(TraceparentHeader, traceparent)
	// tracestate（如果 SpanContext 携带）
	// 当前 OTel Go SDK 通过 sc.TraceState() 提供
	// 这里简化：不强制写入
}

// Extract 从 header 提取 SpanContext。
//
// 行为：
//   - header 不存在 → 返回 invalid SpanContext
//   - header 格式错误 → 返回 invalid SpanContext + error
//   - trace_id 全部为 0 → invalid
//   - parent_id 全部为 0 → invalid
func (p *Propagator) Extract(header http.Header) (trace.SpanContext, error) {
	tp := header.Get(TraceparentHeader)
	if tp == "" {
		return trace.SpanContext{}, nil
	}
	tp = strings.TrimSpace(tp)
	matches := traceparentRegex.FindStringSubmatch(tp)
	if matches == nil {
		return trace.SpanContext{}, fmt.Errorf("telemetry: invalid traceparent format: %q", tp)
	}
	version := matches[1]
	traceIDStr := matches[2]
	parentIDStr := matches[3]
	flagsStr := matches[4]

	// 版本检查
	if version != TraceparentVersion {
		// 未来版本：可选择性解析
		return trace.SpanContext{}, fmt.Errorf("telemetry: unsupported traceparent version: %q", version)
	}

	traceID, err := trace.TraceIDFromHex(traceIDStr)
	if err != nil {
		return trace.SpanContext{}, fmt.Errorf("telemetry: invalid trace_id: %w", err)
	}
	if !traceID.IsValid() {
		return trace.SpanContext{}, errors.New("telemetry: trace_id is all zero")
	}
	spanID, err := trace.SpanIDFromHex(parentIDStr)
	if err != nil {
		return trace.SpanContext{}, fmt.Errorf("telemetry: invalid parent_id: %w", err)
	}
	if !spanID.IsValid() {
		return trace.SpanContext{}, errors.New("telemetry: parent_id is all zero")
	}
	flags := parseFlags(flagsStr)
	return trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: flags,
		Remote:     true,
	}), nil
}

// ContextWithRemoteSpan 返回带远程 span 的 ctx。
//
// 业务用法：
//
//	propagator := telemetry.NewPropagator()
//	sc, _ := propagator.Extract(req.Header)
//	ctx = trace.ContextWithRemoteSpanContext(ctx, sc)
//	ctx, span := tracer.Start(ctx, "handle-request")
func (p *Propagator) ContextWithRemoteSpan(ctx context.Context, header http.Header) (context.Context, error) {
	sc, err := p.Extract(header)
	if err != nil {
		return ctx, err
	}
	if !sc.IsValid() {
		return ctx, nil
	}
	return trace.ContextWithRemoteSpanContext(ctx, sc), nil
}

// =============================================================================
// 工具函数
// =============================================================================

// formatFlags 将 TraceFlags 转为 2 字符 hex。
func formatFlags(f trace.TraceFlags) string {
	if f.IsSampled() {
		return "01"
	}
	return "00"
}

// parseFlags 从 2 字符 hex 解析 TraceFlags。
func parseFlags(s string) trace.TraceFlags {
	if len(s) != 2 {
		return 0
	}
	if s == "01" {
		return trace.FlagsSampled
	}
	return 0
}
