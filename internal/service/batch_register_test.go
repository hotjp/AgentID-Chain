package service

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/agentid-chain/agentid-chain/internal/domain"
)

func TestBatchRegisterService_NilInner(t *testing.T) {
	_, err := NewBatchRegisterService(nil, BatchRegisterConfig{})
	if err == nil {
		t.Error("expected error for nil inner")
	}
}

func TestBatchRegisterService_DefaultConfig(t *testing.T) {
	store := newMockStore()
	provider := &mockProvider{exists: false}
	inner, _ := NewRegisterService(store, nil, nil, provider)
	svc, err := NewBatchRegisterService(inner, BatchRegisterConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if svc.cfg.MaxBatchSize != 100 {
		t.Errorf("MaxBatchSize = %d, want 100", svc.cfg.MaxBatchSize)
	}
	if svc.cfg.Concurrency != 8 {
		t.Errorf("Concurrency = %d, want 8", svc.cfg.Concurrency)
	}
}

func TestBatchRegisterService_Empty(t *testing.T) {
	store := newMockStore()
	provider := &mockProvider{exists: false}
	inner, _ := NewRegisterService(store, nil, nil, provider)
	svc, _ := NewBatchRegisterService(inner, BatchRegisterConfig{})

	_, err := svc.HandleBatchRegister(context.Background(), nil)
	if !errors.Is(err, ErrEmptyBatch) {
		t.Errorf("err = %v, want ErrEmptyBatch", err)
	}
}

func TestBatchRegisterService_TooLarge(t *testing.T) {
	store := newMockStore()
	provider := &mockProvider{exists: false}
	inner, _ := NewRegisterService(store, nil, nil, provider)
	svc, _ := NewBatchRegisterService(inner, BatchRegisterConfig{MaxBatchSize: 2})

	items := []*RegisterAgentRequest{validRequest(t), validRequest(t), validRequest(t)}
	_, err := svc.HandleBatchRegister(context.Background(), items)
	if !errors.Is(err, ErrBatchTooLarge) {
		t.Errorf("err = %v, want ErrBatchTooLarge", err)
	}
}

func TestBatchRegisterService_AllSucceed(t *testing.T) {
	store := newMockStore()
	audit := &mockAudit{}
	provider := &mockProvider{exists: false}
	inner, _ := NewRegisterService(store, nil, audit, provider)
	svc, _ := NewBatchRegisterService(inner, BatchRegisterConfig{Concurrency: 4})

	n := 10
	items := make([]*RegisterAgentRequest, n)
	for i := 0; i < n; i++ {
		items[i] = validRequest(t)
	}
	result, err := svc.HandleBatchRegister(context.Background(), items)
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != n {
		t.Errorf("Total = %d, want %d", result.Total, n)
	}
	if len(result.Succeeded) != n {
		t.Errorf("Succeeded = %d, want %d", len(result.Succeeded), n)
	}
	if len(result.Failed) != 0 {
		t.Errorf("Failed = %d, want 0", len(result.Failed))
	}
	if result.EndedAt.Before(result.StartedAt) {
		t.Error("EndedAt < StartedAt")
	}
}

func TestBatchRegisterService_PartialFailure(t *testing.T) {
	store := newMockStore()
	audit := &mockAudit{}
	// Only first 2 exist (will fail); rest are new.
	provider := &toggleProvider{existing: []bool{true, true, false, false, false}}
	inner, _ := NewRegisterService(store, nil, audit, provider)
	svc, _ := NewBatchRegisterService(inner, BatchRegisterConfig{Concurrency: 2})

	n := 5
	items := make([]*RegisterAgentRequest, n)
	for i := 0; i < n; i++ {
		items[i] = validRequest(t)
	}
	result, err := svc.HandleBatchRegister(context.Background(), items)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Succeeded) != 3 {
		t.Errorf("Succeeded = %d, want 3", len(result.Succeeded))
	}
	if len(result.Failed) != 2 {
		t.Errorf("Failed = %d, want 2", len(result.Failed))
	}
}

// toggleProvider 可在调用间切换 Exists 结果。
type toggleProvider struct {
	IdentityProvider
	mu       sync.Mutex
	calls    int
	existing []bool
}

func (m *toggleProvider) Exists(_ context.Context, _ domain.UUID) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	i := m.calls
	m.calls++
	if i < len(m.existing) {
		return m.existing[i], nil
	}
	return false, nil
}

func (m *toggleProvider) BackendName() string { return "toggle" }
func (m *toggleProvider) HealthCheck(context.Context) error { return nil }

// 编译期：确保 mockProvider 仍满足 IdentityProvider（防止删除方法后其他测试失败）
var _ IdentityProvider = (*mockProvider)(nil)
