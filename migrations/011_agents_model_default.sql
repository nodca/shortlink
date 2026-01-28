-- Align default model name with application default.
ALTER TABLE agents
    ALTER COLUMN model SET DEFAULT 'deepseek-ai/DeepSeek-V3.2';

