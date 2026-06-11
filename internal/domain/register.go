// Package domain RegisterAgent 业务规则。
//
// 业务流程：
//  1. UUID 生成（调用方注入，保证可测试）
//  2. 构造 Agent 实体（NewAgent）
//  3. 不变量校验（CheckInvariants）
//  4. 构造注册事件（NewAgentRegisteredEventV1）
//  5. 事件写入 outbox（Collect）
//
// 业务规则：
//   - 同一 owner_did 下不能有重复 active agent（去重 — 由调用方校验，本函数不强加）
//   - 注册时 Level 必须是合法 LevelType（[0,7]）
//   - 注册事件必须 OperatorDID 非空
package domain

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// =============================================================================
// 错误定义
// =============================================================================

// ErrRegisterFailed 注册失败（通用）。
var ErrRegisterFailed = errors.New("domain: register failed")

// ErrInvariantFailed 不变量校验失败。
var ErrInvariantFailed = errors.New("domain: invariant failed during register")

// UUIDGenerator UUID 生成器（注入以便测试）。
type UUIDGenerator func() (UUID, error)

// defaultUUIDGenerator 默认 UUIDv7 风格生成器（时间排序 16 字节）。
//
// 实际生产应使用 google/uuid 或 ulid；domain 不依赖第三方，
// 所以这里用 crypto/rand + 时间戳组合：
//   - 高 6 字节：UnixNano 时间戳（毫秒）
//   - 低 10 字节：crypto/rand
//   - 36 字符 hex 含 4 个连字符
func defaultUUIDGenerator() (UUID, error) {
	now := time.Now().UnixMilli()
	// 8 字节时间戳（毫秒 + 0 填充）
	tb := make([]byte, 8)
	tb[0] = byte(now >> 56)
	tb[1] = byte(now >> 48)
	tb[2] = byte(now >> 40)
	tb[3] = byte(now >> 32)
	tb[4] = byte(now >> 24)
	tb[5] = byte(now >> 16)
	tb[6] = byte(now >> 8)
	tb[7] = byte(now)
	// 8 字节随机
	rb := make([]byte, 8)
	if _, err := rand.Read(rb); err != nil {
		return "", err
	}
	b := append(tb, rb...)
	// 格式化为 8-4-4-4-12 hex
	h := hex.EncodeToString(b) // 32 chars
	if len(h) != 32 {
		return "", fmt.Errorf("uuid hex len=%d", len(h))
	}
	uuidStr := h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
	return UUID(uuidStr), nil
}

// =============================================================================
// RegisterAgent 业务规则
// =============================================================================

// RegisterInput 注册入参。
type RegisterInput struct {
	OwnerDID    string    // 拥有者 DID
	Owner       Owner     // 业务侧 Owner（与 OwnerDID 配合）
	Level       LevelType // 初始等级
	PublicKey   string    // Ed25519 公钥
	OperatorDID string    // 操作者 DID（通常 = OwnerDID，自注册场景）
	Now         time.Time // 业务时间（注入便于测试）
}

// RegisterOutput 注册出参。
type RegisterOutput struct {
	Agent *Agent                  // 构造好的实体（未持久化；调用方负责入库）
	Event *AgentRegisteredEventV1 // 注册事件
	UUID  UUID                    // 新生成的 UUID
}

// RegisterAgent 注册新 Agent 的纯业务规则（无 IO）。
//
// 不做：
//   - 不调 storage（不写库）
//   - 不调 outbox.Collect（事件由调用方负责入库）
//
// 职责：
//   - 生成 UUID
//   - 构造 Agent 实体
//   - 校验不变量
//   - 构造注册事件
//
// 调用方负责把 Agent 和 Event 持久化（事务内 + outbox.Collect）。
func RegisterAgent(gen UUIDGenerator, in RegisterInput) (*RegisterOutput, error) {
	if gen == nil {
		gen = defaultUUIDGenerator
	}
	if !in.Level.IsValid() {
		return nil, fmt.Errorf("%w: invalid level %d", ErrInvalidLevel, uint8(in.Level))
	}
	if in.PublicKey == "" {
		return nil, errors.New("domain: empty public key")
	}
	if in.OperatorDID == "" {
		return nil, errors.New("domain: empty operator_did")
	}
	if in.Now.IsZero() {
		return nil, errors.New("domain: now is zero")
	}

	// 1. 生成 UUID
	uuid, err := gen()
	if err != nil {
		return nil, fmt.Errorf("%w: generate uuid: %w", ErrRegisterFailed, err)
	}

	// 2. 构造 Agent
	agent, err := NewAgent(uuid, in.Owner, in.Level, in.PublicKey, in.Now)
	if err != nil {
		return nil, fmt.Errorf("%w: new agent: %w", ErrRegisterFailed, err)
	}

	// 3. 不变量校验
	if err := CheckInvariants(agent, in.Now); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvariantFailed, err)
	}

	// 4. 构造事件
	evt, err := NewAgentRegisteredEventV1(
		string(uuid),
		uuid,
		in.OwnerDID,
		in.Level,
		in.OperatorDID,
		in.Now,
	)
	if err != nil {
		return nil, fmt.Errorf("%w: build event: %w", ErrRegisterFailed, err)
	}

	return &RegisterOutput{
		Agent: agent,
		Event: evt,
		UUID:  uuid,
	}, nil
}

// CollectOutbox 便捷包装：把 RegisterOutput.Event 写入 outbox。
//
// 业务典型用法：
//
//	out, _ := domain.RegisterAgent(nil, in)
//	domain.CollectOutbox(ctx, writer, out)
func CollectOutbox(ctx context.Context, w OutboxWriter, out *RegisterOutput) error {
	if out == nil || out.Event == nil {
		return errors.New("domain: nil register output")
	}
	return Collect(ctx, w, out.Event)
}
