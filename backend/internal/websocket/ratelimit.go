package websocket

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// RateLimiter implements token bucket rate limiting for WebSocket connections
type RateLimiter struct {
	// Per-user rate limits
	userLimits map[uuid.UUID]*tokenBucket
	mu         sync.RWMutex

	// Global configuration
	messagesPerMinute int
	burstSize         int
	cleanupInterval   time.Duration
}

// tokenBucket implements the token bucket algorithm
type tokenBucket struct {
	tokens         float64
	lastRefillTime time.Time
	maxTokens      float64
	refillRate     float64 // tokens per second
	mu             sync.Mutex
}

// NewRateLimiter creates a new rate limiter
// messagesPerMinute: maximum messages per minute per user
// burstSize: maximum burst of messages allowed
func NewRateLimiter(messagesPerMinute, burstSize int) *RateLimiter {
	rl := &RateLimiter{
		userLimits:        make(map[uuid.UUID]*tokenBucket),
		messagesPerMinute: messagesPerMinute,
		burstSize:         burstSize,
		cleanupInterval:   5 * time.Minute,
	}

	// Start cleanup goroutine
	go rl.cleanupLoop()

	return rl
}

// Allow checks if a user is allowed to send a message
// Returns true if allowed, false if rate limited
func (rl *RateLimiter) Allow(userID uuid.UUID) bool {
	rl.mu.Lock()
	bucket, exists := rl.userLimits[userID]
	if !exists {
		bucket = newTokenBucket(rl.messagesPerMinute, rl.burstSize)
		rl.userLimits[userID] = bucket
	}
	rl.mu.Unlock()

	return bucket.consume(1)
}

// AllowN checks if a user is allowed to send N messages
func (rl *RateLimiter) AllowN(userID uuid.UUID, n int) bool {
	rl.mu.Lock()
	bucket, exists := rl.userLimits[userID]
	if !exists {
		bucket = newTokenBucket(rl.messagesPerMinute, rl.burstSize)
		rl.userLimits[userID] = bucket
	}
	rl.mu.Unlock()

	return bucket.consume(float64(n))
}

// Reset resets the rate limit for a user (useful for testing or admin actions)
func (rl *RateLimiter) Reset(userID uuid.UUID) {
	rl.mu.Lock()
	delete(rl.userLimits, userID)
	rl.mu.Unlock()
}

// GetStats returns current rate limit statistics for a user
func (rl *RateLimiter) GetStats(userID uuid.UUID) (available float64, max float64) {
	rl.mu.RLock()
	bucket, exists := rl.userLimits[userID]
	rl.mu.RUnlock()

	if !exists {
		return float64(rl.burstSize), float64(rl.burstSize)
	}

	bucket.mu.Lock()
	bucket.refill()
	available = bucket.tokens
	max = bucket.maxTokens
	bucket.mu.Unlock()

	return available, max
}

// cleanupLoop periodically removes inactive rate limiters
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		rl.cleanup()
	}
}

// cleanup removes rate limiters that haven't been used recently
func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for userID, bucket := range rl.userLimits {
		bucket.mu.Lock()
		inactive := now.Sub(bucket.lastRefillTime) > 10*time.Minute
		bucket.mu.Unlock()

		if inactive {
			delete(rl.userLimits, userID)
		}
	}
}

// newTokenBucket creates a new token bucket
func newTokenBucket(messagesPerMinute, burstSize int) *tokenBucket {
	refillRate := float64(messagesPerMinute) / 60.0 // tokens per second

	return &tokenBucket{
		tokens:         float64(burstSize),
		lastRefillTime: time.Now(),
		maxTokens:      float64(burstSize),
		refillRate:     refillRate,
	}
}

// consume attempts to consume n tokens from the bucket
func (tb *tokenBucket) consume(n float64) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refill()

	if tb.tokens >= n {
		tb.tokens -= n
		return true
	}

	return false
}

// refill adds tokens to the bucket based on elapsed time
func (tb *tokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(tb.lastRefillTime).Seconds()

	// Add tokens based on elapsed time
	tokensToAdd := elapsed * tb.refillRate
	tb.tokens = min(tb.tokens+tokensToAdd, tb.maxTokens)
	tb.lastRefillTime = now
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
