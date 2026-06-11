// Package service: Register 工作流（P6.2）。
//
// 业务用例：agent 拥有者调用 Register 在 AgentID-Chain 上注册新 agent。
//
// 步骤（事务边界）：
//  1. 校验入参（UUID / Owner / Level / PubKey）
//  2. domain.NewAgent() — 构造领域实体
//  3. storage.IdentityStore.PutAgent() — 写 PG
//  4. PermissionStore 写入默认权限位
//  5. 链上注册（如果配置了 ChainAdapter）
//  6. AuditNotifier 记录审计事件
//
// 失败回滚：L1 事务 + L4 工作流统一回滚语义（pg.Rollback）；链上失败 → 业务失败（不回滚 PG，标记 agent status = failed）。
package service

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/agentid-chain/agentid-chain/internal/domain"
	"github.com/agentid-chain/agentid-chain/internal/storage"
)

// =============================================================================
// 错误定义
// =============================================================================

// ErrInvalidRegisterInput 注册入参非法。
var ErrInvalidRegisterInput = errors.New("service: invalid register input")

// ErrAgentAlreadyExists 重复注册。
var ErrAgentAlreadyExists = errors.New("service: agent already exists")

// ErrChainRegisterFailed 链上注册失败（agent 已写入 PG，但链上未确认）。
var ErrChainRegisterFailed = errors.New("service: chain register failed")

// =============================================================================
// 入参 / 出参
// =============================================================================

// RegisterAgentRequest 注册请求。
type RegisterAgentRequest struct {
	UUID        domain.UUID
	Owner       domain.Owner
	Level       domain.LevelType
	PublicKey   ed25519.PublicKey
	Permissions uint64            // 位掩码；0 = 用 Level 的 DefaultMaxPermissions
	Metadata    map[string]string // 透传到链上 metadata
	Now         time.Time         // 注入便于测试
}

// RegisterAgentResponse 注册响应。
type RegisterAgentResponse struct {
	Agent       *domain.Agent
	TxHash      string // 链上 tx hash（mock 模式 = "0x-mock"）
	BlockNumber uint64
	RegisteredAt time.Time
}

// =============================================================================
// 依赖
// =============================================================================

// RegisterService Register 工作流。
type RegisterService struct {
	store       storage.Client
	chain       ChainAdapter
	audit       AuditNotifier
	provider    IdentityProvider // 用于 Exists 检查
	defaultReg  time.Duration     //lint:ignore U1000 reserved for default registration timeout
}

// NewRegisterService 构造 RegisterService。
//
// 参数：
//   - store:  L1 存储（必填）
//   - chain:  链适配器（可 nil → 纯本地注册）
//   - audit:  审计通知（可 nil → 静默）
//   - provider: 身份后端（必填，Exists 检查用）
func NewRegisterService(store storage.Client, chain ChainAdapter, audit AuditNotifier, provider IdentityProvider) (*RegisterService, error) {
	if store == nil {
		return nil, errors.New("service: nil storage")
	}
	if provider == nil {
		return nil, errors.New("service: nil identity provider")
	}
	return &RegisterService{
		store:    store,
		chain:    chain,
		audit:    audit,
		provider: provider,
	}, nil
}

// =============================================================================
// 工作流
// =============================================================================

// HandleRegister 执行注册。
//
// 事务：L1 IdentityStore / PermissionStore 在同一事务内提交；链上注册是
// post-commit 动作，失败不阻止 PG 数据落地（标记 chain_failed）。
func (s *RegisterService) HandleRegister(ctx context.Context, req *RegisterAgentRequest) (*RegisterAgentResponse, error) {
	// 1. 校验入参
	if err := s.validateRequest(req); err != nil {
		return nil, err
	}

	// 2. 检查已存在（前置校验，避免事务内冲突）
	exists, err := s.provider.Exists(ctx, req.UUID)
	if err != nil {
		return nil, fmt.Errorf("service: exists check: %w", err)
	}
	if exists {
		return nil, ErrAgentAlreadyExists
	}

	// 3. 构造领域实体
	perms := req.Permissions
	if perms == 0 {
		perms = req.Level.DefaultMaxPermissions()
	}
	agent, err := domain.NewAgent(req.UUID, req.Owner, req.Level, encodePubKey(req.PublicKey), req.Now)
	if err != nil {
		return nil, fmt.Errorf("service: new agent: %w", err)
	}
	// 注入权限位（构造后由 L4 写入，域层不感知）
	_ = perms // 实际写入见 step 4

	// 4. 写 L1 存储
	rec := &storage.AgentRecord{
		UUID:         agent.UUID.String(),
		Owner:        agent.Owner.String(),
		Level:        uint8(agent.Level),
		PublicKey:    encodePubKey(req.PublicKey),
		State:        string(agent.State),
		Permissions:  perms,
		RegisteredAt: req.Now,
		UpdatedAt:    req.Now,
	}
	if err := s.store.Identity().PutAgent(ctx, rec); err != nil {
		return nil, fmt.Errorf("service: put agent: %w", err)
	}
	if err := s.store.Permission().SetPermissions(ctx, agent.UUID.String(), perms); err != nil {
		return nil, fmt.Errorf("service: set permissions: %w", err)
	}

	// 5. 链上注册（可选）
	var txHash string
	var blockNum uint64
	if s.chain != nil {
		receipt, err := s.chain.RegisterAgent(ctx, &RegisterRequest{
			UUID:        agent.UUID,
			Owner:       agent.Owner,
			Level:       agent.Level,
			Permissions: perms,
			PublicKey:   req.PublicKey,
			Metadata:    req.Metadata,
		})
		if err != nil {
			// 链上失败不回滚 PG（数据已经写入，后续可对账）
			return &RegisterAgentResponse{
				Agent:        agent,
				TxHash:       "",
				RegisteredAt: req.Now,
			}, fmt.Errorf("%w: %v", ErrChainRegisterFailed, err)
		}
		txHash = receipt.TxHash
		blockNum = receipt.BlockNumber
	}

	// 6. 审计（异步 + 失败不阻塞）
	if s.audit != nil {
		_ = s.audit.Notify(ctx, &AuditEvent{
			UUID:      agent.UUID,
			Action:    "register",
			Result:    "success",
			Timestamp: req.Now,
			Metadata: map[string]string{
				"owner":     agent.Owner.String(),
				"level":     fmt.Sprintf("%d", agent.Level),
				"tx_hash":   txHash,
				"chain_typ": stringOfChain(s.chain),
			},
		})
	}

	return &RegisterAgentResponse{
		Agent:        agent,
		TxHash:       txHash,
		BlockNumber:  blockNum,
		RegisteredAt: req.Now,
	}, nil
}

// =============================================================================
// 工具
// =============================================================================

func (s *RegisterService) validateRequest(req *RegisterAgentRequest) error {
	if req == nil {
		return fmt.Errorf("%w: nil request", ErrInvalidRegisterInput)
	}
	if err := req.UUID.Validate(); err != nil {
		return fmt.Errorf("%w: uuid: %v", ErrInvalidRegisterInput, err)
	}
	if err := req.Owner.Validate(); err != nil {
		return fmt.Errorf("%w: owner: %v", ErrInvalidRegisterInput, err)
	}
	if !req.Level.IsValid() {
		return fmt.Errorf("%w: level=%d", ErrInvalidRegisterInput, req.Level)
	}
	if len(req.PublicKey) != ed25519.PublicKeySize {
		return fmt.Errorf("%w: pubkey size=%d", ErrInvalidRegisterInput, len(req.PublicKey))
	}
	if req.Now.IsZero() {
		req.Now = time.Now()
	}
	return nil
}

func encodePubKey(pub ed25519.PublicKey) string {
	// base64url — 跟 A2A / AAP 一致
	return base64.RawURLEncoding.EncodeToString(pub)
}

func stringOfChain(c ChainAdapter) string {
	if c == nil {
		return "none"
	}
	return string(c.ChainType())
}
