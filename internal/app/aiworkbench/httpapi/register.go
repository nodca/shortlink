package httpapi

import (
	"time"

	"day.local/gee"
	"day.local/internal/app/aiworkbench/queue"
	"day.local/internal/app/aiworkbench/repo"
	"day.local/internal/platform/auth"
	"day.local/internal/platform/httpmiddleware"
	"day.local/internal/platform/ratelimit"
)

func RegisterAPIRoutes(api *gee.RouterGroup, runsRepo *repo.RunsRepo, keysRepo *repo.APIKeysRepo, q *queue.ResearchQueue, ts auth.TokenService, limiter *ratelimit.Limiter) {
	// Public (API key) endpoints.
	research := api.Group("")
	research.Use(APIKeyRequired(keysRepo))
	research.POST("/research", httpmiddleware.RateLimit(limiter, "research", 10, time.Minute), NewCreateResearchRunHandler(runsRepo, q))
	research.GET("/research/runs/:id", NewGetResearchRunHandler(runsRepo))

	// Authenticated (JWT) endpoints for managing API keys.
	keys := api.Group("")
	keys.Use(httpmiddleware.AuthRequired(ts))
	keys.POST("/api-keys", httpmiddleware.RateLimit(limiter, "api_keys_create", 10, time.Minute), NewCreateAPIKeyHandler(keysRepo))
	keys.GET("/api-keys", NewListAPIKeysHandler(keysRepo))
	keys.DELETE("/api-keys/:id", NewRevokeAPIKeyHandler(keysRepo))
}

