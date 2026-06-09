// Package handler: Prometheus metrics handler（P7.12 占位）。
//
// /metrics 端点由 promhttp.Handler() 直接暴露（在 router 注册时挂载）。
// 本文件为业务自定义指标保留入口。
package handler
