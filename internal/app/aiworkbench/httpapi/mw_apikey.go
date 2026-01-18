package httpapi

import (
	"context"
	"net/http"
	"strings"
	"time"

	"day.local/gee"
	appauth "day.local/internal/app/aiworkbench/auth"
	"day.local/internal/app/aiworkbench/repo"
)

func APIKeyRequired(keysRepo *repo.APIKeysRepo) gee.HandlerFunc {
	return func(ctx *gee.Context) {
		key := strings.TrimSpace(ctx.Req.Header.Get("X-API-Key"))
		if key == "" {
			ctx.AbortWithError(http.StatusUnauthorized, "missing X-API-Key")
			return
		}

		verCtx, cancel := context.WithTimeout(ctx.Req.Context(), 200*time.Millisecond)
		defer cancel()

		id, err := keysRepo.Verify(verCtx, key)
		if err != nil {
			ctx.AbortWithError(http.StatusUnauthorized, "invalid api key")
			return
		}

		ctx.Req = ctx.Req.WithContext(appauth.WithIdentity(ctx.Req.Context(), appauth.Identity{
			UserID:   id.UserID,
			APIKeyID: id.APIKeyID,
		}))
		ctx.Next()
	}
}
