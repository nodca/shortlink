package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"day.local/gee"
	"day.local/internal/app/shortlink/repo"
	"day.local/internal/platform/auth"
	"golang.org/x/crypto/bcrypt"
)

type UserRegistRequest struct {
	UserName string `json:"username"`
	PassWord string `json:"password"`
}

type UserRegistResponse struct {
	Id       int64  `json:"id"`
	UserName string `json:"username"`
}

func NewRegistUserHandler(r *repo.UsersRepo) gee.HandlerFunc {
	return func(ctx *gee.Context) {
		var req UserRegistRequest
		if err := ctx.BindJSON(&req); err != nil {
			slog.Error(err.Error())
			ctx.AbortWithError(http.StatusInternalServerError, err.Error())
			return
		}
		userID, err := r.RegistUser(ctx.Req.Context(), req.UserName, req.PassWord)
		res := UserRegistResponse{
			Id:       userID,
			UserName: req.UserName,
		}
		if err != nil {
			if errors.Is(err, repo.ErrUserAlreadyExists) {
				ctx.AbortWithError(http.StatusConflict, err.Error())
			} else if errors.Is(err, repo.ErrInvalidPassword) || errors.Is(err, repo.ErrInvalidUsername) {
				ctx.AbortWithError(http.StatusBadRequest, err.Error())
			} else {
				ctx.AbortWithError(http.StatusInternalServerError, err.Error())
			}
			return
		}
		ctx.JSON(http.StatusCreated, res)
	}
}

type LoginRequest struct {
	UserName string `json:"username"`
	Password string `json:"password"`
}

func NewLoginHandler(usersRepo *repo.UsersRepo, ts auth.TokenService) gee.HandlerFunc {
	return func(ctx *gee.Context) {
		var req LoginRequest
		if err := ctx.BindJSON(&req); err != nil {
			return
		}
		dbctx, cancel := context.WithTimeout(ctx.Req.Context(), 1*time.Second)
		defer cancel()
		user, err := usersRepo.FindByUsername(dbctx, req.UserName)
		if err != nil {
			if errors.Is(err, repo.ErrUserNotFound) {
				ctx.AbortWithError(http.StatusUnauthorized, "invalid credentials")
				return
			}
			slog.Error("find user failed", "err", err)
			ctx.AbortWithError(http.StatusInternalServerError, "internal error")
			return
		}
		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
			ctx.AbortWithError(http.StatusUnauthorized, "invalid credentials")
			return
		}

		token, err := ts.Sign(strconv.FormatInt(user.ID, 10), user.Role)
		if err != nil {
			ctx.AbortWithError(http.StatusBadGateway, "sign failed")
			return
		}
		ctx.JSON(http.StatusOK, map[string]string{"token": token})
	}
}

func NewUserMeHandler() gee.HandlerFunc {
	return func(ctx *gee.Context) {
		id, ok := auth.GetIdentity(ctx.Req.Context())
		if !ok {
			ctx.AbortWithError(http.StatusInternalServerError, "missing identity")
			return
		}
		ctx.JSON(http.StatusOK, map[string]string{
			"user_id": id.UserID,
			"role":    id.Role,
		})
	}
}

func NewMineHandler(r *repo.ShortlinksRepo) gee.HandlerFunc {
	return func(ctx *gee.Context) {
		userID, ok := mustGetUserID(ctx)
		if !ok {
			return
		}
		list, err := r.ListByUserID(ctx.Req.Context(), userID, 50)
		if err != nil {
			slog.Error("list user shortlinks failed", "user_id", userID, "err", err)
			ctx.AbortWithError(http.StatusInternalServerError, "internal error")
			return
		}
		ctx.JSON(http.StatusOK, list)
	}
}

func NewGetStatsHandler(r *repo.ShortlinksRepo) gee.HandlerFunc {
	return func(ctx *gee.Context) {
		code := ctx.Param("code")
		userID, ok := mustGetUserID(ctx)
		if !ok {
			return
		}
		// 检查权限
		owns, err := r.UserOwnsShortlink(ctx.Req.Context(), userID, code)
		if err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, "internal error")
			return
		}
		if !owns {
			ctx.AbortWithError(http.StatusForbidden, "no permission")
			return
		}

		limit := 20
		if l := ctx.Query("limit"); l != "" {
			if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 100 {
				limit = n
			} else {
				ctx.AbortWithError(http.StatusBadRequest, "invalid limit")
				return
			}
		}
		var cursor int64 = 0
		if c := ctx.Query("cursor"); c != "" {
			if n, err := strconv.ParseInt(c, 10, 64); err == nil && n > 0 {
				cursor = n
			} else {
				ctx.AbortWithError(http.StatusBadRequest, "invalid cursor")
				return
			}
		}

		stats, err := r.ListStatsByCode(ctx.Req.Context(), code, limit, cursor)
		if err != nil {
			slog.Error("list stats failed", "code", code, "err", err)
			ctx.AbortWithError(http.StatusInternalServerError, "internal error")
			return
		}
		ctx.JSON(http.StatusOK, stats)
	}
}
