-- 启用 pgvector
CREATE EXTENSION IF NOT EXISTS vector;

-- 知识库
CREATE TABLE knowledge_bases (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    description TEXT,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

-- 文档（含向量）
CREATE TABLE documents (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kb_id       UUID REFERENCES knowledge_bases(id) ON DELETE CASCADE,
    title       TEXT,
    content     TEXT NOT NULL,           -- 原始内容
    chunk_index INT DEFAULT 0,           -- 分块索引
    embedding   vector(1024),            -- 向量
    metadata    JSONB DEFAULT '{}',
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

-- 向量索引（加速检索）
CREATE INDEX ON documents
USING ivfflat (embedding vector_cosine_ops)
WITH (lists = 100);