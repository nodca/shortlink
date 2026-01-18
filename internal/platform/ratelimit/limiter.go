package ratelimit

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type Limiter struct {
	client *redis.Client
}

func NewLimiter(client *redis.Client) *Limiter {
	return &Limiter{
		client: client,
	}
}

// Allow 返回：allowed、retryAfter（仅当超限时有意义）
func (l *Limiter) Allow(ctx context.Context, key string, limit int, window time.Duration, member string) (bool, time.Duration, error) {
	nowMS := time.Now().UnixMilli()
	windowMS := window.Milliseconds()

	const lua = `
	local key = KEYS[1]
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])
local member = ARGV[4]

local windowStart = now - window
redis.call("ZREMRANGEBYSCORE", key, 0, windowStart)
redis.call("ZADD", key, now, member)
local count = redis.call("ZCARD", key)
redis.call("PEXPIRE", key, window)

if count <= limit then
  return {1, 0}
end

redis.call("ZREM", key, member)

local oldest = redis.call("ZRANGE", key, 0, 0, "WITHSCORES")
if oldest[2] ~= nil then
  local oldestScore = tonumber(oldest[2])
  local retryAfter = (oldestScore + window) - now
  if retryAfter < 0 then retryAfter = 0 end
  return {0, retryAfter}
end
return {0, window}
	`

	res, err := l.client.Eval(ctx, lua, []string{key}, nowMS, windowMS, limit, member).Result()
	if err != nil {
		return false, 0, err
	}

	arr, ok := res.([]any)
	if !ok || len(arr) < 2 {
		return false, 0, fmt.Errorf("unexpected redis eval result: %T %v", res, res)
	}

	allowed, _ := arr[0].(int64)
	var retryAfterMs int64
	switch v := arr[1].(type) {
	case int64:
		retryAfterMs = v
	case string:
		retryAfterMs, _ = strconv.ParseInt(v, 10, 64)
	}

	return allowed == 1, time.Duration(retryAfterMs) * time.Millisecond, nil
}
