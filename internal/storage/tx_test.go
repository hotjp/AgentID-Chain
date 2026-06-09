package storage

import (
	"context"
	"errors"
	"testing"
)

// fakeRunner 测试用 TxRunner。
type fakeRunner struct {
	started int
	fn      func(ctx context.Context, fn func(ctx context.Context) error) error
}

func (f *fakeRunner) RunInTx(ctx context.Context, fn func(ctx context.Context) error) error {
	f.started++
	return f.fn(ctx, fn)
}

func TestInTransaction_Commit(t *testing.T) {
	r := &fakeRunner{
		fn: func(ctx context.Context, fn func(ctx context.Context) error) error {
			return fn(ctx)
		},
	}
	called := false
	err := InTransaction(context.Background(), r, func(_ context.Context, _ any) error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !called {
		t.Fatal("fn not called")
	}
	if r.started != 1 {
		t.Errorf("started=%d want 1", r.started)
	}
}

func TestInTransaction_Rollback(t *testing.T) {
	r := &fakeRunner{
		fn: func(ctx context.Context, fn func(ctx context.Context) error) error {
			return fn(ctx)
		},
	}
	wantErr := errors.New("biz fail")
	err := InTransaction(context.Background(), r, func(_ context.Context, _ any) error {
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Errorf("got %v want %v", err, wantErr)
	}
}

func TestInTransaction_Nested(t *testing.T) {
	r := &fakeRunner{
		fn: func(ctx context.Context, fn func(ctx context.Context) error) error {
			return fn(ctx)
		},
	}
	// 模拟外层已经在事务中
	ctx := WithTx(context.Background())
	calls := 0
	err := InTransaction(ctx, r, func(_ context.Context, _ any) error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if calls != 1 {
		t.Errorf("calls=%d want 1", calls)
	}
	if r.started != 0 {
		t.Errorf("nested should not start new tx; started=%d", r.started)
	}
}

func TestInTransaction_Panic(t *testing.T) {
	r := &fakeRunner{
		fn: func(ctx context.Context, fn func(ctx context.Context) error) error {
			return fn(ctx)
		},
	}
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic")
		}
		err, ok := r.(error)
		if !ok {
			t.Fatalf("panic value is not error: %T", r)
		}
		if !errors.Is(err, ErrTxFailed) {
			t.Errorf("panic err = %v, want wraps ErrTxFailed", err)
		}
	}()
	_ = InTransaction(context.Background(), r, func(_ context.Context, _ any) error {
		panic("boom")
	})
}

func TestIsInTx(t *testing.T) {
	if IsInTx(context.Background()) {
		t.Error("expected false for empty ctx")
	}
	if !IsInTx(WithTx(context.Background())) {
		t.Error("expected true after WithTx")
	}
}
