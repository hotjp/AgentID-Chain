//go:build integration

// Package aap 集成测试：完整 Challenge → Verify → Proof 链路（用 testcontainers Redis）。
//
// 运行方式：
//
//	go test -tags=integration -timeout=120s ./internal/authz/aap/...
//
// 环境要求：Docker daemon。无 Docker 时自动 skip。
package aap

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/agentid-chain/agentid-chain/internal/cache"
	"github.com/agentid-chain/agentid-chain/internal/testutil/testcontainers"
)

// =============================================================================
// 工具
// =============================================================================

func newIntegrationSetup(t *testing.T) (*Generator, *Verifier, *ProofSigner, cache.Cache) {
	t.Helper()
	rdb, err := testcontainers.NewRedisContainer(t, testcontainers.RedisOpts{
		StartupTimeout: 60 * time.Second,
	})
	if err != nil {
		t.Skipf("redis container unavailable: %v", err)
	}
	ctx := context.Background()
	cacheImpl := rdb.Cache(ctx)

	// 域签名密钥
	_, domainPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	gen, err := NewGenerator(cacheImpl, Config{
		DomainKey: domainPriv,
	})
	if err != nil {
		t.Fatal(err)
	}
	ver, err := NewVerifier(gen, VerifierConfig{})
	if err != nil {
		t.Fatal(err)
	}
	signer, err := NewProofSigner(domainPriv, "agentid-chain-test")
	if err != nil {
		t.Fatal(err)
	}
	return gen, ver, signer, cacheImpl
}

// signChallenge 客户端用自己私钥对 (challenge_id || nonce || issued_at || agent_uuid) 签名。
//
// 与 verify.go 的 sign payload 逻辑一致。输出 base64.RawURLEncoding。
func signChallenge(t *testing.T, priv ed25519.PrivateKey, c *Challenge, agentUUID string) string {
	t.Helper()
	payload := responsePayload(c.ChallengeID, c.Nonce, c.IssuedAt, agentUUID)
	sig := ed25519.Sign(priv, payload)
	return base64.RawURLEncoding.EncodeToString(sig)
}

// =============================================================================
// 完整链路：Challenge → Verify → Proof → Verify
// =============================================================================

func TestIntegration_ChallengeVerifyProofRoundTrip(t *testing.T) {
	gen, ver, signer, _ := newIntegrationSetup(t)
	ctx := context.Background()

	// 1. 客户端生成 keypair
	clientPub, clientPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	agentUUID := "11111111-2222-3333-4444-555555555555"

	// 2. 客户端请求 Challenge
	ch, err := gen.Generate(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if ch.ChallengeID == "" || ch.Nonce == "" || ch.DomainSig == "" {
		t.Fatal("challenge incomplete")
	}

	// 3. 客户端签名 response
	resp := signChallenge(t, clientPriv, ch, agentUUID)

	// 4. 服务端 verify
	out, err := ver.Verify(ctx, VerifyInput{
		ChallengeID: ch.ChallengeID,
		Response:    resp,
		AgentPubKey: base64.StdEncoding.EncodeToString(clientPub),
		AgentUUID:   agentUUID,
		Now:         time.Now(),
	})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if string(out.AgentPubKey) != string(clientPub) {
		t.Errorf("agent pubkey mismatch")
	}

	// 5. 服务端 issue proof
	proof, err := ver.IssueProof(out, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if proof.AgentUUID != agentUUID {
		t.Errorf("proof uuid = %q", proof.AgentUUID)
	}

	// 6. proof → JWT → 验证
	agentPubBytes, err := base64.RawURLEncoding.DecodeString(proof.AgentPubKey)
	if err != nil {
		t.Fatal(err)
	}
	token, err := signer.Sign(SignInput{
		AgentUUID:   proof.AgentUUID,
		AgentPubKey: ed25519.PublicKey(agentPubBytes),
		JTI:         proof.ProofID,
		TTL:         5 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	claims, err := signer.Verify(token)
	if err != nil {
		t.Fatalf("proof verify: %v", err)
	}
	if claims.AgentUUID != agentUUID {
		t.Errorf("jwt uuid = %q", claims.AgentUUID)
	}

	// 7. Challenge 必须已消费（防重放）
	if _, err := gen.ConsumeChallenge(ctx, ch.ChallengeID); err == nil {
		t.Error("challenge should be consumed")
	}
}

// =============================================================================
// 重放攻击防护
// =============================================================================

func TestIntegration_ReplayRejected(t *testing.T) {
	gen, ver, _, _ := newIntegrationSetup(t)
	ctx := context.Background()

	clientPub, clientPriv, _ := ed25519.GenerateKey(rand.Reader)
	agentUUID := "11111111-2222-3333-4444-666666666666"

	ch, _ := gen.Generate(ctx)
	resp := signChallenge(t, clientPriv, ch, agentUUID)

	in := VerifyInput{
		ChallengeID: ch.ChallengeID,
		Response:    resp,
		AgentPubKey: base64.StdEncoding.EncodeToString(clientPub),
		AgentUUID:   agentUUID,
		Now:         time.Now(),
	}

	// 第一次 ok
	if _, err := ver.Verify(ctx, in); err != nil {
		t.Fatalf("first verify: %v", err)
	}
	// 第二次必须被拒（challenge 已消费）
	if _, err := ver.Verify(ctx, in); err == nil {
		t.Error("replay should fail")
	}
}

// =============================================================================
// 多客户端并发
// =============================================================================

func TestIntegration_ConcurrentClients(t *testing.T) {
	gen, ver, _, _ := newIntegrationSetup(t)
	ctx := context.Background()

	const N = 10
	var wg sync.WaitGroup
	hashes := make([][32]byte, N)
	errs := make([]error, N)

	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			clientPub, clientPriv, _ := ed25519.GenerateKey(rand.Reader)
			uuid := fmt.Sprintf("11111111-2222-3333-4444-%012d", idx)
			ch, err := gen.Generate(ctx)
			if err != nil {
				errs[idx] = err
				return
			}
			resp := signChallenge(t, clientPriv, ch, uuid)
			_, err = ver.Verify(ctx, VerifyInput{
				ChallengeID: ch.ChallengeID,
				Response:    resp,
				AgentPubKey: base64.StdEncoding.EncodeToString(clientPub),
				AgentUUID:   uuid,
				Now:         time.Now(),
			})
			if err != nil {
				errs[idx] = err
				return
			}
			hashes[idx] = sha256.Sum256([]byte(ch.ChallengeID))
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("client %d: %v", i, err)
		}
	}
	seen := map[[32]byte]bool{}
	for _, h := range hashes {
		if h == ([32]byte{}) {
			continue // 失败的客户端没有 hash
		}
		if seen[h] {
			t.Error("duplicate challenge id")
		}
		seen[h] = true
	}
	if len(seen) < N {
		t.Errorf("got %d unique, want %d", len(seen), N)
	}
}

// =============================================================================
// 错误路径：签名错误 → 拒
// =============================================================================

func TestIntegration_BadSignatureRejected(t *testing.T) {
	gen, ver, _, _ := newIntegrationSetup(t)
	ctx := context.Background()

	clientPub, _, _ := ed25519.GenerateKey(rand.Reader)
	// 用另一把 key 签名
	_, wrongPriv, _ := ed25519.GenerateKey(rand.Reader)
	agentUUID := "11111111-2222-3333-4444-777777777777"

	ch, _ := gen.Generate(ctx)
	resp := signChallenge(t, wrongPriv, ch, agentUUID)

	_, err := ver.Verify(ctx, VerifyInput{
		ChallengeID: ch.ChallengeID,
		Response:    resp,
		AgentPubKey: base64.StdEncoding.EncodeToString(clientPub),
		AgentUUID:   agentUUID,
		Now:         time.Now(),
	})
	if err == nil {
		t.Error("bad signature should be rejected")
	}
}

// =============================================================================
// 错误路径：Challenge 过期
// =============================================================================

func TestIntegration_ChallengeExpired(t *testing.T) {
	gen, ver, _, _ := newIntegrationSetup(t)
	ctx := context.Background()

	// 用 Generator 的 ChallengeTTL（30s 默认），但 Verify 的 Now 推后到 ResponseMaxTTL 之外
	clientPub, clientPriv, _ := ed25519.GenerateKey(rand.Reader)
	agentUUID := "11111111-2222-3333-4444-888888888888"

	ch, _ := gen.Generate(ctx)
	resp := signChallenge(t, clientPriv, ch, agentUUID)

	// Now 推进 11 分钟（ResponseMaxTTL 默认 10 分钟）
	_, err := ver.Verify(ctx, VerifyInput{
		ChallengeID: ch.ChallengeID,
		Response:    resp,
		AgentPubKey: base64.StdEncoding.EncodeToString(clientPub),
		AgentUUID:   agentUUID,
		Now:         time.Now().Add(11 * time.Minute),
	})
	if err == nil {
		t.Error("expired response should be rejected")
	}
}

// =============================================================================
// 跨实例一致性：用 FlushDB 模拟重启，确保数据不污染
// =============================================================================

func TestIntegration_FlushDBClearsState(t *testing.T) {
	rdb, err := testcontainers.NewRedisContainer(t, testcontainers.RedisOpts{})
	if err != nil {
		t.Skipf("redis unavailable: %v", err)
	}
	ctx := context.Background()
	c := rdb.Cache(ctx)

	_, domainPriv, _ := ed25519.GenerateKey(rand.Reader)
	gen, _ := NewGenerator(c, Config{DomainKey: domainPriv})

	ch, err := gen.Generate(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := rdb.FlushDB(ctx); err != nil {
		t.Fatal(err)
	}
	// FlushDB 后消费应返回 miss
	if _, err := gen.ConsumeChallenge(ctx, ch.ChallengeID); err == nil {
		t.Error("after flush, consume should fail")
	}
}

// =============================================================================
// 工具：hex uuid 生成
// =============================================================================

func mustUUIDBytes(t *testing.T) []byte {
	t.Helper()
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		t.Fatal(err)
	}
	hex := hex.EncodeToString(raw)
	if len(hex) != 32 {
		t.Fatalf("uuid hex len = %d", len(hex))
	}
	return []byte(hex)
}
