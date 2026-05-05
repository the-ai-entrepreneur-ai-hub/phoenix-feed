package api

import (
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/abusedmindset/phoenix-feed/internal/auth"
	"github.com/abusedmindset/phoenix-feed/internal/store"
)

func authMiddleware(st Store, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rawKey := strings.TrimSpace(r.Header.Get("X-API-Key"))
			if rawKey == "" {
				identity := auth.Identity{
					Tier:      auth.TierFree,
					ClientID:  requestClientID(r),
					Anonymous: true,
				}
				next.ServeHTTP(w, r.WithContext(auth.WithIdentity(r.Context(), identity)))
				return
			}

			keyHash := auth.HashKey(rawKey)
			key, err := st.LookupAPIKey(r.Context(), keyHash)
			if err != nil {
				if errors.Is(err, store.ErrAPIKeyNotFound) {
					writeError(w, http.StatusUnauthorized, "invalid api key")
					return
				}
				log.Error("lookup api key", "err", err)
				writeError(w, http.StatusInternalServerError, "lookup api key")
				return
			}
			tier, ok := auth.TierFromString(key.Tier)
			if !ok {
				log.Error("invalid api key tier", "key_id", key.ID, "tier", key.Tier)
				writeError(w, http.StatusInternalServerError, "invalid api key tier")
				return
			}

			identity := auth.Identity{
				Tier:      tier,
				KeyID:     key.ID,
				KeyHash:   keyHash,
				ClientID:  requestClientID(r),
				Anonymous: false,
			}
			next.ServeHTTP(w, r.WithContext(auth.WithIdentity(r.Context(), identity)))
		})
	}
}

func requestClientID(r *http.Request) string {
	if v := strings.TrimSpace(r.Header.Get("X-Client-ID")); v != "" {
		return v
	}
	if v := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); v != "" {
		if idx := strings.Index(v, ","); idx >= 0 {
			return strings.TrimSpace(v[:idx])
		}
		return v
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	if r.RemoteAddr != "" {
		return r.RemoteAddr
	}
	return "anonymous"
}
