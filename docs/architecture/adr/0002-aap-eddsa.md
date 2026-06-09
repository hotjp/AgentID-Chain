# ADR-0002: AAP 使用 EdDSA 签名

## 状态

✅ Accepted (2026-01-20)

## 上下文

AAP（Agent Admission Protocol）需要在 Agent 注册时进行 Challenge-Response 验证，防止：
- 重放攻击
- 离线伪造
- 私钥泄露后的大规模滥用

候选签名方案：

| 方案 | 密钥大小 | 签名大小 | 性能 | 生态 |
|------|---------|---------|------|------|
| **RSA-2048** | 256B | 256B | 慢（~1ms） | 极广 |
| **ECDSA P-256** | 32B | 64B | 中（~200μs） | 广 |
| **EdDSA Ed25519** | 32B | 64B | 快（~50μs） | 增长中 |
| **BLS12-381** | 32B | 32B | 慢（聚合） | 区块链 |

## 决策

采用 **EdDSA Ed25519**：
- 性能：基准 53μs/verify（见 [aap-benchmark.md](../../perf/aap-benchmark.md)）
- 安全性：与 ECDSA P-256 同等（128-bit）
- 确定性签名：避免 ECDSA 的随机数问题
- 紧凑：公钥 32B / 签名 64B

## 后果

### 正面
- ✅ 性能优秀（比 ECDSA P-256 快 4 倍）
- ✅ 确定性签名（同一消息 + 私钥 → 同一签名），便于测试
- ✅ 公钥 32B 紧凑，适合 JWT Header 携带
- ✅ 无需大整数运算，跨平台一致性好

### 负面
- ❌ 生态比 RSA / ECDSA 略弱（部分老旧硬件 HSM 不支持）
- ❌ 需要 Go 1.13+（`crypto/ed25519`）

### 中性
- 🔄 与 BIP32-Ed25519 兼容（可派生）
- 🔄 在区块链场景可与 Solana / Polkadot 互操作

## 替代方案

| 方案 | 否决理由 |
|------|---------|
| **RSA-2048** | 性能太差（~1ms），不适合高频调用 |
| **ECDSA P-256** | 慢 4 倍；存在随机数侧信道风险 |
| **BLS12-381** | 主要用于签名聚合；单独签名性能不优；库支持弱 |
| **SM2** (国密) | 国际生态弱；如需支持可作为插件 |

## 实现细节

```go
// 私钥
priv := ed25519.NewKeyPair(rand.Reader)

// Challenge（服务端生成）
challenge := make([]byte, 32)
rand.Read(challenge)
challengeB64 := base64.StdEncoding.EncodeToString(challenge)

// 客户端签名
sig, _ := priv.Sign(rand.Reader, challenge, crypto.Hash(0))
sigB64 := base64.StdEncoding.EncodeToString(sig)

// 服务端验证
ok := ed25519.Verify(pub, challenge, sig)
```

存储：私钥使用 AES-256-GCM 加密后存 PG（key 在 Redis / KMS）。

## 参考

- [RFC 8032: Edwards-Curve Digital Signature Algorithm (EdDSA)](https://www.rfc-editor.org/rfc/rfc8032)
- [perf/aap-benchmark.md](../../perf/aap-benchmark.md)
