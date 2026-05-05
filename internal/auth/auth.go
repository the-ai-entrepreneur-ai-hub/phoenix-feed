// Package auth contains API key hashing and per-request identity helpers.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

type Tier string

const (
	TierFree Tier = "free"
	TierPaid Tier = "paid"
)

type Identity struct {
	Tier      Tier
	KeyID     int64
	KeyHash   string
	ClientID  string
	Anonymous bool
}

type contextKey struct{}

// HashKey returns the SHA-256 hex digest stored in api_keys.key_hash.
func HashKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

// GenerateKey returns a URL-safe random API key. The caller prints it once
// and stores only HashKey(key).
func GenerateKey() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "phx_" + base64.RawURLEncoding.EncodeToString(buf), nil
}

func TierFromString(raw string) (Tier, bool) {
	switch raw {
	case string(TierFree):
		return TierFree, true
	case string(TierPaid):
		return TierPaid, true
	default:
		return "", false
	}
}

func WithIdentity(ctx context.Context, identity Identity) context.Context {
	return context.WithValue(ctx, contextKey{}, identity)
}

func IdentityFromContext(ctx context.Context) Identity {
	identity, ok := ctx.Value(contextKey{}).(Identity)
	if !ok {
		return Identity{Tier: TierFree, Anonymous: true}
	}
	return identity
}
