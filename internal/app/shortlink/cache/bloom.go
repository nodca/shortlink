package cache

import (
	"sync"

	"github.com/bits-and-blooms/bloom/v3"
)

type BloomFilter struct {
	filter *bloom.BloomFilter
	mu     sync.RWMutex
}

// NewBloomFilter 创建布隆过滤器
// expectedItems: 预期存储的元素数量
// falsePositiveRate: 误判率（建议 0.01 即 1%）
func NewBloomFilter(expectedItems uint, falsePositiveRate float64) *BloomFilter {
	return &BloomFilter{
		filter: bloom.NewWithEstimates(expectedItems, falsePositiveRate),
	}
}

// 添加元素到布隆过滤器
func (b *BloomFilter) Add(code string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.filter.AddString(code)
}

// MightExist 检查元素是否可能存在
// 返回 false 表示一定不存在
// 返回 true 表示可能存在（有误判率）
func (b *BloomFilter) MightExist(code string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.filter.TestString(code)
}

// Count 返回已添加的元素数量（估算）
func (b *BloomFilter) Count() uint32 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.filter.ApproximatedSize()
}
