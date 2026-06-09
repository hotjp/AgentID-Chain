// Package telemetry: Propagation 测试。
package telemetry

import (
	"context"
	"net/http"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestPropagator_Inject_Valid(t *testing.T) {
	p := NewPropagator()
	traceID, _ := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	spanID, _ := trace.SpanIDFromHex("00f067aa0ba902b7")
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})
	h := http.Header{}
	p.Inject(sc, h)
	got := h.Get(TraceparentHeader)
	want := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestPropagator_Inject_NotSampled(t *testing.T) {
	p := NewPropagator()
	traceID, _ := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	spanID, _ := trace.SpanIDFromHex("00f067aa0ba902b7")
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID,
		SpanID:  spanID,
	})
	h := http.Header{}
	p.Inject(sc, h)
	got := h.Get(TraceparentHeader)
	want := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-00"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestPropagator_Inject_Invalid(t *testing.T) {
	p := NewPropagator()
	h := http.Header{}
	p.Inject(trace.SpanContext{}, h)
	if h.Get(TraceparentHeader) != "" {
		t.Fatal("invalid SpanContext should not produce traceparent")
	}
}

func TestPropagator_Extract_Valid(t *testing.T) {
	p := NewPropagator()
	h := http.Header{}
	h.Set(TraceparentHeader, "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	sc, err := p.Extract(h)
	if err != nil {
		t.Fatal(err)
	}
	if !sc.IsValid() {
		t.Fatal("expected valid SpanContext")
	}
	if sc.TraceID().String() != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("unexpected trace_id: %s", sc.TraceID())
	}
	if sc.SpanID().String() != "00f067aa0ba902b7" {
		t.Fatalf("unexpected span_id: %s", sc.SpanID())
	}
	if !sc.IsSampled() {
		t.Fatal("expected sampled")
	}
	if !sc.IsRemote() {
		t.Fatal("expected remote")
	}
}

func TestPropagator_Extract_Empty(t *testing.T) {
	p := NewPropagator()
	sc, err := p.Extract(http.Header{})
	if err != nil {
		t.Fatal(err)
	}
	if sc.IsValid() {
		t.Fatal("expected invalid SpanContext for empty header")
	}
}

func TestPropagator_Extract_Malformed(t *testing.T) {
	tests := []string{
		"invalid",
		"00-4bf92f3577b34da6a3ce929d0e0e4736",                       // 缺字段
		"00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7",      // 缺 flags
		"00-zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz-00f067aa0ba902b7-01",  // 非 hex
		"01-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",  // 不支持版本
		"00-00000000000000000000000000000000-00f067aa0ba902b7-01",  // trace_id 全 0
		"00-4bf92f3577b34da6a3ce929d0e0e4736-0000000000000000-01",  // parent_id 全 0
	}
	p := NewPropagator()
	for _, tp := range tests {
		t.Run(tp, func(t *testing.T) {
			h := http.Header{}
			h.Set(TraceparentHeader, tp)
			_, err := p.Extract(h)
			if err == nil {
				t.Fatalf("expected error for %q", tp)
			}
		})
	}
}

func TestPropagator_RoundTrip(t *testing.T) {
	p := NewPropagator()
	originalTraceID, _ := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	originalSpanID, _ := trace.SpanIDFromHex("00f067aa0ba902b7")
	sc1 := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    originalTraceID,
		SpanID:     originalSpanID,
		TraceFlags: trace.FlagsSampled,
	})
	h := http.Header{}
	p.Inject(sc1, h)
	sc2, err := p.Extract(h)
	if err != nil {
		t.Fatal(err)
	}
	if sc1.TraceID() != sc2.TraceID() {
		t.Fatal("trace_id mismatch")
	}
	if sc1.SpanID() != sc2.SpanID() {
		t.Fatal("span_id mismatch")
	}
}

func TestPropagator_ContextWithRemoteSpan(t *testing.T) {
	p := NewPropagator()
	h := http.Header{}
	h.Set(TraceparentHeader, "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	ctx, err := p.ContextWithRemoteSpan(context.Background(), h)
	if err != nil {
		t.Fatal(err)
	}
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		t.Fatal("expected valid context")
	}
}
