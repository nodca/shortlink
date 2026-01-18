CREATE TABLE IF NOT EXISTS users (
    id            bigserial PRIMARY KEY,        -- bigserial:自增整数,底层是 bigint + 一个自增序列 
    username      text      NOT NULL UNIQUE,    -- text字符串类型，不限长度 
    password_hash text      NOT NULL,
    role          text      NOT NULL CHECK (role in ('admin','user')),
    created_at   timestamptz NOT NULL DEFAULT now(),  -- 带时区时间戳 
    updated_at   timestamptz NOT NULL DEFAULT now()
);