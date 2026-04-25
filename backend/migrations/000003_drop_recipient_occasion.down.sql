ALTER TABLE generation_requests
    ADD COLUMN IF NOT EXISTS recipient_name TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS occasion       TEXT NOT NULL DEFAULT '';
