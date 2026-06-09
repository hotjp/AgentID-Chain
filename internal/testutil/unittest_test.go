// Package testutil — 单元测试模板与最佳实践。
//
// 单元测试（Unit Test）规范：
//   - 位置：与被测代码同包（package xxx_test 或 package xxx）
//   - 不依赖外部服务（PG / Redis / HTTP）
//   - 用 gomock / miniredis / fixtures 模拟依赖
//   - 每个 t.Run("case", ...) 一个子测试
//   - t.Helper() 在 helper 函数开头
//   - t.Cleanup() 在 setup 末尾（替代 defer）
//   - t.Parallel() 标记可并行的子测试
//
// 运行：
//   go test -short -race ./...
//
// 模板（复制后修改）：

package testutil

import (
	"errors"
	"testing"
)

// 示例 1：基础 table-driven 测试
func TestExample_TableDriven(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"empty", "", "", false},
		{"valid", "hello", "HELLO", false},
		{"invalid", "!!!", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := exampleFunc(c.input)
			if c.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Errorf("got = %q, want %q", got, c.want)
			}
		})
	}
}

// 示例 2：subtest + t.Parallel
func TestExample_Parallel(t *testing.T) {
	cases := []string{"a", "b", "c"}
	for _, c := range cases {
		c := c
		t.Run(c, func(t *testing.T) {
			t.Parallel()
			// ... 测试
		})
	}
}

// 示例 3：mock 依赖（用 gomock）
// func TestExample_WithMock(t *testing.T) {
//     ctrl := gomock.NewController(t)
//     defer ctrl.Finish()
//
//     mockRepo := mockstorage.NewMockUserRepo(ctrl)
//     mockRepo.EXPECT().Get(gomock.Any(), "alice").Return(&User{ID: "u1"}, nil)
//
//     // ... 用 mockRepo 注入到被测对象
// }

// 示例 4：t.Cleanup
func TestExample_Cleanup(t *testing.T) {
	resource := setupExample(t)
	t.Cleanup(func() { teardownExample(resource) })

	// 测试
}

// 示例 5：errors.Is / errors.As
func TestExample_ErrorCheck(t *testing.T) {
	_, err := exampleFunc("!!!")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrExampleNotFound) {
		t.Errorf("err = %v, want wraps ErrExampleNotFound", err)
	}
}

// ---- 模板辅助代码（实际项目中删除） ----

// exampleFunc 示例函数。
func exampleFunc(s string) (string, error) {
	if s == "!!!" {
		return "", ErrExampleNotFound
	}
	if s == "" {
		return "", nil
	}
	return "HELLO", nil
}

// ErrExampleNotFound 示例错误。
var ErrExampleNotFound = errors.New("example: not found")

// setupExample 示例 setup。
func setupExample(t *testing.T) string {
	t.Helper()
	return "resource"
}

// teardownExample 示例 teardown。
func teardownExample(r string) {
	_ = r
}
