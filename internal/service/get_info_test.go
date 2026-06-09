package service

import (
	"context"
	"errors"
	"testing"

	"github.com/agentid-chain/agentid-chain/internal/domain"
)

func TestGetInfoService_NilStore(t *testing.T) {
	_, err := NewGetAgentInfoService(nil, nil, &mockProvider{})
	if err == nil {
		t.Error("expected error for nil store")
	}
}

func TestGetInfoService_NilProvider(t *testing.T) {
	_, err := NewGetAgentInfoService(newMockStore(), nil, nil)
	if err == nil {
		t.Error("expected error for nil provider")
	}
}

func TestGetInfoService_ZeroUUID(t *testing.T) {
	store := newMockStore()
	provider := &mockProvider{}
	svc, _ := NewGetAgentInfoService(store, nil, provider)

	_, err := svc.HandleGetInfo(context.Background(), domain.UUID(""))
	if !errors.Is(err, ErrInvalidRegisterInput) {
		t.Errorf("err = %v, want ErrInvalidRegisterInput", err)
	}
}

func TestGetInfoService_AgentNotFound(t *testing.T) {
	store := newMockStore()
	provider := &mockProvider{}
	svc, _ := NewGetAgentInfoService(store, nil, provider)

	_, err := svc.HandleGetInfo(context.Background(), newValidUUID())
	if !errors.Is(err, ErrAgentNotFound) {
		t.Errorf("err = %v, want ErrAgentNotFound", err)
	}
}

func TestGetInfoService_HappyPath_NoChain(t *testing.T) {
	store := newMockStore()
	provider := &mockProvider{}
	svc, _ := NewGetAgentInfoService(store, nil, provider)

	rec := seedAgent(t, store, domain.LevelBasic)
	store.perms[rec.UUID] = 0xFF
	info, err := svc.HandleGetInfo(context.Background(), domain.UUID(rec.UUID))
	if err != nil {
		t.Fatal(err)
	}
	if info.UUID != rec.UUID {
		t.Errorf("uuid = %q", info.UUID)
	}
	if info.Owner != rec.Owner {
		t.Errorf("owner = %q", info.Owner)
	}
	if info.State != domain.StateActive {
		t.Errorf("state = %v", info.State)
	}
	if info.Permissions != 0xFF {
		t.Errorf("perms = %d, want 0xFF", info.Permissions)
	}
	if info.ChainState != nil {
		t.Error("ChainState should be nil when no chain")
	}
}

func TestGetInfoService_WithChain(t *testing.T) {
	store := newMockStore()
	chain := &mockChain{} // GetAgentState returns empty ChainAgentState
	provider := &mockProvider{}
	svc, _ := NewGetAgentInfoService(store, chain, provider)

	rec := seedAgent(t, store, domain.LevelBasic)
	info, err := svc.HandleGetInfo(context.Background(), domain.UUID(rec.UUID))
	if err != nil {
		t.Fatal(err)
	}
	if info.ChainState == nil {
		t.Error("ChainState should not be nil with chain adapter")
	}
}

func TestGetInfoService_ChainErrorNonBlocking(t *testing.T) {
	store := newMockStore()
	chain := &mockChain{err: errors.New("chain query fail")}
	provider := &mockProvider{}
	svc, _ := NewGetAgentInfoService(store, chain, provider)

	rec := seedAgent(t, store, domain.LevelBasic)
	info, err := svc.HandleGetInfo(context.Background(), domain.UUID(rec.UUID))
	if err != nil {
		t.Errorf("chain error should not block L1 read, got err = %v", err)
	}
	if info == nil {
		t.Fatal("nil info")
	}
	if info.UUID != rec.UUID {
		t.Errorf("uuid = %q", info.UUID)
	}
}
