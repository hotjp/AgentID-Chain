// Package cli 提供 CLI 侧的 AAP（Agent Admission Protocol）握手客户端。
//
// 流程（与 server 端 aap.Generator / aap.ProofSigner 对齐）：
//
//  1. POST {gateway}/aap/challenge  → Challenge{challenge_id, nonce, issued_at, expires_at, domain_sig}
//  2. 客户端用 Ed25519 私钥对 signPayload(challenge_id, nonce, issued_at) 签名
//  3. POST {gateway}/aap/proof {challenge_id, signature, pubkey} → Token{access_token, expires_in}
//  4. 后续请求带 Authorization: Bearer <access_token>
//
// 设计：
//   - Handshake 是显式调用（Client.AAPHandshake(ctx, opts)）
//   - 成功后的 token 缓存在 *Client 上，doJSON 自动注入
//   - 401 响应触发一次重试（refresh token 后再发一次）
package cli

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// =============================================================================
// 错误定义
// =============================================================================

var (
	// ErrAAPPrivateKeyEmpty 私钥为空。
	ErrAAPPrivateKeyEmpty = errors.New("aap: empty private key")
	// ErrAAPPrivateKeyInvalid PEM 解码失败或非 Ed25519 私钥。
	ErrAAPPrivateKeyInvalid = errors.New("aap: invalid private key (expect PKCS#8 PEM Ed25519)")
	// ErrAAPChallengeFailed challenge 请求失败。
	ErrAAPChallengeFailed = errors.New("aap: challenge request failed")
	// ErrAAPProofFailed proof 请求失败。
	ErrAAPProofFailed = errors.New("aap: proof request failed")
	// ErrAAPTokenEmpty 服务端返回的 token 为空。
	ErrAAPTokenEmpty = errors.New("aap: empty access token")
)

// =============================================================================
// 请求 / 响应 DTO
// =============================================================================

// ChallengeResponse 服务端返回的 challenge。
type ChallengeResponse struct {
	ChallengeID string    `json:"challenge_id"`
	Nonce       string    `json:"nonce"`
	IssuedAt    time.Time `json:"issued_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	DomainSig   string    `json:"domain_sig"`
}

// ProofRequest 客户端提交的 proof。
type ProofRequest struct {
	ChallengeID string `json:"challenge_id"`
	Signature   string `json:"signature"` // base64url(ed25519.Sign)
	PublicKey   string `json:"public_key"` // base64url(32B Ed25519 pub)
}

// TokenResponse 服务端返回的 token。
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"` // seconds
	TokenType   string `json:"token_type,omitempty"`
}

// HandshakeOptions 握手选项。
type HandshakeOptions struct {
	// PrivateKey Ed25519 私钥（PEM PKCS#8 字节 / 文件路径 / 原始 64B）。
	PrivateKey []byte
	// PrivateKeyPath 私钥文件路径（与 PrivateKey 二选一；优先）。
	PrivateKeyPath string
	// KeyID 可选：key 标识（写入 proof 头，便于服务端选 verifier）。
	KeyID string
	// Timeout 覆盖 HTTP 超时；0 = 不覆盖。
	Timeout time.Duration
}

// =============================================================================
// 握手实现
// =============================================================================

// aapToken 是 Client 持有的当前 token（线程安全）。
type aapToken struct {
	mu     sync.RWMutex
	token  string
	expiry time.Time
}

func (t *aapToken) get() (string, time.Time) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.token, t.expiry
}

func (t *aapToken) set(token string, expiresIn time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.token = token
	if expiresIn > 0 {
		t.expiry = time.Now().Add(expiresIn - 5*time.Second) // 提前 5s 失效
	} else {
		t.expiry = time.Time{}
	}
}

func (t *aapToken) clear() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.token = ""
	t.expiry = time.Time{}
}

// AAPHandshake 执行 AAP 三段式握手，返回 AccessToken。
//
// 行为：
//   - 从 PrivateKeyPath / PrivateKey 解析 Ed25519 私钥
//   - POST /aap/challenge
//   - 用私钥对 (challenge_id || ":" || nonce || ":" || issued_at) 签名
//   - POST /aap/proof
//   - 把 token 缓存到 Client（后续 doJSON 自动注入 Authorization 头）
func (c *Client) AAPHandshake(ctx context.Context, opts HandshakeOptions) (*TokenResponse, error) {
	priv, err := loadEd25519PrivateKey(opts.PrivateKey, opts.PrivateKeyPath)
	if err != nil {
		return nil, err
	}
	pub := priv.Public().(ed25519.PublicKey)

	// 1. challenge
	chReq, err := c.doChallenge(ctx)
	if err != nil {
		return nil, err
	}

	// 2. sign
	payload := []byte(chReq.ChallengeID + ":" + chReq.Nonce + ":" + chReq.IssuedAt.Format(time.RFC3339Nano))
	sig := ed25519.Sign(priv, payload)
	sigB64 := base64.RawURLEncoding.EncodeToString(sig)
	pubB64 := base64.RawURLEncoding.EncodeToString(pub)

	// 3. proof
	tok, err := c.doProof(ctx, &ProofRequest{
		ChallengeID: chReq.ChallengeID,
		Signature:   sigB64,
		PublicKey:   pubB64,
	})
	if err != nil {
		return nil, err
	}

	// 4. 缓存
	expIn := time.Duration(tok.ExpiresIn) * time.Second
	c.tok.set(tok.AccessToken, expIn)
	return tok, nil
}

// Token 返回当前缓存的 token（无则返回空）。
func (c *Client) Token() string {
	tok, _ := c.tok.get()
	return tok
}

// ClearToken 清空当前 token。
func (c *Client) ClearToken() { c.tok.clear() }

// doChallenge POST /aap/challenge。
func (c *Client) doChallenge(ctx context.Context) (*ChallengeResponse, error) {
	url := strings.TrimRight(c.cfg.Gateway, "/") + "/aap/challenge"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader([]byte("{}")))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.cfg.APIKey != "" {
		req.Header.Set("X-API-Key", c.cfg.APIKey)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAAPChallengeFailed, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status=%d body=%s", ErrAAPChallengeFailed, resp.StatusCode, string(body))
	}
	var out ChallengeResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("%w: decode: %v", ErrAAPChallengeFailed, err)
	}
	if out.ChallengeID == "" || out.Nonce == "" {
		return nil, fmt.Errorf("%w: empty challenge fields", ErrAAPChallengeFailed)
	}
	return &out, nil
}

// doProof POST /aap/proof。
func (c *Client) doProof(ctx context.Context, p *ProofRequest) (*TokenResponse, error) {
	data, _ := json.Marshal(p)
	url := strings.TrimRight(c.cfg.Gateway, "/") + "/aap/proof"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.cfg.APIKey != "" {
		req.Header.Set("X-API-Key", c.cfg.APIKey)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAAPProofFailed, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status=%d body=%s", ErrAAPProofFailed, resp.StatusCode, string(body))
	}
	var out TokenResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("%w: decode: %v", ErrAAPProofFailed, err)
	}
	if out.AccessToken == "" {
		return nil, ErrAAPTokenEmpty
	}
	return &out, nil
}

// =============================================================================
// 私钥加载
// =============================================================================

// loadEd25519PrivateKey 加载 Ed25519 私钥。
//
// 优先级：PrivateKeyPath > PrivateKey > 32 字节 seed（用 32B 私钥直接当 ed25519.PrivateKey）。
//
// 支持格式：
//   - PEM PKCS#8（"PRIVATE KEY" block）
//   - raw 64 字节（ed25519.PrivateKey）
//   - raw 32 字节（seed，扩展为 64B）
func loadEd25519PrivateKey(blob []byte, path string) (ed25519.PrivateKey, error) {
	src := blob
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("%w: read %s: %v", ErrAAPPrivateKeyInvalid, path, err)
		}
		src = data
	}
	if len(src) == 0 {
		return nil, ErrAAPPrivateKeyEmpty
	}

	// PEM
	if bytes.HasPrefix(src, []byte("-----BEGIN")) {
		block, _ := pem.Decode(src)
		if block == nil {
			return nil, fmt.Errorf("%w: pem decode failed", ErrAAPPrivateKeyInvalid)
		}
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("%w: parse pkcs8: %v", ErrAAPPrivateKeyInvalid, err)
		}
		edk, ok := key.(ed25519.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("%w: not ed25519", ErrAAPPrivateKeyInvalid)
		}
		return edk, nil
	}

	// raw
	switch len(src) {
	case ed25519.PrivateKeySize: // 64B
		return ed25519.PrivateKey(src), nil
	case ed25519.SeedSize: // 32B seed
		return ed25519.NewKeyFromSeed(src), nil
	default:
		return nil, fmt.Errorf("%w: bad length %d", ErrAAPPrivateKeyInvalid, len(src))
	}
}
