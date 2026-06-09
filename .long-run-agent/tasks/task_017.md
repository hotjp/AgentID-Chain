# task_017

## 描述

[P15] 测试基础设施

## 需求 (requirements)

P14 完成

## 验收标准 (acceptance)

- testcontainers（PG/Redis）+ miniredis + gomock 模板
- 单元 / 集成 / e2e 三层测试结构
- coverage / race / benchmark / fuzz 工具
- 覆盖率门槛强制（≥70%）

## 交付物 (deliverables)

- internal/testutil/*
- testdata/fixtures.yaml
- tests/e2e/
- scripts/check_coverage.sh
- Makefile（coverage / test-race）
- *_test.go 模板
