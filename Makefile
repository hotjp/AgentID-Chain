# =============================================================================
# AgentID-Chain Makefile (v2.0.1)
# =============================================================================
# Usage: make help
# Convention: 每个 target 之前的 ## 注释会自动出现在 make help 输出中
# =============================================================================

SHELL          := /usr/bin/env bash
.SHELLFLAGS    := -eu -o pipefail -c

# ---------- 项目变量 ---------------------------------------------------------
MODULE         := github.com/agentid-chain/agentid-chain
BIN_DIR        := bin
BIN_NAME       := agentid
CMD_DIR        := ./cmd/agentid
PKGS           := ./...
COVERAGE_FILE  := coverage.out
COVERAGE_HTML  := coverage.html
COVERAGE_MIN   := 70

# ---------- Docker 变量 ------------------------------------------------------
DOCKER_REGISTRY := agentid-chain
DOCKER_IMAGE    := agentid-chain
DOCKER_TAG      := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
DOCKER_SHA      := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DOCKER_COMMIT   := $(DOCKER_SHA)
DOCKER_BUILD_DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
COMPOSE_FILE    := docker-compose.dev.yml
# 镜像命名：{registry}/{name}:{tag}
DOCKER_GATEWAY     := $(DOCKER_REGISTRY)/gateway
DOCKER_CLI         := $(DOCKER_REGISTRY)/cli
DOCKER_MIGRATION   := $(DOCKER_REGISTRY)/migration
DOCKER_MOCK_CHAIN  := $(DOCKER_REGISTRY)/mock-chain

# ---------- Go 工具 ---------------------------------------------------------
GO             := go
GOFLAGS        :=
LDFLAGS        := -s -w \
                  -X 'main.Version=$(DOCKER_TAG)' \
                  -X 'main.Commit=$(shell git rev-parse --short HEAD 2>/dev/null || echo none)' \
                  -X 'main.BuildDate=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)'

# ---------- 默认 target ------------------------------------------------------
.DEFAULT_GOAL := help

# =============================================================================
# 帮助
# =============================================================================
.PHONY: help
help: ## 显示所有可用 target
	@printf "\033[1mAgentID-Chain Makefile\033[0m\n"
	@printf "Usage: make <target>\n\n"
	@awk 'BEGIN {FS = ":.*?## "} \
		/^[a-zA-Z0-9_-]+:.*?## / { printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2 } \
		/^## / { printf "\n\033[1m%s\033[0m\n", substr($$0,4) }' \
		$(MAKEFILE_LIST)

# =============================================================================
## 构建
# =============================================================================

.PHONY: build
build: ## 编译二进制到 bin/agentid
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BIN_NAME) $(CMD_DIR)
	@echo "✅ 已生成 $(BIN_DIR)/$(BIN_NAME)"

.PHONY: build-all
build-all: ## 交叉编译 linux/amd64 + linux/arm64 + darwin/arm64
	@mkdir -p $(BIN_DIR)
	GOOS=linux  GOARCH=amd64 $(GO) build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BIN_NAME)-linux-amd64  $(CMD_DIR)
	GOOS=linux  GOARCH=arm64 $(GO) build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BIN_NAME)-linux-arm64  $(CMD_DIR)
	GOOS=darwin GOARCH=arm64 $(GO) build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BIN_NAME)-darwin-arm64 $(CMD_DIR)
	@echo "✅ 已生成跨平台二进制 (linux/amd64, linux/arm64, darwin/arm64)"

.PHONY: install
install: ## 把二进制安装到 $$GOBIN
	$(GO) install -ldflags "$(LDFLAGS)" $(CMD_DIR)

# =============================================================================
## 测试 & 覆盖率
# =============================================================================

.PHONY: test
test: ## 跑单元测试（race + verbose）
	$(GO) test -race -v $(PKGS)

.PHONY: test-short
test-short: ## 跑短测试（跳过慢测试）
	$(GO) test -race -short $(PKGS)

.PHONY: test-integration
test-integration: ## 跑集成测试（需要 PostgreSQL/Redis 起好；tag=integration）
	$(GO) test -race -tags=integration -v $(PKGS)

.PHONY: coverage
coverage: ## 生成覆盖率，并校验 ≥ $(COVERAGE_MIN)%
	$(GO) test -race -coverprofile=$(COVERAGE_FILE) -covermode=atomic $(PKGS)
	@$(GO) tool cover -func=$(COVERAGE_FILE) | tail -1
	@total=$$($(GO) tool cover -func=$(COVERAGE_FILE) | tail -1 | awk '{print $$3}' | tr -d '%'); \
	awk -v t=$$total -v m=$(COVERAGE_MIN) 'BEGIN { if (t+0 < m+0) { printf "❌ 覆盖率 %s%% < %s%%\n", t, m; exit 1 } else { printf "✅ 覆盖率 %s%% ≥ %s%%\n", t, m } }'

.PHONY: coverage-html
coverage-html: coverage ## 生成 HTML 覆盖率报告 (coverage.html)
	$(GO) tool cover -html=$(COVERAGE_FILE) -o $(COVERAGE_HTML)
	@echo "✅ 已生成 $(COVERAGE_HTML)"

# =============================================================================
## 代码质量
# =============================================================================

.PHONY: fmt
fmt: ## 格式化代码
	$(GO) fmt $(PKGS)

.PHONY: vet
vet: ## go vet 静态检查
	$(GO) vet $(PKGS)

.PHONY: lint
lint: ## golangci-lint 全量检查
	golangci-lint run $(PKGS)

.PHONY: lint-fix
lint-fix: ## golangci-lint --fix 自动修复
	golangci-lint run --fix $(PKGS)

.PHONY: tidy
tidy: ## go mod tidy
	$(GO) mod tidy

.PHONY: check
check: fmt vet lint test ## fmt + vet + lint + test 一键检查

# =============================================================================
## Docker
# =============================================================================

# 公共 build args
DOCKER_BUILD_ARGS := \
	--build-arg VERSION=$(DOCKER_TAG) \
	--build-arg COMMIT=$(DOCKER_COMMIT) \
	--build-arg BUILD_DATE=$(DOCKER_BUILD_DATE)

.PHONY: docker-build
docker-build: docker-build-gateway docker-build-cli docker-build-migration docker-build-mock-chain ## 构建所有 Docker 镜像（gateway/cli/migration/mock-chain）

.PHONY: docker-build-gateway
docker-build-gateway: ## 构建 gateway 镜像
	docker build $(DOCKER_BUILD_ARGS) \
		-f docker/Dockerfile.gateway \
		-t $(DOCKER_GATEWAY):$(DOCKER_TAG) \
		-t $(DOCKER_GATEWAY):latest \
		-t $(DOCKER_GATEWAY):sha-$(DOCKER_SHA) \
		.

.PHONY: docker-build-cli
docker-build-cli: ## 构建 cli 镜像
	docker build $(DOCKER_BUILD_ARGS) \
		-f docker/Dockerfile.cli \
		-t $(DOCKER_CLI):$(DOCKER_TAG) \
		-t $(DOCKER_CLI):latest \
		-t $(DOCKER_CLI):sha-$(DOCKER_SHA) \
		.

.PHONY: docker-build-migration
docker-build-migration: ## 构建 migration 镜像
	docker build $(DOCKER_BUILD_ARGS) \
		-f docker/Dockerfile.migration \
		-t $(DOCKER_MIGRATION):$(DOCKER_TAG) \
		-t $(DOCKER_MIGRATION):latest \
		-t $(DOCKER_MIGRATION):sha-$(DOCKER_SHA) \
		.

.PHONY: docker-build-mock-chain
docker-build-mock-chain: ## 构建 mock-chain 镜像
	docker build $(DOCKER_BUILD_ARGS) \
		-f docker/Dockerfile.mock-chain \
		-t $(DOCKER_MOCK_CHAIN):$(DOCKER_TAG) \
		-t $(DOCKER_MOCK_CHAIN):latest \
		-t $(DOCKER_MOCK_CHAIN):sha-$(DOCKER_SHA) \
		.

.PHONY: docker-push
docker-push: docker-push-gateway docker-push-cli docker-push-migration docker-push-mock-chain ## 推送所有镜像到仓库

.PHONY: docker-push-gateway
docker-push-gateway: ## 推送 gateway 镜像
	docker push $(DOCKER_GATEWAY):$(DOCKER_TAG)
	docker push $(DOCKER_GATEWAY):latest
	docker push $(DOCKER_GATEWAY):sha-$(DOCKER_SHA)

.PHONY: docker-push-cli
docker-push-cli: ## 推送 cli 镜像
	docker push $(DOCKER_CLI):$(DOCKER_TAG)
	docker push $(DOCKER_CLI):latest
	docker push $(DOCKER_CLI):sha-$(DOCKER_SHA)

.PHONY: docker-push-migration
docker-push-migration: ## 推送 migration 镜像
	docker push $(DOCKER_MIGRATION):$(DOCKER_TAG)
	docker push $(DOCKER_MIGRATION):latest
	docker push $(DOCKER_MIGRATION):sha-$(DOCKER_SHA)

.PHONY: docker-push-mock-chain
docker-push-mock-chain: ## 推送 mock-chain 镜像
	docker push $(DOCKER_MOCK_CHAIN):$(DOCKER_TAG)
	docker push $(DOCKER_MOCK_CHAIN):latest
	docker push $(DOCKER_MOCK_CHAIN):sha-$(DOCKER_SHA)

.PHONY: docker-up
docker-up: ## 起 docker compose 全栈
	docker-compose -f $(COMPOSE_FILE) up -d

.PHONY: docker-up-infra
docker-up-infra: ## 只起基础设施（postgres + redis）
	docker-compose -f $(COMPOSE_FILE) up -d postgres redis

.PHONY: docker-up-local
docker-up-local: ## 起 local-only compose（无端口暴露）
	docker-compose -f docker/compose/docker-compose.local.yml up -d

.PHONY: docker-up-hybrid
docker-up-hybrid: ## 起 hybrid compose（+ mock-chain）
	docker-compose -f docker/compose/docker-compose.hybrid.yml up -d

.PHONY: docker-down
docker-down: ## 停 docker compose 全栈
	docker-compose -f $(COMPOSE_FILE) down

.PHONY: docker-down-v
docker-down-v: ## 停并清数据卷（谨慎）
	docker-compose -f $(COMPOSE_FILE) down -v

.PHONY: docker-logs
docker-logs: ## 跟随业务日志
	docker-compose -f $(COMPOSE_FILE) logs -f api-gateway auth-center tag-sense

.PHONY: docker-clean
docker-clean: ## 清理 dangling 镜像与未使用卷
	docker image prune -f
	docker volume prune -f

# =============================================================================
## Docker Buildx（多架构构建）
# =============================================================================

BUILDX_BUILDER ?= agentid-builder
PLATFORMS      ?= linux/amd64,linux/arm64

.PHONY: docker-buildx-create
docker-buildx-create: ## 创建 buildx builder（多架构）
	docker buildx create --name $(BUILDX_BUILDER) --driver docker-container --bootstrap || true
	docker buildx use $(BUILDX_BUILDER)

.PHONY: docker-buildx
docker-buildx: docker-buildx-create ## 多架构构建并推送（amd64 + arm64）
	docker buildx build --platform $(PLATFORMS) \
		$(DOCKER_BUILD_ARGS) \
		--push \
		-f docker/Dockerfile.gateway \
		-t $(DOCKER_GATEWAY):$(DOCKER_TAG) \
		-t $(DOCKER_GATEWAY):latest \
		.
	docker buildx build --platform $(PLATFORMS) \
		$(DOCKER_BUILD_ARGS) \
		--push \
		-f docker/Dockerfile.cli \
		-t $(DOCKER_CLI):$(DOCKER_TAG) \
		-t $(DOCKER_CLI):latest \
		.
	docker buildx build --platform $(PLATFORMS) \
		$(DOCKER_BUILD_ARGS) \
		--push \
		-f docker/Dockerfile.migration \
		-t $(DOCKER_MIGRATION):$(DOCKER_TAG) \
		-t $(DOCKER_MIGRATION):latest \
		.
	docker buildx build --platform $(PLATFORMS) \
		$(DOCKER_BUILD_ARGS) \
		--push \
		-f docker/Dockerfile.mock-chain \
		-t $(DOCKER_MOCK_CHAIN):$(DOCKER_TAG) \
		-t $(DOCKER_MOCK_CHAIN):latest \
		.

.PHONY: docker-buildx-load
docker-buildx-load: docker-buildx-create ## 多架构构建并 load 到本地（仅 amd64）
	docker buildx build --platform linux/amd64 \
		$(DOCKER_BUILD_ARGS) \
		--load \
		-f docker/Dockerfile.gateway \
		-t $(DOCKER_GATEWAY):$(DOCKER_TAG) \
		.

.PHONY: docker-buildx-inspect
docker-buildx-inspect: ## 查看当前 buildx 状态
	docker buildx ls
	docker buildx inspect $(BUILDX_BUILDER)

# =============================================================================
## 镜像签名（cosign）
# =============================================================================

COSIGN_KEY ?= cosign.key
COSIGN_PUB ?= cosign.pub
COSIGN_PASSWORD ?= $(shell command -v cosign >/dev/null && cosign generate-key-pair 2>/dev/null || echo "")

.PHONY: cosign-keygen
cosign-keygen: ## 生成 cosign 签名密钥对（仅首次）
	@if [ ! -f $(COSIGN_KEY) ]; then \
		echo "==> 生成 cosign 密钥对..."; \
		COSIGN_PASSWORD="" cosign generate-key-pair; \
	else \
		echo "==> 密钥对已存在：$(COSIGN_KEY) / $(COSIGN_PUB)"; \
	fi

.PHONY: cosign-sign
cosign-sign: cosign-keygen ## 用 cosign 签名所有镜像
	@echo "==> Signing gateway:$(DOCKER_TAG)"
	COSIGN_PASSWORD="" cosign sign --key $(COSIGN_KEY) $(DOCKER_GATEWAY):$(DOCKER_TAG)
	@echo "==> Signing cli:$(DOCKER_TAG)"
	COSIGN_PASSWORD="" cosign sign --key $(COSIGN_KEY) $(DOCKER_CLI):$(DOCKER_TAG)
	@echo "==> Signing migration:$(DOCKER_TAG)"
	COSIGN_PASSWORD="" cosign sign --key $(COSIGN_KEY) $(DOCKER_MIGRATION):$(DOCKER_TAG)
	@echo "==> Signing mock-chain:$(DOCKER_TAG)"
	COSIGN_PASSWORD="" cosign sign --key $(COSIGN_KEY) $(DOCKER_MOCK_CHAIN):$(DOCKER_TAG)

.PHONY: cosign-verify
cosign-verify: ## 验证 gateway 镜像签名
	cosign verify --key $(COSIGN_PUB) $(DOCKER_GATEWAY):$(DOCKER_TAG)

.PHONY: cosign-verify-all
cosign-verify-all: ## 验证所有镜像签名
	@echo "==> Verifying gateway"
	cosign verify --key $(COSIGN_PUB) $(DOCKER_GATEWAY):$(DOCKER_TAG)
	@echo "==> Verifying cli"
	cosign verify --key $(COSIGN_PUB) $(DOCKER_CLI):$(DOCKER_TAG)
	@echo "==> Verifying migration"
	cosign verify --key $(COSIGN_PUB) $(DOCKER_MIGRATION):$(DOCKER_TAG)
	@echo "==> Verifying mock-chain"
	cosign verify --key $(COSIGN_PUB) $(DOCKER_MOCK_CHAIN):$(DOCKER_TAG)

# =============================================================================
## 数据库 / 迁移
# =============================================================================

.PHONY: migrate
migrate: ## 执行 ent migrations（待 P3 完成后启用）
	@echo "ℹ️  migrate target — 待 P3 storage 层完成后接 atlas/ent migrate"

.PHONY: db-shell
db-shell: ## 进入 dev-postgres psql shell
	docker exec -it dev-postgres psql -U devuser -d apigateway

.PHONY: redis-shell
redis-shell: ## 进入 dev-redis redis-cli
	docker exec -it dev-redis redis-cli

# =============================================================================
## 清理
# =============================================================================

.PHONY: clean
clean: ## 清理构建产物与覆盖率报告
	rm -rf $(BIN_DIR) $(COVERAGE_FILE) $(COVERAGE_HTML) *.prof *.pprof
	@echo "✅ 已清理"

.PHONY: clean-all
clean-all: clean docker-down-v ## 清理一切（含 docker 卷）

# =============================================================================
## 工具版本
# =============================================================================

.PHONY: tools-version
tools-version: ## 显示工具链版本
	@echo "Go:            $$($(GO) version)"
	@echo "golangci-lint: $$(golangci-lint --version 2>/dev/null || echo 'NOT INSTALLED')"
	@echo "Docker:        $$(docker --version 2>/dev/null || echo 'NOT INSTALLED')"
	@echo "Compose:       $$(docker-compose --version 2>/dev/null || echo 'NOT INSTALLED')"
