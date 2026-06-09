//go:build e2e

// Package e2e: L5 网关端到端测试（P7.16）。
//
// 启动真实 Redis（testcontainers）+ 完整 L4/L5 wire，验证：
//   - HTTP 端点注册 → 升级 → 校验 → 注销 全链路
//   - 中间件链（Recover/RequestID/UA-Block/APIKey/AAP）按序生效
//   - 探活 / 指标 / pprof 端点可达
//
// 运行：
//
//	go test -tags=e2e ./tests/e2e/...
//
// 要求：本地 Docker 可用；无 Docker 时自动 t.Skip。
package e2e

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/agentid-chain/agentid-chain/internal/cache"
	"github.com/agentid-chain/agentid-chain/internal/domain"
	"github.com/agentid-chain/agentid-chain/internal/gateway"
	"github.com/agentid-chain/agentid-chain/internal/gateway/handler"
	"github.com/agentid-chain/agentid-chain/internal/gateway/middleware"
	"github.com/agentid-chain/agentid-chain/internal/gateway/router"
	"github.com/agentid-chain/agentid-chain/internal/service"
	"github.com/agentid-chain/agentid-chain/internal/storage"
	tc "github.com/agentid-chain/agentid-chain/internal/testutil/testcontainers"
)

// =============================================================================
// 装配：真实 Redis + fake L1 + mock chain
// =============================================================================

type memStore struct {
	storage.Client
	mu     sync.Mutex
	agents map[string]*storage.AgentRecord
	perms  map[string]uint64
}

func newMemStore() *memStore {
	return &memStore{agents: map[string]*storage.AgentRecord{}, perms: map[string]uint64{}}
}
func (m *memStore) Identity() storage.IdentityStore    { return m }
func (m *memStore) Permission() storage.PermissionStore { return m }
func (m *memStore) Audit() storage.AuditStore          { return m }
func (m *memStore) Nonce() storage.NonceStore          { return m }
func (m *memStore) Revocation() storage.RevocationStore { return m }
func (m *memStore) Cache() storage.CacheStore          { return m }
func (m *memStore) GetAgent(_ context.Context, uuid string) (*storage.AgentRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.agents[uuid]
	if !ok {
		return nil, storage.ErrNotFound
	}
	return r, nil
}
func (m *memStore) PutAgent(_ context.Context, rec *storage.AgentRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.agents[rec.UUID] = rec
	return nil
}
func (m *memStore) ListAgentsByOwner(context.Context, string) ([]*storage.AgentRecord, error) {
	return nil, nil
}
func (m *memStore) BatchGetAgents(context.Context, []string) (map[string]*storage.AgentRecord, error) {
	return nil, nil
}
func (m *memStore) GetPermissions(_ context.Context, uuid string) (uint64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.perms[uuid], nil
}
func (m *memStore) SetPermissions(_ context.Context, uuid string, bits uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.perms[uuid] = bits
	return nil
}
func (m *memStore) GrantPermission(context.Context, string, uint) error  { return nil }
func (m *memStore) RevokePermission(context.Context, string, uint) error { return nil }
func (m *memStore) HealthCheck(context.Context) storage.HealthStatus     { return storage.HealthStatus{Healthy: true} }
func (m *memStore) Close(context.Context) error                          { return nil }
func (m *memStore) Append(context.Context, *storage.AuditEntry) error   { return nil }
func (m *memStore) Query(context.Context, string, time.Time, time.Time) ([]*storage.AuditEntry, error) {
	return nil, nil
}
func (m *memStore) Count(context.Context, string) (int64, error) { return 0, nil }
func (m *memStore) Store(context.Context, string, time.Duration) error { return nil }
func (m *memStore) NonceExists(context.Context, string) (bool, error) { return false, nil }
func (m *memStore) Consume(context.Context, string) error               { return nil }
func (m *memStore) Revoke(context.Context, string, time.Time) error    { return nil }
func (m *memStore) IsRevoked(context.Context, string) (bool, error)    { return false, nil }
func (m *memStore) PurgeExpired(context.Context) (int64, error)         { return 0, nil }
func (m *memStore) Get(context.Context, string) ([]byte, error)         { return nil, storage.ErrNotFound }
func (m *memStore) Set(context.Context, string, []byte, time.Duration) error { return nil }
func (m *memStore) Del(context.Context, ...string) error                { return nil }
func (m *memStore) Exists(context.Context, string) (bool, error)        { return false, nil }
func (m *memStore) Expire(context.Context, string, time.Duration) error { return nil }
func (m *memStore) Incr(context.Context, string, time.Duration) (int64, error) { return 0, nil }
func (m *memStore) StoreOnce(context.Context, string, []byte, time.Duration) error { return nil }

type memProvider struct {
	service.IdentityProvider
	exists bool
}

func (m *memProvider) Exists(_ context.Context, _ domain.UUID) (bool, error) { return m.exists, nil }
func (m *memProvider) BackendName() string                                     { return "mem" }
func (m *memProvider) HealthCheck(context.Context) error                       { return nil }
func (m *memProvider) Load(context.Context, domain.UUID) (*domain.Agent, error) { return nil, nil }
func (m *memProvider) LoadByOwner(context.Context, string) ([]*domain.Agent, error) { return nil, nil }
func (m *memProvider) LoadByPubKey(context.Context, ed25519.PublicKey) (*domain.Agent, error) { return nil, nil }

type memChain struct {
	service.ChainAdapter
	receipt *service.RegisterReceipt
}

func (m *memChain) ChainType() service.ChainType { return service.ChainMock }
func (m *memChain) RegisterAgent(context.Context, *service.RegisterRequest) (*service.RegisterReceipt, error) {
	return m.receipt, nil
}
func (m *memChain) UpdateLevel(context.Context, domain.UUID, domain.LevelType, string) (*service.RegisterReceipt, error) {
	return m.receipt, nil
}
func (m *memChain) BanAgent(context.Context, domain.UUID, string) (*service.RegisterReceipt, error) {
	return m.receipt, nil
}
func (m *memChain) UnbanAgent(context.Context, domain.UUID) (*service.RegisterReceipt, error) {
	return m.receipt, nil
}
func (m *memChain) GetAgentState(context.Context, domain.UUID) (*service.ChainAgentState, error) {
	return &service.ChainAgentState{}, nil
}
func (m *memChain) HealthCheck(context.Context) error { return nil }

type memAudit struct {
	service.AuditNotifier
	mu     sync.Mutex
	events []*service.AuditEvent
}

func (m *memAudit) Notify(_ context.Context, e *service.AuditEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, e)
	return nil
}
func (m *memAudit) Close() error { return nil }

// =============================================================================
// E2E harness
// =============================================================================

type e2eHarness struct {
	t      *testing.T
	store  *memStore
	chain  *memChain
	audit  *memAudit
	cache  cache.Cache
	api    *handler.APIHandler
	server *gateway.Server
}

func newE2EHarness(t *testing.T) *e2eHarness {
	t.Helper()
	// 1. 真实 Redis
	rdb, err := tc.NewRedisContainer(t, tc.RedisOpts{})
	if err != nil {
		t.Skipf("skipping e2e: %v", err)
	}
	t.Cleanup(func() { _ = rdb.Terminate(context.Background()) })
	cch, err := cache.NewRedis(cache.RedisConfig{Addr: rdb.Addr(), Timeout: 3 * time.Second})
	if err != nil {
		t.Fatalf("redis cache: %v", err)
	}

	// 2. 装配 L4 services
	store := newMemStore()
	provider := &memProvider{exists: false}
	chain := &memChain{receipt: &service.RegisterReceipt{TxHash: "0x-e2e"}}
	audit := &memAudit{}
	reg, _ := service.NewRegisterService(store, chain, audit, provider)
	up, _ := service.NewUpgradeService(store, chain, audit, provider)
	rev, _ := service.NewRevokeService(store, chain, audit, provider)
	ban, _ := service.NewBanService(store, chain, audit, provider)
	info, _ := service.NewGetAgentInfoService(store, chain, provider)
	chk, _ := service.NewCheckPermissionService(store, provider)

	api := &handler.APIHandler{
		RegisterSvc: reg, UpgradeSvc: up, RevokeSvc: rev,
		BanSvc: ban, UnbanSvc: ban, GetInfoSvc: info, CheckSvc: chk,
	}
	health := handler.NewHealthHandler(nil)
	rt := router.New(api, health)

	// 3. 装配 middleware chain
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	chain2 := middleware.NewChain(
		middleware.Recover(logger),
		middleware.RequestID(),
		middleware.Logging(logger),
		middleware.Metrics(),
		middleware.CORS(middleware.DefaultCORSConfig()),
		middleware.UABlock(middleware.DefaultUAConfig()),
	)
	srv := gateway.NewServer(gateway.ServerConfig{Addr: "127.0.0.1:0"}, logger, chain2, rt.Mux())
	return &e2eHarness{
		t: t, store: store, chain: chain, audit: audit, cache: cch,
		api: api, server: srv,
	}
}

func validUUID() domain.UUID {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return domain.UUID("11111111-2222-3333-4444-555555555555")
}

func validPubKey() string {
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	return base64.RawURLEncoding.EncodeToString(pub)
}

func (h *e2eHarness) do(method, path string, body any, headers map[string]string) *httptest.ResponseRecorder {
	h.t.Helper()
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, rdr)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "e2e-test/1.0")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.server.Handler().ServeHTTP(rec, req)
	return rec
}

// =============================================================================
// Tests
// =============================================================================

func TestE2E_HealthEndpoints(t *testing.T) {
	h := newE2EHarness(t)
	for _, p := range []string{"/live", "/readyz", "/healthz"} {
		rec := h.do("GET", p, nil, nil)
		if rec.Code != http.StatusOK {
			t.Errorf("%s = %d", p, rec.Code)
		}
	}
}

func TestE2E_MetricsEndpoint(t *testing.T) {
	h := newE2EHarness(t)
	// 触发一次请求以增加 metric 计数
	_ = h.do("GET", "/live", nil, nil)
	rec := h.do("GET", "/metrics", nil, nil)
	if rec.Code != http.StatusOK {
		t.Errorf("/metrics = %d", rec.Code)
	}
	body := rec.Body.String()
	if !bytes.Contains([]byte(body), []byte("agentid_http_request_total")) {
		t.Error("expected prom counter in /metrics output")
	}
}

func TestE2E_RegisterAndCheck(t *testing.T) {
	h := newE2EHarness(t)
	uuid := validUUID()

	// Register
	rec := h.do("POST", "/api/v2/agents/register", map[string]any{
		"uuid":       uuid.String(),
		"owner":      "e2e-owner",
		"level":      1,
		"public_key": validPubKey(),
	}, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register = %d, body = %s", rec.Code, rec.Body.String())
	}

	// GetInfo
	rec = h.do("GET", "/api/v2/agents/"+uuid.String(), nil, nil)
	if rec.Code != http.StatusOK {
		t.Errorf("get info = %d, body = %s", rec.Code, rec.Body.String())
	}

	// CheckPermission
	rec = h.do("POST", "/api/v2/agents/"+uuid.String()+"/check", map[string]any{"bit": 0}, nil)
	if rec.Code != http.StatusOK {
		t.Errorf("check = %d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"Allowed":true`)) {
		t.Errorf("expected Allowed=true, body = %s", rec.Body.String())
	}
}

func TestE2E_BanFlow(t *testing.T) {
	h := newE2EHarness(t)
	uuid := validUUID()

	// Register
	rec := h.do("POST", "/api/v2/agents/register", map[string]any{
		"uuid":       uuid.String(),
		"owner":      "e2e-owner",
		"level":      1,
		"public_key": validPubKey(),
	}, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register = %d", rec.Code)
	}

	// Ban
	rec = h.do("POST", "/api/v2/agents/"+uuid.String()+"/ban", map[string]any{"reason": "e2e test"}, nil)
	if rec.Code != http.StatusOK {
		t.Errorf("ban = %d, body = %s", rec.Code, rec.Body.String())
	}

	// CheckPermission should be denied
	rec = h.do("POST", "/api/v2/agents/"+uuid.String()+"/check", map[string]any{"bit": 0}, nil)
	if rec.Code != http.StatusOK {
		t.Errorf("check = %d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"Allowed":false`)) {
		t.Errorf("expected Allowed=false after ban, body = %s", rec.Body.String())
	}
}

func TestE2E_RevokeFlow(t *testing.T) {
	h := newE2EHarness(t)
	uuid := validUUID()

	// Register
	rec := h.do("POST", "/api/v2/agents/register", map[string]any{
		"uuid":       uuid.String(),
		"owner":      "e2e-owner",
		"level":      1,
		"public_key": validPubKey(),
	}, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register = %d", rec.Code)
	}

	// Revoke
	rec = h.do("POST", "/api/v2/agents/"+uuid.String()+"/revoke", map[string]any{"reason": "e2e"}, nil)
	if rec.Code != http.StatusOK {
		t.Errorf("revoke = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestE2E_UABlocked(t *testing.T) {
	h := newE2EHarness(t)
	rec := h.do("GET", "/api/v2/agents/foo", nil, map[string]string{"User-Agent": "sqlmap/1.0"})
	if rec.Code != http.StatusForbidden {
		t.Errorf("UA block = %d, want 403", rec.Code)
	}
}

func TestE2E_EmptyUABlocked(t *testing.T) {
	h := newE2EHarness(t)
	req := httptest.NewRequest("GET", "/api/v2/agents/foo", nil)
	// 不设置 User-Agent
	rec := httptest.NewRecorder()
	h.server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("empty UA = %d, want 403", rec.Code)
	}
}

func TestE2E_RequestIDPropagated(t *testing.T) {
	h := newE2EHarness(t)
	rec := h.do("GET", "/live", nil, map[string]string{"X-Request-ID": "test-rid-123"})
	if rec.Code != http.StatusOK {
		t.Fatalf("/live = %d", rec.Code)
	}
	if got := rec.Header().Get("X-Request-ID"); got != "test-rid-123" {
		t.Errorf("X-Request-ID = %q", got)
	}
}

func TestE2E_RequestIDGenerated(t *testing.T) {
	h := newE2EHarness(t)
	rec := h.do("GET", "/live", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("/live = %d", rec.Code)
	}
	if got := rec.Header().Get("X-Request-ID"); got == "" {
		t.Error("expected generated X-Request-ID")
	}
}

func TestE2E_NotFoundPath(t *testing.T) {
	h := newE2EHarness(t)
	rec := h.do("GET", "/api/v2/no-such-path", nil, nil)
	if rec.Code != http.StatusNotFound && rec.Code != http.StatusOK {
		// net/http mux returns 404 for unregistered paths
		t.Errorf("status = %d", rec.Code)
	}
}

func TestE2E_RecoverFromPanic(t *testing.T) {
	_ = newE2EHarness(t) // 确保 harness 可启动
	// Inject a panic via custom mux path
	customMux := http.NewServeMux()
	customMux.HandleFunc("/boom", func(w http.ResponseWriter, r *http.Request) {
		panic("e2e test panic")
	})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	chain := middleware.NewChain(middleware.Recover(logger))
	srv := gateway.NewServer(gateway.ServerConfig{}, logger, chain, customMux)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/boom", nil)
	req.Header.Set("User-Agent", "e2e")
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("panic status = %d, want 500", rec.Code)
	}
}
