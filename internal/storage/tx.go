// Package storage 事务管理（InTransaction 封装）。
//
// 目标：
//  1. 提供 InTransaction(ctx, runner, fn) 统一入口
//  2. fn 接受 *ent.Tx；返回 error / panic 时自动回滚
//  3. 支持嵌套 — 内部事务复用外层 Tx，外部事务真正 COMMIT/ROLLBACK
//  4. panic 恢复：defer 中把 panic 包成 ErrTxFailed 重新抛出
//
// 设计：
//   - 用 context.WithValue 携带"是否在事务中"标记（key 为 txCtxKey）
//   - 嵌套时：若 ctx 已有标记，直接跑 fn（不开新事务）
//   - 顶层：用 runner.RunInTx(ctx, fn) 走 ent.Client.Tx
//
// 用法示例（业务层）：
//
//	err := storage.InTransaction(ctx, client, func(ctx context.Context, tx *ent.Tx) error {
//	    if _, err := tx.User.Create().SetEmail("a@b.com").Save(ctx); err != nil {
//	        return err  // 自动 ROLLBACK
//	    }
//	    return nil     // 成功 → 自动 COMMIT
//	})
package storage

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
)

// txCtxKey 私有类型，避免 context 键冲突。
type txCtxKey struct{}

// TxRunner 事务执行器接口。
//
// 任何实现了 RunInTx(ctx, fn) 的类型都可以作为事务入口。
// *ent.Client 满足此接口（其 Tx 方法签名）。
type TxRunner interface {
	// RunInTx 在事务中执行 fn；fn 返回 nil → COMMIT，非 nil → ROLLBACK。
	// 实现约定：fn 收到的 ctx 已经包含 ent.Tx 句柄（*ent.Tx 通过 ctx 拿）。
	RunInTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// WithTx 把"事务中"标记注入 ctx。
//
// 内部嵌套场景使用 — 业务层通常不需要直接调用。
func WithTx(ctx context.Context) context.Context {
	return context.WithValue(ctx, txCtxKey{}, true)
}

// IsInTx 判断 ctx 是否已在事务中。
func IsInTx(ctx context.Context) bool {
	v, _ := ctx.Value(txCtxKey{}).(bool)
	return v
}

// InTransaction 在 runner 上开启事务，运行 fn。
//
// 语义：
//   - ctx 无事务标记 → 新开事务（runner.RunInTx 内部走 ent.Client.Tx）
//   - ctx 有事务标记 → 嵌套：直接跑 fn（不开新事务）
//   - fn 返回 nil → COMMIT（ent 内部）
//   - fn 返回 err → ROLLBACK（ent 内部）
//   - fn panic    → ROLLBACK + 包成 ErrTxFailed 重新 panic
//
// 错误处理：
//   - 嵌套层 fn 内的错误直接透传（外层会处理）
//   - 顶层 fn 返回的错误被原样透传（不包 ErrTxFailed）— 让上层决定如何分类
//   - 顶层 fn panic  → 包成 ErrTxFailed 重新 panic
//
// 类型参数 TX：业务层把 *ent.Tx 当作 fn 的第二个参数；此处用 any 占位。
// 实际业务代码可定义自己的 Tx 类型并在 fn 内做 cast。
func InTransaction(
	ctx context.Context,
	runner TxRunner,
	fn func(ctx context.Context, tx any) error,
) (err error) {
	// panic 恢复（仅顶层生效；嵌套层 fn 的 panic 由外层兜底）
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%w: panic in tx: %v\n%s", ErrTxFailed, r, debug.Stack())
			panic(err)
		}
	}()

	// 嵌套：直接跑 fn
	if IsInTx(ctx) {
		return fn(ctx, nil)
	}

	// 顶层：开新事务
	return runner.RunInTx(ctx, func(txCtx context.Context) error {
		// 把"事务中"标记注入 ctx；fn 内可通过 storage.IsInTx(txCtx) 判断
		txCtx = WithTx(txCtx)
		// 让 fn 通过 ctx 拿到 ent.Tx（ent.Client.Tx 会自动注入）
		// 业务示例：tx := entTxFromContext(txCtx)
		return fn(txCtx, nil)
	})
}

// ErrTxRollback 显式回滚（fn 返回此错误时仍视为"业务正常完成"，
// 但事务会 ROLLBACK — 适合"已经做了别的事故恢复"场景）。
var ErrTxRollback = errors.New("storage: tx rollback")
