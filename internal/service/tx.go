// Package service: 事务封装（P6.9）。
//
// 设计：L4 Service 层用 Tx 包装 L1 写入，保证一个工作流的多个写
// 在同一事务内提交。Tx 与 ent.Tx / pgx.Tx 解耦，通过 TxFunc 回调
// 把事务句柄传给调用方。
//
// 用法：
//
//	err := service.InTx(ctx, store, func(tx Tx) error {
//	    if err := tx.Identity().PutAgent(ctx, rec); err != nil { return err }
//	    return tx.Permission().SetPermissions(ctx, rec.UUID, bits)
//	})
package service

import (
	"context"
	"errors"

	"github.com/agentid-chain/agentid-chain/internal/storage"
)

// ErrTxNotSupported 当前 L1 Client 不支持事务（如缓存层）。
var ErrTxNotSupported = errors.New("service: storage client does not support transactions")

// Tx 事务句柄抽象（与 ent.Tx / pgx.Tx 解耦）。
//
// 实现策略：L4 不直接拿底层 *sql.Tx；通过本接口委托 L1 决定如何传事务。
// L1 Client 需实现 Txable 接口以支持事务。
type Tx interface {
	Identity() storage.IdentityStore
	Permission() storage.PermissionStore
	Audit() storage.AuditStore
	Nonce() storage.NonceStore
	Revocation() storage.RevocationStore
	Cache() storage.CacheStore
}

// TxFunc 事务内执行体。
type TxFunc func(tx Tx) error

// Txable 可事务化的 L1 Client。
type Txable interface {
	InTx(ctx context.Context, fn func(tx Tx) error) error
}

// InTx 在事务中执行 fn。
//
// 若 store 实现了 Txable，则委托之；否则退化为 InTxNoop（无事务，
// 适用于纯缓存/无状态场景）。
func InTx(ctx context.Context, store storage.Client, fn TxFunc) error {
	if tx, ok := store.(Txable); ok {
		return tx.InTx(ctx, fn)
	}
	// 无事务支持：直接调用（生产 PG 实现必须支持 Txable）
	return fn(noopTx{store: store})
}

// noopTx 退化实现（无真实事务边界，仅作单步调用透传）。
type noopTx struct {
	store storage.Client
}

func (n noopTx) Identity() storage.IdentityStore    { return n.store.Identity() }
func (n noopTx) Permission() storage.PermissionStore { return n.store.Permission() }
func (n noopTx) Audit() storage.AuditStore          { return n.store.Audit() }
func (n noopTx) Nonce() storage.NonceStore          { return n.store.Nonce() }
func (n noopTx) Revocation() storage.RevocationStore { return n.store.Revocation() }
func (n noopTx) Cache() storage.CacheStore          { return n.store.Cache() }
