CREATE TABLE IF NOT EXISTS shortlinks (
    id bigserial PRIMARY KEY,
    code text UNIQUE, --短链
    url  text UNIQUE NOT NULL, --原始URL
    created_at   timestamptz NOT NULL DEFAULT now(),  -- 带时区时间戳 
    updated_at   timestamptz NOT NULL DEFAULT now(),
    expires_at   timestamptz,
    disabled     boolean NOT NULL,
    redirect_type text NOT NULL DEFAULT '302' CHECK (redirect_type IN ('301','302'))
);