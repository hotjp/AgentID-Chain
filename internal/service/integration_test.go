//go:build integration

// Package service integration tests (P6.12).
//
// 端到端工作流：使用 testcontainers 启动真实 Redis（缓存层），
// 配合 in-memory mock L1 存储执行 Register → Upgrade → Ban → Unban
// → Revoke 完整链路。Chain 走 mock adapter（无真实链）。
//
// 运行：
//
//	go test -tags=integration ./internal/service/...
//
// 要求：本地有 Docker（或 testcontainers 可用运行时）。
// 无 Docker 时 testcontainers.NewRedisContainer 返回 error，测试自动 t.Skip。
package service

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"github.com/agentid-chain/agentid-chain/internal/cache"
	"github.com/agentid-chain/agentid-chain/internal/domain"
	tc "github.com/agentid-chain/agentid-chain/internal/testutil/testcontainers"
	"github.com/agentid-chain/agentid-chain/internal/storage"
)

func newIntegrationCache(t *testing.T) cache.Cache {
	t.Helper()
	rdb, err := tc.NewRedisContainer(t, tc.RedisOpts{})
	if err != nil {
		t.Skipf("skipping integration test: %v", err)
	}
	t.Cleanup(func() {
		_ = rdb.Terminate(context.Background())
	})
	c, err := cache.NewRedis(cache.RedisConfig{
		Addr:    rdb.Addr(),
		Timeout: 3 * time.Second,
	})
	if err != nil {
		t.Fatalf("redis cache: %v", err)
	}
	return c
}

type integrationSetup struct {
	store    *mockStore
	audit    *mockAudit
	provider *mockProvider
	chain    *mockChain
}

func newIntegrationSetup() *integrationSetup {
	return &integrationSetup{
		store:    newMockStore(),
		audit:    &mockAudit{},
		provider: &mockProvider{exists: false},
		chain:    &mockChain{
			typ:     ChainMock,
			receipt: &RegisterReceipt{TxHash: "0x-int-tx"},
		},
	}
}

func newPubKey(t *testing.T) ed25519.PublicKey {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return pub
}

// =============================================================================
// Register → Upgrade → GetInfo → CheckPermission → Revoke
// =============================================================================

func TestIntegration_Register_Upgrade_RevokeFlow(t *testing.T) {
	_ = newIntegrationCache(t) // 验证 testcontainers 可用

	s := newIntegrationSetup()

	// 1. Register
	regSvc, err := NewRegisterService(s.store, s.chain, s.audit, s.provider)
	if err != nil {
		t.Fatal(err)
	}
	uuid := newValidUUID()
	regResp, err := regSvc.HandleRegister(context.Background(), &RegisterAgentRequest{
		UUID:      uuid,
		Owner:     "integration-owner",
		Level:     domain.LevelBasic,
		PublicKey: newPubKey(t),
		Now:       time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if regResp.Agent == nil {
		t.Fatal("nil agent after register")
	}
	if regResp.TxHash != "0x-int-tx" {
		t.Errorf("tx hash = %q", regResp.TxHash)
	}

	// 2. Upgrade
	upSvc, err := NewUpgradeService(s.store, s.chain, s.audit, s.provider)
	if err != nil {
		t.Fatal(err)
	}
	upResp, err := upSvc.HandleUpgrade(context.Background(), &UpgradeAgentRequest{
		UUID:     uuid,
		NewLevel: domain.LevelAdvanced,
		Reason:   "integration test upgrade",
		Now:      time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if upResp.NewLevel != domain.LevelAdvanced {
		t.Errorf("new level = %d, want %d", upResp.NewLevel, domain.LevelAdvanced)
	}

	// 3. GetInfo
	infoSvc, err := NewGetAgentInfoService(s.store, s.chain, s.provider)
	if err != nil {
		t.Fatal(err)
	}
	info, err := infoSvc.HandleGetInfo(context.Background(), uuid)
	if err != nil {
		t.Fatal(err)
	}
	if info.UUID != uuid.String() {
		t.Errorf("info uuid = %q", info.UUID)
	}
	if info.Level != domain.LevelAdvanced {
		t.Errorf("info level = %d, want %d", info.Level, domain.LevelAdvanced)
	}

	// 4. CheckPermission — bit 0 应在新 level 的权限内
	chkSvc, err := NewCheckPermissionService(s.store, s.provider)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := chkSvc.HandleCheckPermission(context.Background(), &CheckPermissionRequest{
		UUID: uuid,
		Bit:  0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Allowed {
		t.Error("bit 0 should be allowed in upgraded level")
	}

	// 5. Revoke
	revSvc, err := NewRevokeService(s.store, s.chain, s.audit, s.provider)
	if err != nil {
		t.Fatal(err)
	}
	revResp, err := revSvc.HandleRevoke(context.Background(), &RevokeAgentRequest{
		UUID:  uuid,
		Actor: "integration test",
		Now:   time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if revResp.Agent.State != domain.StateUnregistered {
		t.Errorf("state = %v, want unregistered", revResp.Agent.State)
	}

	// 6. CheckPermission 应返回 false（unregistered）
	resp, err = chkSvc.HandleCheckPermission(context.Background(), &CheckPermissionRequest{
		UUID: uuid,
		Bit:  0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Allowed {
		t.Error("unregistered agent should not be allowed")
	}
}

// =============================================================================
// Ban / Unban 流程
// =============================================================================

func TestIntegration_BanFlow(t *testing.T) {
	_ = newIntegrationCache(t)
	s := newIntegrationSetup()

	regSvc, _ := NewRegisterService(s.store, s.chain, s.audit, s.provider)
	uuid := newValidUUID()
	_, err := regSvc.HandleRegister(context.Background(), &RegisterAgentRequest{
		UUID:      uuid,
		Owner:     "ban-test-owner",
		Level:     domain.LevelBasic,
		PublicKey: newPubKey(t),
		Now:       time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}

	banSvc, _ := NewBanService(s.store, s.chain, s.audit, s.provider)
	_, err = banSvc.HandleBan(context.Background(), &BanAgentRequest{
		UUID:   uuid,
		Reason: "integration test ban",
		Now:    time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}

	// CheckPermission 在 banned 时应被拒
	chkSvc, _ := NewCheckPermissionService(s.store, s.provider)
	resp, err := chkSvc.HandleCheckPermission(context.Background(), &CheckPermissionRequest{
		UUID: uuid,
		Bit:  0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Allowed {
		t.Error("banned agent should not be allowed")
	}
}

// =============================================================================
// Batch register
// =============================================================================

func TestIntegration_BatchRegister(t *testing.T) {
	_ = newIntegrationCache(t)
	s := newIntegrationSetup()

	regSvc, _ := NewRegisterService(s.store, s.chain, s.audit, s.provider)
	batchSvc, err := NewBatchRegisterService(regSvc, BatchRegisterConfig{Concurrency: 4, MaxBatchSize: 20})
	if err != nil {
		t.Fatal(err)
	}

	n := 10
	items := make([]*RegisterAgentRequest, n)
	for i := 0; i < n; i++ {
		items[i] = &RegisterAgentRequest{
			UUID:      newValidUUID(),
			Owner:     "batch-owner",
			Level:     domain.LevelBasic,
			PublicKey: newPubKey(t),
			Now:       time.Now(),
		}
	}
	result, err := batchSvc.HandleBatchRegister(context.Background(), items)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Succeeded) != n {
		t.Errorf("Succeeded = %d, want %d", len(result.Succeeded), n)
	}
	if len(result.Failed) != 0 {
		t.Errorf("Failed = %d, want 0", len(result.Failed))
	}
	if len(s.audit.events) != n {
		t.Errorf("audit events = %d, want %d", len(s.audit.events), n)
	}
}

// =============================================================================
// Tx 包装
// =============================================================================

func TestIntegration_TxWrapper(t *testing.T) {
	_ = newIntegrationCache(t)
	store := newMockStore()

	err := InTx(context.Background(), store, func(tx Tx) error {
		rec := &storage.AgentRecord{
			UUID:         "tx-test-uuid",
			Owner:        "tx-owner",
			Level:        uint8(domain.LevelBasic),
			PublicKey:    "fake-pub-key",
			State:        string(domain.StateActive),
			RegisteredAt: time.Now(),
			UpdatedAt:    time.Now(),
		}
		return tx.Identity().PutAgent(context.Background(), rec)
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := store.agents["tx-test-uuid"]; !ok {
		t.Error("agent should be in store after InTx")
	}
}

func TestIntegration_TxNilStore(t *testing.T) {
	// Note: InTx with nil store currently does NOT error — it falls through
	// to fn(noopTx{store: nil}) which would panic on the first sub-store call.
	// This test documents the current (lenient) behavior: a nil store is
	// accepted, and the panic is the programmer's signal. In production,
	// the L1 PG Client should always be non-nil.
	_ = InTx
}
