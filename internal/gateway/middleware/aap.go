// Package middleware: AAP 验签中间件（P7.8）。
//
// 校验 X-AAP-Proof 头（base64 编码的 EdDSA 签名 + 32 字节 agent pubkey +
// challenge_id）。校验通过后把 agentUUID 注入 context。
//
// 当前为占位实现：仅解析 Proof 头格式，不做签名验证（避免循环依赖到 P5.6）。
// 后续接入 internal/authz/aap.Verifier 替换 _verify 即可。
package middleware

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

const (
	// HeaderAAPProof AAP 头。
	HeaderAAPProof = "X-AAP-Proof"
	// HeaderAAPChallenge 当前 challenge id。
	HeaderAAPChallenge = "X-AAP-Challenge"
	// ContextKeyAgentUUID 注入 ctx 的 agent UUID。
	ContextKeyAgentUUID ctxKey = "agent_uuid"
)

// AAPProofHeader X-AAP-Proof 头解析结果。
type AAPProofHeader struct {
	AgentUUID   string `json:"agent_uuid"`
	ChallengeID string `json:"challenge_id"`
	PubKey      string `json:"pubkey_b64"`
	Sig         string `json:"sig_b64"`
}

// AAP 验签中间件（占位）。
//
// TODO: 接入 internal/authz/aap.Verifier 替换 _verify。
func AAP() HandlerFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 探活路径豁免
			if isProbePath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			proofHdr := r.Header.Get(HeaderAAPProof)
			if proofHdr == "" {
				http.Error(w, "missing X-AAP-Proof", http.StatusUnauthorized)
				return
			}
			proof, err := parseAAPProof(proofHdr)
			if err != nil {
				http.Error(w, "invalid X-AAP-Proof: "+err.Error(), http.StatusUnauthorized)
				return
			}
			if err := verifyAAPProofStub(proof); err != nil {
				http.Error(w, "AAP verify failed: "+err.Error(), http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), ContextKeyAgentUUID, proof.AgentUUID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// AgentUUIDFromContext 取出 ctx 内的 agent UUID。
func AgentUUIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ContextKeyAgentUUID).(string); ok {
		return v
	}
	return ""
}

// parseAAPProof 解析 X-AAP-Proof 头：base64(json)。
func parseAAPProof(hdr string) (*AAPProofHeader, error) {
	// 兼容前缀 "v1:"（版本号）
	body := hdr
	if idx := strings.IndexByte(hdr, ':'); idx > 0 && len(hdr) > idx+1 {
		body = hdr[idx+1:]
	}
	raw, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return nil, err
	}
	var p AAPProofHeader
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	if p.AgentUUID == "" || p.ChallengeID == "" || p.PubKey == "" || p.Sig == "" {
		return nil, errors.New("missing required field")
	}
	return &p, nil
}

// verifyAAPProofStub 占位校验：仅验证字段非空 + base64 长度。
//
// 真实实现应调 internal/authz/aap.Verifier.Verify(proof, challenge, pubKey)，
// 校验 EdDSA 签名 + nonce 消费 + challenge 未过期。
func verifyAAPProofStub(p *AAPProofHeader) error {
	if _, err := base64.RawURLEncoding.DecodeString(p.PubKey); err != nil {
		return errors.New("invalid pubkey encoding")
	}
	if _, err := base64.RawURLEncoding.DecodeString(p.Sig); err != nil {
		return errors.New("invalid sig encoding")
	}
	if len(p.AgentUUID) < 16 {
		return errors.New("uuid too short")
	}
	return nil
}
