CREATE TABLE generation_sessions (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title          VARCHAR(300) NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX generation_sessions_user_id ON generation_sessions (user_id, updated_at DESC);

ALTER TABLE generation_requests
    ADD COLUMN session_id UUID REFERENCES generation_sessions(id) ON DELETE SET NULL,
    ADD COLUMN parent_id  UUID REFERENCES generation_requests(id) ON DELETE SET NULL;

CREATE INDEX generation_requests_session_id ON generation_requests (session_id, created_at ASC);
