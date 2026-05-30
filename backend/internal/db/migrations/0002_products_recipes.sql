CREATE TABLE products (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    category    TEXT NOT NULL DEFAULT '',
    brand       TEXT NOT NULL DEFAULT '',
    kcal100     DOUBLE PRECISION,
    protein100  DOUBLE PRECISION,
    fat100      DOUBLE PRECISION,
    carbs100    DOUBLE PRECISION,
    source      TEXT NOT NULL DEFAULT 'manual',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_products_user ON products (user_id, name);

CREATE TABLE recipes (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    servings     INT NOT NULL DEFAULT 1,
    instructions TEXT NOT NULL DEFAULT '',
    meal_types   TEXT[] NOT NULL DEFAULT '{}',
    source       TEXT NOT NULL DEFAULT 'manual',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_recipes_user ON recipes (user_id, name);

CREATE TABLE recipe_ingredients (
    id         BIGSERIAL PRIMARY KEY,
    recipe_id  BIGINT NOT NULL REFERENCES recipes(id) ON DELETE CASCADE,
    product_id BIGINT NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
    grams      DOUBLE PRECISION NOT NULL
);

CREATE INDEX idx_recipe_ingredients_recipe ON recipe_ingredients (recipe_id);
