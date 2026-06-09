package service

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"testing"
	"time"

	"github.com/agentid-chain/agentid-chain/internal/domain"
	"github.com/agentid-chain/agentid-chain/internal/storage"
)

// =============================================================================
// 工具：构造一个已存在的 agent
// =============================================================================

func seedAgent(t *testing.T, store *mockStore, level domain.LevelType) *storage.AgentRecord {
	t.Helper()
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	uuid := newValidUUID()
	now := time.Now()
	rec := &storage.AgentRecord{
		UUID:         uuid.String(),
		Owner:        "test-owner",
		Level:        uint8(level),
		PublicKey:    encodePubKey(pub),
		State:        string(domain.StateActive),
		Permissions:  level.DefaultMaxPermissions(),
		RegisteredAt: now,
		UpdatedAt:    now,
	}
	if err := store.PutAgent(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
	return rec
}

func TestUpgradeService_NilStore(t *testing.T) {
	_, err := NewUpgradeService(nil, nil, nil, &mockProvider{})
	if err == nil {
		t.Error("expected error")
	}
}

func TestUpgradeService_HappyPath(t *testing.T) {
	store := newMockStore()
	chain := &mockChain{
		typ:     ChainMock,
		receipt: &RegisterReceipt{TxHash: "0x-upgrade"},
	}
	audit := &mockAudit{}
	provider := &mockProvider{}
	svc, _ := NewUpgradeService(store, chain, audit, provider)

	rec := seedAgent(t, store, domain.LevelTest)
	oldLevel := domain.LevelType(rec.Level)

	resp, err := svc.HandleUpgrade(context.Background(), &UpgradeAgentRequest{
		UUID:     domain.UUID(rec.UUID),
		NewLevel: domain.LevelBasic, // +1
		Reason:   "user requested",
		Actor:    "test-user",
		Now:      time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.NewLevel != domain.LevelBasic {
		t.Errorf("new level = %d", resp.NewLevel)
	}
	if resp.OldLevel != oldLevel {
		t.Errorf("old level = %d", resp.OldLevel)
	}
	if resp.TxHash != "0x-upgrade" {
		t.Errorf("tx hash = %q", resp.TxHash)
	}
	if len(audit.events) != 1 {
		t.Errorf("audit events = %d", len(audit.events))
	}
	// 权限位应更新为新 Level 的 default
	if store.perms[rec.UUID] != domain.LevelBasic.DefaultMaxPermissions() {
		t.Errorf("perms = %d, want %d", store.perms[rec.UUID], domain.LevelBasic.DefaultMaxPermissions())
	}
}

func TestUpgradeService_RejectsSkipLevel(t *testing.T) {
	store := newMockStore()
	provider := &mockProvider{}
	svc, _ := NewUpgradeService(store, nil, nil, provider)

	rec := seedAgent(t, store, domain.LevelTest)
	_, err := svc.HandleUpgrade(context.Background(), &UpgradeAgentRequest{
		UUID:     domain.UUID(rec.UUID),
		NewLevel: domain.LevelAdvanced, // 跳 2 级 — domain.Agent.Upgrade 会拒
		Now:      time.Now(),
	})
	if !errors.Is(err, ErrInvalidUpgradeLevel) {
		t.Errorf("err = %v", err)
	}
}

func TestUpgradeService_RejectsDowngrade(t *testing.T) {
	store := newMockStore()
	provider := &mockProvider{}
	svc, _ := NewUpgradeService(store, nil, nil, provider)

	rec := seedAgent(t, store, domain.LevelBasic)
	_, err := svc.HandleUpgrade(context.Background(), &UpgradeAgentRequest{
		UUID:     domain.UUID(rec.UUID),
		NewLevel: domain.LevelTest, // 降级
		Now:      time.Now(),
	})
	if !errors.Is(err, ErrInvalidUpgradeLevel) {
		t.Errorf("err = %v", err)
	}
}

func TestUpgradeService_AgentNotFound(t *testing.T) {
	store := newMockStore()
	provider := &mockProvider{}
	svc, _ := NewUpgradeService(store, nil, nil, provider)

	_, err := svc.HandleUpgrade(context.Background(), &UpgradeAgentRequest{
		UUID:     newValidUUID(),
		NewLevel: domain.LevelBasic,
		Now:      time.Now(),
	})
	if !errors.Is(err, ErrAgentNotFound) {
		t.Errorf("err = %v", err)
	}
}

func TestUpgradeService_RejectsBanned(t *testing.T) {
	store := newMockStore()
	provider := &mockProvider{}
	svc, _ := NewUpgradeService(store, nil, nil, provider)

	rec := seedAgent(t, store, domain.LevelTest)
	rec.State = string(domain.StateBanned) // 封禁
	store.agents[rec.UUID] = rec

	_, err := svc.HandleUpgrade(context.Background(), &UpgradeAgentRequest{
		UUID:     domain.UUID(rec.UUID),
		NewLevel: domain.LevelBasic,
		Now:      time.Now(),
	})
	if !errors.Is(err, ErrAgentNotUpgradable) {
		t.Errorf("err = %v", err)
	}
}

func TestUpgradeService_ChainErrorReturnsPartial(t *testing.T) {
	store := newMockStore()
	chain := &mockChain{err: errors.New("chain rpc fail")}
	provider := &mockProvider{}
	svc, _ := NewUpgradeService(store, chain, nil, provider)

	rec := seedAgent(t, store, domain.LevelTest)
	resp, err := svc.HandleUpgrade(context.Background(), &UpgradeAgentRequest{
		UUID:     domain.UUID(rec.UUID),
		NewLevel: domain.LevelBasic,
		Now:      time.Now(),
	})
	if !errors.Is(err, ErrChainRegisterFailed) {
		t.Errorf("err = %v", err)
	}
	if resp == nil || resp.NewLevel != domain.LevelBasic {
		t.Error("should return partial with new level applied")
	}
}
