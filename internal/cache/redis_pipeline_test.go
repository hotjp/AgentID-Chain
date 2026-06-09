// Package cache: Pipeline 集成测试（使用 miniredis 模拟）。
package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// newTestPipeline 创建一个绑定到 miniredis 的 Pipeline。
func newTestPipeline(t testing.TB) (*Pipeline, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return &Pipeline{rdb: rdb}, mr
}

func TestPipeline_MGet_Empty(t *testing.T) {
	p, mr := newTestPipeline(t)
	defer mr.Close()
	out, err := p.MGet(context.Background(), nil)
	if err != nil || out != nil {
		t.Fatalf("expected (nil, nil), got (%v, %v)", out, err)
	}
}

func TestPipeline_MSet_MGet(t *testing.T) {
	p, mr := newTestPipeline(t)
	defer mr.Close()
	ctx := context.Background()
	kv := map[string][]byte{
		"a": []byte("1"),
		"b": []byte("2"),
		"c": []byte("3"),
	}
	if err := p.MSet(ctx, kv); err != nil {
		t.Fatal(err)
	}
	got, err := p.MGet(ctx, []string{"a", "b", "c", "d"})
	if err != nil {
		t.Fatal(err)
	}
	if string(got[0]) != "1" || string(got[1]) != "2" || string(got[2]) != "3" {
		t.Fatalf("unexpected values: %v", got)
	}
	if got[3] != nil {
		t.Fatalf("expected nil for missing key, got %v", got[3])
	}
}

func TestPipeline_MSetWithTTL(t *testing.T) {
	p, mr := newTestPipeline(t)
	defer mr.Close()
	ctx := context.Background()
	items := []SetItem{
		{Key: "x", Value: []byte("X"), TTL: 10 * time.Second},
		{Key: "y", Value: []byte("Y"), TTL: 20 * time.Second},
	}
	if err := p.MSetWithTTL(ctx, items); err != nil {
		t.Fatal(err)
	}
	got, _ := p.MGet(ctx, []string{"x", "y"})
	if string(got[0]) != "X" || string(got[1]) != "Y" {
		t.Fatalf("unexpected: %v", got)
	}
}

func TestPipeline_MDel(t *testing.T) {
	p, mr := newTestPipeline(t)
	defer mr.Close()
	ctx := context.Background()
	_ = p.MSet(ctx, map[string][]byte{"a": []byte("1"), "b": []byte("2")})
	n, err := p.MDel(ctx, []string{"a", "b", "c"})
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("expected 2 deleted, got %d", n)
	}
}

func TestPipeline_MExists(t *testing.T) {
	p, mr := newTestPipeline(t)
	defer mr.Close()
	ctx := context.Background()
	_ = p.MSet(ctx, map[string][]byte{"a": []byte("1")})
	got, err := p.MExists(ctx, []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	if !got[0] || got[1] {
		t.Fatalf("expected [true, false], got %v", got)
	}
}

func TestPipeline_MIncrBy(t *testing.T) {
	p, mr := newTestPipeline(t)
	defer mr.Close()
	ctx := context.Background()
	items := []IncrItem{
		{Key: "c1", Delta: 1, NewTTL: time.Minute},
		{Key: "c2", Delta: 5},
	}
	got, err := p.MIncrBy(ctx, items)
	if err != nil {
		t.Fatal(err)
	}
	if got[0] != 1 || got[1] != 5 {
		t.Fatalf("expected [1, 5], got %v", got)
	}
}

func TestPipeline_NilSafety(t *testing.T) {
	var p *Pipeline
	_, err := p.MGet(context.Background(), []string{"x"})
	if err == nil {
		t.Fatal("expected error from nil pipeline")
	}
}
