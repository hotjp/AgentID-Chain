package backend

import (
	"context"
	"errors"
	"testing"
)

func TestNewBackend_DefaultIsLocal(t *testing.T) {
	be, err := NewBackend(Config{})
	if err != nil {
		t.Fatal(err)
	}
	if got := be.BackendType(); got != TypeLocal {
		t.Errorf("type = %q, want local", got)
	}
}

func TestNewBackend_Local(t *testing.T) {
	be, err := NewBackend(Config{Type: TypeLocal, Persistence: NewMemoryPersistence()})
	if err != nil {
		t.Fatal(err)
	}
	if got := be.BackendType(); got != TypeLocal {
		t.Errorf("type = %q, want local", got)
	}
	// 烟雾测试
	cred, err := be.RegisterAgent(context.Background(), validRequest())
	if err != nil {
		t.Fatal(err)
	}
	if cred.UUID == "" {
		t.Error("empty uuid")
	}
}

func TestNewBackend_Onchain(t *testing.T) {
	adp := newMockChainAdapter() // 辅助函数（下方定义）
	be, err := NewBackend(Config{Type: TypeOnchain, ChainAdapter: adp})
	if err != nil {
		t.Fatal(err)
	}
	if got := be.BackendType(); got != TypeOnchain {
		t.Errorf("type = %q, want onchain", got)
	}
	cred, err := be.RegisterAgent(context.Background(), validRequest())
	if err != nil {
		t.Fatal(err)
	}
	if cred.UUID == "" {
		t.Error("empty uuid")
	}
}

func TestNewBackend_Onchain_NilAdapter(t *testing.T) {
	_, err := NewBackend(Config{Type: TypeOnchain})
	if err == nil {
		t.Error("expected error for nil adapter")
	}
}

func TestNewBackend_Hybrid(t *testing.T) {
	be, err := NewBackend(Config{
		Type:         TypeHybrid,
		ChainAdapter: newMockChainAdapter(),
		Persistence:  NewMemoryPersistence(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := be.BackendType(); got != TypeHybrid {
		t.Errorf("type = %q, want hybrid", got)
	}
	cred, err := be.RegisterAgent(context.Background(), validRequest())
	if err != nil {
		t.Fatal(err)
	}
	if cred.UUID == "" {
		t.Error("empty uuid")
	}
}

func TestNewBackend_Hybrid_NilAdapter(t *testing.T) {
	_, err := NewBackend(Config{Type: TypeHybrid, Persistence: NewMemoryPersistence()})
	if err == nil {
		t.Error("expected error for nil adapter")
	}
}

func TestNewBackend_Hybrid_NilPersistence(t *testing.T) {
	_, err := NewBackend(Config{Type: TypeHybrid, ChainAdapter: newMockChainAdapter()})
	if err == nil {
		t.Error("expected error for nil persistence")
	}
}

func TestNewBackend_Mock(t *testing.T) {
	be, err := NewBackend(Config{Type: TypeMock})
	if err != nil {
		t.Fatal(err)
	}
	// mock 模式包装为 hybrid 行为
	if got := be.BackendType(); got != TypeHybrid {
		t.Errorf("type = %q, want hybrid (mock=hybrid)", got)
	}
	cred, err := be.RegisterAgent(context.Background(), validRequest())
	if err != nil {
		t.Fatal(err)
	}
	if cred.UUID == "" {
		t.Error("empty uuid")
	}
}

func TestNewBackend_UnknownType(t *testing.T) {
	_, err := NewBackend(Config{Type: BackendType("nonsense")})
	if err == nil {
		t.Error("expected error for unknown type")
	}
	if !errors.Is(err, ErrBackendUnavailable) {
		t.Errorf("err = %v, want ErrBackendUnavailable", err)
	}
}

func TestNewBackend_CustomOwnerKey(t *testing.T) {
	be, err := NewBackend(Config{
		Type:      TypeLocal,
		OwnerKey:  "custom:",
	})
	if err != nil {
		t.Fatal(err)
	}
	if be.BackendType() != TypeLocal {
		t.Error("type wrong")
	}
}

func TestNewBackend_NilPersistenceFallsBackToMemory(t *testing.T) {
	be, err := NewBackend(Config{Type: TypeLocal})
	if err != nil {
		t.Fatal(err)
	}
	// 写入一个 agent
	cred, err := be.RegisterAgent(context.Background(), validRequest())
	if err != nil {
		t.Fatal(err)
	}
	// 读回
	info, err := be.GetAgentInfo(context.Background(), cred.UUID)
	if err != nil {
		t.Fatal(err)
	}
	if info.UUID != cred.UUID {
		t.Errorf("uuid = %q", info.UUID)
	}
}
