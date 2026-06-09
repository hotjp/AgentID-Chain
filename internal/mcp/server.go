// Package mcp 提供 AgentID-Chain 的 MCP（Model Context Protocol）服务器实现。
//
// 协议：MCP over JSON-RPC 2.0，传输层 stdio。
//
// 支持的方法（v1.0）：
//   - initialize          客户端启动握手（返回 serverInfo + capabilities）
//   - tools/list          列出所有已注册 tool
//   - tools/call          调用具体 tool
//   - ping                心跳
//   - notifications/initialized  客户端确认初始化完成（无响应）
//
// 数据流（与 docs/AgentID-Chain-技术文档-v2.0.1.md §4.2 MCP 接入对齐）：
//
//	Client (Claude / GPT / Cursor ...)
//	   │   {"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"agentid_register",...}}
//	   ▼
//	stdio ───► JSON-RPC parser ──► Tool dispatcher ──► core/backend
//	   │                                                              │
//	   └──── ◄── JSON-RPC response {"jsonrpc":"2.0","id":1,"result":{...}}
//
// 设计约束：
//   - 不引入外部 MCP SDK（保持依赖最小；本文件手写 JSON-RPC 2.0）
//   - 工具是 typed Handler（func(ctx, args) (any, error)），不强求参数 schema
//   - 鉴权通过 authKey 在 initialize 时一次性验证
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
)

// =============================================================================
// 错误
// =============================================================================

// ErrUnauthorized 未授权（authKey 校验失败）。
var ErrUnauthorized = errors.New("mcp: unauthorized")

// ErrToolNotFound 工具未注册。
var ErrToolNotFound = errors.New("mcp: tool not found")

// ErrInvalidRequest JSON-RPC 请求格式非法。
var ErrInvalidRequest = errors.New("mcp: invalid request")

// =============================================================================
// JSON-RPC 2.0 数据结构
// =============================================================================

// JSONRPCVersion 固定为 "2.0"。
const JSONRPCVersion = "2.0"

// Request JSON-RPC 2.0 请求。
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"` // string | number | null
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response JSON-RPC 2.0 响应。
type Response struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id,omitempty"`
	Result  any    `json:"result,omitempty"`
	Error   *Error `json:"error,omitempty"`
}

// Error JSON-RPC 2.0 错误。
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// 标准错误码（JSON-RPC 2.0 + MCP 扩展）。
const (
	CodeParse          = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternal       = -32603
	// MCP 扩展
	CodeUnauthorized = -32001
	CodeToolNotFound = -32002
)

// =============================================================================
// MCP 协议结构
// =============================================================================

// ServerInfo MCP server 元信息。
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ServerCapabilities 描述 server 支持的能力。
type ServerCapabilities struct {
	Tools *ToolsCapability `json:"tools,omitempty"`
}

// ToolsCapability tools 子能力。
type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// Tool 一个可被 LLM 调用的工具。
type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	// InputSchema JSON Schema 描述参数（可选；MCP 1.0 必填，给空对象）。
	InputSchema any `json:"inputSchema"`
}

// CallParams tools/call 参数。
type CallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// CallResult tools/call 结果。
type CallResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// ContentBlock 内容块（text / image / resource）。
type ContentBlock struct {
	Type string `json:"type"` // "text" | "image" | "resource"
	Text string `json:"text,omitempty"`
	Data any    `json:"data,omitempty"`
	Mime string `json:"mimeType,omitempty"`
}

// =============================================================================
// Handler & Server
// =============================================================================

// ToolHandler 工具处理函数。
//
// args 是 arguments 的 raw JSON（handler 自行 Unmarshal）。
// 返回 result 序列化为 JSON；error 转为 JSON-RPC 错误。
type ToolHandler func(ctx context.Context, args json.RawMessage) (any, error)

// ToolDescriptor 注册时的描述。
type ToolDescriptor struct {
	Tool
	Handler ToolHandler
}

// Server MCP 服务器。
type Server struct {
	info         ServerInfo
	capabilities ServerCapabilities

	mu    sync.RWMutex
	tools map[string]*ToolDescriptor

	// authKey 期望的 API Key（initialize 时校验；空 = 不校验）。
	authKey string
	// authed 已通过 initialize 鉴权。
	authed bool

	// 测试用：可注入 stdin/stdout
	in  io.Reader
	out io.Writer
}

// NewServer 构造服务器。
func NewServer(info ServerInfo, authKey string) *Server {
	return &Server{
		info: info,
		capabilities: ServerCapabilities{
			Tools: &ToolsCapability{ListChanged: false},
		},
		tools:   make(map[string]*ToolDescriptor),
		authKey: authKey,
		in:      os.Stdin,
		out:     os.Stdout,
	}
}

// SetIO 注入 IO（测试用）。
func (s *Server) SetIO(in io.Reader, out io.Writer) {
	s.in = in
	s.out = out
}

// RegisterTool 注册工具。
func (s *Server) RegisterTool(name, description string, schema any, h ToolHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tools[name] = &ToolDescriptor{
		Tool: Tool{
			Name:        name,
			Description: description,
			InputSchema: schema,
		},
		Handler: h,
	}
}

// Tools 返回已注册工具列表。
func (s *Server) Tools() []Tool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Tool, 0, len(s.tools))
	for _, t := range s.tools {
		out = append(out, t.Tool)
	}
	return out
}

// Serve 启动服务器（阻塞；从 in 读 JSON-RPC 请求 → out 写响应）。
//
// 行为：每行一条 JSON 消息（Content-Length 头可选；MCP stdio 标准是 header
// + 空行 + body；本实现兼容两种）。
func (s *Server) Serve(ctx context.Context) error {
	return s.serve(ctx)
}

// =============================================================================
// stdio 帧解析
// =============================================================================

// serve 主循环。
func (s *Server) serve(ctx context.Context) error {
	scanner := bufio.NewScanner(s.in)
	// MCP 消息可能较长；放宽 buffer。
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		req, err := readRequest(scanner)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			// 协议错误 → 写一个错误响应（id 未知）。
			s.writeError(nil, &Error{Code: CodeParse, Message: err.Error()})
			continue
		}
		resp := s.handle(ctx, req)
		if resp != nil {
			s.write(resp)
		}
	}
}

// readRequest 读一条 JSON-RPC 请求。
//
// 兼容两种 framing：
//   1) LSP 风格：Content-Length: N\r\n\r\n<body>
//   2) 行式：<json>\n
func readRequest(scanner *bufio.Scanner) (*Request, error) {
	// 跳过空行
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			// 可能是 Content-Length 头
			if isContentLengthHeader(line) {
				return readFramed(scanner, parseContentLength(line))
			}
			// 整行就是 JSON
			var r Request
			if err := json.Unmarshal([]byte(line), &r); err != nil {
				return nil, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
			}
			return &r, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return nil, io.EOF
}

func isContentLengthHeader(line string) bool {
	return len(line) > 16 && (line[:15] == "Content-Length:" || line[:16] == "content-length:")
}

func parseContentLength(line string) int {
	var n int
	fmt.Sscanf(line, "Content-Length: %d", &n)
	return n
}

func readFramed(scanner *bufio.Scanner, n int) (*Request, error) {
	// 已读 \r\n 后空行；现在读 body
	if !scanner.Scan() {
		return nil, io.ErrUnexpectedEOF
	}
	body := make([]byte, 0, n)
	body = append(body, scanner.Bytes()...)
	// 补读到 n
	for len(body) < n && scanner.Scan() {
		body = append(body, '\n')
		body = append(body, scanner.Bytes()...)
	}
	if len(body) < n {
		return nil, io.ErrUnexpectedEOF
	}
	var r Request
	if err := json.Unmarshal(body[:n], &r); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}
	return &r, nil
}

// =============================================================================
// 请求分派
// =============================================================================

// handle 处理一条请求，返回响应（nil 表示是 notification）。
func (s *Server) handle(ctx context.Context, req *Request) *Response {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(ctx, req)
	case "ping":
		return okResp(req, map[string]any{"ok": true})
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(ctx, req)
	case "notifications/initialized":
		// 客户端通知，无需响应
		return nil
	case "notifications/cancelled":
		return nil
	default:
		return errResp(req, &Error{Code: CodeMethodNotFound, Message: "method not found: " + req.Method})
	}
}

// handleInitialize 处理 initialize 请求（鉴权 + 返回 serverInfo）。
func (s *Server) handleInitialize(_ context.Context, req *Request) *Response {
	// 鉴权（若配置了 authKey）：从 params.apiKey / params.authKey 读取
	if s.authKey != "" {
		var p struct {
			APIKey  string `json:"apiKey"`
			AuthKey string `json:"authKey"`
		}
		_ = json.Unmarshal(req.Params, &p)
		key := p.APIKey
		if key == "" {
			key = p.AuthKey
		}
		if key != s.authKey {
			return errResp(req, &Error{Code: CodeUnauthorized, Message: ErrUnauthorized.Error()})
		}
		s.authed = true
	}
	result := map[string]any{
		"protocolVersion": "2024-11-05",
		"serverInfo":      s.info,
		"capabilities":    s.capabilities,
	}
	return okResp(req, result)
}

// handleToolsList 列出所有工具。
func (s *Server) handleToolsList(req *Request) *Response {
	if err := s.requireAuth(req); err != nil {
		return errResp(req, err)
	}
	tools := s.Tools()
	return okResp(req, map[string]any{"tools": tools})
}

// handleToolsCall 调用具体工具。
func (s *Server) handleToolsCall(ctx context.Context, req *Request) *Response {
	if err := s.requireAuth(req); err != nil {
		return errResp(req, err)
	}
	var p CallParams
	if err := json.Unmarshal(req.Params, &p); err != nil {
		return errResp(req, &Error{Code: CodeInvalidParams, Message: err.Error()})
	}
	s.mu.RLock()
	t, ok := s.tools[p.Name]
	s.mu.RUnlock()
	if !ok {
		return errResp(req, &Error{Code: CodeToolNotFound, Message: p.Name + ": " + ErrToolNotFound.Error()})
	}
	result, err := t.Handler(ctx, p.Arguments)
	if err != nil {
		// 业务错误 → 200 + isError=true（避免客户端误判协议层失败）
		return okResp(req, &CallResult{
			Content: []ContentBlock{{Type: "text", Text: err.Error()}},
			IsError: true,
		})
	}
	// 把 result 序列化为 text content
	text, _ := json.Marshal(result)
	return okResp(req, &CallResult{
		Content: []ContentBlock{{Type: "text", Text: string(text)}},
	})
}

func (s *Server) requireAuth(req *Request) *Error {
	if s.authKey == "" {
		return nil
	}
	if s.authed {
		return nil
	}
	return &Error{Code: CodeUnauthorized, Message: ErrUnauthorized.Error()}
}

// =============================================================================
// 响应构造 & 写
// =============================================================================

func okResp(req *Request, result any) *Response {
	return &Response{JSONRPC: JSONRPCVersion, ID: req.ID, Result: result}
}

func errResp(req *Request, e *Error) *Response {
	return &Response{JSONRPC: JSONRPCVersion, ID: req.ID, Error: e}
}

// write 写一条响应（行式 JSON + \n）。
func (s *Server) write(r *Response) {
	data, _ := json.Marshal(r)
	data = append(data, '\n')
	_, _ = s.out.Write(data)
}

func (s *Server) writeError(id any, e *Error) {
	s.write(&Response{JSONRPC: JSONRPCVersion, ID: id, Error: e})
}
