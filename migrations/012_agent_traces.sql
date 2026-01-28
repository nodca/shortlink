-- Agent observability: traces + spans
-- Requires pgcrypto for gen_random_uuid()
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS traces (
    trace_id    TEXT PRIMARY KEY,
    run_id      UUID REFERENCES agent_runs(id) ON DELETE CASCADE,
    agent_id    UUID REFERENCES agents(id) ON DELETE CASCADE,
    status      TEXT NOT NULL DEFAULT 'success', -- success/failed
    error       TEXT,
    started_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ,
    metadata    JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_traces_run_id ON traces(run_id);
CREATE INDEX IF NOT EXISTS idx_traces_agent_id ON traces(agent_id);
CREATE INDEX IF NOT EXISTS idx_traces_started_at ON traces(started_at DESC);

CREATE TABLE IF NOT EXISTS spans (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    trace_id      TEXT NOT NULL REFERENCES traces(trace_id) ON DELETE CASCADE,
    parent_id     UUID,
    name          TEXT NOT NULL,
    kind          TEXT NOT NULL, -- llm/tool/memory/workflow/other
    status        TEXT NOT NULL DEFAULT 'success',
    error         TEXT,
    started_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at   TIMESTAMPTZ,
    attributes    JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_spans_trace_id ON spans(trace_id);
CREATE INDEX IF NOT EXISTS idx_spans_started_at ON spans(started_at DESC);

