// Package service: GetAgentInfo 工作流（P6.7）。
//
// 业务用例：读 agent 完整信息（用于 L5 网关响应 / 调试 / 监管查询）。
//
// 设计：
//   - 优先 L1 读（带 cache）
//   - 链路：GetAgent → 拼装 view（聚合 permission / state / public key / 链上状态）
//   - 不存在 → ErrAgentNotFound
package service

import (
	"context"
	"errors"
	"time"

	"github.com/agentid-chain/agentid-chain/internal/domain"
	"github.com/agentid-chain/agentid-chain/internal/storage"
)

// AgentInfo 完整 agent 视图（API 响应）。
type AgentInfo struct {
	UUID         string
	Owner        string
	Level        domain.LevelType
	State        domain.AgentState
	Permissions  uint64
	PublicKey    string
	TxHash       string
	RegisteredAt time.Time
	UpdatedAt    time.Time
	ChainState   *ChainAgentState // 可选，链上状态（如调用 GetAgentState）
}

// GetAgentInfoService 读工作流。
type GetAgentInfoService struct {
	store    storage.Client
	chain    ChainAdapter // 可选，nil = 不查链上
	provider IdentityProvider
}

// NewGetAgentInfoService 构造。
func NewGetAgentInfoService(store storage.Client, chain ChainAdapter, provider IdentityProvider) (*GetAgentInfoService, error) {
	if store == nil {
		return nil, errors.New("service: nil storage")
	}
	if provider == nil {
		return nil, errors.New("service: nil identity provider")
	}
	return &GetAgentInfoService{store: store, chain: chain, provider: provider}, nil
}

// HandleGetInfo 读取 agent 完整信息。
func (s *GetAgentInfoService) HandleGetInfo(ctx context.Context, uuid domain.UUID) (*AgentInfo, error) {
	if uuid.IsZero() {
		return nil, ErrInvalidRegisterInput
	}

	rec, err := s.store.Identity().GetAgent(ctx, uuid.String())
	if err != nil {
		return nil, ErrAgentNotFound
	}
	perms, err := s.store.Permission().GetPermissions(ctx, uuid.String())
	if err != nil {
		return nil, err
	}

	info := &AgentInfo{
		UUID:         rec.UUID,
		Owner:        rec.Owner,
		Level:        domain.LevelType(rec.Level),
		State:        domain.AgentState(rec.State),
		Permissions:  perms,
		PublicKey:    rec.PublicKey,
		TxHash:       rec.TxHash,
		RegisteredAt: rec.RegisteredAt,
		UpdatedAt:    rec.UpdatedAt,
	}

	// 可选：链上状态（用于对账展示）
	if s.chain != nil {
		cs, err := s.chain.GetAgentState(ctx, uuid)
		if err == nil {
			info.ChainState = cs
		}
		// 链上查询失败不阻塞 L1 数据
	}

	return info, nil
}
