// Package config — 向后兼容 shim（指向 loader.go）。
//
// 新代码请直接使用 loader.go 中的 Load / MustLoad / LoadString / Snapshot。
// 本文件保留以避免破坏引用了 Load / LoadString / Snapshot 的旧代码。
package config

// 本文件的实现已迁移到 loader.go；此处仅保留 API 占位。
// 真正的实现在 loader.go 中（koanf 单例 + LoadWith 增强）。
