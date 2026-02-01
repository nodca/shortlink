package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"day.local/gee"
	"day.local/internal/app/shortlink"
	"day.local/internal/app/shortlink/repo"
	"day.local/internal/app/shortlink/stats"
	"day.local/internal/platform/httpmiddleware"
	"day.local/internal/platform/metrics"
)

// NOTE: 本包目前是短链 MVP handlers 的占位。
// 当你实现 MVP 时，建议：
// - request/response 结构体放在这里（传输层）
// - 领域逻辑放在 internal/app/shortlink
// - SQL 访问放在 internal/app/shortlink/repo（短链业务私有）
//
// 设计原因（为什么要单独一个 httpapi 包）：
// - 让领域层（internal/app/shortlink）不依赖 HTTP 框架（gee），更容易测试与复用
// - handler 只做“翻译”：HTTP <-> 领域（参数校验、错误映射、响应格式），避免堆业务
// - 未来加 blog 时也可以遵循同样模式：internal/app/blog/httpapi + internal/app/blog

type ShortLinksRequest struct {
	URL      string `json:"url"`
	ExpireIn string `json:"expire_in,omitempty"`
	Code     string `json:"code,omitempty"`
}

type ShortLinksResponse struct {
	Code     string `json:"code"`
	ShortURL string `json:"short_url"`
	URL      string `json:"url"`
}

func NewCreateHandler(r *repo.ShortlinksRepo) gee.HandlerFunc {
	return func(ctx *gee.Context) {
		var req ShortLinksRequest
		if err := ctx.BindJSON(&req); err != nil {
			return
		}
		if err := shortlink.ValidateURL(req.URL); err != nil {
			ctx.AbortWithError(http.StatusBadRequest, err.Error())
			return
		}
		customCode := strings.TrimSpace(req.Code)
		if customCode != "" {
			if err := shortlink.ValidateCode(customCode); err != nil {
				ctx.AbortWithError(http.StatusBadRequest, err.Error())
				return
			}
		}

		userID, ok := tryGetUserID(ctx)
		if !ok {
			return
		}

		var code string
		var err error
		if customCode != "" {
			code, err = r.CreateWithCustomCode(ctx.Req.Context(), req.URL, customCode, userID)
			if err != nil {
				if errors.Is(err, repo.ErrShortlinkCodeAlreadyExists) || errors.Is(err, repo.ErrShortlinkURLAlreadyHasDifferentCode) {
					ctx.AbortWithError(http.StatusConflict, err.Error())
					return
				}
				ctx.AbortWithError(http.StatusInternalServerError, "shortlink create failed")
				return
			}
		} else {
			code, err = r.Create(ctx.Req.Context(), req.URL, userID)
			if err != nil {
				ctx.AbortWithError(http.StatusInternalServerError, "shortlink create failed")
				return
			}
		}

		path := "/" + code
		scheme := ctx.Req.Header.Get("X-Forwarded-Proto")
		if scheme == "" {
			scheme = "http"
		}
		shortURL := path
		if host := ctx.Req.Host; host != "" {
			shortURL = scheme + "://" + host + path
		}

		ctx.JSON(http.StatusOK, ShortLinksResponse{
			Code:     code,
			ShortURL: shortURL,
			URL:      req.URL,
		})
	}
}

func NewRedirectHandler(r *repo.ShortlinksRepo, collector stats.Collector) gee.HandlerFunc {
	return func(ctx *gee.Context) {
		code := ctx.Param("code")
		url := r.Resolve(ctx.Req.Context(), code)
		if url == "" {
			ctx.AbortWithError(http.StatusNotFound, "url not found")
			return
		}
		// 记录跳转
		metrics.ShortlinkRedirects.Inc()

		//异步记录点击
		collector.Collect(stats.ClickEvent{
			Code:      code,
			ClickedAt: time.Now(),
			IP:        httpmiddleware.ClientIP(ctx.Req),
			UserAgent: ctx.Req.UserAgent(),
			Referer:   ctx.Req.Referer(),
		})

		ctx.SetHeader("Location", url)
		ctx.Status(http.StatusFound)
	}
}

func NewFindShortlinksHandler(r *repo.ShortlinksRepo) gee.HandlerFunc {
	return func(ctx *gee.Context) {
		code := ctx.Param("code")
		data, err := r.FindByCode(ctx.Req.Context(), code)
		if err != nil {
			if errors.Is(err, repo.ErrShortlinkNotFound) {
				ctx.AbortWithError(http.StatusNotFound, err.Error())
				return
			}
			ctx.AbortWithError(http.StatusInternalServerError, err.Error())
			return
		}
		ctx.JSON(http.StatusOK, data)
	}
}

func NewDisablesHandler(r *repo.ShortlinksRepo) gee.HandlerFunc {
	return func(ctx *gee.Context) {
		code := ctx.Param("code")
		err := r.DisableByCode(ctx.Req.Context(), code)
		if err != nil {
			if errors.Is(err, repo.ErrShortlinkNotFound) {
				ctx.AbortWithError(http.StatusNotFound, err.Error())
				return
			}
			if errors.Is(err, repo.ErrAlreadyDisabled) {
				ctx.AbortWithError(http.StatusConflict, err.Error())
				return
			}
			ctx.AbortWithError(http.StatusInternalServerError, err.Error())
			return
		}
		ctx.Status(http.StatusOK)
	}
}

func NewRemoveFromMineHandler(r *repo.ShortlinksRepo) gee.HandlerFunc {
	return func(ctx *gee.Context) {
		code := ctx.Param("code")
		userID, ok := mustGetUserID(ctx)
		if !ok {
			return
		}
		if err := r.RemoveFromUserList(ctx.Req.Context(), userID, code); err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, "internal error")
			return
		}
		ctx.Status(http.StatusOK)
	}
}
