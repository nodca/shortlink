-- Dev seed users (idempotent).
-- Requires pgcrypto for crypt()/gen_salt().

CREATE EXTENSION IF NOT EXISTS pgcrypto;

INSERT INTO users (username, password_hash, role)
VALUES
  ('admin', crypt('admin', gen_salt('bf')), 'admin'),
  ('user',  crypt('user',  gen_salt('bf')), 'user')
ON CONFLICT (username) DO NOTHING;

