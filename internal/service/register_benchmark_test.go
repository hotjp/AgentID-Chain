// Package service: Register 流程基准测试 (P18.4)。
//
// 目标：单次注册 < 50ms（含 mock DB；真实 PG 略高）。
package service

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"github.com/agentid-chain/agentid-chain/internal/domain"
)

// setupBenchRegisterService 构造一个完整的 RegisterService。
// 使用进程内 mock（newMockStore / mockChain / mockAudit / mockProvider）。
func setupBenchRegisterService(b *testing.B) *RegisterService {
	b.Helper()
	store := newMockStore()
	chain := &mockChain{
		typ: ChainMock,
		receipt: &RegisterReceipt{
			TxHash:      "0xbench",
			BlockNumber: 1,
		},
	}
	audit := &mockAudit{}
	provider := &mockProvider{agents: map[string]*domain.Agent{}}
	svc, err := NewRegisterService(store, chain, audit, provider)
	if err != nil {
		b.Fatal(err)
	}
	return svc
}

func newBenchRegisterRequest(tb testing.TB, i int) *RegisterAgentRequest {
	tb.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		tb.Fatal(err)
	}
	uuid := make([]byte, 16)
	if _, err := rand.Read(uuid); err != nil {
		tb.Fatal(err)
	}
	return &RegisterAgentRequest{
		UUID:      domain.UUID(encodeHex(uuid)),
		Owner:     "bench-owner",
		Level:     domain.LevelBasic,
		PublicKey: pub,
		// 用时间戳 + i 保证不重复
		Now: time.Now().Add(time.Duration(i) * time.Nanosecond),
	}
}

func encodeHex(b []byte) string {
	const hexchars = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, c := range b {
		out[i*2] = hexchars[c>>4]
		out[i*2+1] = hexchars[c&0x0f]
	}
	return string(out)
}

// BenchmarkRegister 单次注册（无 DB 真实 IO；mock store）。
func BenchmarkRegister(b *testing.B) {
	svc := setupBenchRegisterService(b)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := newBenchRegisterRequest(b, i)
		_, err := svc.HandleRegister(ctx, req)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRegister_Parallel 并发注册。
func BenchmarkRegister_Parallel(b *testing.B) {
	svc := setupBenchRegisterService(b)
	ctx := context.Background()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			req := newBenchRegisterRequest(b, i)
			_, err := svc.HandleRegister(ctx, req)
			if err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
}

// BenchmarkValidateRequest 仅校验入参（零 IO）。
func BenchmarkValidateRequest(b *testing.B) {
	svc := setupBenchRegisterService(b)
	req := newBenchRegisterRequest(b, 0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := svc.validateRequest(req); err != nil {
			b.Fatal(err)
		}
	}
}
