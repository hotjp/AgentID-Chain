package service

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/agentid-chain/agentid-chain/internal/domain"
	"github.com/agentid-chain/agentid-chain/internal/storage"
)

// =============================================================================
// mocks
// =============================================================================

type mockStore struct {
	storage.Client
	mu           sync.Mutex
	agents       map[string]*storage.AgentRecord
	perms        map[string]uint64
	putAgentErr  error
	existsBefore bool //lint:ignore U1000 reserved for exists-check test scenarios
}

func newMockStore() *mockStore {
	return &mockStore{
		agents: map[string]*storage.AgentRecord{},
		perms:  map[string]uint64{},
	}
}

func (m *mockStore) Identity() storage.IdentityStore    { return m }
func (m *mockStore) Permission() storage.PermissionStore { return m }
func (m *mockStore) Audit() storage.AuditStore          { return m }
func (m *mockStore) Nonce() storage.NonceStore          { return m }
func (m *mockStore) Revocation() storage.RevocationStore { return m }
func (m *mockStore) Cache() storage.CacheStore          { return m }

func (m *mockStore) GetAgent(_ context.Context, uuid string) (*storage.AgentRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.agents[uuid]
	if !ok {
		return nil, storage.ErrNotFound
	}
	return rec, nil
}

func (m *mockStore) PutAgent(_ context.Context, rec *storage.AgentRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.putAgentErr != nil {
		return m.putAgentErr
	}
	// upsert: 覆盖（生产 PG 用 ON CONFLICT DO UPDATE；mock 直接覆盖）
	m.agents[rec.UUID] = rec
	return nil
}

func (m *mockStore) ListAgentsByOwner(_ context.Context, owner string) ([]*storage.AgentRecord, error) {
	var out []*storage.AgentRecord
	for _, a := range m.agents {
		if a.Owner == owner {
			out = append(out, a)
		}
	}
	return out, nil
}

func (m *mockStore) BatchGetAgents(_ context.Context, uuids []string) (map[string]*storage.AgentRecord, error) {
	out := map[string]*storage.AgentRecord{}
	for _, u := range uuids {
		if a, ok := m.agents[u]; ok {
			out[u] = a
		}
	}
	return out, nil
}

func (m *mockStore) GetPermissions(_ context.Context, uuid string) (uint64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.perms[uuid], nil
}

func (m *mockStore) SetPermissions(_ context.Context, uuid string, bits uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.perms[uuid] = bits
	return nil
}

func (m *mockStore) GrantPermission(_ context.Context, uuid string, bit uint) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.perms[uuid] |= 1 << bit
	return nil
}

func (m *mockStore) RevokePermission(_ context.Context, uuid string, bit uint) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.perms[uuid] &^= 1 << bit
	return nil
}

func (m *mockStore) HealthCheck(_ context.Context) storage.HealthStatus { return storage.HealthStatus{Healthy: true} }
func (m *mockStore) Close(_ context.Context) error                     { return nil }

// =============================================================================
// 其他子接口 no-op
// =============================================================================

func (m *mockStore) Append(_ context.Context, _ *storage.AuditEntry) error { return nil }
func (m *mockStore) Query(_ context.Context, _ string, _, _ time.Time) ([]*storage.AuditEntry, error) {
	return nil, nil
}
func (m *mockStore) Count(_ context.Context, _ string) (int64, error) { return 0, nil }
func (m *mockStore) StoreOnce(_ context.Context, _ string, _ []byte, _ time.Duration) error {
	return nil
}
func (m *mockStore) Get(_ context.Context, _ string) ([]byte, error)        { return nil, storage.ErrNotFound }
func (m *mockStore) Set(_ context.Context, _ string, _ []byte, _ time.Duration) error { return nil }
func (m *mockStore) Del(_ context.Context, _ ...string) error               { return nil }
func (m *mockStore) Exists(_ context.Context, _ string) (bool, error)       { return false, nil }
func (m *mockStore) Expire(_ context.Context, _ string, _ time.Duration) error { return nil }
func (m *mockStore) Incr(_ context.Context, _ string, _ time.Duration) (int64, error) {
	return 0, nil
}
func (m *mockStore) Store(_ context.Context, _ string, _ time.Duration) error { return nil }
func (m *mockStore) NonceExists(_ context.Context, _ string) (bool, error) { return false, nil }
func (m *mockStore) Consume(_ context.Context, _ string) error { return nil }
func (m *mockStore) Revoke(_ context.Context, _ string, _ time.Time) error { return nil }
func (m *mockStore) IsRevoked(_ context.Context, _ string) (bool, error) { return false, nil }
func (m *mockStore) PurgeExpired(_ context.Context) (int64, error) { return 0, nil }

// =============================================================================
// Chain + Audit + Provider mocks
// =============================================================================

type mockChain struct {
	ChainAdapter
	typ       ChainType
	receipt   *RegisterReceipt
	err       error
}

func (m *mockChain) ChainType() ChainType { return m.typ }
func (m *mockChain) RegisterAgent(_ context.Context, _ *RegisterRequest) (*RegisterReceipt, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.receipt, nil
}
func (m *mockChain) UpdateLevel(context.Context, domain.UUID, domain.LevelType, string) (*RegisterReceipt, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.receipt, nil
}
func (m *mockChain) BanAgent(context.Context, domain.UUID, string) (*RegisterReceipt, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.receipt, nil
}
func (m *mockChain) UnbanAgent(context.Context, domain.UUID) (*RegisterReceipt, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.receipt, nil
}
func (m *mockChain) GetAgentState(context.Context, domain.UUID) (*ChainAgentState, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &ChainAgentState{}, nil
}
func (m *mockChain) HealthCheck(context.Context) error { return nil }

type mockAudit struct {
	AuditNotifier
	mu     sync.Mutex
	events []*AuditEvent
}

func (m *mockAudit) Notify(_ context.Context, e *AuditEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, e)
	return nil
}
func (m *mockAudit) Close() error { return nil }

type mockProvider struct {
	IdentityProvider
	agents map[string]*domain.Agent
	exists bool
}

func (m *mockProvider) BackendName() string { return "mock" }
func (m *mockProvider) Load(_ context.Context, uuid domain.UUID) (*domain.Agent, error) {
	if a, ok := m.agents[uuid.String()]; ok {
		return a, nil
	}
	return nil, errors.New("not found")
}
func (m *mockProvider) LoadByOwner(context.Context, string) ([]*domain.Agent, error) {
	return nil, nil
}
func (m *mockProvider) LoadByPubKey(context.Context, ed25519.PublicKey) (*domain.Agent, error) {
	return nil, nil
}
func (m *mockProvider) Exists(_ context.Context, uuid domain.UUID) (bool, error) {
	return m.exists, nil
}
func (m *mockProvider) HealthCheck(context.Context) error { return nil }

// =============================================================================
// 测试
// =============================================================================

func newValidUUID() domain.UUID {
	raw := make([]byte, 16)
	_, _ = rand.Read(raw)
	// 简化：用 hex；生产 UUIDv7 由 domain.UUID 提供
	return domain.UUID("11111111-2222-3333-4444-" + hexEncode(raw[:6]))
}

func hexEncode(b []byte) string {
	const hex = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2] = hex[v>>4]
		out[i*2+1] = hex[v&0xf]
	}
	return string(out)
}

func validRequest(t *testing.T) *RegisterAgentRequest {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return &RegisterAgentRequest{
		UUID:      newValidUUID(),
		Owner:     "test-user-owner",
		Level:     domain.LevelBasic,
		PublicKey: pub,
		Now:       time.Now(),
	}
}

func TestRegisterService_NilStore(t *testing.T) {
	_, err := NewRegisterService(nil, nil, nil, &mockProvider{})
	if err == nil {
		t.Error("expected error for nil store")
	}
}

func TestRegisterService_NilProvider(t *testing.T) {
	_, err := NewRegisterService(newMockStore(), nil, nil, nil)
	if err == nil {
		t.Error("expected error for nil provider")
	}
}

func TestRegisterService_HappyPath_NoChain(t *testing.T) {
	store := newMockStore()
	provider := &mockProvider{exists: false}
	audit := &mockAudit{}
	svc, err := NewRegisterService(store, nil, audit, provider)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := svc.HandleRegister(context.Background(), validRequest(t))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Agent == nil {
		t.Fatal("nil agent")
	}
	if resp.TxHash != "" {
		t.Errorf("expected empty tx hash (no chain), got %q", resp.TxHash)
	}
	if len(audit.events) != 1 {
		t.Errorf("expected 1 audit event, got %d", len(audit.events))
	}
	rec := store.agents[resp.Agent.UUID.String()]
	if rec == nil {
		t.Fatal("agent not stored")
	}
	if rec.State != string(domain.StateRegistered) {
		t.Errorf("state = %q", rec.State)
	}
}

func TestRegisterService_WithChain(t *testing.T) {
	store := newMockStore()
	chain := &mockChain{
		typ: ChainMock,
		receipt: &RegisterReceipt{
			TxHash:      "0x-mock-tx",
			BlockNumber: 12345,
			GasUsed:     21000,
			ConfirmedAt: time.Now(),
		},
	}
	provider := &mockProvider{exists: false}
	svc, _ := NewRegisterService(store, chain, nil, provider)

	resp, err := svc.HandleRegister(context.Background(), validRequest(t))
	if err != nil {
		t.Fatal(err)
	}
	if resp.TxHash != "0x-mock-tx" {
		t.Errorf("tx hash = %q", resp.TxHash)
	}
	if resp.BlockNumber != 12345 {
		t.Errorf("block = %d", resp.BlockNumber)
	}
}

func TestRegisterService_ChainErrorReturnsPartial(t *testing.T) {
	store := newMockStore()
	chain := &mockChain{err: errors.New("rpc timeout")}
	provider := &mockProvider{exists: false}
	svc, _ := NewRegisterService(store, chain, nil, provider)

	resp, err := svc.HandleRegister(context.Background(), validRequest(t))
	if !errors.Is(err, ErrChainRegisterFailed) {
		t.Errorf("err = %v, want ErrChainRegisterFailed", err)
	}
	if resp == nil {
		t.Fatal("nil response")
	}
	if resp.Agent == nil {
		t.Fatal("nil agent in response")
	}
	// 链上失败时 PG 数据已写入（后续可对账）
	if _, ok := store.agents[resp.Agent.UUID.String()]; !ok {
		t.Error("agent should be in store even on chain failure")
	}
}

func TestRegisterService_DuplicateRejected(t *testing.T) {
	store := newMockStore()
	provider := &mockProvider{exists: true} // already exists
	svc, _ := NewRegisterService(store, nil, nil, provider)

	_, err := svc.HandleRegister(context.Background(), validRequest(t))
	if !errors.Is(err, ErrAgentAlreadyExists) {
		t.Errorf("err = %v", err)
	}
}

func TestRegisterService_InvalidInput(t *testing.T) {
	tests := []struct {
		name string
		req  *RegisterAgentRequest
	}{
		{"nil", nil},
		{"empty uuid", &RegisterAgentRequest{Owner: "did:agentid:user:x", Level: domain.LevelBasic, PublicKey: make(ed25519.PublicKey, 32), Now: time.Now()}},
		{"empty owner", &RegisterAgentRequest{UUID: newValidUUID(), Level: domain.LevelBasic, PublicKey: make(ed25519.PublicKey, 32), Now: time.Now()}},
		{"invalid level", &RegisterAgentRequest{UUID: newValidUUID(), Owner: "did:agentid:user:x", Level: domain.LevelType(99), PublicKey: make(ed25519.PublicKey, 32), Now: time.Now()}},
		{"short pubkey", &RegisterAgentRequest{UUID: newValidUUID(), Owner: "did:agentid:user:x", Level: domain.LevelBasic, PublicKey: make(ed25519.PublicKey, 16), Now: time.Now()}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _ := NewRegisterService(newMockStore(), nil, nil, &mockProvider{exists: false})
			_, err := svc.HandleRegister(context.Background(), tt.req)
			if err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestRegisterService_DefaultPermissions(t *testing.T) {
	store := newMockStore()
	provider := &mockProvider{exists: false}
	svc, _ := NewRegisterService(store, nil, nil, provider)

	req := validRequest(t)
	req.Permissions = 0 // 触发 default
	resp, err := svc.HandleRegister(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	wantPerms := req.Level.DefaultMaxPermissions()
	if got := store.perms[resp.Agent.UUID.String()]; got != wantPerms {
		t.Errorf("perms = %d, want %d", got, wantPerms)
	}
}
