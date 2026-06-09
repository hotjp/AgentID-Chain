// Package telemetry 提供 AgentID-Chain 统一观测能力初始化。
//
// 三件套：
//   - Metrics: Prometheus exporter（自带 /metrics 端点）
//   - Traces:  OpenTelemetry OTLP（可选；endpoint 空时不导出）
//   - Logs:    slog 包装（提供 trace_id / span_id 注入；与 telemetry.go 配合使用）
//
// 生命周期：
//   - Init(cfg) 启动时调用一次
//   - Shutdown(ctx) 优雅关闭
//   - 任何 panic/退出路径都应 defer Shutdown
package telemetry
