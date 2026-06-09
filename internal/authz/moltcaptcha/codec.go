package moltcaptcha

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// challengeEnvelope JSON 序列化的信封（v1）。
//
// 格式：{"v":1,"challenge_id":"...","topic":"...","difficulty":"medium",
//        "hops":2,"issued_at":"...","expires_at":"...","time_limit_ns":...,
//        "hints":["..."],"prompt":"..."}
type challengeEnvelope struct {
	Version      int      `json:"v"`
	ChallengeID  string   `json:"challenge_id"`
	Topic        string   `json:"topic"`
	Difficulty   string   `json:"difficulty"`
	Hops         int      `json:"hops"`
	IssuedAt     string   `json:"issued_at"`
	ExpiresAt    string   `json:"expires_at"`
	TimeLimitNs  int64    `json:"time_limit_ns"`
	Hints        []string `json:"hints"`
	Prompt       string   `json:"prompt"`
}

// encodeChallenge 序列化为 JSON。
func encodeChallenge(c *Challenge) ([]byte, error) {
	if c == nil {
		return nil, errors.New("moltcaptcha: nil challenge")
	}
	env := challengeEnvelope{
		Version:     1,
		ChallengeID: c.ChallengeID,
		Topic:       c.Topic,
		Difficulty:  string(c.Difficulty),
		Hops:        c.Hops,
		IssuedAt:    c.IssuedAt.Format(time.RFC3339Nano),
		ExpiresAt:   c.ExpiresAt.Format(time.RFC3339Nano),
		TimeLimitNs: int64(c.TimeLimit),
		Hints:       c.Hints,
		Prompt:      c.PromptTemplate,
	}
	return json.Marshal(env)
}

// decodeChallenge 反序列化。
func decodeChallenge(data []byte) (*Challenge, error) {
	var env challengeEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("moltcaptcha: decode: %w", err)
	}
	if env.Version != 1 {
		return nil, fmt.Errorf("moltcaptcha: unsupported version %d", env.Version)
	}
	issued, err := time.Parse(time.RFC3339Nano, env.IssuedAt)
	if err != nil {
		return nil, fmt.Errorf("moltcaptcha: parse issued_at: %w", err)
	}
	expires, err := time.Parse(time.RFC3339Nano, env.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("moltcaptcha: parse expires_at: %w", err)
	}
	return &Challenge{
		ChallengeID:    env.ChallengeID,
		Topic:          env.Topic,
		Difficulty:     Difficulty(env.Difficulty),
		Hops:           env.Hops,
		IssuedAt:       issued,
		ExpiresAt:      expires,
		TimeLimit:      time.Duration(env.TimeLimitNs),
		Hints:          env.Hints,
		PromptTemplate: env.Prompt,
	}, nil
}

// renderPrompt 渲染 prompt 模板（替换 {topic} / {hops}）。
func renderPrompt(tpl, topic string, hops int) string {
	s := tpl
	s = strings.ReplaceAll(s, "{topic}", topic)
	s = strings.ReplaceAll(s, "{hops}", fmt.Sprintf("%d", hops))
	return s
}
