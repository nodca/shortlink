package httpapi

import (
	"net/http"
	"time"

	"day.local/gee"
	"day.local/internal/app/shortlink/repo"
	"day.local/internal/app/shortlink/stats"
	"day.local/internal/platform/auth"
	"day.local/internal/platform/httpmiddleware"
	"day.local/internal/platform/ratelimit"
)

// RegisterAPIRoutes 用于在给定的路由分组下挂载短链 API 路由（例如 /api/v1）。
//
// 约定：本包只做“传输层（transport）”工作；领域逻辑放在 internal/app/shortlink。
//

// 设计原因：
// - cmd/api 只负责"组装"和"挂载"，各业务模块自己提供 Register*Routes，避免路由散落在 main.go
// - API 路由一般用于机器调用（JSON），统一放在 /api/v1 下便于版本化
func RegisterAPIRoutes(api *gee.RouterGroup, slRepo *repo.ShortlinksRepo, usersRepo *repo.UsersRepo, ts auth.TokenService, limiter *ratelimit.Limiter) {
	//无需登录的路由
	api.Use(httpmiddleware.AuthOptional(ts))
	//创建短链 限流 10次/分钟
	api.POST("/shortlinks", httpmiddleware.RateLimit(limiter, "create", 10, time.Minute), NewCreateHandler(slRepo))
	api.GET("/shortlinks/:code", NewFindShortlinksHandler(slRepo))
	//注册 3次/分钟
	api.POST("/register", httpmiddleware.RateLimit(limiter, "register", 3, time.Minute), NewRegistUserHandler(usersRepo))
	//登录-  5次/分钟
	api.POST("/login", httpmiddleware.RateLimit(limiter, "login", 5, time.Minute), NewLoginHandler(usersRepo, ts))

	// 需要登录的路由
	users := api.Group("/users")
	users.Use(httpmiddleware.AuthRequired(ts))
	users.GET("/me", NewUserMeHandler())
	users.GET("/mine", NewMineHandler(slRepo))
	users.DELETE("/mine/:code", NewRemoveFromMineHandler(slRepo))
	users.GET("/shortlinks/:code/stats", NewGetStatsHandler(slRepo))

	//需要管理员的路由

	// 需要管理员的
	admin := api.Group("/admin")
	admin.Use(httpmiddleware.AuthRequired(ts), httpmiddleware.RequireRole("admin"))
	admin.GET("/ping", func(ctx *gee.Context) {
		ctx.String(http.StatusOK, "pong")
	})
	admin.POST("/shortlinks/:code/disable", NewDisablesHandler(slRepo))

}

// RegisterPublicRoutes 用于在根路由上挂载“公开短链”相关路由（例如 GET /r/:code）。
//
// 跳转入口刻意不放在 /api/v1 下，方便用户直接在浏览器输入短链 URL。
//
// 设计原因：
// - “短链”的使用体验是直接访问 /r/{code}，而不是 /api/v1/...
// - 将 public 与 api 分开，后续做域名拆分（s.example.com 与 api.example.com）更顺滑
func RegisterPublicRoutes(engine *gee.Engine, r *repo.ShortlinksRepo, collector stats.Collector, limiter *ratelimit.Limiter) {
	//跳转 100次/分钟
	engine.GET("/:code", httpmiddleware.RateLimit(limiter, "redirect", 100, time.Minute), NewRedirectHandler(r, collector))
}
