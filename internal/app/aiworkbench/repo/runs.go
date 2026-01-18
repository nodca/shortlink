package repo

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RunsRepo struct {
	db *pgxpool.Pool
}

func NewRunsRepo(db *pgxpool.Pool) *RunsRepo {
	return &RunsRepo{db: db}
}

type CreateResearchRunParams struct {
	UserID   int64
	APIKeyID int64
	Topic    string
	Sources  []string
	Language string
}

type Run struct {
	ID         int64      `json:"id"`
	UserID     int64      `json:"user_id"`
	APIKeyID   int64      `json:"api_key_id"`
	Status     string     `json:"status"`
	Topic      string     `json:"topic"`
	Sources    []string   `json:"sources"`
	Language   string     `json:"language"`
	ResultMD   *string    `json:"result_md,omitempty"`
	Error      *string    `json:"error,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	TokensUsed int        `json:"tokens_used"`
	CostUSD    *float64   `json:"cost_usd,omitempty"`
}

func (r *RunsRepo) CreateResearchRun(ctx context.Context, p CreateResearchRunParams) (int64, error) {
	srcBytes, _ := json.Marshal(p.Sources)

	dbctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var id int64
	err := r.db.QueryRow(dbctx, `
INSERT INTO ai_research_runs (user_id, api_key_id, status, topic, sources, language)
VALUES ($1,$2,'pending',$3,$4,$5)
RETURNING id`, p.UserID, p.APIKeyID, p.Topic, srcBytes, p.Language).Scan(&id)
	return id, err
}

func (r *RunsRepo) GetRunForUser(ctx context.Context, runID, userID int64) (Run, error) {
	dbctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var run Run
	var sourcesJSON []byte
	err := r.db.QueryRow(dbctx, `
SELECT id, user_id, api_key_id, status, topic, sources, language, result_md, error, created_at, started_at, finished_at, tokens_used, cost_usd
FROM ai_research_runs
WHERE id=$1 AND user_id=$2
LIMIT 1`, runID, userID).Scan(
		&run.ID, &run.UserID, &run.APIKeyID, &run.Status, &run.Topic, &sourcesJSON, &run.Language,
		&run.ResultMD, &run.Error, &run.CreatedAt, &run.StartedAt, &run.FinishedAt, &run.TokensUsed, &run.CostUSD,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Run{}, ErrNotFound
		}
		return Run{}, err
	}
	_ = json.Unmarshal(sourcesJSON, &run.Sources)
	return run, nil
}

func (r *RunsRepo) MarkRunning(ctx context.Context, runID int64) error {
	dbctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err := r.db.Exec(dbctx, `
UPDATE ai_research_runs
SET status='running', started_at = COALESCE(started_at, now())
WHERE id=$1 AND status IN ('pending','running')`, runID)
	return err
}

func (r *RunsRepo) MarkSucceeded(ctx context.Context, runID int64, resultMD string, tokensUsed int, costUSD float64) error {
	dbctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err := r.db.Exec(dbctx, `
UPDATE ai_research_runs
SET status='succeeded', result_md=$2, tokens_used=$3, cost_usd=$4, finished_at=now(), error=NULL
WHERE id=$1`, runID, resultMD, tokensUsed, costUSD)
	return err
}

func (r *RunsRepo) MarkFailed(ctx context.Context, runID int64, errMsg string) error {
	dbctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err := r.db.Exec(dbctx, `
UPDATE ai_research_runs
SET status='failed', error=$2, finished_at=now()
WHERE id=$1`, runID, errMsg)
	return err
}
