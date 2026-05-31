CREATE TABLE store_matches (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    meal_plan_id BIGINT NOT NULL REFERENCES meal_plans(id) ON DELETE CASCADE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_store_matches_user_plan ON store_matches (user_id, meal_plan_id, created_at DESC);

CREATE TABLE store_match_items (
    id             BIGSERIAL PRIMARY KEY,
    store_match_id BIGINT NOT NULL REFERENCES store_matches(id) ON DELETE CASCADE,
    needed_name    TEXT NOT NULL,
    needed_grams   DOUBLE PRECISION NOT NULL,
    matched_text   TEXT NOT NULL DEFAULT '',
    found          BOOLEAN NOT NULL DEFAULT false
);

CREATE INDEX idx_store_match_items_match ON store_match_items (store_match_id);
