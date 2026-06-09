// Package service: BatchRegister 工作流（P6.4）。
//
// 业务用例：批量注册多个 agent（CI 场景 / 子账号批量开通）。
//
// 设计要点：
//   - 每条独立事务（一条失败不影响其他）
//   - 并发上限（默认 8）— 避免 PG 连接池打爆
//   - 汇总结果：成功列表 + 失败列表（含原因）
package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// =============================================================================
// 错误
// =============================================================================

// ErrEmptyBatch 空批量请求。
var ErrEmptyBatch = errors.New("service: empty batch")

// ErrBatchTooLarge 超过 MaxBatchSize。
var ErrBatchTooLarge = errors.New("service: batch too large")

// =============================================================================
// 入参 / 出参
// =============================================================================

// BatchRegisterItem 批量中的单项。
type BatchRegisterItem struct {
	Index   int                     // 客户端索引
	Request *RegisterAgentRequest
}

// BatchRegisterFailure 单项失败。
type BatchRegisterFailure struct {
	Index int
	UUID  string
	Err   error
}

// BatchRegisterResult 汇总结果。
type BatchRegisterResult struct {
	Succeeded []*RegisterAgentResponse
	Failed    []BatchRegisterFailure
	Total     int
	StartedAt time.Time
	EndedAt   time.Time
}

// =============================================================================
// 依赖
// =============================================================================

// BatchRegisterConfig 批量配置。
type BatchRegisterConfig struct {
	MaxBatchSize int           // 单次最大条目（默认 100）
	Concurrency  int           // 并发上限（默认 8）
}

// BatchRegisterService 批量注册。
type BatchRegisterService struct {
	inner *RegisterService
	cfg   BatchRegisterConfig
}

// NewBatchRegisterService 构造。
func NewBatchRegisterService(inner *RegisterService, cfg BatchRegisterConfig) (*BatchRegisterService, error) {
	if inner == nil {
		return nil, errors.New("service: nil inner register service")
	}
	if cfg.MaxBatchSize == 0 {
		cfg.MaxBatchSize = 100
	}
	if cfg.Concurrency == 0 {
		cfg.Concurrency = 8
	}
	return &BatchRegisterService{inner: inner, cfg: cfg}, nil
}

// HandleBatchRegister 执行批量。
func (s *BatchRegisterService) HandleBatchRegister(ctx context.Context, items []*RegisterAgentRequest) (*BatchRegisterResult, error) {
	if len(items) == 0 {
		return nil, ErrEmptyBatch
	}
	if len(items) > s.cfg.MaxBatchSize {
		return nil, fmt.Errorf("%w: %d > %d", ErrBatchTooLarge, len(items), s.cfg.MaxBatchSize)
	}

	started := time.Now()
	result := &BatchRegisterResult{Total: len(items), StartedAt: started}
	var mu sync.Mutex
	sem := make(chan struct{}, s.cfg.Concurrency)
	var wg sync.WaitGroup

	for i, req := range items {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, r *RegisterAgentRequest) {
			defer wg.Done()
			defer func() { <-sem }()

			resp, err := s.inner.HandleRegister(ctx, r)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				uuidStr := ""
				if r != nil {
					uuidStr = r.UUID.String()
				}
				result.Failed = append(result.Failed, BatchRegisterFailure{
					Index: idx, UUID: uuidStr, Err: err,
				})
				return
			}
			result.Succeeded = append(result.Succeeded, resp)
		}(i, req)
	}
	wg.Wait()
	result.EndedAt = time.Now()
	return result, nil
}
