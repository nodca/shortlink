package httpmiddleware

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"sync/atomic"
	"strconv"
	"strings"
	"time"

	"day.local/gee"
	"day.local/internal/platform/ratelimit"
)

var rateLimitMemberSeq uint64

// ClientIP 获取“真实客户端 IP”（用于限流/审计/统计）。
//
// 只有当请求来自“可信代理”（如同机 Caddy / 内网 / docker bridge）时，才信任转发头；
// 否则客户端可以伪造 X-Forwarded-For 绕过按 IP 的限流。
func ClientIP(req *http.Request) string {
	remoteHost, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		remoteHost = req.RemoteAddr
	}
	remoteIP := net.ParseIP(remoteHost)

	// 只有当请求来自“可信代理”（如同机 Caddy / 内网 / docker bridge）时，才信任转发头；
	// 否则客户端可以伪造 X-Forwarded-For 绕过按 IP 的限流。
	if remoteIP == nil || !isTrustedProxy(remoteIP) {
		return remoteHost
	}

	// Cloudflare -> Caddy -> app：优先使用 CF-Connecting-IP（Cloudflare 注入的真实客户端 IP）。
	if cf := strings.TrimSpace(req.Header.Get("CF-Connecting-IP")); cf != "" {
		if net.ParseIP(cf) != nil {
			return cf
		}
	}

	// 反向代理常用头。第一个 IP 一般是原始客户端 IP（后面会追加经过的代理 IP）。
	if xff := req.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			xff = xff[:i]
		}
		xff = strings.TrimSpace(xff)
		if net.ParseIP(xff) != nil {
			return xff
		}
	}

	if xrip := strings.TrimSpace(req.Header.Get("X-Real-IP")); xrip != "" {
		if net.ParseIP(xrip) != nil {
			return xrip
		}
	}

	return remoteHost
}

func isTrustedProxy(ip net.IP) bool {
	// 同机反代（如 Caddy 与应用在同一台机器）。
	if ip.IsLoopback() {
		return true
	}

	// RFC1918 私网网段（docker bridge / 内网转发）。
	ip4 := ip.To4()
	if ip4 == nil {
		// IPv6 ULA：fc00::/7
		return len(ip) == net.IPv6len && (ip[0]&0xfe) == 0xfc
	}
	if ip4[0] == 10 {
		return true
	}
	if ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31 {
		return true
	}
	if ip4[0] == 192 && ip4[1] == 168 {
		return true
	}
	return false
}

func RateLimit(limiter *ratelimit.Limiter, prefix string, limit int, window time.Duration) gee.HandlerFunc {
	return func(ctx *gee.Context) {
		ip := ClientIP(ctx.Req)

		var builder strings.Builder
		builder.WriteString("rl:")
		builder.WriteString(prefix)
		builder.WriteString(":")
		builder.WriteString(ip)
		key := builder.String()

		if limiter == nil {
			ctx.Next()
			return
		}
		// member 必须“每次请求唯一”，否则 ZADD 会覆盖同一个 member。
		// 在 Windows/虚拟化环境中 time.Now().UnixNano() 可能短时间内重复；加序列号保证唯一。
		member := strconv.FormatInt(time.Now().UnixNano(), 10) + "-" + strconv.FormatUint(atomic.AddUint64(&rateLimitMemberSeq, 1), 10)
		rlCtx, cancel := context.WithTimeout(ctx.Req.Context(), 50*time.Millisecond)
		defer cancel()
		allowed, retryAfter, err := limiter.Allow(rlCtx, key, limit, window, member)
		if err != nil {
			slog.Error("rate limit check failed", "err", err)
			ctx.Next() // Redis 故障时放行
			return
		}
		if !allowed {
			if retryAfter > 0 {
				// 标准语义：Retry-After 单位是秒。
				secs := int64((retryAfter + time.Second - 1) / time.Second) // ceil
				ctx.SetHeader("Retry-After", strconv.FormatInt(secs, 10))
			}
			ctx.AbortWithError(http.StatusTooManyRequests, "rate limit exceeded")
			return
		}

		ctx.Next()
	}
}
