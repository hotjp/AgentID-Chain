package handler

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/agentid-chain/agentid-chain/internal/domain"
	"github.com/agentid-chain/agentid-chain/internal/service"
	"github.com/agentid-chain/agentid-chain/internal/storage"
)

// =============================================================================
// 全 mock 服务 + 全 mock 存储：构造完整 wire
// =============================================================================

type fakeStore struct {
	storage.Client
	mu     sync.Mutex
	agents map[string]*storage.AgentRecord
	perms  map[string]uint64
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		agents: map[string]*storage.AgentRecord{},
		perms:  map[string]uint64{},
	}
}

func (m *fakeStore) Identity() storage.IdentityStore    { return m }
func (m *fakeStore) Permission() storage.PermissionStore { return m }
func (m *fakeStore) Audit() storage.AuditStore          { return m }
func (m *fakeStore) Nonce() storage.NonceStore          { return m }
func (m *fakeStore) Revocation() storage.RevocationStore { return m }
func (m *fakeStore) Cache() storage.CacheStore          { return m }

func (m *fakeStore) GetAgent(_ context.Context, uuid string) (*storage.AgentRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.agents[uuid]
	if !ok {
		return nil, storage.ErrNotFound
	}
	return rec, nil
}
func (m *fakeStore) PutAgent(_ context.Context, rec *storage.AgentRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.agents[rec.UUID] = rec
	return nil
}
func (m *fakeStore) ListAgentsByOwner(context.Context, string) ([]*storage.AgentRecord, error) {
	return nil, nil
}
func (m *fakeStore) BatchGetAgents(context.Context, []string) (map[string]*storage.AgentRecord, error) {
	return nil, nil
}
func (m *fakeStore) GetPermissions(_ context.Context, uuid string) (uint64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.perms[uuid], nil
}
func (m *fakeStore) SetPermissions(_ context.Context, uuid string, bits uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.perms[uuid] = bits
	return nil
}
func (m *fakeStore) GrantPermission(_ context.Context, uuid string, bit uint) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.perms[uuid] |= 1 << bit
	return nil
}
func (m *fakeStore) RevokePermission(_ context.Context, uuid string, bit uint) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.perms[uuid] &^= 1 << bit
	return nil
}
func (m *fakeStore) HealthCheck(context.Context) storage.HealthStatus { return storage.HealthStatus{Healthy: true} }
func (m *fakeStore) Close(context.Context) error                     { return nil }
func (m *fakeStore) Append(context.Context, *storage.AuditEntry) error { return nil }
func (m *fakeStore) Query(context.Context, string, time.Time, time.Time) ([]*storage.AuditEntry, error) {
	return nil, nil
}
func (m *fakeStore) Count(context.Context, string) (int64, error) { return 0, nil }
func (m *fakeStore) Store(context.Context, string, time.Duration) error { return nil }
func (m *fakeStore) NonceExists(context.Context, string) (bool, error) { return false, nil }
func (m *fakeStore) Consume(context.Context, string) error { return nil }
func (m *fakeStore) Revoke(context.Context, string, time.Time) error { return nil }
func (m *fakeStore) IsRevoked(context.Context, string) (bool, error) { return false, nil }
func (m *fakeStore) PurgeExpired(context.Context) (int64, error) { return 0, nil }
func (m *fakeStore) Get(context.Context, string) ([]byte, error) { return nil, storage.ErrNotFound }
func (m *fakeStore) Set(context.Context, string, []byte, time.Duration) error { return nil }
func (m *fakeStore) Del(context.Context, ...string) error { return nil }
func (m *fakeStore) Exists(context.Context, string) (bool, error) { return false, nil }
func (m *fakeStore) Expire(context.Context, string, time.Duration) error { return nil }
func (m *fakeStore) Incr(context.Context, string, time.Duration) (int64, error) { return 0, nil }
func (m *fakeStore) StoreOnce(context.Context, string, []byte, time.Duration) error { return nil }

// fakeProvider 同 mockProvider。
type fakeProvider struct {
	service.IdentityProvider
	exists bool
}

func (m *fakeProvider) Exists(_ context.Context, _ domain.UUID) (bool, error) { return m.exists, nil }
func (m *fakeProvider) BackendName() string                                       { return "fake" }
func (m *fakeProvider) HealthCheck(context.Context) error                          { return nil }
func (m *fakeProvider) Load(context.Context, domain.UUID) (*domain.Agent, error)  {
	return nil, nil
}
func (m *fakeProvider) LoadByOwner(context.Context, string) ([]*domain.Agent, error) {
	return nil, nil
}
func (m *fakeProvider) LoadByPubKey(context.Context, ed25519.PublicKey) (*domain.Agent, error) {
	return nil, nil
}

// fakeChain 简单回执。
type fakeChain struct {
	service.ChainAdapter
	receipt *service.RegisterReceipt
	err     error
}

func (m *fakeChain) ChainType() service.ChainType { return service.ChainMock }
func (m *fakeChain) RegisterAgent(_ context.Context, _ *service.RegisterRequest) (*service.RegisterReceipt, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.receipt, nil
}
func (m *fakeChain) UpdateLevel(context.Context, domain.UUID, domain.LevelType, string) (*service.RegisterReceipt, error) {
	return m.receipt, nil
}
func (m *fakeChain) BanAgent(context.Context, domain.UUID, string) (*service.RegisterReceipt, error) {
	return m.receipt, nil
}
func (m *fakeChain) UnbanAgent(context.Context, domain.UUID) (*service.RegisterReceipt, error) {
	return m.receipt, nil
}
func (m *fakeChain) GetAgentState(context.Context, domain.UUID) (*service.ChainAgentState, error) {
	return &service.ChainAgentState{}, nil
}
func (m *fakeChain) HealthCheck(context.Context) error { return nil }

type fakeAudit struct {
	service.AuditNotifier
	mu     sync.Mutex
	events []*service.AuditEvent
}

func (m *fakeAudit) Notify(_ context.Context, e *service.AuditEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, e)
	return nil
}
func (m *fakeAudit) Close() error { return nil }

// =============================================================================
// 真实 wire 测试
// =============================================================================

func newWiredHandler() *APIHandler {
	store := newFakeStore()
	provider := &fakeProvider{exists: false}
	chain := &fakeChain{receipt: &service.RegisterReceipt{TxHash: "0x-wire"}}
	audit := &fakeAudit{}
	reg, _ := service.NewRegisterService(store, chain, audit, provider)
	up, _ := service.NewUpgradeService(store, chain, audit, provider)
	rev, _ := service.NewRevokeService(store, chain, audit, provider)
	ban, _ := service.NewBanService(store, chain, audit, provider)
	info, _ := service.NewGetAgentInfoService(store, chain, provider)
	chk, _ := service.NewCheckPermissionService(store, provider)
	return &APIHandler{
		RegisterSvc: reg,
		UpgradeSvc:  up,
		RevokeSvc:   rev,
		BanSvc:      ban,
		UnbanSvc:    ban,
		GetInfoSvc:  info,
		CheckSvc:    chk,
	}
}

func TestAPI_Register_HappyPath(t *testing.T) {
	h := newWiredHandler()
	uuid := validUUID()
	pub := validPubKeyB64()
	body, _ := json.Marshal(RegisterRequest{
		UUID:      uuid.String(),
		Owner:     "wire-owner",
		Level:     1,
		PublicKey: pub,
	})
	rec := httptest.NewRecorder()
	h.Register(rec, httptest.NewRequest("POST", "/api/v2/agents/register", bytes.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestAPI_Register_BadJSON(t *testing.T) {
	h := newWiredHandler()
	rec := httptest.NewRecorder()
	h.Register(rec, httptest.NewRequest("POST", "/api/v2/agents/register", bytes.NewReader([]byte("not-json"))))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestAPI_Register_BadPubKey_Wired(t *testing.T) {
	h := newWiredHandler()
	uuid := validUUID()
	body, _ := json.Marshal(RegisterRequest{
		UUID:      uuid.String(),
		Owner:     "wire-owner",
		Level:     1,
		PublicKey: "not-base64!",
	})
	rec := httptest.NewRecorder()
	h.Register(rec, httptest.NewRequest("POST", "/api/v2/agents/register", bytes.NewReader(body)))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestAPI_Register_WrongMethod(t *testing.T) {
	h := newWiredHandler()
	rec := httptest.NewRecorder()
	h.Register(rec, httptest.NewRequest("GET", "/api/v2/agents/register", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestAPI_AgentByPath_GetInfo_NotFound(t *testing.T) {
	h := newWiredHandler()
	rec := httptest.NewRecorder()
	h.AgentByPath(rec, httptest.NewRequest("GET", "/api/v2/agents/"+validUUID().String(), nil))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("not-found should be 400, got %d", rec.Code)
	}
}

func TestAPI_AgentByPath_Upgrade_NotFound(t *testing.T) {
	h := newWiredHandler()
	body := bytes.NewBufferString(`{"new_level":1}`)
	rec := httptest.NewRecorder()
	h.AgentByPath(rec, httptest.NewRequest("POST", "/api/v2/agents/"+validUUID().String()+"/upgrade", body))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("not-found should be 400, got %d", rec.Code)
	}
}

func TestAPI_AgentByPath_Check_NotFound(t *testing.T) {
	h := newWiredHandler()
	body := bytes.NewBufferString(`{"bit":0}`)
	rec := httptest.NewRecorder()
	h.AgentByPath(rec, httptest.NewRequest("POST", "/api/v2/agents/"+validUUID().String()+"/check", body))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("not-found should be 400, got %d", rec.Code)
	}
}

func TestAPI_AgentByPath_Ban_NotFound(t *testing.T) {
	h := newWiredHandler()
	body := bytes.NewBufferString(`{"reason":"test"}`)
	rec := httptest.NewRecorder()
	h.AgentByPath(rec, httptest.NewRequest("POST", "/api/v2/agents/"+validUUID().String()+"/ban", body))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("not-found should be 400, got %d", rec.Code)
	}
}

func TestAPI_AgentByPath_Unban_NotFound(t *testing.T) {
	h := newWiredHandler()
	body := bytes.NewBufferString(`{}`)
	rec := httptest.NewRecorder()
	h.AgentByPath(rec, httptest.NewRequest("POST", "/api/v2/agents/"+validUUID().String()+"/unban", body))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("not-found should be 400, got %d", rec.Code)
	}
}

func TestAPI_AgentByPath_Revoke_NotFound(t *testing.T) {
	h := newWiredHandler()
	body := bytes.NewBufferString(`{"reason":"x"}`)
	rec := httptest.NewRecorder()
	h.AgentByPath(rec, httptest.NewRequest("POST", "/api/v2/agents/"+validUUID().String()+"/revoke", body))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("not-found should be 400, got %d", rec.Code)
	}
}

func TestAPI_AgentByPath_WrongPath(t *testing.T) {
	h := newWiredHandler()
	rec := httptest.NewRecorder()
	h.AgentByPath(rec, httptest.NewRequest("GET", "/api/v2/agents/"+validUUID().String()+"/unknown", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestAPI_Upgrade_BadJSON_Wired(t *testing.T) {
	h := newWiredHandler()
	rec := httptest.NewRecorder()
	body := bytes.NewBufferString(`{not json`)
	h.upgrade(rec, httptest.NewRequest("POST", "/", body), validUUID())
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestAPI_Ban_BadJSON(t *testing.T) {
	h := newWiredHandler()
	rec := httptest.NewRecorder()
	h.ban(rec, httptest.NewRequest("POST", "/", bytes.NewBufferString(`{`)), validUUID())
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestAPI_Revoke_BadJSON(t *testing.T) {
	h := newWiredHandler()
	rec := httptest.NewRecorder()
	h.revoke(rec, httptest.NewRequest("POST", "/", bytes.NewBufferString(`{`)), validUUID())
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestAPI_CheckPerm_BadJSON(t *testing.T) {
	h := newWiredHandler()
	rec := httptest.NewRecorder()
	h.checkPerm(rec, httptest.NewRequest("POST", "/", bytes.NewBufferString(`{`)), validUUID())
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestAPI_Unban_BadJSON(t *testing.T) {
	h := newWiredHandler()
	rec := httptest.NewRecorder()
	h.unban(rec, httptest.NewRequest("POST", "/", bytes.NewBufferString(`{`)), validUUID())
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d", rec.Code)
	}
}

// base64.RawURLEncoding helper 防止 unused
var _ = base64.RawURLEncoding.EncodeToString
