-- Phase 3: configurable meals per day + weekly meal plan grid.

-- Ordered list of meal slots the user eats each day; its length is the number
-- of meals per day. Empty default keeps existing profiles valid; the API fills
-- a sensible default on read.
ALTER TABLE profiles ADD COLUMN meal_slots TEXT[] NOT NULL DEFAULT '{}';

CREATE TABLE meal_plans (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name       TEXT NOT NULL DEFAULT '',
    start_date DATE NOT NULL,
    end_date   DATE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_meal_plans_user ON meal_plans (user_id, start_date);

CREATE TABLE meal_plan_items (
    id           BIGSERIAL PRIMARY KEY,
    meal_plan_id BIGINT NOT NULL REFERENCES meal_plans(id) ON DELETE CASCADE,
    day_date     DATE NOT NULL,
    meal_slot    TEXT NOT NULL,
    recipe_id    BIGINT NOT NULL REFERENCES recipes(id) ON DELETE RESTRICT,
    servings     DOUBLE PRECISION NOT NULL DEFAULT 1
);

CREATE INDEX idx_meal_plan_items_plan ON meal_plan_items (meal_plan_id, day_date, meal_slot);
