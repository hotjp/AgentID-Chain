// Package ent 提供 ent generate 入口。
//
// 运行 `go generate ./ent` 时触发 entc：
//   1. 扫描 ent/schema/*.go
//   2. 在 ent/ 目录下生成 client.go / user.go / user_query.go / ... 等代码
//   3. 同时生成 ent/migrate/migrations/* 的 atlas 配置
//
// 注意：本文件本身不需要修改；新增 schema 只需在 ent/schema/ 下追加文件。
//go:generate go run -mod=mod entgo.io/ent/cmd/ent generate ./schema
package ent
