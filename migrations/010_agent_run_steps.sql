-- Agent runs: store steps separately for replay/debugging.
ALTER TABLE agent_runs
    ADD COLUMN IF NOT EXISTS steps JSONB DEFAULT '[]'::jsonb;

