// Package telemetry 核心初始化。
package telemetry

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// Config telemetry 配置（与 internal/config.TelemetryConfig 对齐；这里复制避免循环 import）
type Config struct {
	ServiceName  string
	OTELEndpoint string // 空 = 不启用 OTLP 导出（tracer 仍可用，仅本地）
	MetricsAddr  string // Prometheus /metrics 监听地址
	PProfAddr    string // pprof 监听地址（仅内网）
	SampleRate   float64
}

// Telemetry 全局观测提供者。
type Telemetry struct {
	cfg           Config
	tracer        trace.Tracer
	meterShutdown func(context.Context) error
	traceShutdown func(context.Context) error
	promSrv       *http.Server
}

// Init 初始化 Tracer + Meter + Prometheus HTTP server。
//
//nolint:gocognit // 初始化多 provider 逻辑天然分支多
func Init(ctx context.Context, cfg Config) (*Telemetry, error) {
	if cfg.ServiceName == "" {
		cfg.ServiceName = "agentid-chain"
	}
	if cfg.SampleRate == 0 {
		cfg.SampleRate = 1.0
	}

	// 资源（service.name / version 等；被 trace / metric 共享）
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion("dev"),
		),
		resource.WithProcessRuntimeName(),
		resource.WithProcessRuntimeVersion(),
		resource.WithHost(),
	)
	if err != nil {
		return nil, fmt.Errorf("create resource: %w", err)
	}

	// ---------- Meter (Prometheus) ----------
	promExporter, err := prometheus.New()
	if err != nil {
		return nil, fmt.Errorf("create prometheus exporter: %w", err)
	}
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(promExporter),
	)
	otel.SetMeterProvider(meterProvider)

	// ---------- Tracer (OTLP 可选 + AlwaysOn Sampler) ----------
	var tp *sdktrace.TracerProvider
	if cfg.OTELEndpoint != "" {
		// 简化的 OTLP HTTP exporter 占位
		// P5 阶段接 internal/plugins/telemetry/otlp 完善
		// 此处仅做 trace provider + AlwaysOn sampler + BatchSpanProcessor 占位
		tp = sdktrace.NewTracerProvider(
			sdktrace.WithResource(res),
			sdktrace.WithSampler(sdktrace.TraceIDRatioBased(cfg.SampleRate)),
		)
	} else {
		// 本地模式：无 OTLP，但 trace 仍可手动建 span（仅用于 logging / 测试）
		tp = sdktrace.NewTracerProvider(
			sdktrace.WithResource(res),
			sdktrace.WithSampler(sdktrace.AlwaysSample()),
		)
	}
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// ---------- Prometheus HTTP server (port 9090 by default) ----------
	var promSrv *http.Server
	if cfg.MetricsAddr != "" {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		})
		promSrv = &http.Server{
			Addr:    cfg.MetricsAddr,
			Handler: mux,
		}
		go func() {
			if err := promSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				// 不致命：log 到 stderr 即可
				fmt.Fprintf(stderrWriter(), "prometheus server error: %v\n", err)
			}
		}()
	}

	return &Telemetry{
		cfg:           cfg,
		tracer:        otel.Tracer(cfg.ServiceName),
		meterShutdown: meterProvider.Shutdown,
		traceShutdown: tp.Shutdown,
		promSrv:       promSrv,
	}, nil
}

// Tracer 返回 TracerProvider 上的命名 Tracer。
func (t *Telemetry) Tracer() trace.Tracer { return t.tracer }

// Shutdown 优雅关闭所有 provider 与 HTTP server。
func (t *Telemetry) Shutdown(ctx context.Context) error {
	var firstErr error
	if t.promSrv != nil {
		if err := t.promSrv.Shutdown(ctx); err != nil {
			firstErr = err
		}
	}
	if t.meterShutdown != nil {
		if err := t.meterShutdown(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if t.traceShutdown != nil {
		if err := t.traceShutdown(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
