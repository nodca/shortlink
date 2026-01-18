-- AI Workbench (Research) - MVP tables

CREATE TABLE IF NOT EXISTS ai_api_keys (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users(id),
    name       TEXT NOT NULL,
    prefix     TEXT NOT NULL, -- first 8 chars of sha256 hash
    hash       TEXT NOT NULL, -- sha256 hex of full api key
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ai_api_keys_hash_unique ON ai_api_keys(hash);
CREATE INDEX IF NOT EXISTS idx_ai_api_keys_prefix ON ai_api_keys(prefix);
CREATE INDEX IF NOT EXISTS idx_ai_api_keys_user_id ON ai_api_keys(user_id);

CREATE TABLE IF NOT EXISTS ai_research_runs (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id),
    api_key_id  BIGINT NOT NULL REFERENCES ai_api_keys(id),
    status      TEXT NOT NULL CHECK (status IN ('pending','running','succeeded','failed')),
    topic       TEXT NOT NULL,
    sources     JSONB NOT NULL DEFAULT '[]'::jsonb,
    language    TEXT NOT NULL DEFAULT 'zh',
    result_md   TEXT,
    error       TEXT,
    tokens_used INT NOT NULL DEFAULT 0,
    cost_usd    NUMERIC(12,6) NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at  TIMESTAMPTZ,
    finished_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_ai_research_runs_user_id ON ai_research_runs(user_id);
CREATE INDEX IF NOT EXISTS idx_ai_research_runs_status ON ai_research_runs(status);
CREATE INDEX IF NOT EXISTS idx_ai_research_runs_created_at ON ai_research_runs(created_at);

