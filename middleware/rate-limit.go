package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

var timeFormat = "2006-01-02T15:04:05.000Z"

var inMemoryRateLimiter common.InMemoryRateLimiter

var atomicSlidingWindowScript = redis.NewScript(`
local limit = tonumber(ARGV[1])
local duration = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local expiry = tonumber(ARGV[4])
if limit <= 0 then return 1 end
local n = redis.call('LLEN', KEYS[1])
if n < limit then
  redis.call('LPUSH', KEYS[1], tostring(now))
  redis.call('EXPIRE', KEYS[1], expiry)
  return 1
end
local oldest = tonumber(redis.call('LINDEX', KEYS[1], -1))
if not oldest then return -1 end
if now - oldest < duration then
  redis.call('EXPIRE', KEYS[1], expiry)
  return 0
end
redis.call('LPUSH', KEYS[1], tostring(now))
redis.call('LTRIM', KEYS[1], 0, limit - 1)
redis.call('EXPIRE', KEYS[1], expiry)
return 1
`)

var defNext = func(c *gin.Context) {
	c.Next()
}

func redisRateLimiter(c *gin.Context, maxRequestNum int, duration int64, mark string) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 800*time.Millisecond)
	defer cancel()
	rdb := common.RDB
	key := "rateLimit:v2:" + mark + c.ClientIP()
	allowed, err := atomicSlidingWindowScript.Run(ctx, rdb, []string{key}, maxRequestNum, duration, time.Now().Unix(), int64(common.RateLimitKeyExpirationDuration/time.Second)).Int()
	if err != nil || allowed < 0 {
		c.Header("Retry-After", "2")
		c.Status(http.StatusServiceUnavailable)
		c.Abort()
		return
	}
	if allowed == 0 {
		c.Header("Retry-After", fmt.Sprint(duration))
		c.Status(http.StatusTooManyRequests)
		c.Abort()
	}
}

func memoryRateLimiter(c *gin.Context, maxRequestNum int, duration int64, mark string) {
	key := mark + c.ClientIP()
	if !inMemoryRateLimiter.Request(key, maxRequestNum, duration) {
		c.Status(http.StatusTooManyRequests)
		c.Abort()
		return
	}
}

func rateLimitFactory(maxRequestNum int, duration int64, mark string) func(c *gin.Context) {
	if common.RedisEnabled {
		return func(c *gin.Context) {
			redisRateLimiter(c, maxRequestNum, duration, mark)
		}
	} else {
		// It's safe to call multi times.
		inMemoryRateLimiter.Init(common.RateLimitKeyExpirationDuration)
		return func(c *gin.Context) {
			memoryRateLimiter(c, maxRequestNum, duration, mark)
		}
	}
}

func GlobalWebRateLimit() func(c *gin.Context) {
	if common.GlobalWebRateLimitEnable {
		return rateLimitFactory(common.GlobalWebRateLimitNum, common.GlobalWebRateLimitDuration, "GW")
	}
	return defNext
}

func GlobalAPIRateLimit() func(c *gin.Context) {
	if common.GlobalApiRateLimitEnable {
		return rateLimitFactory(common.GlobalApiRateLimitNum, common.GlobalApiRateLimitDuration, "GA")
	}
	return defNext
}

func CriticalRateLimit() func(c *gin.Context) {
	if common.CriticalRateLimitEnable {
		return rateLimitFactory(common.CriticalRateLimitNum, common.CriticalRateLimitDuration, "CT")
	}
	return defNext
}

func DownloadRateLimit() func(c *gin.Context) {
	return rateLimitFactory(common.DownloadRateLimitNum, common.DownloadRateLimitDuration, "DW")
}

func UploadRateLimit() func(c *gin.Context) {
	return rateLimitFactory(common.UploadRateLimitNum, common.UploadRateLimitDuration, "UP")
}

// userRateLimitFactory creates a rate limiter keyed by authenticated user ID
// instead of client IP, making it resistant to proxy rotation attacks.
// Must be used AFTER authentication middleware (UserAuth).
func userRateLimitFactory(maxRequestNum int, duration int64, mark string) func(c *gin.Context) {
	if common.RedisEnabled {
		return func(c *gin.Context) {
			userId := c.GetInt("id")
			if userId == 0 {
				c.Status(http.StatusUnauthorized)
				c.Abort()
				return
			}
			key := fmt.Sprintf("rateLimit:%s:user:%d", mark, userId)
			userRedisRateLimiter(c, maxRequestNum, duration, key)
		}
	}
	// It's safe to call multi times.
	inMemoryRateLimiter.Init(common.RateLimitKeyExpirationDuration)
	return func(c *gin.Context) {
		userId := c.GetInt("id")
		if userId == 0 {
			c.Status(http.StatusUnauthorized)
			c.Abort()
			return
		}
		key := fmt.Sprintf("%s:user:%d", mark, userId)
		if !inMemoryRateLimiter.Request(key, maxRequestNum, duration) {
			c.Status(http.StatusTooManyRequests)
			c.Abort()
			return
		}
	}
}

// userRedisRateLimiter is like redisRateLimiter but accepts a pre-built key
// (to support user-ID-based keys).
func userRedisRateLimiter(c *gin.Context, maxRequestNum int, duration int64, key string) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 800*time.Millisecond)
	defer cancel()
	rdb := common.RDB
	key = key + ":v2"
	allowed, err := atomicSlidingWindowScript.Run(ctx, rdb, []string{key}, maxRequestNum, duration, time.Now().Unix(), int64(common.RateLimitKeyExpirationDuration/time.Second)).Int()
	if err != nil || allowed < 0 {
		c.Header("Retry-After", "2")
		c.Status(http.StatusServiceUnavailable)
		c.Abort()
		return
	}
	if allowed == 0 {
		c.Header("Retry-After", fmt.Sprint(duration))
		c.Status(http.StatusTooManyRequests)
		c.Abort()
	}
}

// SearchRateLimit returns a per-user rate limiter for search endpoints.
// Configurable via SEARCH_RATE_LIMIT_ENABLE / SEARCH_RATE_LIMIT / SEARCH_RATE_LIMIT_DURATION.
func SearchRateLimit() func(c *gin.Context) {
	if !common.SearchRateLimitEnable {
		return defNext
	}
	return userRateLimitFactory(common.SearchRateLimitNum, common.SearchRateLimitDuration, "SR")
}

// UsageStatisticsRateLimit protects the relatively expensive, read-only
// usage aggregation endpoint without sharing a public-IP bucket with login,
// payment, or other critical operations.
// Configurable via USAGE_STATISTICS_RATE_LIMIT_ENABLE / USAGE_STATISTICS_RATE_LIMIT /
// USAGE_STATISTICS_RATE_LIMIT_DURATION.
func UsageStatisticsRateLimit() func(c *gin.Context) {
	if !common.UsageStatisticsRateLimitEnable {
		return defNext
	}
	return userRateLimitFactory(
		common.UsageStatisticsRateLimitNum,
		common.UsageStatisticsRateLimitDuration,
		"US",
	)
}

// TokenUsageRateLimit protects the read-only token usage polling endpoint
// without consuming the public-IP bucket shared by login and payment.
// It must run after TokenAuthReadOnly so the authenticated user ID is present.
func TokenUsageRateLimit() func(c *gin.Context) {
	if !common.TokenUsageRateLimitEnable {
		return defNext
	}
	return userRateLimitFactory(
		common.TokenUsageRateLimitNum,
		common.TokenUsageRateLimitDuration,
		"TU",
	)
}
