package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"day.local/gee"
	"day.local/internal/app/aiworkbench/repo"
	"day.local/internal/platform/auth"
)

type CreateAPIKeyRequest struct {
	Name string `json:"name"`
}

func NewCreateAPIKeyHandler(keysRepo *repo.APIKeysRepo) gee.HandlerFunc {
	return func(ctx *gee.Context) {
		id, ok := auth.GetIdentity(ctx.Req.Context())
		if !ok {
			ctx.AbortWithError(http.StatusUnauthorized, "missing identity")
			return
		}
		userID, err := strconv.ParseInt(id.UserID, 10, 64)
		if err != nil || userID <= 0 {
			ctx.AbortWithError(http.StatusUnauthorized, "invalid identity")
			return
		}

		var req CreateAPIKeyRequest
		if err := ctx.BindJSON(&req); err != nil {
			ctx.AbortWithError(http.StatusBadRequest, "invalid json")
			return
		}
		req.Name = strings.TrimSpace(req.Name)
		if req.Name == "" || len(req.Name) > 64 {
			ctx.AbortWithError(http.StatusBadRequest, "invalid name")
			return
		}

		key, row, err := keysRepo.Create(ctx.Req.Context(), userID, req.Name)
		if err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, "create api key failed")
			return
		}

		// Only returned once.
		ctx.JSON(http.StatusCreated, map[string]any{
			"id":         row.ID,
			"name":       row.Name,
			"prefix":     row.Prefix,
			"created_at": row.CreatedAt,
			"api_key":    key,
		})
	}
}

func NewListAPIKeysHandler(keysRepo *repo.APIKeysRepo) gee.HandlerFunc {
	return func(ctx *gee.Context) {
		id, ok := auth.GetIdentity(ctx.Req.Context())
		if !ok {
			ctx.AbortWithError(http.StatusUnauthorized, "missing identity")
			return
		}
		userID, err := strconv.ParseInt(id.UserID, 10, 64)
		if err != nil || userID <= 0 {
			ctx.AbortWithError(http.StatusUnauthorized, "invalid identity")
			return
		}

		rows, err := keysRepo.List(ctx.Req.Context(), userID, 100)
		if err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, "list failed")
			return
		}
		ctx.JSON(http.StatusOK, rows)
	}
}

func NewRevokeAPIKeyHandler(keysRepo *repo.APIKeysRepo) gee.HandlerFunc {
	return func(ctx *gee.Context) {
		id, ok := auth.GetIdentity(ctx.Req.Context())
		if !ok {
			ctx.AbortWithError(http.StatusUnauthorized, "missing identity")
			return
		}
		userID, err := strconv.ParseInt(id.UserID, 10, 64)
		if err != nil || userID <= 0 {
			ctx.AbortWithError(http.StatusUnauthorized, "invalid identity")
			return
		}

		keyID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
		if err != nil || keyID <= 0 {
			ctx.AbortWithError(http.StatusBadRequest, "invalid id")
			return
		}

		err = keysRepo.Revoke(ctx.Req.Context(), userID, keyID)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				ctx.AbortWithError(http.StatusNotFound, "not found")
				return
			}
			ctx.AbortWithError(http.StatusInternalServerError, "revoke failed")
			return
		}
		ctx.JSON(http.StatusOK, map[string]any{"ok": true})
	}
}

