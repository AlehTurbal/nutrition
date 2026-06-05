-- A plan cell may now hold a raw product (product_id + grams) instead of a
-- recipe. A row is an XOR: exactly one of recipe_id / product_id.
ALTER TABLE meal_plan_items ALTER COLUMN recipe_id DROP NOT NULL;
ALTER TABLE meal_plan_items
    ADD COLUMN product_id BIGINT REFERENCES products(id) ON DELETE RESTRICT,
    ADD COLUMN grams       DOUBLE PRECISION;
ALTER TABLE meal_plan_items
    ADD CONSTRAINT meal_plan_items_recipe_xor_product
    CHECK ((recipe_id IS NOT NULL) <> (product_id IS NOT NULL));
