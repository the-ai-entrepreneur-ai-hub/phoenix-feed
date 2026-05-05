package api

import (
	"math"
	"net/http"
	"strconv"

	"github.com/abusedmindset/phoenix-feed/internal/auth"
	"github.com/abusedmindset/phoenix-feed/internal/ratelimit"
)

func rateLimitMiddleware(limiter *ratelimit.Limiter, scope ratelimit.Scope) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identity := auth.IdentityFromContext(r.Context())
			decision := limiter.Allow(identity, scope)
			if !decision.Allowed {
				retrySeconds := int(math.Ceil(decision.RetryAfter.Seconds()))
				if retrySeconds < 1 {
					retrySeconds = 1
				}
				w.Header().Set("Retry-After", strconv.Itoa(retrySeconds))
				writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
