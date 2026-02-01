package httpmiddleware

import (
	"net/http"
	"strings"

	"day.local/gee"
	"day.local/internal/platform/auth"
)

// parseBearer 解析 Authorization header 中的 Bearer token
// 返回 token 字符串，如果格式不正确返回空字符串
func parseBearer(header string) string {
	fields := strings.Fields(header)
	if len(fields) != 2 || !strings.EqualFold(fields[0], "Bearer") {
		return ""
	}
	return fields[1]
}

// AuthRequired 要求请求必须携带有效的 JWT token
func AuthRequired(ts auth.TokenService) gee.HandlerFunc {
	return func(ctx *gee.Context) {
		tokenStr := ctx.Req.Header.Get("Authorization")
		if tokenStr == "" {
			ctx.AbortWithError(http.StatusUnauthorized, "missing authorization header")
			return
		}
		token := parseBearer(tokenStr)
		if token == "" {
			ctx.AbortWithError(http.StatusUnauthorized, "invalid authorization format")
			return
		}
		claim, err := ts.Verify(token)
		if err != nil {
			ctx.AbortWithError(http.StatusUnauthorized, "invalid token")
			return
		}
		ctx.Req = ctx.Req.WithContext(auth.WithIdentity(ctx.Req.Context(), auth.Identity{
			UserID: claim.UserID,
			Role:   claim.Role,
		}))
		ctx.Next()
	}
}

// AuthOptional 可选认证，有 token 则解析，无 token 或 token 无效则跳过
func AuthOptional(ts auth.TokenService) gee.HandlerFunc {
	return func(ctx *gee.Context) {
		tokenStr := ctx.Req.Header.Get("Authorization")
		if tokenStr == "" {
			ctx.Next()
			return
		}
		token := parseBearer(tokenStr)
		if token == "" {
			ctx.Next()
			return
		}
		claim, err := ts.Verify(token)
		if err != nil {
			ctx.Next()
			return
		}
		ctx.Req = ctx.Req.WithContext(auth.WithIdentity(ctx.Req.Context(), auth.Identity{
			UserID: claim.UserID,
			Role:   claim.Role,
		}))
		ctx.Next()
	}
}

// RequireRole 要求用户具有指定角色
func RequireRole(role string) gee.HandlerFunc {
	return func(ctx *gee.Context) {
		id, ok := auth.GetIdentity(ctx.Req.Context())
		if !ok {
			ctx.AbortWithError(http.StatusUnauthorized, "unauthorized")
			return
		}
		if id.Role != role {
			ctx.AbortWithError(http.StatusForbidden, "forbidden")
			return
		}
		ctx.Next()
	}
}
