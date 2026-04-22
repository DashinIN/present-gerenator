CREATE TABLE users (
    id           BIGSERIAL PRIMARY KEY,
    email        VARCHAR(320) NOT NULL UNIQUE,
    display_name VARCHAR(200) NOT NULL DEFAULT '',
    avatar_url   TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE user_identities (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider    VARCHAR(50) NOT NULL,
    provider_id VARCHAR(200) NOT NULL,
    email       VARCHAR(320) NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (provider, provider_id)
);

CREATE TABLE tariffs (
    id              SERIAL PRIMARY KEY,
    name            VARCHAR(100) NOT NULL,
    price_per_image INT NOT NULL DEFAULT 5,
    price_per_song  INT NOT NULL DEFAULT 5,
    is_active       BOOLEAN NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Только один активный тариф одновременно
CREATE UNIQUE INDEX tariffs_one_active ON tariffs (is_active) WHERE is_active = true;

INSERT INTO tariffs (name, price_per_image, price_per_song, is_active)
VALUES ('default', 5, 5, true);

CREATE TABLE credit_transactions (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount       INT NOT NULL,
    type         VARCHAR(50) NOT NULL,
    reference_id UUID,
    description  TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX credit_transactions_user_id ON credit_transactions (user_id, created_at DESC);

CREATE TABLE generation_requests (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          BIGINT NOT NULL REFERENCES users(id),
    status           VARCHAR(30) NOT NULL DEFAULT 'pending',
    recipient_name   VARCHAR(200) NOT NULL DEFAULT '',
    occasion         VARCHAR(100) NOT NULL DEFAULT '',
    image_prompt     TEXT NOT NULL DEFAULT '',
    song_lyrics      TEXT NOT NULL DEFAULT '',
    song_style       VARCHAR(200) NOT NULL DEFAULT '',
    image_count      INT NOT NULL DEFAULT 3,
    song_count       INT NOT NULL DEFAULT 1,
    input_photos     TEXT[] NOT NULL DEFAULT '{}',
    input_audio_key  TEXT NOT NULL DEFAULT '',
    result_images    TEXT[] NOT NULL DEFAULT '{}',
    result_audios    TEXT[] NOT NULL DEFAULT '{}',
    error_message    TEXT NOT NULL DEFAULT '',
    credits_spent    INT NOT NULL DEFAULT 0,
    tariff_id        INT NOT NULL REFERENCES tariffs(id),
    retry_count      INT NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at     TIMESTAMPTZ
);

CREATE INDEX generation_requests_user_id ON generation_requests (user_id, created_at DESC);
CREATE INDEX generation_requests_status ON generation_requests (status) WHERE status IN ('pending', 'processing_images', 'processing_audio');
