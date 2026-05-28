package ratelimit

import (
	"context"
	"time"

	"golang.org/x/time/rate"
)

// TokenBucket is a small general-purpose wrapper for process-local outbound
// rate limits such as third-party API calls.
type TokenBucket struct {
	limiter *rate.Limiter
}

func NewTokenBucket(every time.Duration, burst int) *TokenBucket {
	if every <= 0 {
		every = time.Second
	}
	if burst <= 0 {
		burst = 1
	}
	return &TokenBucket{limiter: rate.NewLimiter(rate.Every(every), burst)}
}

func (b *TokenBucket) Wait(ctx context.Context) error {
	return b.limiter.Wait(ctx)
}
