# 数据迁移

> 数据库 schema 变更与回滚

## 🛠️ 工具

`cmd/migration-tool`

## 📋 命令

```bash
# 应用所有待执行迁移
go run ./cmd/migration-tool up

# 回滚最近 N 步
go run ./cmd/migration-tool down --steps 1

# 查看状态
go run ./cmd/migration-tool status

# 创建新迁移
go run ./cmd/migration-tool create add_user_email
```

## 📂 迁移文件结构

```
internal/storage/migrations/
├── 0001_init.up.sql
├── 0001_init.down.sql
├── 0002_add_agents_table.up.sql
├── 0002_add_agents_table.down.sql
├── 0003_add_chain_status.up.sql
└── 0003_add_chain_status.down.sql
```

### 命名规则
- `<version>_<description>.up.sql`
- `<version>_<description>.down.sql`
- version: 4 位数字
- 永远**同时**写 up 和 down

## ✍️ 编写规范

### 好的迁移

```sql
-- 0004_add_agent_metadata.up.sql
ALTER TABLE agents ADD COLUMN metadata JSONB NOT NULL DEFAULT '{}'::jsonb;
CREATE INDEX idx_agents_owner_level ON agents (owner, level);
```

```sql
-- 0004_add_agent_metadata.down.sql
DROP INDEX IF EXISTS idx_agents_owner_level;
ALTER TABLE agents DROP COLUMN IF EXISTS metadata;
```

### 禁止

```sql
-- ❌ 一次性大量数据修改
UPDATE agents SET metadata = '{}' WHERE metadata IS NULL;  -- 可能锁表

-- ❌ 删除列（不可恢复）
ALTER TABLE agents DROP COLUMN old_field;  -- 先 rename 一段时间

-- ❌ 改类型
ALTER TABLE agents ALTER COLUMN uuid TYPE varchar(50);  -- 用新建列+迁移+删除
```

## 🛡️ 安全检查清单

迁移提交前：

- [ ] up 和 down 文件**都存在**
- [ ] 本地 `up` 成功
- [ ] 本地 `down` 成功
- [ ] 本地 `up` 再次成功
- [ ] 包含 `IF NOT EXISTS` / `IF EXISTS`（幂等）
- [ ] 大表操作使用 `CONCURRENTLY` 索引
- [ ] 无 `UPDATE ... WHERE` 锁表风险
- [ ] 涉及 NOT NULL 的列有默认值

## 🔄 生产发布流程

```bash
# 1. 准备
git pull origin main
go run ./cmd/migration-tool status  # 确认无 pending

# 2. 备份（重要！）
pg_dump -h $PG_HOST -U $PG_USER -d agentid -F c -f backup-$(date +%Y%m%d-%H%M%S).dump

# 3. 灰度（10% 流量 → 50% → 100%）
# 边发布新代码（包含 schema 变更）边观察

# 4. 应用迁移
go run ./cmd/migration-tool up

# 5. 验证
go run ./cmd/migration-tool status
psql "$POSTGRES_DSN" -c "\dt"

# 6. 监控 30 分钟
# 关注：错误率↑ / 慢查询↑ / 连接池↑
```

## 🔙 回滚

```bash
# 1. 紧急：先回滚代码
helm rollback agentid-chain  # 或 docker-compose 重启旧镜像

# 2. 回滚 schema（如必要）
go run ./cmd/migration-tool down --steps 1

# 3. 验证
go run ./cmd/migration-tool status
```

## 📊 大表迁移（> 10M 行）

使用 `pg_repack` 避免锁表：

```bash
# 安装
apt-get install postgresql-15-repack

# 在线重建表
pg_repack -t agents -d agentid
```

## 🔧 工具配置

`configs/app.yaml`:
```yaml
migration:
  table: schema_migrations
  dir: internal/storage/migrations
  timeout: 5m
  lock_timeout: 30s
```

## 📚 相关

- [本地开发](local-dev.md)
- [部署](deployment.md)
- [故障排查](troubleshooting.md)
