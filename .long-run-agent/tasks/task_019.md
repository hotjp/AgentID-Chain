# task_019

## 描述

[P17] 安全与合规

## 需求 (requirements)

P16 完成

## 验收标准 (acceptance)

- gitleaks / gosec / govulncheck / license check / SBOM
- cosign 镜像签名
- TLS 强制
- 安全响应头
- Rate Limit 阈值
- Auth 审查清单
- Secret 轮转策略
- 日志脱敏

## 交付物 (deliverables)

- .gitleaks.toml
- .gosec.json
- .licensei.toml
- scripts/sbom.sh
- configs/tls.yaml
- internal/gateway/middleware/security.go
- docs/security/{rate-limit,auth-audit,secret-rotation,log-redaction}.md
