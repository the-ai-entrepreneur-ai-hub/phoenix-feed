package ratelimit

import (
	"sync"
	"testing"
	"time"

	"github.com/abusedmindset/phoenix-feed/internal/auth"
)

func TestLimiterFreeClientAllowsOnePerWindow(t *testing.T) {
	limiter := New(Config{FreeEvery: 10 * time.Minute, PaidEvery: 50 * time.Second, ManualEvery: 120 * time.Second})
	identity := auth.Identity{Tier: auth.TierFree, ClientID: "device-1", Anonymous: true}

	first := limiter.Allow(identity, ScopeIncidentRead)
	second := limiter.Allow(identity, ScopeIncidentRead)

	if !first.Allowed {
		t.Fatal("first request should be allowed")
	}
	if second.Allowed {
		t.Fatal("second request should be limited")
	}
	if second.RetryAfter <= 0 {
		t.Fatalf("RetryAfter = %s, want positive", second.RetryAfter)
	}
}

func TestLimiterPaidClientUsesSeparatePaidBucket(t *testing.T) {
	limiter := New(Config{FreeEvery: 10 * time.Minute, PaidEvery: 50 * time.Second, ManualEvery: 120 * time.Second})
	free := auth.Identity{Tier: auth.TierFree, ClientID: "same-device", Anonymous: true}
	paid := auth.Identity{Tier: auth.TierPaid, KeyHash: "paid-hash", ClientID: "same-device"}

	if !limiter.Allow(free, ScopeIncidentRead).Allowed {
		t.Fatal("free first request should be allowed")
	}
	if !limiter.Allow(paid, ScopeIncidentRead).Allowed {
		t.Fatal("paid first request should use a separate key bucket")
	}
	if limiter.Allow(paid, ScopeIncidentRead).Allowed {
		t.Fatal("paid second request should be limited")
	}
}

func TestLimiterConcurrentBurstOnlyAllowsOne(t *testing.T) {
	limiter := New(Config{FreeEvery: 10 * time.Minute, PaidEvery: 50 * time.Second, ManualEvery: 120 * time.Second})
	identity := auth.Identity{Tier: auth.TierFree, ClientID: "burst-device", Anonymous: true}

	const workers = 32
	var wg sync.WaitGroup
	allowed := make(chan bool, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			allowed <- limiter.Allow(identity, ScopeIncidentRead).Allowed
		}()
	}
	wg.Wait()
	close(allowed)

	count := 0
	for ok := range allowed {
		if ok {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("allowed count = %d, want 1", count)
	}
}
