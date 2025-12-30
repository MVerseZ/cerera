package connection

import (
	"sync"
	"time"
)

// RateLimiter implements per-connection rate limiting
type RateLimiter struct {
	mu              sync.RWMutex
	connections     map[string]*connectionLimiter
	maxMessagesPerSecond int
	burstSize       int
	cleanupInterval time.Duration
	stopChan        chan struct{}
}

// connectionLimiter tracks rate limiting for a single connection
type connectionLimiter struct {
	lastReset    time.Time
	messageCount int
	mu           sync.Mutex
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(maxMessagesPerSecond, burstSize int) *RateLimiter {
	rl := &RateLimiter{
		connections:          make(map[string]*connectionLimiter),
		maxMessagesPerSecond: maxMessagesPerSecond,
		burstSize:            burstSize,
		cleanupInterval:      1 * time.Minute,
		stopChan:             make(chan struct{}),
	}
	
	// Start cleanup goroutine
	go rl.cleanup()
	
	return rl
}

// Allow checks if a message is allowed for a connection
func (rl *RateLimiter) Allow(connID string) bool {
	rl.mu.Lock()
	limiter, exists := rl.connections[connID]
	if !exists {
		limiter = &connectionLimiter{
			lastReset:    time.Now(),
			messageCount: 0,
		}
		rl.connections[connID] = limiter
	}
	rl.mu.Unlock()
	
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	
	// Reset counter if a second has passed
	now := time.Now()
	if now.Sub(limiter.lastReset) >= time.Second {
		limiter.messageCount = 0
		limiter.lastReset = now
	}
	
	// Check if we've exceeded the rate limit
	if limiter.messageCount >= rl.maxMessagesPerSecond {
		return false
	}
	
	// Allow the message
	limiter.messageCount++
	return true
}

// Remove removes a connection from rate limiting
func (rl *RateLimiter) Remove(connID string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.connections, connID)
}

// cleanup periodically removes old connection limiters
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.cleanupInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-rl.stopChan:
			return
		case <-ticker.C:
			rl.mu.Lock()
			now := time.Now()
			for connID, limiter := range rl.connections {
				limiter.mu.Lock()
				// Remove if no activity for 5 minutes
				if now.Sub(limiter.lastReset) > 5*time.Minute {
					delete(rl.connections, connID)
				}
				limiter.mu.Unlock()
			}
			rl.mu.Unlock()
		}
	}
}

// Stop stops the rate limiter
func (rl *RateLimiter) Stop() {
	close(rl.stopChan)
}

