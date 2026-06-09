package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChain_Order(t *testing.T) {
	var order []string
	mw := func(name string) HandlerFunc {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, name)
				next.ServeHTTP(w, r)
			})
		}
	}
	chain := NewChain(mw("a"), mw("b"), mw("c"))
	chain.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "handler")
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))

	// Append 顺序：a 最外层，先入
	want := []string{"a", "b", "c", "handler"}
	if len(order) != len(want) {
		t.Fatalf("len = %d, want %d", len(order), len(want))
	}
	for i, w := range want {
		if order[i] != w {
			t.Errorf("order[%d] = %q, want %q", i, order[i], w)
		}
	}
}

func TestChain_Len(t *testing.T) {
	c := NewChain()
	if c.Len() != 0 {
		t.Errorf("empty len = %d", c.Len())
	}
	c.Append(func(h http.Handler) http.Handler { return h })
	if c.Len() != 1 {
		t.Errorf("len = %d", c.Len())
	}
}
