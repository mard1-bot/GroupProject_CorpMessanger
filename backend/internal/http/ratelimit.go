package http

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisRateLimiter implements rate limiting using Redis
type RedisRateLimiter struct {
	client *redis.Client
}

// NewRedisRateLimiter creates a new Redis-based rate limiter
func NewRedisRateLimiter(redisAddr string) *RedisRateLimiter {
	client := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: "", // No password for local Redis
		DB:       0,  // Default DB
	})

	return &RedisRateLimiter{client: client}
}

// CheckRateLimit checks if the IP is within rate limits
// Returns (allowed, remaining, resetTime, error)
func (r *RedisRateLimiter) CheckRateLimit(ctx context.Context, ip string, requests int, windowSeconds int) (bool, int, time.Time, error) {
	key := fmt.Sprintf("rate_limit:%s", ip)

	// Use Redis INCR with EXPIRE for atomic rate limiting
	current, err := r.client.Incr(ctx, key).Result()
	if err != nil {
		return false, 0, time.Time{}, err
	}

	// Convert int64 to int for comparison
	currentInt := int(current)

	// Set expiration on first request
	if currentInt == 1 {
		r.client.Expire(ctx, key, time.Duration(windowSeconds)*time.Second)
	}

	// Check if over limit
	if currentInt > requests {
		// Get TTL for reset time
		ttl, _ := r.client.TTL(ctx, key).Result()
		resetTime := time.Now().Add(ttl)
		return false, 0, resetTime, nil
	}

	// Calculate remaining
	remaining := requests - currentInt

	// Get TTL for reset time
	ttl, _ := r.client.TTL(ctx, key).Result()
	resetTime := time.Now().Add(ttl)

	return true, remaining, resetTime, nil
}

// Close closes the Redis connection
func (r *RedisRateLimiter) Close() error {
	return r.client.Close()
}
