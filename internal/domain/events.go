// Package domain 领域事件（Domain Events）。
//
// 4 个核心事件（与 docs §3 状态机对齐）：
//   - AgentRegisteredV1  Agent 注册成功（首次）
//   - AgentUpgradedV1    Agent 升级（level/perms 变化）
//   - AgentBannedV1      Agent 封禁
//   - AgentRevokedV1     Agent 撤销（终态）
//
// 设计：
//   - V1 命名（带版本号；未来加字段时升 V2）
//   - 事件是不可变的 value object
//   - 序列化/反序列化由 L4 Service 提供（domain 不引入 encoding/json 依赖）
//   - 事件发布走 L1 Storage Outbox（事务性发件箱）
package domain

import (
	"errors"
	"fmt"
	"time"
)

// =============================================================================
// 事件类型常量
// =============================================================================

// EventType 事件类型（与 outbox_events.event_type 字段对应）。
const (
	EventAgentRegisteredV1   = "agent.registered.v1"
	EventAgentUpgradedV1     = "agent.upgraded.v1"
	EventAgentBannedV1       = "agent.banned.v1"
	EventAgentRevokedV1      = "agent.revoked.v1"
	EventAgentUnbannedV1     = "agent.unbanned.v1"
	EventPermissionGrantedV1 = "permission.granted.v1"
	EventPermissionRevokedV1 = "permission.revoked.v1"
)

// validEventTypes 合法事件白名单。
var validEventTypes = map[string]struct{}{
	EventAgentRegisteredV1:   {},
	EventAgentUpgradedV1:     {},
	EventAgentBannedV1:       {},
	EventAgentRevokedV1:      {},
	EventAgentUnbannedV1:     {},
	EventPermissionGrantedV1: {},
	EventPermissionRevokedV1: {},
}

// IsValidEventType 校验事件类型是否合法。
func IsValidEventType(t string) bool {
	_, ok := validEventTypes[t]
	return ok
}

// =============================================================================
// 错误定义
// =============================================================================

// ErrInvalidEventType 事件类型不在白名单。
var ErrInvalidEventType = errors.New("domain: invalid event type")

// ErrEventVersionMismatch 事件版本不匹配。
var ErrEventVersionMismatch = errors.New("domain: event version mismatch")

// =============================================================================
// 事件值对象（不可变）
// =============================================================================

// DomainEvent 领域事件通用字段。
type DomainEvent struct {
	EventID     string    // 事件唯一 ID（ULID / UUIDv7）
	EventType   string    // 事件类型（EventAgentRegisteredV1 等）
	OccurredAt  time.Time // 发生时间
	AgentUUID   UUID      // 聚合根 ID
	OperatorDID string    // 操作者 DID
}

// AgentRegisteredEventV1 注册事件。
type AgentRegisteredEventV1 struct {
	DomainEvent
	OwnerDID string    // 拥有者 DID
	Level    LevelType // 初始等级
}

// AgentUpgradedEventV1 升级事件。
type AgentUpgradedEventV1 struct {
	DomainEvent
	OldLevel LevelType
	NewLevel LevelType
	OldPerms uint64
	NewPerms uint64
	Reason   string
}

// AgentBannedEventV1 封禁事件。
type AgentBannedEventV1 struct {
	DomainEvent
	Reason       string
	BanExpiresAt *time.Time // 可选（临时封禁）
}

// AgentRevokedEventV1 撤销事件。
type AgentRevokedEventV1 struct {
	DomainEvent
	Reason string
}

// =============================================================================
// 事件构造器（含校验）
// =============================================================================

// NewAgentRegisteredEventV1 构造注册事件。
func NewAgentRegisteredEventV1(
	eventID string,
	agentUUID UUID,
	ownerDID string,
	level LevelType,
	operatorDID string,
	now time.Time,
) (*AgentRegisteredEventV1, error) {
	if err := baseEventCheck(eventID, EventAgentRegisteredV1, agentUUID, operatorDID); err != nil {
		return nil, err
	}
	if ownerDID == "" {
		return nil, errors.New("domain: owner_did is empty")
	}
	if !level.IsValid() {
		return nil, fmt.Errorf("%w: level=%d", ErrInvalidLevel, uint8(level))
	}
	return &AgentRegisteredEventV1{
		DomainEvent: DomainEvent{
			EventID:     eventID,
			EventType:   EventAgentRegisteredV1,
			OccurredAt:  now,
			AgentUUID:   agentUUID,
			OperatorDID: operatorDID,
		},
		OwnerDID: ownerDID,
		Level:    level,
	}, nil
}

// NewAgentUpgradedEventV1 构造升级事件。
func NewAgentUpgradedEventV1(
	eventID string,
	agentUUID UUID,
	oldLevel, newLevel LevelType,
	oldPerms, newPerms uint64,
	reason, operatorDID string,
	now time.Time,
) (*AgentUpgradedEventV1, error) {
	if err := baseEventCheck(eventID, EventAgentUpgradedV1, agentUUID, operatorDID); err != nil {
		return nil, err
	}
	if !oldLevel.IsValid() || !newLevel.IsValid() {
		return nil, fmt.Errorf("%w: level", ErrInvalidLevel)
	}
	if newLevel <= oldLevel {
		return nil, fmt.Errorf("%w: %s → %s not upgrade", ErrInvalidLevel, oldLevel, newLevel)
	}
	return &AgentUpgradedEventV1{
		DomainEvent: DomainEvent{
			EventID:     eventID,
			EventType:   EventAgentUpgradedV1,
			OccurredAt:  now,
			AgentUUID:   agentUUID,
			OperatorDID: operatorDID,
		},
		OldLevel: oldLevel,
		NewLevel: newLevel,
		OldPerms: oldPerms,
		NewPerms: newPerms,
		Reason:   reason,
	}, nil
}

// NewAgentBannedEventV1 构造封禁事件。
func NewAgentBannedEventV1(
	eventID string,
	agentUUID UUID,
	reason, operatorDID string,
	banExpiresAt *time.Time,
	now time.Time,
) (*AgentBannedEventV1, error) {
	if err := baseEventCheck(eventID, EventAgentBannedV1, agentUUID, operatorDID); err != nil {
		return nil, err
	}
	if reason == "" {
		return nil, errors.New("domain: ban reason is empty")
	}
	return &AgentBannedEventV1{
		DomainEvent: DomainEvent{
			EventID:     eventID,
			EventType:   EventAgentBannedV1,
			OccurredAt:  now,
			AgentUUID:   agentUUID,
			OperatorDID: operatorDID,
		},
		Reason:       reason,
		BanExpiresAt: banExpiresAt,
	}, nil
}

// NewAgentRevokedEventV1 构造撤销事件。
func NewAgentRevokedEventV1(
	eventID string,
	agentUUID UUID,
	reason, operatorDID string,
	now time.Time,
) (*AgentRevokedEventV1, error) {
	if err := baseEventCheck(eventID, EventAgentRevokedV1, agentUUID, operatorDID); err != nil {
		return nil, err
	}
	return &AgentRevokedEventV1{
		DomainEvent: DomainEvent{
			EventID:     eventID,
			EventType:   EventAgentRevokedV1,
			OccurredAt:  now,
			AgentUUID:   agentUUID,
			OperatorDID: operatorDID,
		},
		Reason: reason,
	}, nil
}

// baseEventCheck 共享事件字段校验。
func baseEventCheck(eventID, eventType string, agentUUID UUID, operatorDID string) error {
	if eventID == "" {
		return errors.New("domain: event_id is empty")
	}
	if !IsValidEventType(eventType) {
		return fmt.Errorf("%w: %s", ErrInvalidEventType, eventType)
	}
	if agentUUID.IsZero() {
		return fmt.Errorf("%w: agent uuid is empty", ErrInvalidUUID)
	}
	if operatorDID == "" {
		return errors.New("domain: operator_did is empty")
	}
	return nil
}
