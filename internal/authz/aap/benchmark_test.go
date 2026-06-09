// Package aap: 基准测试 (P18.2)。
//
// 目标：单次 Challenge 生成 + 验签 < 5ms。
// 不依赖真实 Redis（使用 miniredis 进程内 Redis）。
package aap

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"

	"github.com/agentid-chain/agentid-chain/internal/cache"
)

type benchEnv struct {
	gen       *Generator
	verifier  *Verifier
	agentPub  ed25519.PublicKey
	agentPriv ed25519.PrivateKey
	agentUUID string
	mr        *miniredis.Miniredis
	store     cache.Cache
}

func setupBenchEnv(b *testing.B) *benchEnv {
	b.Helper()
	mr := miniredis.RunT(b)
	store := cache.NewMiniredis(mr)

	domainPub, domainPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		b.Fatal(err)
	}

	agentPub, agentPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		b.Fatal(err)
	}

	cfg := Config{
		ChallengeTTL: 30 * time.Second,
		DomainKey:    domainPriv,
		Clock:        time.Now,
	}
	gen, err := NewGenerator(store, cfg)
	if err != nil {
		b.Fatal(err)
	}

	verifier, err := NewVerifier(gen, VerifierConfig{
		Generator:      gen,
		ResponseMaxTTL: 10 * time.Minute,
		Clock:          time.Now,
	})
	if err != nil {
		b.Fatal(err)
	}
	_ = domainPub // 用于域验签

	uuidBytes := make([]byte, 16)
	if _, err := rand.Read(uuidBytes); err != nil {
		b.Fatal(err)
	}

	return &benchEnv{
		gen:       gen,
		verifier:  verifier,
		agentPub:  agentPub,
		agentPriv: agentPriv,
		agentUUID: hex.EncodeToString(uuidBytes),
		mr:        mr,
		store:     store,
	}
}

// 构造 response 签名（与 verify.go 中的 expectedPayload 保持一致）
func (e *benchEnv) signResponse(c *Challenge) string {
	payload := []byte(c.ChallengeID + ":" + c.Nonce + ":" + c.IssuedAt.Format(time.RFC3339Nano) + ":" + e.agentUUID)
	sig := ed25519.Sign(e.agentPriv, payload)
	return base64.RawURLEncoding.EncodeToString(sig)
}

// BenchmarkChallenge 单次 challenge 生成。
func BenchmarkChallenge(b *testing.B) {
	env := setupBenchEnv(b)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := env.gen.Generate(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkVerify 单次 verify（含 challenge 生成、签名、验签）。
func BenchmarkVerify(b *testing.B) {
	env := setupBenchEnv(b)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c, err := env.gen.Generate(ctx)
		if err != nil {
			b.Fatal(err)
		}
		response := env.signResponse(c)
		_, err = env.verifier.Verify(ctx, VerifyInput{
			ChallengeID: c.ChallengeID,
			Response:    response,
			AgentPubKey: hex.EncodeToString(env.agentPub),
			AgentUUID:   env.agentUUID,
			Now:         time.Now(),
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkChallengeVerify 完整 handshake 流程（Generate + Verify）。
func BenchmarkChallengeVerify(b *testing.B) {
	env := setupBenchEnv(b)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c, err := env.gen.Generate(ctx)
		if err != nil {
			b.Fatal(err)
		}
		response := env.signResponse(c)
		_, err = env.verifier.Verify(ctx, VerifyInput{
			ChallengeID: c.ChallengeID,
			Response:    response,
			AgentPubKey: hex.EncodeToString(env.agentPub),
			AgentUUID:   env.agentUUID,
			Now:         time.Now(),
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkParallelVerify 并发 verify（验证横向扩展能力）。
func BenchmarkParallelVerify(b *testing.B) {
	env := setupBenchEnv(b)
	ctx := context.Background()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			c, err := env.gen.Generate(ctx)
			if err != nil {
				b.Fatal(err)
			}
			response := env.signResponse(c)
			_, err = env.verifier.Verify(ctx, VerifyInput{
				ChallengeID: c.ChallengeID,
				Response:    response,
				AgentPubKey: hex.EncodeToString(env.agentPub),
				AgentUUID:   env.agentUUID,
				Now:         time.Now(),
			})
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
