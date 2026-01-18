package cache

import (
	"time"

	"github.com/dgraph-io/ristretto"
)

// LocalCache 基于 ristretto 的本地内存缓存
type LocalCache struct {
	cache    *ristretto.Cache
	ttl      time.Duration
	emptyTTL time.Duration
}

// NewLocalCache 创建本地缓存
// maxItems: 最大缓存条目数（建议 10000-100000）
// maxCost: 最大内存占用（字节，建议 16MB-64MB）
func NewLocalCache(maxItems int64, maxCost int64) (*LocalCache, error) {
	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: maxItems * 10, // 计数器数量，建议为 maxItems 的 10 倍
		MaxCost:     maxCost,
		BufferItems: 64, // 每个 Get 缓冲区大小
	})
	if err != nil {
		return nil, err
	}
	return &LocalCache{
		cache:    cache,
		ttl:      5 * time.Minute,  // 本地缓存 TTL 短一些，保证多实例一致性
		emptyTTL: 10 * time.Second, // 负缓存 TTL
	}, nil
}

func (l *LocalCache) Get(code string) (string, bool) {
	if v, ok := l.cache.Get(code); ok {
		return v.(string), true
	}
	return "", false
}

func (l *LocalCache) Set(code, url string) {
	// cost=1 表示按条目数限制
	l.cache.SetWithTTL(code, url, 1, l.ttl)
}

func (l *LocalCache) SetNotFound(code string) {
	l.cache.SetWithTTL(code, notFoundSentinel, 1, l.emptyTTL)
}

func (l *LocalCache) Del(code string) {
	l.cache.Del(code)
}

func (l *LocalCache) Close() {
	l.cache.Close()
}
