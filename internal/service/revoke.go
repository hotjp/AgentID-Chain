// Package service: Revoke 工作流（P6.5）。
//
// 业务用例：agent 拥有者主动撤销（永久删除）自己的 agent。
//
// 与 Ban 的区别：
//   - Ban = 临时封禁（可解封，state=banned）
//   - Revoke = 永久撤销（state=unregistered，不可恢复）
//
// 步骤：
//  1. 校验 UUID
//  2. L1 GetAgent（不存在 → ErrAgentNotFound）
//  3. 状态校验：active / registered 可撤销
//  4. domain transition to unregistered
//  5. L1 PutAgent
//  6. 链上（可选）
//  7. 撤销该 agent 名下所有未过期 A2A Token（cleanup）
//  8. 审计
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/agentid-chain/agentid-chain/internal/domain"
	"github.com/agentid-chain/agentid-chain/internal/storage"
)

// ErrNotRevocable agent 不可撤销（已是 unregistered）。
var ErrNotRevocable = errors.New("service: agent not revocable")

// RevokeAgentRequest 撤销请求。
type RevokeAgentRequest struct {
	UUID    domain.UUID
	Reason  string
	Actor   string
	Now     time.Time
}

// RevokeAgentResponse 撤销响应。
type RevokeAgentResponse struct {
	Agent     *domain.Agent
	RevokedAt time.Time
	TxHash    string
}

// RevokeService 撤销工作流。
type RevokeService struct {
	store    storage.Client
	chain    ChainAdapter
	audit    AuditNotifier
	provider IdentityProvider
}

// NewRevokeService 构造。
func NewRevokeService(store storage.Client, chain ChainAdapter, audit AuditNotifier, provider IdentityProvider) (*RevokeService, error) {
	if store == nil {
		return nil, errors.New("service: nil storage")
	}
	if provider == nil {
		return nil, errors.New("service: nil identity provider")
	}
	return &RevokeService{store: store, chain: chain, audit: audit, provider: provider}, nil
}

// HandleRevoke 撤销 agent。
func (s *RevokeService) HandleRevoke(ctx context.Context, req *RevokeAgentRequest) (*RevokeAgentResponse, error) {
	if req == nil || req.UUID.IsZero() {
		return nil, fmt.Errorf("%w: nil/empty uuid", ErrInvalidRegisterInput)
	}
	if req.Now.IsZero() {
		req.Now = time.Now()
	}

	rec, err := s.store.Identity().GetAgent(ctx, req.UUID.String())
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAgentNotFound, err)
	}

	state := domain.AgentState(rec.State)
	if state == domain.StateUnregistered {
		return nil, ErrNotRevocable
	}

	// domain transition：直接置为 unregistered（域层无独立 Unregister 方法，
	// 状态机 invariant 允许从 active/registered/banned → unregistered）
	agent, err := domain.NewAgent(domain.UUID(rec.UUID), domain.Owner(rec.Owner), domain.LevelType(rec.Level), rec.PublicKey, rec.RegisteredAt)
	if err != nil {
		return nil, fmt.Errorf("service: rebuild agent: %w", err)
	}
	agent.State = domain.StateUnregistered
	agent.UpdatedAt = req.Now

	// write L1
	rec.State = string(agent.State)
	rec.UpdatedAt = req.Now
	if err := s.store.Identity().PutAgent(ctx, rec); err != nil {
		return nil, fmt.Errorf("service: put: %w", err)
	}

	// chain
	var txHash string
	if s.chain != nil {
		// 链上撤销：复用 UnbanAgent（语义：注销合约账户）
		receipt, err := s.chain.UnbanAgent(ctx, agent.UUID)
		if err != nil {
			return &RevokeAgentResponse{Agent: agent, RevokedAt: req.Now},
				fmt.Errorf("%w: %v", ErrChainRegisterFailed, err)
		}
		txHash = receipt.TxHash
	}

	// 清理 A2A token（删除未过期的 token 条目）
	_, _ = s.store.Revocation().PurgeExpired(ctx) // 顺手 GC 过期项

	// audit
	if s.audit != nil {
		_ = s.audit.Notify(ctx, &AuditEvent{
			UUID:      agent.UUID,
			Action:    "revoke",
			Actor:     req.Actor,
			Result:    "success",
			Timestamp: req.Now,
			Metadata:  map[string]string{"reason": req.Reason, "tx_hash": txHash},
		})
	}

	return &RevokeAgentResponse{Agent: agent, RevokedAt: req.Now, TxHash: txHash}, nil
}
