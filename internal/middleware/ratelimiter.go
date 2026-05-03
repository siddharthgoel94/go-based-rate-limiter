package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

type RateLimiter struct {
	client     *redis.Client
	limit      int64
	windowSecs int64
}

func NewRateLimiter(client *redis.Client, limit int64, windowSecs int64) *RateLimiter {
	return &RateLimiter{client: client, limit: limit, windowSecs: windowSecs}
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientID := r.Header.Get("X-API-Key")
		if clientID == "" {
			clientID = r.RemoteAddr
		}

		allowed, remaining, err := rl.isAllowed(r.Context(), clientID)
		if err != nil {
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", rl.limit))
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))

		if !allowed {
			w.Header().Set("Retry-After", fmt.Sprintf("%d", rl.windowSecs))
			http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (rl *RateLimiter) isAllowed(ctx context.Context, clientID string) (bool, int64, error) {
	key := "ratelimit:" + clientID
	now := time.Now().UnixMilli()
	windowStart := now - (rl.windowSecs * 1000)

	pipe := rl.client.TxPipeline()
	pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart))
	countCmd := pipe.ZCard(ctx, key)
	pipe.ZAdd(ctx, key, redis.Z{Score: float64(now), Member: now})
	pipe.Expire(ctx, key, time.Duration(rl.windowSecs)*time.Second)

	if _, err := pipe.Exec(ctx); err != nil {
		return false, 0, err
	}

	count := countCmd.Val()
	if count >= rl.limit {
		rl.client.ZRem(ctx, key, now)
		return false, 0, nil
	}

	return true, rl.limit - count - 1, nil
}
