// Package ratelimit contains in-memory per-client token buckets.
package ratelimit

import (
	"fmt"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/abusedmindset/phoenix-feed/internal/auth"
)

type Scope string

const (
	ScopeIncidentRead  Scope = "incident_read"
	ScopeManualRefresh Scope = "manual_refresh"
)

type Config struct {
	FreeEvery   time.Duration
	PaidEvery   time.Duration
	ManualEvery time.Duration
}

type Decision struct {
	Allowed      bool
	RetryAfter   time.Duration
	RefreshEvery time.Duration
}

type Limiter struct {
	mu      sync.Mutex
	buckets map[string]*rate.Limiter
	cfg     Config
}

func New(cfg Config) *Limiter {
	if cfg.FreeEvery <= 0 {
		cfg.FreeEvery = 10 * time.Minute
	}
	if cfg.PaidEvery <= 0 {
		cfg.PaidEvery = 50 * time.Second
	}
	if cfg.ManualEvery <= 0 {
		cfg.ManualEvery = 120 * time.Second
	}
	return &Limiter{buckets: map[string]*rate.Limiter{}, cfg: cfg}
}

func NewDefault() *Limiter {
	return New(Config{})
}

func (l *Limiter) Allow(identity auth.Identity, scope Scope) Decision {
	every := l.refreshEvery(identity, scope)
	key := bucketKey(identity, scope)

	l.mu.Lock()
	bucket, ok := l.buckets[key]
	if !ok {
		bucket = rate.NewLimiter(rate.Every(every), 1)
		l.buckets[key] = bucket
	}
	l.mu.Unlock()

	if bucket.Allow() {
		return Decision{Allowed: true, RefreshEvery: every}
	}
	return Decision{Allowed: false, RetryAfter: every, RefreshEvery: every}
}

func (l *Limiter) RefreshEvery(identity auth.Identity, scope Scope) time.Duration {
	return l.refreshEvery(identity, scope)
}

func (l *Limiter) refreshEvery(identity auth.Identity, scope Scope) time.Duration {
	if scope == ScopeManualRefresh {
		return l.cfg.ManualEvery
	}
	if identity.Tier == auth.TierPaid {
		return l.cfg.PaidEvery
	}
	return l.cfg.FreeEvery
}

func bucketKey(identity auth.Identity, scope Scope) string {
	identifier := identity.ClientID
	if identity.KeyHash != "" {
		identifier = "key:" + identity.KeyHash
	} else {
		identifier = "anon:" + identifier
	}
	return fmt.Sprintf("%s:%s:%s", scope, identity.Tier, identifier)
}
