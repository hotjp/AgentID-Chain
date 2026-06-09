package service

import (
	"context"
	"errors"
	"testing"

	"github.com/agentid-chain/agentid-chain/internal/domain"
)

func TestCheckPermissionService_NilStore(t *testing.T) {
	_, err := NewCheckPermissionService(nil, &mockProvider{})
	if err == nil {
		t.Error("expected error for nil store")
	}
}

func TestCheckPermissionService_NilProvider(t *testing.T) {
	_, err := NewCheckPermissionService(newMockStore(), nil)
	if err == nil {
		t.Error("expected error for nil provider")
	}
}

func TestCheckPermissionService_NilRequest(t *testing.T) {
	store := newMockStore()
	provider := &mockProvider{}
	svc, _ := NewCheckPermissionService(store, provider)

	_, err := svc.HandleCheckPermission(context.Background(), nil)
	if err == nil {
		t.Error("expected error")
	}
}

func TestCheckPermissionService_EmptyUUID(t *testing.T) {
	store := newMockStore()
	provider := &mockProvider{}
	svc, _ := NewCheckPermissionService(store, provider)

	_, err := svc.HandleCheckPermission(context.Background(), &CheckPermissionRequest{
		UUID: domain.UUID(""),
		Bit:  0,
	})
	if !errors.Is(err, ErrInvalidRegisterInput) {
		t.Errorf("err = %v, want ErrInvalidRegisterInput", err)
	}
}

func TestCheckPermissionService_BitOutOfRange(t *testing.T) {
	store := newMockStore()
	provider := &mockProvider{}
	svc, _ := NewCheckPermissionService(store, provider)

	_, err := svc.HandleCheckPermission(context.Background(), &CheckPermissionRequest{
		UUID: newValidUUID(),
		Bit:  64,
	})
	if err == nil {
		t.Error("expected error for bit >= 64")
	}
}

func TestCheckPermissionService_AgentNotFound(t *testing.T) {
	store := newMockStore()
	provider := &mockProvider{}
	svc, _ := NewCheckPermissionService(store, provider)

	resp, err := svc.HandleCheckPermission(context.Background(), &CheckPermissionRequest{
		UUID: newValidUUID(),
		Bit:  0,
	})
	if !errors.Is(err, ErrAgentNotFound) {
		t.Errorf("err = %v", err)
	}
	if resp == nil || resp.Allowed {
		t.Error("should return Allowed=false on not-found")
	}
}

func TestCheckPermissionService_HasBit(t *testing.T) {
	store := newMockStore()
	provider := &mockProvider{}
	svc, _ := NewCheckPermissionService(store, provider)

	rec := seedAgent(t, store, domain.LevelBasic)
	store.perms[rec.UUID] = uint64(1) // bit 0
	resp, err := svc.HandleCheckPermission(context.Background(), &CheckPermissionRequest{
		UUID: domain.UUID(rec.UUID),
		Bit:  0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Allowed {
		t.Error("bit 0 should be allowed when perms bit is set")
	}
	if !resp.HasBit {
		t.Error("HasBit should be true")
	}
}

func TestCheckPermissionService_BitNotSet(t *testing.T) {
	store := newMockStore()
	provider := &mockProvider{}
	svc, _ := NewCheckPermissionService(store, provider)

	rec := seedAgent(t, store, domain.LevelBasic)
	store.perms[rec.UUID] = 0 // clear all
	resp, err := svc.HandleCheckPermission(context.Background(), &CheckPermissionRequest{
		UUID: domain.UUID(rec.UUID),
		Bit:  0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Allowed {
		t.Error("should not be allowed")
	}
	if resp.HasBit {
		t.Error("HasBit should be false")
	}
}

func TestCheckPermissionService_BannedAgent(t *testing.T) {
	store := newMockStore()
	provider := &mockProvider{}
	svc, _ := NewCheckPermissionService(store, provider)

	rec := seedAgent(t, store, domain.LevelBasic)
	rec.State = string(domain.StateBanned)
	store.agents[rec.UUID] = rec
	store.perms[rec.UUID] = 0xFFFFFFFF // all bits set

	resp, err := svc.HandleCheckPermission(context.Background(), &CheckPermissionRequest{
		UUID: domain.UUID(rec.UUID),
		Bit:  0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Allowed {
		t.Error("banned agent should not be allowed")
	}
}

func TestCheckPermissionService_HighBit(t *testing.T) {
	store := newMockStore()
	provider := &mockProvider{}
	svc, _ := NewCheckPermissionService(store, provider)

	rec := seedAgent(t, store, domain.LevelBasic)
	store.perms[rec.UUID] = uint64(1) << 63 // bit 63
	resp, err := svc.HandleCheckPermission(context.Background(), &CheckPermissionRequest{
		UUID: domain.UUID(rec.UUID),
		Bit:  63,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Allowed {
		t.Error("bit 63 should be allowed")
	}
}
