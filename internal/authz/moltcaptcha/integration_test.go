//go:build integration

// Package moltcaptcha 集成测试：完整 Challenge → Verify 链路（用 testcontainers Redis）。
//
// 运行方式：
//
//	go test -tags=integration -timeout=120s ./internal/authz/moltcaptcha/...
//
// 环境要求：Docker daemon。无 Docker 时自动 skip。
package moltcaptcha

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/agentid-chain/agentid-chain/internal/testutil/testcontainers"
)

// =============================================================================
// 工具
// =============================================================================

// defaultTestTopicPool 测试用主题池（保证 Generate 一定能 pick 到 topic）。
var defaultTestTopicPool = []string{
	"machine-learning", "distributed-systems", "cryptography",
	"quantum-computing", "blockchain", "operating-systems",
	"computer-networks", "databases", "compilers", "robotics",
	"natural-language-processing", "computer-vision", "cybersecurity",
	"cloud-computing", "edge-computing", "iot", "embedded-systems",
	"signal-processing", "control-systems", "formal-verification",
}

func newIntegrationSetup(t *testing.T) (*Generator, *Verifier) {
	t.Helper()
	rdb, err := testcontainers.NewRedisContainer(t, testcontainers.RedisOpts{
		StartupTimeout: 60 * time.Second,
	})
	if err != nil {
		t.Skipf("redis container unavailable: %v", err)
	}
	ctx := context.Background()
	cache := rdb.Cache(ctx)

	gen, err := NewGenerator(GeneratorConfig{
		Cache:             cache,
		DefaultDifficulty: DifficultyMedium,
		TopicPool:         defaultTestTopicPool,
	})
	if err != nil {
		t.Fatal(err)
	}
	ver, err := NewVerifier(gen, VerifierConfig{})
	if err != nil {
		t.Fatal(err)
	}
	return gen, ver
}

// =============================================================================
// 真实链路：Generate → Verify（用 LLM 难以一次性生成 perfect chain）
// =============================================================================

// TestIntegration_GenerateAndReconstruct 模拟"agent 用 LLM 找到语义链"。
//
// 真实场景下，agent 拿到 challenge 后通过外部知识库/LLM 生成答案。
// 本测试使用 VerifyInput 直接传入（不调用 LLM），但确保 challenge→verify
// 链路在真实 Redis 上端到端工作。
func TestIntegration_GenerateAndReconstruct(t *testing.T) {
	gen, ver := newIntegrationSetup(t)
	ctx := context.Background()

	ch, err := gen.Generate(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if ch.ChallengeID == "" {
		t.Fatal("empty challenge id")
	}
	if ch.Topic == "" {
		t.Fatal("empty topic")
	}
	if ch.Hops <= 0 {
		t.Fatal("hops should be > 0")
	}

	// 验证 challenge 已被存储
	// 用 LoadChallenge 拿回，确认 cache 写入
	loaded, err := gen.LoadChallenge(ctx, ch.ChallengeID)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Topic != ch.Topic {
		t.Errorf("topic mismatch: %s vs %s", loaded.Topic, ch.Topic)
	}

	// 提交语义链：使用 ChallengeID + 空 words（无法 verify 通过）
	// 这只是验证接口工作，不强求通过
	_, err = ver.Verify(ctx, VerifyInput{
		ChallengeID: ch.ChallengeID,
		Words:       []string{},
		SubmittedAt: time.Now(),
	})
	// 空 words 必失败（业务规则）
	if err == nil {
		t.Error("empty words should fail verify")
	}
}

// =============================================================================
// 难度参数对链路的影响
// =============================================================================

func TestIntegration_AllDifficultiesGenerate(t *testing.T) {
	gen, _ := newIntegrationSetup(t)
	ctx := context.Background()

	for _, d := range AllDifficulties() {
		if !d.IsValid() {
			continue
		}
		ch, err := gen.GenerateWithDifficulty(ctx, d)
		if err != nil {
			t.Errorf("diff=%s: %v", d, err)
			continue
		}
		if ch.Hops != d.Hops() {
			t.Errorf("diff=%s hops=%d want %d", d, ch.Hops, d.Hops())
		}
	}
}

// =============================================================================
// Challenge 过期
// =============================================================================

func TestIntegration_ChallengeTTL(t *testing.T) {
	rdb, err := testcontainers.NewRedisContainer(t, testcontainers.RedisOpts{})
	if err != nil {
		t.Skipf("redis unavailable: %v", err)
	}
	ctx := context.Background()
	cache := rdb.Cache(ctx)

	gen, err := NewGenerator(GeneratorConfig{
		Cache:             cache,
		DefaultDifficulty: DifficultyMedium,
		TopicPool:         defaultTestTopicPool,
		DefaultTTL:        1 * time.Second, // 短 TTL
	})
	if err != nil {
		t.Fatal(err)
	}
	ch, err := gen.Generate(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// 等 1.2s 后再 load
	time.Sleep(1200 * time.Millisecond)
	_, err = gen.LoadChallenge(ctx, ch.ChallengeID)
	if err == nil {
		t.Error("expired challenge should not load")
	}
}

// =============================================================================
// 并发 Generate
// =============================================================================

func TestIntegration_ConcurrentGenerate(t *testing.T) {
	gen, _ := newIntegrationSetup(t)
	ctx := context.Background()

	const N = 20
	ids := make([]string, N)
	errs := make([]error, N)
	var wg sync.WaitGroup
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ch, err := gen.Generate(ctx)
			if err != nil {
				errs[idx] = err
				return
			}
			ids[idx] = ch.ChallengeID
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("client %d: %v", i, err)
		}
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if id == "" {
			continue
		}
		if seen[id] {
			t.Errorf("duplicate id: %s", id)
		}
		seen[id] = true
	}
	if len(seen) < N {
		t.Errorf("got %d unique, want %d", len(seen), N)
	}
}

// =============================================================================
// 真实 TopicPool 注入
// =============================================================================

func TestIntegration_CustomTopicPool(t *testing.T) {
	rdb, err := testcontainers.NewRedisContainer(t, testcontainers.RedisOpts{})
	if err != nil {
		t.Skipf("redis unavailable: %v", err)
	}
	ctx := context.Background()
	cache := rdb.Cache(ctx)

	pool := []string{"quantum-computing", "machine-learning", "kubernetes"}
	gen, err := NewGenerator(GeneratorConfig{
		Cache:             cache,
		DefaultDifficulty: DifficultyEasy,
		TopicPool:         pool,
	})
	if err != nil {
		t.Fatal(err)
	}

	// 多次 generate，topic 必须在 pool 内
	for i := 0; i < 10; i++ {
		ch, err := gen.Generate(ctx)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, p := range pool {
			if ch.Topic == p {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("topic %q not in pool", ch.Topic)
		}
	}
}

// =============================================================================
// 错误路径：非法 UUID
// =============================================================================

func TestIntegration_InvalidChallengeID(t *testing.T) {
	_, ver := newIntegrationSetup(t)
	ctx := context.Background()

	_, err := ver.Verify(ctx, VerifyInput{
		ChallengeID: "non-existent-id",
		Words:       []string{"x"},
		SubmittedAt: time.Now(),
	})
	if err == nil {
		t.Error("invalid id should fail")
	}
}

// =============================================================================
// FlushDB 模拟重启
// =============================================================================

func TestIntegration_FlushDBClearsState(t *testing.T) {
	rdb, err := testcontainers.NewRedisContainer(t, testcontainers.RedisOpts{})
	if err != nil {
		t.Skipf("redis unavailable: %v", err)
	}
	ctx := context.Background()
	cache := rdb.Cache(ctx)

	gen, err := NewGenerator(GeneratorConfig{
		Cache:             cache,
		DefaultDifficulty: DifficultyMedium,
		TopicPool:         defaultTestTopicPool,
	})
	if err != nil {
		t.Fatal(err)
	}
	ch, err := gen.Generate(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := rdb.FlushDB(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := gen.LoadChallenge(ctx, ch.ChallengeID); err == nil {
		t.Error("after flush, load should fail")
	}
}

// =============================================================================
// benchmark-style：连续 N 次 generate 的稳定性
// =============================================================================

func TestIntegration_Stability_Generate100(t *testing.T) {
	gen, _ := newIntegrationSetup(t)
	ctx := context.Background()

	for i := 0; i < 100; i++ {
		if _, err := gen.Generate(ctx); err != nil {
			t.Fatalf("iter %d: %v", i, err)
		}
	}
}

// 隐藏 fmt 使用避免未用警告
var _ = fmt.Sprintf
