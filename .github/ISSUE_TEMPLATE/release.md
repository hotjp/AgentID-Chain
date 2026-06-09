---
name: Release Checklist
about: Use this to track release progress
title: "Release vX.Y.Z"
labels: ["release"]
assignees: []
---

# Release vX.Y.Z

## 📋 Pre-release

- [ ] All PRs targeting main are merged
- [ ] All CI checks pass on main
- [ ] Documentation updated
- [ ] CHANGELOG.md updated
- [ ] Security scan clean
- [ ] Performance regression test passed
- [ ] Pre-release check passes: `./scripts/pre-release-check.sh`

## 🚀 Release

- [ ] `./scripts/release.sh [patch|minor|major|vX.Y.Z]`
- [ ] GitHub Actions release workflow succeeded
- [ ] Docker images built and signed
- [ ] Helm chart packaged
- [ ] GitHub Release created
- [ ] Slack notification sent

## 📊 Post-release

- [ ] Smoke test: register a test agent
- [ ] Grafana dashboards show vX.Y.Z
- [ ] Error rate < 0.1%
- [ ] P99 latency < 100ms
- [ ] ArgoCD synced to production
- [ ] Customer-facing changelog updated

## 🔍 Verification Commands

```bash
# Health
curl https://api.agentid-chain.example.com/live

# Version
curl https://api.agentid-chain.example.com:9090/metrics | grep version

# Register
go run ./cmd/agentid register --owner smoketest --level test
```

## 🆘 Rollback Plan

If issues detected:
```bash
helm rollback agentid-chain
# or
helm install agentid-chain deploy/helm/agentid-chain/ --version PREVIOUS
```
