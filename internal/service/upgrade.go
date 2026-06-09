// Package service: Upgrade 工作流（P6.3）。
//
// 业务用例：agent 拥有者升级 agent 的等级（Level），同步更新 L1 + 链上。
//
// 步骤：
//  1. 校验入参（UUID / NewLevel / Reason）
//  2. 加载 agent（不存在 → ErrAgentNotFound）
//  3. 状态校验：必须是 active（不能从 banned / unregistered 升级）
//  4. domain.Agent.Upgrade(newLevel, now) — 域状态机
//  5. 写 L1
//  6. 链上 UpdateLevel（可选，失败不回滚 PG）
//  7. 审计
//
// 等级规则（v2.0.1 §4.2）：每次只能升 1 级（Test → Basic → Advanced → Pro）。
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/agentid-chain/agentid-chain/internal/domain"
	"github.com/agentid-chain/agentid-chain/internal/storage"
)

// =============================================================================
// 错误
// =============================================================================

// ErrAgentNotFound 升级时找不到 agent。
var ErrAgentNotFound = errors.New("service: agent not found")

// ErrAgentNotUpgradable agent 状态不允许升级。
var ErrAgentNotUpgradable = errors.New("service: agent not in upgradable state")

// ErrInvalidUpgradeLevel 等级非法（不递增、跨级、未知）。
var ErrInvalidUpgradeLevel = errors.New("service: invalid upgrade level")

// =============================================================================
// 入参 / 出参
// =============================================================================

// UpgradeAgentRequest 升级请求。
type UpgradeAgentRequest struct {
	UUID     domain.UUID
	NewLevel domain.LevelType
	Reason   string
	Actor    string // 谁触发（user DID / "system:auto-promote"）
	Now      time.Time
}

// UpgradeAgentResponse 升级响应。
type UpgradeAgentResponse struct {
	Agent       *domain.Agent
	OldLevel    domain.LevelType
	NewLevel    domain.LevelType
	TxHash      string
	UpgradedAt  time.Time
}

// =============================================================================
// 依赖
// =============================================================================

// UpgradeService 升级工作流。
type UpgradeService struct {
	store    storage.Client
	chain    ChainAdapter
	audit    AuditNotifier
	provider IdentityProvider
}

// NewUpgradeService 构造。
func NewUpgradeService(store storage.Client, chain ChainAdapter, audit AuditNotifier, provider IdentityProvider) (*UpgradeService, error) {
	if store == nil {
		return nil, errors.New("service: nil storage")
	}
	if provider == nil {
		return nil, errors.New("service: nil identity provider")
	}
	return &UpgradeService{store: store, chain: chain, audit: audit, provider: provider}, nil
}

// =============================================================================
// 工作流
// =============================================================================

// HandleUpgrade 升级。
func (s *UpgradeService) HandleUpgrade(ctx context.Context, req *UpgradeAgentRequest) (*UpgradeAgentResponse, error) {
	// 1. 校验
	if err := s.validateRequest(req); err != nil {
		return nil, err
	}

	// 2. 加载 agent
	rec, err := s.store.Identity().GetAgent(ctx, req.UUID.String())
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAgentNotFound, err)
	}

	// 3. 状态校验
	state := domain.AgentState(rec.State)
	if !isUpgradable(state) {
		return nil, fmt.Errorf("%w: state=%s", ErrAgentNotUpgradable, state)
	}

	// 4. 构造域实体 + 状态机升级
	agent, err := domain.NewAgent(domain.UUID(rec.UUID), domain.Owner(rec.Owner), domain.LevelType(rec.Level), rec.PublicKey, rec.RegisteredAt)
	if err != nil {
		return nil, fmt.Errorf("service: rebuild agent: %w", err)
	}
	oldLevel := agent.Level
	if err := agent.Upgrade(req.NewLevel, req.Now); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidUpgradeLevel, err)
	}

	// 5. 写 L1
	rec.Level = uint8(agent.Level)
	rec.UpdatedAt = req.Now
	if err := s.store.Identity().PutAgent(ctx, rec); err != nil {
		return nil, fmt.Errorf("service: put agent: %w", err)
	}
	// 权限位重算（按新 Level 的 DefaultMaxPermissions 重新设置，
	// 但保留原 perms 的高位 — 实际业务可配置）
	_ = s.store.Permission().SetPermissions(ctx, rec.UUID, req.NewLevel.DefaultMaxPermissions())

	// 6. 链上（可选）
	var txHash string
	if s.chain != nil {
		receipt, err := s.chain.UpdateLevel(ctx, agent.UUID, agent.Level, req.Reason)
		if err != nil {
			return &UpgradeAgentResponse{
				Agent: agent, OldLevel: oldLevel, NewLevel: agent.Level,
				UpgradedAt: req.Now,
			}, fmt.Errorf("%w: %v", ErrChainRegisterFailed, err)
		}
		txHash = receipt.TxHash
	}

	// 7. 审计
	if s.audit != nil {
		_ = s.audit.Notify(ctx, &AuditEvent{
			UUID:      agent.UUID,
			Action:    "upgrade",
			Actor:     req.Actor,
			Result:    "success",
			Timestamp: req.Now,
			Metadata: map[string]string{
				"old_level": fmt.Sprintf("%d", oldLevel),
				"new_level": fmt.Sprintf("%d", agent.Level),
				"reason":    req.Reason,
				"tx_hash":   txHash,
			},
		})
	}

	return &UpgradeAgentResponse{
		Agent:      agent,
		OldLevel:   oldLevel,
		NewLevel:   agent.Level,
		TxHash:     txHash,
		UpgradedAt: req.Now,
	}, nil
}

// =============================================================================
// 工具
// =============================================================================

func (s *UpgradeService) validateRequest(req *UpgradeAgentRequest) error {
	if req == nil {
		return fmt.Errorf("%w: nil", ErrInvalidUpgradeLevel)
	}
	if err := req.UUID.Validate(); err != nil {
		return fmt.Errorf("%w: uuid: %v", ErrInvalidUpgradeLevel, err)
	}
	if !req.NewLevel.IsValid() {
		return fmt.Errorf("%w: level=%d", ErrInvalidUpgradeLevel, req.NewLevel)
	}
	if req.Now.IsZero() {
		req.Now = time.Now()
	}
	return nil
}

// isUpgradable 只有 active 状态可升级。
func isUpgradable(s domain.AgentState) bool {
	return s == domain.StateActive || s == domain.StateRegistered
}
