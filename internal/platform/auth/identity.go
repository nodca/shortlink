package auth

import "context"

type Identity struct {
	UserID string
	Role   string
}

type identityKey struct{}

func WithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, identityKey{}, id)
}

func GetIdentity(ctx context.Context) (Identity, bool) {
	v := ctx.Value(identityKey{})
	id, ok := v.(Identity)
	return id, ok
}
