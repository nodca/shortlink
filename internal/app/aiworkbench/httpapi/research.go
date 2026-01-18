package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"day.local/gee"
	appauth "day.local/internal/app/aiworkbench/auth"
	"day.local/internal/app/aiworkbench/queue"
	"day.local/internal/app/aiworkbench/repo"
)

type CreateResearchRunRequest struct {
	Topic    string   `json:"topic"`
	Sources  []string `json:"sources"`
	Language string   `json:"language"`
}

type CreateResearchRunResponse struct {
	RunID int64 `json:"run_id"`
}

func NewCreateResearchRunHandler(runsRepo *repo.RunsRepo, q *queue.ResearchQueue) gee.HandlerFunc {
	return func(ctx *gee.Context) {
		id, ok := appauth.GetIdentity(ctx.Req.Context())
		if !ok {
			ctx.AbortWithError(http.StatusUnauthorized, "missing identity")
			return
		}

		var req CreateResearchRunRequest
		if err := ctx.BindJSON(&req); err != nil {
			ctx.AbortWithError(http.StatusBadRequest, "invalid json")
			return
		}
		req.Topic = strings.TrimSpace(req.Topic)
		if req.Topic == "" || len(req.Topic) > 200 {
			ctx.AbortWithError(http.StatusBadRequest, "invalid topic")
			return
		}
		if req.Language == "" {
			req.Language = "zh"
		}

		runID, err := runsRepo.CreateResearchRun(ctx.Req.Context(), repo.CreateResearchRunParams{
			UserID:   id.UserID,
			APIKeyID: id.APIKeyID,
			Topic:    req.Topic,
			Sources:  req.Sources,
			Language: req.Language,
		})
		if err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, "create run failed")
			return
		}
		if err := q.Enqueue(ctx.Req.Context(), runID); err != nil {
			_ = runsRepo.MarkFailed(ctx.Req.Context(), runID, "enqueue failed")
			ctx.AbortWithError(http.StatusBadGateway, "enqueue failed")
			return
		}

		ctx.JSON(http.StatusAccepted, CreateResearchRunResponse{RunID: runID})
	}
}

func NewGetResearchRunHandler(runsRepo *repo.RunsRepo) gee.HandlerFunc {
	return func(ctx *gee.Context) {
		id, ok := appauth.GetIdentity(ctx.Req.Context())
		if !ok {
			ctx.AbortWithError(http.StatusUnauthorized, "missing identity")
			return
		}

		runID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
		if err != nil || runID <= 0 {
			ctx.AbortWithError(http.StatusBadRequest, "invalid id")
			return
		}

		run, err := runsRepo.GetRunForUser(ctx.Req.Context(), runID, id.UserID)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				ctx.AbortWithError(http.StatusNotFound, "not found")
				return
			}
			ctx.AbortWithError(http.StatusInternalServerError, "query failed")
			return
		}
		ctx.JSON(http.StatusOK, run)
	}
}
