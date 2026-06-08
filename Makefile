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
DOCKER_IMAGE   := agentid-chain
DOCKER_TAG     := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMPOSE_FILE   := docker-compose.dev.yml

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

.PHONY: docker-build
docker-build: ## 构建 Docker 镜像
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) -t $(DOCKER_IMAGE):latest .

.PHONY: docker-up
docker-up: ## 起 docker compose 全栈
	docker-compose -f $(COMPOSE_FILE) up -d

.PHONY: docker-up-infra
docker-up-infra: ## 只起基础设施（postgres + redis）
	docker-compose -f $(COMPOSE_FILE) up -d postgres redis

.PHONY: docker-down
docker-down: ## 停 docker compose 全栈
	docker-compose -f $(COMPOSE_FILE) down

.PHONY: docker-down-v
docker-down-v: ## 停并清数据卷（谨慎）
	docker-compose -f $(COMPOSE_FILE) down -v

.PHONY: docker-logs
docker-logs: ## 跟随业务日志
	docker-compose -f $(COMPOSE_FILE) logs -f api-gateway auth-center tag-sense

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
