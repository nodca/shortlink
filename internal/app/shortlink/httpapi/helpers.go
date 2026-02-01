package httpapi

import (
	"net/http"
	"strconv"

	"day.local/gee"
	"day.local/internal/platform/auth"
)

// mustGetUserID 从上下文中获取用户ID，失败时返回错误响应
// 返回 userID 和是否成功，失败时已写入错误响应
func mustGetUserID(ctx *gee.Context) (int64, bool) {
	identity, ok := auth.GetIdentity(ctx.Req.Context())
	if !ok {
		ctx.AbortWithError(http.StatusUnauthorized, "not login")
		return 0, false
	}
	userID, err := strconv.ParseInt(identity.UserID, 10, 64)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, "invalid user id")
		return 0, false
	}
	return userID, true
}

// tryGetUserID 尝试从上下文中获取用户ID（可选认证场景）
// 返回 userID 指针，未登录时返回 nil
func tryGetUserID(ctx *gee.Context) (*int64, bool) {
	identity, ok := auth.GetIdentity(ctx.Req.Context())
	if !ok {
		return nil, true
	}
	userID, err := strconv.ParseInt(identity.UserID, 10, 64)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, "invalid user id")
		return nil, false
	}
	return &userID, true
}
