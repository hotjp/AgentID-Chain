// Package service: Ban/Unban 工作流（P6.6）。
//
// 业务用例：合规要求临时封禁 agent；可解封。
//
// 与 Revoke 的区别：
//   - Ban = 临时封禁（state=banned，可 Unban 回到 active）
//   - Revoke = 永久撤销（state=unregistered，不可恢复）
//
// 步骤：
//  1. 校验
//  2. L1 GetAgent
//  3. Ban / Unban 状态机
//  4. 写 L1
//  5. 链上 BanAgent / UnbanAgent
//  6. 撤销该 agent 名下所有未过期 A2A Token（让封禁立即生效）
//  7. 审计
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/agentid-chain/agentid-chain/internal/domain"
	"github.com/agentid-chain/agentid-chain/internal/storage"
)

// ErrAgentAlreadyBanned 重复封禁。
var ErrAgentAlreadyBanned = errors.New("service: agent already banned")

// ErrAgentNotBanned 解封时未在封禁态。
var ErrAgentNotBanned = errors.New("service: agent not banned")

// BanAgentRequest 封禁请求。
type BanAgentRequest struct {
	UUID      domain.UUID
	Reason    string
	Actor     string
	Now       time.Time
}

// UnbanAgentRequest 解封请求。
type UnbanAgentRequest struct {
	UUID  domain.UUID
	Actor string
	Now   time.Time
}

// BanAgentResponse 封禁响应。
type BanAgentResponse struct {
	Agent    *domain.Agent
	BannedAt time.Time
	TxHash   string
}

// UnbanAgentResponse 解封响应。
type UnbanAgentResponse struct {
	Agent       *domain.Agent
	UnbannedAt  time.Time
	TxHash      string
}

// BanService 封禁/解封工作流。
type BanService struct {
	store    storage.Client
	chain    ChainAdapter
	audit    AuditNotifier
	provider IdentityProvider
}

// NewBanService 构造。
func NewBanService(store storage.Client, chain ChainAdapter, audit AuditNotifier, provider IdentityProvider) (*BanService, error) {
	if store == nil {
		return nil, errors.New("service: nil storage")
	}
	if provider == nil {
		return nil, errors.New("service: nil identity provider")
	}
	return &BanService{store: store, chain: chain, audit: audit, provider: provider}, nil
}

// HandleBan 封禁。
func (s *BanService) HandleBan(ctx context.Context, req *BanAgentRequest) (*BanAgentResponse, error) {
	if req == nil || req.UUID.IsZero() {
		return nil, fmt.Errorf("%w: nil/empty", ErrInvalidRegisterInput)
	}
	if req.Now.IsZero() {
		req.Now = time.Now()
	}

	rec, err := s.store.Identity().GetAgent(ctx, req.UUID.String())
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAgentNotFound, err)
	}

	agent, err := domain.NewAgent(domain.UUID(rec.UUID), domain.Owner(rec.Owner), domain.LevelType(rec.Level), rec.PublicKey, rec.RegisteredAt)
	if err != nil {
		return nil, fmt.Errorf("service: rebuild: %w", err)
	}
	if agent.State == domain.StateBanned {
		return nil, ErrAgentAlreadyBanned
	}
	if err := agent.Ban(req.Reason, req.Now); err != nil {
		return nil, fmt.Errorf("service: ban: %w", err)
	}

	rec.State = string(agent.State)
	rec.UpdatedAt = req.Now
	if err := s.store.Identity().PutAgent(ctx, rec); err != nil {
		return nil, fmt.Errorf("service: put: %w", err)
	}

	var txHash string
	if s.chain != nil {
		receipt, err := s.chain.BanAgent(ctx, agent.UUID, req.Reason)
		if err != nil {
			return &BanAgentResponse{Agent: agent, BannedAt: req.Now},
				fmt.Errorf("%w: %v", ErrChainRegisterFailed, err)
		}
		txHash = receipt.TxHash
	}

	// 撤销所有 A2A Token（封禁立即生效）
	_, _ = s.store.Revocation().PurgeExpired(ctx)

	if s.audit != nil {
		_ = s.audit.Notify(ctx, &AuditEvent{
			UUID:      agent.UUID,
			Action:    "ban",
			Actor:     req.Actor,
			Result:    "success",
			Timestamp: req.Now,
			Metadata:  map[string]string{"reason": req.Reason, "tx_hash": txHash},
		})
	}

	return &BanAgentResponse{Agent: agent, BannedAt: req.Now, TxHash: txHash}, nil
}

// HandleUnban 解封。
func (s *BanService) HandleUnban(ctx context.Context, req *UnbanAgentRequest) (*UnbanAgentResponse, error) {
	if req == nil || req.UUID.IsZero() {
		return nil, fmt.Errorf("%w: nil/empty", ErrInvalidRegisterInput)
	}
	if req.Now.IsZero() {
		req.Now = time.Now()
	}

	rec, err := s.store.Identity().GetAgent(ctx, req.UUID.String())
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAgentNotFound, err)
	}

	agent, err := domain.NewAgent(domain.UUID(rec.UUID), domain.Owner(rec.Owner), domain.LevelType(rec.Level), rec.PublicKey, rec.RegisteredAt)
	if err != nil {
		return nil, fmt.Errorf("service: rebuild: %w", err)
	}
	if agent.State != domain.StateBanned {
		return nil, ErrAgentNotBanned
	}
	if err := agent.Unban(req.Now); err != nil {
		return nil, fmt.Errorf("service: unban: %w", err)
	}

	rec.State = string(agent.State)
	rec.UpdatedAt = req.Now
	if err := s.store.Identity().PutAgent(ctx, rec); err != nil {
		return nil, fmt.Errorf("service: put: %w", err)
	}

	var txHash string
	if s.chain != nil {
		receipt, err := s.chain.UnbanAgent(ctx, agent.UUID)
		if err != nil {
			return &UnbanAgentResponse{Agent: agent, UnbannedAt: req.Now},
				fmt.Errorf("%w: %v", ErrChainRegisterFailed, err)
		}
		txHash = receipt.TxHash
	}

	if s.audit != nil {
		_ = s.audit.Notify(ctx, &AuditEvent{
			UUID:      agent.UUID,
			Action:    "unban",
			Actor:     req.Actor,
			Result:    "success",
			Timestamp: req.Now,
			Metadata:  map[string]string{"tx_hash": txHash},
		})
	}

	return &UnbanAgentResponse{Agent: agent, UnbannedAt: req.Now, TxHash: txHash}, nil
}
