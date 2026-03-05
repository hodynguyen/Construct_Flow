package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// RateLimitMiddleware implements a sliding window counter per client IP using Redis.
// requestsPerMinute is the maximum number of requests allowed in a 60s window.
func RateLimitMiddleware(rdb *redis.Client, requestsPerMinute int) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		key := fmt.Sprintf("ratelimit:%s", ip)
		ctx := c.Request.Context()

		count, err := rdb.Incr(ctx, key).Result()
		if err != nil {
			// Redis unavailable — fail open (do not block requests)
			c.Next()
			return
		}

		// Set expiry on first request in the window
		if count == 1 {
			rdb.Expire(ctx, key, time.Minute)
		}

		if int(count) > requestsPerMinute {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": fmt.Sprintf("rate limit exceeded: max %d requests per minute", requestsPerMinute),
			})
			return
		}

		c.Next()
	}
}
