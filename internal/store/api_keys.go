package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

var ErrAPIKeyNotFound = errors.New("api key not found")

type APIKey struct {
	ID         int64
	KeyHash    string
	Tier       string
	Label      string
	CreatedAt  time.Time
	RevokedAt  *time.Time
	OwnerEmail string
}

type CreateAPIKeyParams struct {
	KeyHash    string
	Tier       string
	Label      string
	OwnerEmail string
}

// LookupAPIKey returns a non-revoked API key by SHA-256 hash.
func (s *Store) LookupAPIKey(ctx context.Context, keyHash string) (APIKey, error) {
	const q = `
		SELECT id, key_hash, tier, label, created_at, revoked_at, owner_email
		FROM api_keys
		WHERE key_hash = $1 AND revoked_at IS NULL`
	var key APIKey
	var revokedAt sql.NullTime
	var ownerEmail sql.NullString
	err := s.pool.QueryRow(ctx, q, keyHash).Scan(
		&key.ID, &key.KeyHash, &key.Tier, &key.Label, &key.CreatedAt, &revokedAt, &ownerEmail,
	)
	if err == pgx.ErrNoRows {
		return APIKey{}, ErrAPIKeyNotFound
	}
	if err != nil {
		return APIKey{}, fmt.Errorf("lookup api key: %w", err)
	}
	if revokedAt.Valid {
		key.RevokedAt = &revokedAt.Time
	}
	if ownerEmail.Valid {
		key.OwnerEmail = ownerEmail.String
	}
	return key, nil
}

// CreateAPIKey stores one hashed API key. Plaintext must never reach this layer.
func (s *Store) CreateAPIKey(ctx context.Context, params CreateAPIKeyParams) (APIKey, error) {
	const q = `
		INSERT INTO api_keys (key_hash, tier, label, owner_email)
		VALUES ($1, $2, $3, NULLIF($4, ''))
		RETURNING id, key_hash, tier, label, created_at, revoked_at, owner_email`
	var key APIKey
	var revokedAt sql.NullTime
	var ownerEmail sql.NullString
	err := s.pool.QueryRow(ctx, q, params.KeyHash, params.Tier, params.Label, params.OwnerEmail).Scan(
		&key.ID, &key.KeyHash, &key.Tier, &key.Label, &key.CreatedAt, &revokedAt, &ownerEmail,
	)
	if err != nil {
		return APIKey{}, fmt.Errorf("create api key: %w", err)
	}
	if revokedAt.Valid {
		key.RevokedAt = &revokedAt.Time
	}
	if ownerEmail.Valid {
		key.OwnerEmail = ownerEmail.String
	}
	return key, nil
}
