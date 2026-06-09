package backend

import (
	"github.com/agentid-chain/agentid-chain/core/chain_adapter"
	"github.com/agentid-chain/agentid-chain/core/chain_adapter/mock"
)

// newMockChainAdapter 构造一个 mock 链适配器（供 factory_test.go 使用）。
func newMockChainAdapter() chain_adapter.BaseChainAdapter {
	return mock.NewMockAdapter()
}
