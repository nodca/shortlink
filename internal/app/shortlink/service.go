package shortlink

import (
	"context"
	"time"
)

// Shortlink 是短链领域对象（domain model）的最小表示。
//
// 说明：
// - Code：短码（用于拼接成最终短链 URL，例如 https://s.example.com/{code}）
// - URL：原始长链接
//
// 设计原因：
// - 领域层只关心“业务含义”，不携带 HTTP/DB 细节（例如状态码、SQL 字段、JSON tag）
type Shortlink struct {
	Code string
	URL  string
}

// Creator 表示“创建短链”的用例能力。
//
// 参数约定：
// - createdBy：可空，表示匿名创建或未接入用户体系
// - expiresAt：可空，表示不过期（MVP 可先不实现过期逻辑）
//
// 设计原因：
// - 用接口表达用例：便于你后续实现不同版本（内存版/DB版/带缓存版）
// - 上层（HTTP）只依赖接口：减少耦合，便于测试（mock）
type Creator interface {
	Create(ctx context.Context, url string, createdBy *int64, expiresAt *time.Time) (Shortlink, error)
}

// Resolver 表示“解析短码并返回目标 URL”的用例能力。
//
// now 作为入参是为了让逻辑更易测试（避免在函数内部直接 time.Now()）。
//
// 设计原因：
// - 读取路径通常是高 QPS 热点：后续可以在实现里加入缓存/负缓存/限流，而不影响接口
type Resolver interface {
	Resolve(ctx context.Context, code string, now time.Time) (string, error)
}
