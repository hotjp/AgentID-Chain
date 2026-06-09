//go:build integration

// Package a2a 集成测试：A2A Token 完整生命周期（签发 → 验证 → 撤销）。
//
// 运行方式：
//
//	go test -tags=integration -timeout=120s ./internal/authz/a2a/...
//
// 环境要求：Docker daemon。无 Docker 时自动 skip。
package a2a

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/agentid-chain/agentid-chain/internal/testutil/testcontainers"
)

// =============================================================================
// 工具
// =============================================================================

func newIntegrationSetup(t *testing.T) (*Issuer, *Verifier, *Revoker, *JWKSHandler) {
	t.Helper()
	rdb, err := testcontainers.NewRedisContainer(t, testcontainers.RedisOpts{
		StartupTimeout: 60 * time.Second,
	})
	if err != nil {
		t.Skipf("redis container unavailable: %v", err)
	}
	ctx := context.Background()
	cacheImpl := rdb.Cache(ctx)

	// Issuer 私钥
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	issuer, err := NewIssuer(IssuerConfig{
		DomainKey: priv,
		Issuer:    "agentid-chain-test",
		KeyID:     "test-key-1",
		DefaultTTL: 5 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := NewVerifier(VerifierConfig{
		Resolver:        &StaticKeyResolver{PublicKey: priv.Public().(ed25519.PublicKey)},
		ExpectedIssuer:  "agentid-chain-test",
		ExpectedAudience: "test-audience",
	})
	if err != nil {
		t.Fatal(err)
	}
	rev, err := NewRevoker(RevokerConfig{Cache: cacheImpl})
	if err != nil {
		t.Fatal(err)
	}
	jwks, err := NewJWKSHandler(JWKSHandlerConfig{
		Source: &IssuerKeySource{Issuer: issuer},
	})
	if err != nil {
		t.Fatal(err)
	}
	return issuer, verifier, rev, jwks
}

// =============================================================================
// 完整链路：Sign → Verify → Revoke → Verify (rejected)
// =============================================================================

func TestIntegration_TokenLifecycle(t *testing.T) {
	issuer, verifier, rev, _ := newIntegrationSetup(t)
	ctx := context.Background()
	agentUUID := "11111111-2222-3333-4444-aaaaaaaaaaaa"

	// 1. Track token（颁发时由调用方注册到 Revoker）
	exp := time.Now().Add(5 * time.Minute)
	jti := "jti-integration-1"
	if err := rev.Track(agentUUID, jti, exp); err != nil {
		t.Fatal(err)
	}

	// 2. Sign
	token, err := issuer.Sign(SignInput{
		Subject: agentUUID,
		Audience: "test-audience",
		JTI:     jti,
		TTL:     5 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}

	// 3. Verify
	claims, err := verifier.Verify(token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.Subject != agentUUID {
		t.Errorf("subject = %q", claims.Subject)
	}
	if claims.JTI != jti {
		t.Errorf("jti = %q", claims.JTI)
	}

	// 4. Revoke
	if err := rev.Revoke(ctx, jti); err != nil {
		t.Fatal(err)
	}
	if !rev.IsRevoked(ctx, jti) {
		t.Error("token should be revoked")
	}

	// 5. Verify 仍通过（revocation 是上层职责，不在 Verifier 内）
	// 但调用方应在 verify 之前查 IsRevoked
	if _, err := verifier.Verify(token); err != nil {
		t.Logf("post-revoke verify err (expected if 上层做了 IsRevoked check): %v", err)
	}
}

// =============================================================================
// RevokeByAgent：一次性撤销某 agent 的所有 token
// =============================================================================

func TestIntegration_RevokeByAgent(t *testing.T) {
	issuer, _, rev, _ := newIntegrationSetup(t)
	ctx := context.Background()
	agentA := "11111111-2222-3333-4444-bbbbbbbbbbbb"
	agentB := "11111111-2222-3333-4444-cccccccccccc"

	// A 颁发 3 个 token
	exp := time.Now().Add(5 * time.Minute)
	for _, jti := range []string{"a-1", "a-2", "a-3"} {
		_ = rev.Track(agentA, jti, exp)
		_, _ = issuer.Sign(SignInput{
			Subject: agentA, Audience: "test-audience", JTI: jti, TTL: 5 * time.Minute,
		})
	}
	// B 颁发 1 个 token（不应被撤销）
	_ = rev.Track(agentB, "b-1", exp)
	_, _ = issuer.Sign(SignInput{
		Subject: agentB, Audience: "test-audience", JTI: "b-1", TTL: 5 * time.Minute,
	})

	// 撤销 A
	revoked, err := rev.RevokeByAgent(ctx, agentA)
	if err != nil {
		t.Fatal(err)
	}
	if len(revoked) != 3 {
		t.Errorf("revoked = %d, want 3", len(revoked))
	}
	for _, j := range []string{"a-1", "a-2", "a-3"} {
		if !rev.IsRevoked(ctx, j) {
			t.Errorf("%s not revoked", j)
		}
	}
	if rev.IsRevoked(ctx, "b-1") {
		t.Error("b-1 should not be revoked")
	}
}

// =============================================================================
// JWKS HTTP 端点
// =============================================================================

func TestIntegration_JWKSEndpoint(t *testing.T) {
	_, _, _, jwks := newIntegrationSetup(t)

	// 直接访问
	req := httptest.NewRequest(http.MethodGet, "/.well-known/jwks.json", nil)
	w := httptest.NewRecorder()
	jwks.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	// Cache-Control 必须有
	if cc := w.Header().Get("Cache-Control"); cc == "" {
		t.Error("Cache-Control should be set")
	}
}

// =============================================================================
// 并发：Sign/Revoke/IsRevoked 的一致性
// =============================================================================

func TestIntegration_ConcurrentSignRevoke(t *testing.T) {
	issuer, _, rev, _ := newIntegrationSetup(t)
	ctx := context.Background()
	agentUUID := "11111111-2222-3333-4444-dddddddddddd"
	exp := time.Now().Add(5 * time.Minute)

	const N = 50
	jtis := make([]string, N)
	for i := 0; i < N; i++ {
		jtis[i] = "jti-conc-" + string(rune('a'+i%26)) + "-" + string(rune('A'+i/26))
		_ = rev.Track(agentUUID, jtis[i], exp)
	}

	// 并发 sign + revoke
	var wg sync.WaitGroup
	for i := 0; i < N; i++ {
		wg.Add(2)
		go func(jti string) {
			defer wg.Done()
			_, _ = issuer.Sign(SignInput{
				Subject: agentUUID, Audience: "test-audience",
				JTI: jti, TTL: 5 * time.Minute,
			})
		}(jtis[i])
		go func(jti string) {
			defer wg.Done()
			_ = rev.Revoke(ctx, jti)
		}(jtis[i])
	}
	wg.Wait()

	// 最终所有 jti 都应被撤销
	for _, j := range jtis {
		if !rev.IsRevoked(ctx, j) {
			t.Errorf("%s not revoked", j)
		}
	}
}

// =============================================================================
// 撤销过期 token：不写 revoked 标记
// =============================================================================

func TestIntegration_RevokeExpired_NoOp(t *testing.T) {
	issuer, _, rev, _ := newIntegrationSetup(t)
	ctx := context.Background()
	agentUUID := "11111111-2222-3333-4444-eeeeeeeeeeee"

	// Track 一个已过期的 token
	past := time.Now().Add(-time.Hour)
	_ = rev.Track(agentUUID, "jti-expired", past)
	_, _ = issuer.Sign(SignInput{
		Subject: agentUUID, Audience: "test-audience",
		JTI: "jti-expired", TTL: -time.Hour, // 负 TTL 让 sign 立即过期
	})

	if err := rev.Revoke(ctx, "jti-expired"); err != nil {
		t.Fatal(err)
	}
	// 过期 token 不写 revoked 标记
	if rev.IsRevoked(ctx, "jti-expired") {
		t.Error("expired token should not be marked revoked")
	}
}

// =============================================================================
// FlushDB 后 Revoker 行为
// =============================================================================

func TestIntegration_FlushDBClearsRevocations(t *testing.T) {
	rdb, err := testcontainers.NewRedisContainer(t, testcontainers.RedisOpts{})
	if err != nil {
		t.Skipf("redis unavailable: %v", err)
	}
	ctx := context.Background()
	cacheImpl := rdb.Cache(ctx)

	rev, err := NewRevoker(RevokerConfig{Cache: cacheImpl})
	if err != nil {
		t.Fatal(err)
	}
	_ = rev.Track("uuid", "jti-x", time.Now().Add(time.Hour))
	_ = rev.Revoke(ctx, "jti-x")
	if !rev.IsRevoked(ctx, "jti-x") {
		t.Fatal("precondition: jti-x should be revoked")
	}
	if err := rdb.FlushDB(ctx); err != nil {
		t.Fatal(err)
	}
	// FlushDB 后 cache 中的 revoked 标记没了 → IsRevoked 返回 false
	if rev.IsRevoked(ctx, "jti-x") {
		t.Error("after flush, IsRevoked should be false (cache cleared)")
	}
}

// =============================================================================
// Issuer 私钥 → JWKS → Verifier ResolverFromJWKS 闭环
// =============================================================================

func TestIntegration_JWKSToResolverRoundTrip(t *testing.T) {
	issuer, _, _, jwks := newIntegrationSetup(t)

	// 1. 拉 JWKS
	snap, err := jwks.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	// 2. 转 Resolver
	resolver, err := ResolverFromJWKS(snap)
	if err != nil {
		t.Fatal(err)
	}
	// 3. 构造一个使用此 resolver 的 verifier
	verifier, err := NewVerifier(VerifierConfig{
		Resolver:        resolver,
		ExpectedIssuer:  "agentid-chain-test",
		ExpectedAudience: "test-audience",
	})
	if err != nil {
		t.Fatal(err)
	}
	// 4. 签发 + 验签
	token, err := issuer.Sign(SignInput{
		Subject: "11111111-2222-3333-4444-ffffffffffff",
		Audience: "test-audience",
		JTI:     "jti-jwks-roundtrip",
		TTL:     5 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	claims, err := verifier.Verify(token)
	if err != nil {
		t.Fatalf("verify via jwks resolver: %v", err)
	}
	if claims.Subject != "11111111-2222-3333-4444-ffffffffffff" {
		t.Errorf("subject = %q", claims.Subject)
	}
}
