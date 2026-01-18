CREATE TABLE IF NOT EXISTS click_stats (
    id BIGSERIAL PRIMARY KEY,
    code TEXT NOT NULL,
    clicked_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ip TEXT,
    user_agent TEXT,
    referer TEXT
);

-- 索引：按短链 code 查询
CREATE INDEX IF NOT EXISTS idx_click_stats_code ON click_stats(code);
-- 索引：按时间查询（用于统计报表）
CREATE INDEX IF NOT EXISTS idx_click_stats_clicked_at ON click_stats(clicked_at);
-- 在 shortlinks 表加计数器字段（用于快速查询总数)
ALTER TABLE shortlinks ADD COLUMN IF NOT EXISTS click_count BIGINT DEFAULT 0;
