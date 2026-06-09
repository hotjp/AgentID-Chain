// Package backend: 内存 Persistence（in-memory 存储）。
//
// 用途：
//   - LocalBackend 内部默认（生产替换为 ent + PG）
//   - 单元测试 / 集成测试
//   - 演示模式（demo / playground）
//
// 特性：
//   - 线程安全（sync.RWMutex）
//   - 索引：UUID → agent；owner → [uuids]
//   - 审计 append-only
//   - 不持久化（重启即清空）
package backend

import (
	"context"
	"sort"
	"sync"
)

// MemoryPersistence 内存持久化（线程安全）。
type MemoryPersistence struct {
	mu      sync.RWMutex
	agents  map[string]*AgentInfo            // uuid → agent
	byOwner map[string]map[string]struct{}   // owner → set<uuid>
	logs    map[string][]ChangeLog           // uuid → logs
}

// NewMemoryPersistence 构造。
func NewMemoryPersistence() *MemoryPersistence {
	return &MemoryPersistence{
		agents:  map[string]*AgentInfo{},
		byOwner: map[string]map[string]struct{}{},
		logs:    map[string][]ChangeLog{},
	}
}

// PutAgent upsert。
func (m *MemoryPersistence) PutAgent(_ context.Context, rec *AgentInfo) error {
	if rec == nil || rec.UUID == "" {
		return ErrBackendUnavailable
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	// 复制避免外部修改
	cp := *rec
	m.agents[rec.UUID] = &cp
	if _, ok := m.byOwner[rec.Owner]; !ok {
		m.byOwner[rec.Owner] = map[string]struct{}{}
	}
	m.byOwner[rec.Owner][rec.UUID] = struct{}{}
	return nil
}

// GetAgent 查询。
func (m *MemoryPersistence) GetAgent(_ context.Context, uuid string) (*AgentInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rec, ok := m.agents[uuid]
	if !ok {
		return nil, ErrAgentNotFound
	}
	cp := *rec
	return &cp, nil
}

// ListAgentsByOwner 列出 owner 名下所有 agent。
func (m *MemoryPersistence) ListAgentsByOwner(_ context.Context, owner string) ([]*AgentInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	set := m.byOwner[owner]
	if len(set) == 0 {
		return nil, nil
	}
	out := make([]*AgentInfo, 0, len(set))
	for u := range set {
		if a, ok := m.agents[u]; ok {
			cp := *a
			out = append(out, &cp)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UUID < out[j].UUID })
	return out, nil
}

// AppendLog 追加。
func (m *MemoryPersistence) AppendLog(_ context.Context, log *ChangeLog) error {
	if log == nil || log.UUID == "" {
		return ErrBackendUnavailable
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *log
	m.logs[log.UUID] = append(m.logs[log.UUID], cp)
	return nil
}

// ListLogs 查询。
func (m *MemoryPersistence) ListLogs(_ context.Context, uuid string, limit int) ([]ChangeLog, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	logs := m.logs[uuid]
	if limit > 0 && len(logs) > limit {
		// 返回最新的 limit 条
		logs = logs[len(logs)-limit:]
	}
	out := make([]ChangeLog, len(logs))
	copy(out, logs)
	return out, nil
}

// BatchGet 批量。
func (m *MemoryPersistence) BatchGet(_ context.Context, uuids []string) (map[string]*AgentInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := map[string]*AgentInfo{}
	for _, u := range uuids {
		if a, ok := m.agents[u]; ok {
			cp := *a
			out[u] = &cp
		}
	}
	return out, nil
}

// Count 返回 agent 总数（测试用）。
func (m *MemoryPersistence) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.agents)
}
