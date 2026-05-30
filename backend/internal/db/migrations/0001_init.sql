CREATE TABLE users (
    id            BIGSERIAL PRIMARY KEY,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE profiles (
    user_id        BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    sex            TEXT NOT NULL,
    height_cm      DOUBLE PRECISION NOT NULL,
    age            INT NOT NULL,
    activity_level TEXT NOT NULL,
    goal           TEXT NOT NULL,
    protein_per_kg DOUBLE PRECISION NOT NULL DEFAULT 0,
    fat_pct        DOUBLE PRECISION NOT NULL DEFAULT 0,
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE weight_entries (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    weight_kg   DOUBLE PRECISION NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_weight_entries_user_recorded
    ON weight_entries (user_id, recorded_at DESC);
