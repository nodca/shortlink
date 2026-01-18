package repo

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type APIKeysRepo struct {
	db *pgxpool.Pool
}

func NewAPIKeysRepo(db *pgxpool.Pool) *APIKeysRepo {
	return &APIKeysRepo{db: db}
}

type APIKeyRow struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Name      string    `json:"name"`
	Prefix    string    `json:"prefix"`
	CreatedAt time.Time `json:"created_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
}

type APIKeyIdentity struct {
	UserID   int64
	APIKeyID int64
}

func (r *APIKeysRepo) Create(ctx context.Context, userID int64, name string) (plain string, row APIKeyRow, err error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", APIKeyRow{}, errors.New("empty name")
	}

	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", APIKeyRow{}, err
	}
	plain = "daysk_" + base64.RawURLEncoding.EncodeToString(secret)

	sum := sha256.Sum256([]byte(plain))
	hashHex := hex.EncodeToString(sum[:])
	prefix := hashHex[:8]

	dbctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	err = r.db.QueryRow(dbctx, `
INSERT INTO ai_api_keys (user_id, name, prefix, hash)
VALUES ($1,$2,$3,$4)
RETURNING id, user_id, name, prefix, created_at, revoked_at
`, userID, name, prefix, hashHex).Scan(&row.ID, &row.UserID, &row.Name, &row.Prefix, &row.CreatedAt, &row.RevokedAt)
	if err != nil {
		return "", APIKeyRow{}, err
	}
	return plain, row, nil
}

func (r *APIKeysRepo) List(ctx context.Context, userID int64, limit int) ([]APIKeyRow, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	dbctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	rows, err := r.db.Query(dbctx, `
SELECT id, user_id, name, prefix, created_at, revoked_at
FROM ai_api_keys
WHERE user_id=$1
ORDER BY id DESC
LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []APIKeyRow
	for rows.Next() {
		var row APIKeyRow
		if err := rows.Scan(&row.ID, &row.UserID, &row.Name, &row.Prefix, &row.CreatedAt, &row.RevokedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *APIKeysRepo) Revoke(ctx context.Context, userID, keyID int64) error {
	dbctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	cmd, err := r.db.Exec(dbctx, `
UPDATE ai_api_keys
SET revoked_at = now()
WHERE id=$1 AND user_id=$2 AND revoked_at IS NULL
`, keyID, userID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *APIKeysRepo) Verify(ctx context.Context, apiKey string) (APIKeyIdentity, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" || len(apiKey) > 256 {
		return APIKeyIdentity{}, ErrNotFound
	}
	sum := sha256.Sum256([]byte(apiKey))
	hashHex := hex.EncodeToString(sum[:])
	prefix := hashHex[:8]

	dbctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	var out APIKeyIdentity
	err := r.db.QueryRow(dbctx, `
SELECT id, user_id
FROM ai_api_keys
WHERE prefix=$1 AND hash=$2 AND revoked_at IS NULL
LIMIT 1`, prefix, hashHex).Scan(&out.APIKeyID, &out.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return APIKeyIdentity{}, ErrNotFound
		}
		return APIKeyIdentity{}, err
	}
	return out, nil
}

