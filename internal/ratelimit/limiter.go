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
	ScopeAdminDispatch Scope = "admin_dispatch"
)

type Config struct {
	FreeEvery          time.Duration
	PaidEvery          time.Duration
	ManualEvery        time.Duration
	AdminDispatchEvery time.Duration
	AdminDispatchBurst int
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
	if cfg.AdminDispatchEvery <= 0 {
		cfg.AdminDispatchEvery = time.Minute / 60
	}
	if cfg.AdminDispatchBurst <= 0 {
		cfg.AdminDispatchBurst = 60
	}
	return &Limiter{buckets: map[string]*rate.Limiter{}, cfg: cfg}
}

func NewDefault() *Limiter {
	return New(Config{})
}

func (l *Limiter) Allow(identity auth.Identity, scope Scope) Decision {
	every, burst := l.settings(identity, scope)
	key := bucketKey(identity, scope)

	l.mu.Lock()
	bucket, ok := l.buckets[key]
	if !ok {
		bucket = rate.NewLimiter(rate.Every(every), burst)
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
	every, _ := l.settings(identity, scope)
	return every
}

func (l *Limiter) settings(identity auth.Identity, scope Scope) (time.Duration, int) {
	if scope == ScopeAdminDispatch {
		return l.cfg.AdminDispatchEvery, l.cfg.AdminDispatchBurst
	}
	if scope == ScopeManualRefresh {
		return l.cfg.ManualEvery, 1
	}
	if identity.Tier == auth.TierPaid {
		return l.cfg.PaidEvery, 1
	}
	return l.cfg.FreeEvery, 1
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
