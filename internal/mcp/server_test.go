package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// fakeIO 提供一对 (in, out) 用于测试。
type fakeIO struct {
	in  *strings.Reader
	out *strings.Builder
}

func newFakeIO(input string) *fakeIO {
	return &fakeIO{
		in:  strings.NewReader(input),
		out: &strings.Builder{},
	}
}

func (f *fakeIO) Read(p []byte) (int, error) { return f.in.Read(p) }
func (f *fakeIO) Write(p []byte) (int, error) { return f.out.Write(p) }

// parseResponses 解析 Serve 写出的多行 JSON 响应。
func parseResponses(t *testing.T, raw string) []Response {
	t.Helper()
	var out []Response
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var r Response
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("invalid response: %v\n%s", err, line)
		}
		out = append(out, r)
	}
	return out
}

func TestServe_InitializeAndPing(t *testing.T) {
	io := newFakeIO(strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"ping"}`,
		"",
	}, "\n"))
	s := NewServer(ServerInfo{Name: "test", Version: "0.0.1"}, "")
	s.SetIO(io, io)

	if err := s.Serve(context.Background()); err != nil {
		t.Fatalf("Serve err = %v", err)
	}
	resps := parseResponses(t, io.out.String())
	if len(resps) != 2 {
		t.Fatalf("got %d responses, want 2", len(resps))
	}
	var init Result
	if err := unmarshalResult(resps[0].Result, &init); err != nil {
		t.Fatalf("init result unmarshal: %v", err)
	}
}

type Result = map[string]any

func TestServe_Unauthorized(t *testing.T) {
	io := newFakeIO(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n")
	s := NewServer(ServerInfo{Name: "t"}, "secret")
	s.SetIO(io, io)
	if err := s.Serve(context.Background()); err != nil {
		t.Fatalf("Serve err = %v", err)
	}
	resps := parseResponses(t, io.out.String())
	if len(resps) != 1 {
		t.Fatalf("got %d responses", len(resps))
	}
	if resps[0].Error == nil || resps[0].Error.Code != CodeUnauthorized {
		t.Errorf("expected unauthorized error, got %+v", resps[0].Error)
	}
}

func TestServe_AuthorizedList(t *testing.T) {
	io := newFakeIO(strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"apiKey":"secret"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		"",
	}, "\n"))
	s := NewServer(ServerInfo{Name: "t"}, "secret")
	s.RegisterTool("noop", "no-op tool", map[string]any{"type": "object"}, func(_ context.Context, _ json.RawMessage) (any, error) {
		return nil, nil
	})
	s.SetIO(io, io)
	if err := s.Serve(context.Background()); err != nil {
		t.Fatalf("Serve err = %v", err)
	}
	resps := parseResponses(t, io.out.String())
	// initialize + tools/list (no notification response)
	if len(resps) != 2 {
		t.Fatalf("got %d responses: %s", len(resps), io.out.String())
	}
	var toolsResult struct {
		Tools []Tool `json:"tools"`
	}
	if err := unmarshalResult(resps[1].Result, &toolsResult); err != nil {
		t.Fatalf("tools/list unmarshal: %v", err)
	}
	if len(toolsResult.Tools) == 0 {
		t.Error("expected non-empty tools list")
	}
}

func TestServe_ToolNotFound(t *testing.T) {
	io := newFakeIO(strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"apiKey":"k"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"nope","arguments":{}}}`,
		"",
	}, "\n"))
	s := NewServer(ServerInfo{Name: "t"}, "k")
	s.SetIO(io, io)
	if err := s.Serve(context.Background()); err != nil {
		t.Fatalf("Serve err = %v", err)
	}
	resps := parseResponses(t, io.out.String())
	// 最后一条应该是 tool not found
	last := resps[len(resps)-1]
	if last.Error == nil || last.Error.Code != CodeToolNotFound {
		t.Errorf("expected tool not found, got %+v", last)
	}
}

func TestServe_MethodNotFound(t *testing.T) {
	io := newFakeIO(`{"jsonrpc":"2.0","id":1,"method":"unknown/method"}` + "\n")
	s := NewServer(ServerInfo{Name: "t"}, "")
	s.SetIO(io, io)
	if err := s.Serve(context.Background()); err != nil {
		t.Fatalf("Serve err = %v", err)
	}
	resps := parseResponses(t, io.out.String())
	if len(resps) != 1 || resps[0].Error == nil {
		t.Fatalf("expected error response, got %+v", resps)
	}
	if resps[0].Error.Code != CodeMethodNotFound {
		t.Errorf("expected method not found, got code %d", resps[0].Error.Code)
	}
}

func TestServe_CallRegisteredTool(t *testing.T) {
	io := newFakeIO(strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"echo","arguments":{"x":42}}}`,
		"",
	}, "\n"))
	s := NewServer(ServerInfo{Name: "t"}, "")
	s.RegisterTool("echo", "echo back", map[string]any{"type": "object"}, func(_ context.Context, args json.RawMessage) (any, error) {
		var m map[string]any
		_ = json.Unmarshal(args, &m)
		return map[string]any{"got": m["x"]}, nil
	})
	s.SetIO(io, io)
	if err := s.Serve(context.Background()); err != nil {
		t.Fatalf("Serve err = %v", err)
	}
	resps := parseResponses(t, io.out.String())
	last := resps[len(resps)-1]
	if last.Error != nil {
		t.Fatalf("expected success, got error %+v", last.Error)
	}
	var cr CallResult
	if err := unmarshalResult(last.Result, &cr); err != nil {
		t.Fatalf("unmarshal CallResult: %v", err)
	}
	if cr.IsError {
		t.Error("CallResult.IsError = true")
	}
	if len(cr.Content) == 0 || !strings.Contains(cr.Content[0].Text, "42") {
		t.Errorf("content = %+v", cr.Content)
	}
}

func TestServe_ToolError(t *testing.T) {
	io := newFakeIO(strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"bad","arguments":{}}}`,
		"",
	}, "\n"))
	s := NewServer(ServerInfo{Name: "t"}, "")
	s.RegisterTool("bad", "always errors", nil, func(_ context.Context, _ json.RawMessage) (any, error) {
		return nil, errors.New("bad thing")
	})
	s.SetIO(io, io)
	if err := s.Serve(context.Background()); err != nil {
		t.Fatalf("Serve err = %v", err)
	}
	resps := parseResponses(t, io.out.String())
	last := resps[len(resps)-1]
	if last.Error != nil {
		t.Fatalf("tool errors should be 200 + isError=true, got protocol error %+v", last.Error)
	}
	var cr CallResult
	_ = unmarshalResult(last.Result, &cr)
	if !cr.IsError {
		t.Error("expected IsError=true on tool error")
	}
}

func TestServe_InvalidJSON(t *testing.T) {
	io := newFakeIO("not-json\n")
	s := NewServer(ServerInfo{Name: "t"}, "")
	s.SetIO(io, io)
	err := s.Serve(context.Background())
	if err != nil {
		t.Fatalf("Serve err = %v", err)
	}
	resps := parseResponses(t, io.out.String())
	if len(resps) == 0 {
		t.Fatal("expected parse error response")
	}
	if resps[0].Error == nil || resps[0].Error.Code != CodeParse {
		t.Errorf("expected parse error, got %+v", resps[0].Error)
	}
}

// unmarshalResult 辅助函数：把 any（已 unmarshal 过的）重新 marshal → unmarshal 到 dst。
func unmarshalResult(src any, dst any) error {
	b, err := json.Marshal(src)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dst)
}

func TestServe_NotificationNoResponse(t *testing.T) {
	io := newFakeIO(strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"id":99}}`,
		`{"jsonrpc":"2.0","id":2,"method":"ping"}`,
		"",
	}, "\n"))
	s := NewServer(ServerInfo{Name: "t"}, "")
	s.SetIO(io, io)
	if err := s.Serve(context.Background()); err != nil {
		t.Fatalf("Serve err = %v", err)
	}
	resps := parseResponses(t, io.out.String())
	// init + ping = 2 responses (notifications skipped)
	if len(resps) != 2 {
		t.Errorf("got %d responses, want 2 (notifications skipped)", len(resps))
	}
}
