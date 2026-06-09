# ADR (Architecture Decision Records)

> 关键架构决策的可追溯记录

## 📋 ADR 索引

| ID | 标题 | 状态 | 日期 |
|----|------|------|------|
| [0001](0001-storage-hybrid.md) | 混合存储架构 | ✅ Accepted | 2026-01-15 |
| [0002](0002-aap-eddsa.md) | AAP 使用 EdDSA 签名 | ✅ Accepted | 2026-01-20 |
| [0003](0003-uuid-v7.md) | 默认使用 UUID v7 | ✅ Accepted | 2026-02-01 |

## 📝 ADR 模板

使用 [MADR](https://adr.github.io/madr/) 格式：

```markdown
# ADR-NNNN: <简短标题>

## 状态

Proposed | Accepted | Deprecated | Superseded by ADR-XXXX

## 上下文

需要解决的问题 / 决策背景。

## 决策

我们决定...

## 后果

### 正面
- ...

### 负面
- ...

### 中性
- ...

## 替代方案

| 方案 | 优点 | 缺点 |
|------|------|------|
| ... | ... | ... |

## 参考

- 相关 RFC / 文档链接
```

## 🔄 提交流程

1. 复制本目录的 `template.md` 为 `NNNN-short-title.md`
2. 填写所有章节
3. 在本 README 添加索引行
4. 提交 PR，标题：`docs(adr): ADR-NNNN <标题>`
5. Review 通过后合并

## 🗃️ 归档

已 Deprecated 的 ADR 保留在原位，仅修改状态字段，不删除。
