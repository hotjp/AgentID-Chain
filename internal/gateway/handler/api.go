// Package handler: HTTP REST API 处理器（P7.14）。
//
// 端点：
//   POST   /api/v2/agents/register        — 注册
//   GET    /api/v2/agents/{uuid}          — 查询
//   POST   /api/v2/agents/{uuid}/upgrade  — 升级
//   POST   /api/v2/agents/{uuid}/check    — 权限校验
//   POST   /api/v2/captcha/moltcaptcha/challenge — 申请 challenge
//   POST   /api/v2/captcha/moltcaptcha/verify    — 提交验证
//
// 所有端点都委托 L4 service（*RegisterService / *GetAgentInfoService / ...）。
// 任何业务逻辑 0 行；只做 JSON 解析、参数校验、调用 L4、写响应。
package handler

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/agentid-chain/agentid-chain/internal/domain"
	"github.com/agentid-chain/agentid-chain/internal/service"
)

// APIHandler HTTP API 处理器聚合。
type APIHandler struct {
	// 注入 L4 services（nil-safe；nil = 端点返回 503）。
	RegisterSvc *service.RegisterService
	UpgradeSvc  *service.UpgradeService
	RevokeSvc   *service.RevokeService
	BanSvc      *service.BanService
	UnbanSvc    *service.BanService
	GetInfoSvc  *service.GetAgentInfoService
	CheckSvc    *service.CheckPermissionService
	BatchSvc    *service.BatchRegisterService

	// MoltCaptcha 占位（P5.6 接入）
	CaptchaChallenge http.HandlerFunc
	CaptchaVerify    http.HandlerFunc
}

// =============================================================================
// POST /api/v2/agents/register
// =============================================================================

// RegisterRequest 注册请求（JSON）。
type RegisterRequest struct {
	UUID       string `json:"uuid"`
	Owner      string `json:"owner"`
	Level      uint8  `json:"level"`
	PublicKey  string `json:"public_key"` // base64.RawURLEncoding
	Permission uint64 `json:"permissions,omitempty"`
}

// Register 注册 handler。
func (h *APIHandler) Register(w http.ResponseWriter, r *http.Request) {
	if h.RegisterSvc == nil {
		http.Error(w, "register service unavailable", http.StatusServiceUnavailable)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	pubBytes, err := base64.RawURLEncoding.DecodeString(req.PublicKey)
	if err != nil {
		http.Error(w, "invalid public_key encoding", http.StatusBadRequest)
		return
	}
	if len(pubBytes) != ed25519.PublicKeySize {
		http.Error(w, "invalid public_key length", http.StatusBadRequest)
		return
	}
	resp, err := h.RegisterSvc.HandleRegister(r.Context(), &service.RegisterAgentRequest{
		UUID:        domain.UUID(req.UUID),
		Owner:       domain.Owner(req.Owner),
		Level:       domain.LevelType(req.Level),
		PublicKey:   ed25519.PublicKey(pubBytes),
		Permissions: req.Permission,
		Now:         time.Now(),
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"agent":        resp.Agent,
		"tx_hash":      resp.TxHash,
		"block_number": resp.BlockNumber,
	})
}

// =============================================================================
// GET /api/v2/agents/{uuid}  POST /api/v2/agents/{uuid}/upgrade  POST .../check
// =============================================================================

// AgentByPath 路由到子端点（按 path 后缀分发）。
func (h *APIHandler) AgentByPath(w http.ResponseWriter, r *http.Request) {
	// path: /api/v2/agents/{uuid}[/upgrade|/check]
	tail := strings.TrimPrefix(r.URL.Path, "/api/v2/agents/")
	parts := strings.Split(tail, "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "missing uuid", http.StatusBadRequest)
		return
	}
	uuid := domain.UUID(parts[0])
	switch {
	case len(parts) == 1 && r.Method == http.MethodGet:
		h.getInfo(w, r, uuid)
	case len(parts) == 2 && parts[1] == "upgrade" && r.Method == http.MethodPost:
		h.upgrade(w, r, uuid)
	case len(parts) == 2 && parts[1] == "check" && r.Method == http.MethodPost:
		h.checkPerm(w, r, uuid)
	case len(parts) == 2 && parts[1] == "ban" && r.Method == http.MethodPost:
		h.ban(w, r, uuid)
	case len(parts) == 2 && parts[1] == "unban" && r.Method == http.MethodPost:
		h.unban(w, r, uuid)
	case len(parts) == 2 && parts[1] == "revoke" && r.Method == http.MethodPost:
		h.revoke(w, r, uuid)
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

func (h *APIHandler) getInfo(w http.ResponseWriter, r *http.Request, uuid domain.UUID) {
	if h.GetInfoSvc == nil {
		http.Error(w, "get_info service unavailable", http.StatusServiceUnavailable)
		return
	}
	info, err := h.GetInfoSvc.HandleGetInfo(r.Context(), uuid)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

type upgradeBody struct {
	NewLevel uint8  `json:"new_level"`
	Reason   string `json:"reason"`
	Actor    string `json:"actor"`
}

func (h *APIHandler) upgrade(w http.ResponseWriter, r *http.Request, uuid domain.UUID) {
	if h.UpgradeSvc == nil {
		http.Error(w, "upgrade service unavailable", http.StatusServiceUnavailable)
		return
	}
	var b upgradeBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	resp, err := h.UpgradeSvc.HandleUpgrade(r.Context(), &service.UpgradeAgentRequest{
		UUID:     uuid,
		NewLevel: domain.LevelType(b.NewLevel),
		Reason:   b.Reason,
		Actor:    b.Actor,
		Now:      time.Now(),
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

type checkBody struct {
	Bit uint `json:"bit"`
}

func (h *APIHandler) checkPerm(w http.ResponseWriter, r *http.Request, uuid domain.UUID) {
	if h.CheckSvc == nil {
		http.Error(w, "check_permission service unavailable", http.StatusServiceUnavailable)
		return
	}
	var b checkBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	resp, err := h.CheckSvc.HandleCheckPermission(r.Context(), &service.CheckPermissionRequest{
		UUID: uuid,
		Bit:  b.Bit,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

type banBody struct {
	Reason string `json:"reason"`
	Actor  string `json:"actor"`
}

func (h *APIHandler) ban(w http.ResponseWriter, r *http.Request, uuid domain.UUID) {
	if h.BanSvc == nil {
		http.Error(w, "ban service unavailable", http.StatusServiceUnavailable)
		return
	}
	var b banBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	resp, err := h.BanSvc.HandleBan(r.Context(), &service.BanAgentRequest{
		UUID:   uuid,
		Reason: b.Reason,
		Actor:  b.Actor,
		Now:    time.Now(),
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *APIHandler) unban(w http.ResponseWriter, r *http.Request, uuid domain.UUID) {
	if h.UnbanSvc == nil {
		http.Error(w, "unban service unavailable", http.StatusServiceUnavailable)
		return
	}
	var b banBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	resp, err := h.UnbanSvc.HandleUnban(r.Context(), &service.UnbanAgentRequest{
		UUID:  uuid,
		Actor: b.Actor,
		Now:   time.Now(),
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *APIHandler) revoke(w http.ResponseWriter, r *http.Request, uuid domain.UUID) {
	if h.RevokeSvc == nil {
		http.Error(w, "revoke service unavailable", http.StatusServiceUnavailable)
		return
	}
	var b banBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	resp, err := h.RevokeSvc.HandleRevoke(r.Context(), &service.RevokeAgentRequest{
		UUID:   uuid,
		Reason: b.Reason,
		Actor:  b.Actor,
		Now:    time.Now(),
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// =============================================================================
// 工具
// =============================================================================

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeServiceError 把 service 层错误映射到 HTTP 状态码。
func writeServiceError(w http.ResponseWriter, err error) {
	// 上下文错误（超时/取消）→ 504/499
	if errors.Is(err, context.DeadlineExceeded) {
		http.Error(w, "deadline exceeded", http.StatusGatewayTimeout)
		return
	}
	if errors.Is(err, context.Canceled) {
		http.Error(w, "request canceled", 499)
		return
	}
	// service 错误按类型映射
	switch {
	case errors.Is(err, service.ErrAgentNotFound),
		errors.Is(err, service.ErrInvalidRegisterInput):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, service.ErrAgentAlreadyExists),
		errors.Is(err, service.ErrAgentAlreadyBanned):
		http.Error(w, err.Error(), http.StatusConflict)
	case errors.Is(err, service.ErrAgentNotBanned),
		errors.Is(err, service.ErrNotRevocable),
		errors.Is(err, service.ErrInvalidUpgradeLevel),
		errors.Is(err, service.ErrAgentNotUpgradable):
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
	case errors.Is(err, service.ErrChainRegisterFailed):
		http.Error(w, err.Error(), http.StatusBadGateway)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
