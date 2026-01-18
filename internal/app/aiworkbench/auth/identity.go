package auth

import "context"

type Identity struct {
	UserID   int64
	APIKeyID int64
}

type ctxKey struct{}

func WithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

func GetIdentity(ctx context.Context) (Identity, bool) {
	v := ctx.Value(ctxKey{})
	id, ok := v.(Identity)
	return id, ok
}

