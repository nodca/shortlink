CREATE TABLE IF NOT EXISTS user_shortlinks (
    user_id      BIGINT NOT NULL REFERENCES users(id),
    shortlink_id BIGINT NOT NULL REFERENCES shortlinks(id),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, shortlink_id)
);
