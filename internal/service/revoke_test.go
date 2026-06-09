package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/agentid-chain/agentid-chain/internal/domain"
	"github.com/agentid-chain/agentid-chain/internal/storage"
)

func TestRevokeService_NilStore(t *testing.T) {
	_, err := NewRevokeService(nil, nil, nil, &mockProvider{})
	if err == nil {
		t.Error("expected error for nil store")
	}
}

func TestRevokeService_NilProvider(t *testing.T) {
	_, err := NewRevokeService(newMockStore(), nil, nil, nil)
	if err == nil {
		t.Error("expected error for nil provider")
	}
}

func TestRevokeService_HappyPath_NoChain(t *testing.T) {
	store := newMockStore()
	audit := &mockAudit{}
	provider := &mockProvider{}
	svc, err := NewRevokeService(store, nil, audit, provider)
	if err != nil {
		t.Fatal(err)
	}

	rec := seedAgent(t, store, domain.LevelBasic)
	now := time.Now()

	resp, err := svc.HandleRevoke(context.Background(), &RevokeAgentRequest{
		UUID:   domain.UUID(rec.UUID),
		Reason: "user requested",
		Actor:  "test-user",
		Now:    now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Agent == nil {
		t.Fatal("nil agent")
	}
	if resp.Agent.State != domain.StateUnregistered {
		t.Errorf("state = %v, want unregistered", resp.Agent.State)
	}
	if resp.RevokedAt.IsZero() {
		t.Error("revoked_at is zero")
	}
	if resp.TxHash != "" {
		t.Errorf("tx hash = %q (no chain)", resp.TxHash)
	}
	if len(audit.events) != 1 {
		t.Errorf("audit events = %d", len(audit.events))
	}
	// L1 状态应已更新
	if got := store.agents[rec.UUID].State; got != string(domain.StateUnregistered) {
		t.Errorf("stored state = %q", got)
	}
}

func TestRevokeService_WithChain(t *testing.T) {
	store := newMockStore()
	chain := &mockChain{
		typ:     ChainMock,
		receipt: &RegisterReceipt{TxHash: "0x-revoke-tx"},
	}
	provider := &mockProvider{}
	svc, _ := NewRevokeService(store, chain, nil, provider)

	rec := seedAgent(t, store, domain.LevelBasic)
	resp, err := svc.HandleRevoke(context.Background(), &RevokeAgentRequest{
		UUID:  domain.UUID(rec.UUID),
		Actor: "test-user",
		Now:   time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.TxHash != "0x-revoke-tx" {
		t.Errorf("tx hash = %q", resp.TxHash)
	}
}

func TestRevokeService_NilRequest(t *testing.T) {
	store := newMockStore()
	provider := &mockProvider{}
	svc, _ := NewRevokeService(store, nil, nil, provider)

	_, err := svc.HandleRevoke(context.Background(), nil)
	if err == nil {
		t.Error("expected error for nil request")
	}
}

func TestRevokeService_AgentNotFound(t *testing.T) {
	store := newMockStore()
	provider := &mockProvider{}
	svc, _ := NewRevokeService(store, nil, nil, provider)

	_, err := svc.HandleRevoke(context.Background(), &RevokeAgentRequest{
		UUID: newValidUUID(),
		Now:  time.Now(),
	})
	if !errors.Is(err, ErrAgentNotFound) {
		t.Errorf("err = %v", err)
	}
}

func TestRevokeService_AlreadyUnregistered(t *testing.T) {
	store := newMockStore()
	provider := &mockProvider{}
	svc, _ := NewRevokeService(store, nil, nil, provider)

	rec := seedAgent(t, store, domain.LevelBasic)
	rec.State = string(domain.StateUnregistered)
	store.agents[rec.UUID] = rec

	_, err := svc.HandleRevoke(context.Background(), &RevokeAgentRequest{
		UUID: domain.UUID(rec.UUID),
		Now:  time.Now(),
	})
	if !errors.Is(err, ErrNotRevocable) {
		t.Errorf("err = %v, want ErrNotRevocable", err)
	}
}

func TestRevokeService_ChainErrorReturnsPartial(t *testing.T) {
	store := newMockStore()
	chain := &mockChain{err: errors.New("chain rpc fail")}
	provider := &mockProvider{}
	svc, _ := NewRevokeService(store, chain, nil, provider)

	rec := seedAgent(t, store, domain.LevelBasic)
	resp, err := svc.HandleRevoke(context.Background(), &RevokeAgentRequest{
		UUID: domain.UUID(rec.UUID),
		Now:  time.Now(),
	})
	if !errors.Is(err, ErrChainRegisterFailed) {
		t.Errorf("err = %v", err)
	}
	if resp == nil {
		t.Fatal("nil response")
	}
	if resp.Agent.State != domain.StateUnregistered {
		t.Error("L1 state should still be unregistered")
	}
}

func TestRevokeService_DefaultNow(t *testing.T) {
	store := newMockStore()
	provider := &mockProvider{}
	svc, _ := NewRevokeService(store, nil, nil, provider)

	rec := seedAgent(t, store, domain.LevelBasic)
	req := &RevokeAgentRequest{
		UUID: domain.UUID(rec.UUID),
		// Now omitted
	}
	resp, err := svc.HandleRevoke(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.RevokedAt.IsZero() {
		t.Error("Now should default to time.Now()")
	}
}

// ensure mockStore has all required signatures
var _ storage.IdentityStore = (*mockStore)(nil)
