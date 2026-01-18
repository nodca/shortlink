package worker

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"day.local/internal/app/aiworkbench/queue"
	"day.local/internal/app/aiworkbench/repo"
)

type ResearchWorker struct {
	q        *queue.ResearchQueue
	runsRepo *repo.RunsRepo
}

func NewResearchWorker(q *queue.ResearchQueue, runsRepo *repo.RunsRepo) *ResearchWorker {
	return &ResearchWorker{q: q, runsRepo: runsRepo}
}

func (w *ResearchWorker) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		jobs, err := w.q.Read(ctx, 2*time.Second)
		if err != nil {
			slog.Error("research worker: read failed", "err", err)
			time.Sleep(200 * time.Millisecond)
			continue
		}
		if len(jobs) == 0 {
			continue
		}

		for _, job := range jobs {
			if err := w.handle(ctx, job); err != nil {
				slog.Error("research worker: handle failed", "err", err, "run_id", job.RunID)
			}
			_ = w.q.Ack(ctx, job.MessageID)
		}
	}
}

func (w *ResearchWorker) handle(ctx context.Context, job queue.ResearchJob) error {
	// MVP: only marks succeeded with a placeholder. Replace with real pipeline:
	// WebSearch -> WebScrape -> LLM summarize -> LLM report.
	if err := w.runsRepo.MarkRunning(ctx, job.RunID); err != nil {
		return err
	}
	result := fmt.Sprintf("# Research Run %d\n\n- Status: placeholder (pipeline not implemented yet)\n", job.RunID)
	if err := w.runsRepo.MarkSucceeded(ctx, job.RunID, result, 0, 0); err != nil {
		return err
	}
	return nil
}
