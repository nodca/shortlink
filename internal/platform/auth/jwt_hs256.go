package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type hs256Service struct {
	secret []byte
	issuer string
	ttl    time.Duration
}

func (h *hs256Service) Sign(userID string, role string) (string, error) {
	if userID == "" {
		return "", errors.New("empty user id")
	}
	now := time.Now()

	claims := jwtClaims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    h.issuer,
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(h.ttl)),
		},
	}

	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString(h.secret)
}

func (h *hs256Service) Verify(tokenString string) (Claims, error) {
	var parsed jwtClaims
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(h.issuer),
		jwt.WithExpirationRequired(),
	)
	_, err := parser.ParseWithClaims(tokenString, &parsed, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, errors.New("unexpected jwt signing method")
		}
		return h.secret, nil
	})
	if err != nil {
		return Claims{}, err
	}
	return Claims{
		UserID: parsed.Subject,
		Role:   parsed.Role,
	}, nil
}

