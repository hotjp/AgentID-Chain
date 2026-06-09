// Package service: CheckPermission 工作流（P6.8）。
//
// 业务用例：检查 agent 是否拥有某个权限位（位掩码 RBAC）。
//
// 用途：
//   - L5 网关前置鉴权（被 authz/ 包装）
//   - 业务代码内调用（agent 是否有 X 权限）
//
// 设计：纯读，无副作用。
package service

import (
	"context"
	"errors"

	"github.com/agentid-chain/agentid-chain/internal/domain"
	"github.com/agentid-chain/agentid-chain/internal/storage"
)

// CheckPermissionRequest 校验请求。
type CheckPermissionRequest struct {
	UUID  domain.UUID
	Bit   uint // 位索引（0-63）
}

// CheckPermissionResponse 校验响应。
type CheckPermissionResponse struct {
	Allowed     bool
	HasBit      bool
	Permissions uint64
}

// CheckPermissionService 权限校验。
type CheckPermissionService struct {
	store    storage.Client
	provider IdentityProvider
}

// NewCheckPermissionService 构造。
func NewCheckPermissionService(store storage.Client, provider IdentityProvider) (*CheckPermissionService, error) {
	if store == nil {
		return nil, errors.New("service: nil storage")
	}
	if provider == nil {
		return nil, errors.New("service: nil identity provider")
	}
	return &CheckPermissionService{store: store, provider: provider}, nil
}

// HandleCheckPermission 校验。
//
// 决策：
//   - agent 不存在 → Allowed=false
//   - state=unregistered / banned → Allowed=false
//   - perms & (1<<Bit) != 0 → Allowed=true
//   - else → Allowed=false
func (s *CheckPermissionService) HandleCheckPermission(ctx context.Context, req *CheckPermissionRequest) (*CheckPermissionResponse, error) {
	if req == nil || req.UUID.IsZero() {
		return nil, ErrInvalidRegisterInput
	}
	if req.Bit >= 64 {
		return nil, errors.New("service: bit >= 64")
	}

	// 先加载 agent 看状态
	rec, err := s.store.Identity().GetAgent(ctx, req.UUID.String())
	if err != nil {
		return &CheckPermissionResponse{Allowed: false}, ErrAgentNotFound
	}
	// 状态门控
	if rec.State == string(domain.StateUnregistered) || rec.State == string(domain.StateBanned) {
		return &CheckPermissionResponse{Allowed: false, Permissions: 0}, nil
	}

	perms, err := s.store.Permission().GetPermissions(ctx, req.UUID.String())
	if err != nil {
		return nil, err
	}
	mask := uint64(1) << req.Bit
	hasBit := perms&mask != 0

	return &CheckPermissionResponse{
		Allowed:     hasBit,
		HasBit:      hasBit,
		Permissions: perms,
	}, nil
}
