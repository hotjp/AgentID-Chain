// Package rbac: 基准测试 (P18.3)。
//
// 目标：单次权限校验 < 1μs（核心热路径）。
package rbac

import (
	"testing"

	"github.com/agentid-chain/agentid-chain/internal/domain"
)

func setupBenchEngine(b *testing.B) *Engine {
	b.Helper()
	return NewEngine(NewDefaultLevelTemplate())
}

// BenchmarkCheck 单权限位校验。
func BenchmarkCheck(b *testing.B) {
	engine := setupBenchEngine(b)
	const testPerm uint = 1 << 5 // bit 5
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := engine.Check(testPerm, domain.LevelBasic)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCheckMask 多权限位掩码校验（AND）。
func BenchmarkCheckMask(b *testing.B) {
	engine := setupBenchEngine(b)
	const mask uint64 = 1<<5 | 1<<10 | 1<<20
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := engine.CheckMask(mask, domain.LevelAdvanced)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkMustCheck 单权限位 MustCheck（无 error 路径）。
func BenchmarkMustCheck(b *testing.B) {
	engine := setupBenchEngine(b)
	const testPerm uint = 1 << 5
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = engine.MustCheck(testPerm, domain.LevelBasic)
	}
}

// BenchmarkHasAny 任一权限位匹配。
func BenchmarkHasAny(b *testing.B) {
	engine := setupBenchEngine(b)
	const perms uint64 = 1<<3 | 1<<7 | 1<<15
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = engine.HasAny(perms, domain.LevelAdvanced)
	}
}

// BenchmarkCheck_Parallel 并发权限校验。
func BenchmarkCheck_Parallel(b *testing.B) {
	engine := setupBenchEngine(b)
	const testPerm uint = 1 << 5
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = engine.Check(testPerm, domain.LevelBasic)
		}
	})
}

// BenchmarkAllowedBits 列出某个 level 的所有允许位。
func BenchmarkAllowedBits(b *testing.B) {
	engine := setupBenchEngine(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := engine.AllowedBits(domain.LevelAdvanced)
		if err != nil {
			b.Fatal(err)
		}
	}
}
