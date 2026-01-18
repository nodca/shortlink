package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID string
	Role   string
}

type jwtClaims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

type TokenService interface {
	Sign(userID string, role string) (string, error)
	Verify(token string) (Claims, error)
}

func NewHS256Service(secret, issuer string, ttl time.Duration) (TokenService, error) {
	if secret == "" {
		return nil, errors.New("jwt secret is empty")
	}
	if issuer == "" {
		return nil, errors.New("jwt issuer is empty")
	}
	if ttl <= 0 {
		return nil, errors.New("jwt ttl must be > 0")
	}
	return &hs256Service{
		secret: []byte(secret),
		issuer: issuer,
		ttl:    ttl,
	}, nil
}
