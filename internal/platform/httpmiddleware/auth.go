package httpmiddleware

import (
	"strings"

	"day.local/gee"
	"day.local/internal/platform/auth"
)

func AuthRequired(ts auth.TokenService) gee.HandlerFunc {
	return func(ctx *gee.Context) {
		tokenStr := ctx.Req.Header.Get("Authorization")
		if tokenStr == "" {
			ctx.AbortWithError(401, "lack Authorization")
			return
		}
		fields := strings.Fields(tokenStr)
		if len(fields) != 2 {
			ctx.AbortWithError(401, "format err")
			return
		}
		if !strings.EqualFold(fields[0], "Bearer") {
			ctx.AbortWithError(401, "format err")
			return
		}
		claim, err := ts.Verify(fields[1])
		if err != nil {
			ctx.AbortWithError(401, "Verify err")
			return
		}
		ctx.Req = ctx.Req.WithContext(auth.WithIdentity(ctx.Req.Context(), auth.Identity{
			UserID: claim.UserID,
			Role:   claim.Role}))

		ctx.Next()
	}
}

func AuthOptional(ts auth.TokenService) gee.HandlerFunc {
	return func(ctx *gee.Context) {
		tokenStr := ctx.Req.Header.Get("Authorization")
		if tokenStr == "" {
			ctx.Next()
			return
		}
		fields := strings.Fields(tokenStr)
		if len(fields) != 2 {
			ctx.Next()
			return
		}
		if !strings.EqualFold(fields[0], "Bearer") {
			ctx.Next()
			return
		}
		claim, err := ts.Verify(fields[1])
		if err != nil {
			ctx.Next()
			return
		}
		ctx.Req = ctx.Req.WithContext(auth.WithIdentity(ctx.Req.Context(), auth.Identity{
			UserID: claim.UserID,
			Role:   claim.Role}))

		ctx.Next()
	}
}

func RequireRole(role string) gee.HandlerFunc {
	return func(ctx *gee.Context) {
		id, ok := auth.GetIdentity(ctx.Req.Context())
		if !ok {
			ctx.AbortWithError(401, "unauthorized")
			return
		}
		if id.Role != role {
			ctx.AbortWithError(403, "forbidden")
			return
		}
		ctx.Next()
	}
}
