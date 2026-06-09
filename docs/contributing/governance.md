# 治理

> Constitution + ADR + 季度评审 — 项目持续改进机制

## 📜 治理三角

```
     Constitution
       (宪法)
      /        \
     /          \
    /            \
   v              v
  ADR           季度评审
(决策)          (评估)
```

## 🏛️ Constitution（宪法）

不可妥协 / 强制 / 可配置的规则集合。

**位置**：`.long-run-agent/constitution.yaml`

详见 [governance.md](../../.long-run-agent/governance.md)

## 📐 ADR（架构决策记录）

记录**为什么**做了某个架构决策（不是做什么）。

**位置**：`docs/architecture/adr/`

### 何时写 ADR

- ✅ 新增 / 修改模块
- ✅ 引入新依赖
- ✅ 改变数据模型
- ✅ 改变协议 / 接口
- ✅ 改变安全模型
- ❌ Bug 修复（用 PR 描述）
- ❌ 文档修改（用 PR 描述）
- ❌ 重构（仅当影响架构）

### 模板

```bash
cp templates/adr-template.md docs/architecture/adr/NNNN-short-title.md
# 填写内容
# 提交 PR
```

### 状态机

```
Proposed ──> Accepted ──> Deprecated
                │
                └──> Superseded by ADR-XXXX
```

## 🔄 季度评审

每季度（Q1/Q2/Q3/Q4）评审治理有效性。

**位置**：`.long-run-agent/quarterly-review.md`

详见 [quarterly-review.md](../../.long-run-agent/quarterly-review.md)

## 🛠️ 工具

```bash
# 验证 Constitution 遵守
./scripts/constitution-check.sh

# 自检文档
./scripts/check-docs.sh

# 发布前自检
./scripts/pre-release-check.sh
```

## 📊 治理指标

| 指标 | 目标 | 当前 |
|------|------|------|
| Constitution 检查通过率 | 100% | |
| ADR 平均 Review 时间 | < 5 天 | |
| 季度评审完成率 | 100% | |
| L2 零依赖违规数 | 0 | |
| 提交规范符合率 | ≥ 90% | |

## 📚 相关

- [贡献者指南](development.md)
- [PR 流程](pr-process.md)
- [代码规范](style.md)
