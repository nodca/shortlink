-- Phase 3: memory system foundation (conversation + graph).
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Conversation history (persistent)
CREATE TABLE IF NOT EXISTS conversation_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID REFERENCES agents(id) ON DELETE CASCADE,
    session_id TEXT NOT NULL,
    role TEXT NOT NULL, -- user/assistant/tool
    content TEXT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_conversation_session ON conversation_history(session_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_conversation_agent_session ON conversation_history(agent_id, session_id, created_at DESC);

-- Entities (graph memory)
CREATE TABLE IF NOT EXISTS entities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID REFERENCES agents(id) ON DELETE CASCADE,
    session_id TEXT,
    name TEXT NOT NULL,
    type TEXT,
    properties JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_entities_agent_session ON entities(agent_id, session_id);

-- Entity relations (graph memory)
CREATE TABLE IF NOT EXISTS entity_relations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    from_entity_id UUID REFERENCES entities(id) ON DELETE CASCADE,
    to_entity_id UUID REFERENCES entities(id) ON DELETE CASCADE,
    relation_type TEXT NOT NULL,
    properties JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_relations_from ON entity_relations(from_entity_id);
CREATE INDEX IF NOT EXISTS idx_relations_to ON entity_relations(to_entity_id);

