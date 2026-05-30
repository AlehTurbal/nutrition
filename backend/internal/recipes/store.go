package recipes

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	// ErrNotFound is returned when a recipe does not exist for the user.
	ErrNotFound = errors.New("recipe not found")
	// ErrInvalidIngredient is returned when an ingredient references a product
	// that does not belong to the user.
	ErrInvalidIngredient = errors.New("ingredient references unknown product")
)

// Ingredient is one component of a recipe plus the denormalized per-100g
// nutrition of its product (loaded on read for macro computation/display).
type Ingredient struct {
	ID          int64    `json:"id"`
	ProductID   int64    `json:"product_id"`
	ProductName string   `json:"product_name"`
	Grams       float64  `json:"grams"`
	Kcal100     *float64 `json:"kcal100"`
	Protein100  *float64 `json:"protein100"`
	Fat100      *float64 `json:"fat100"`
	Carbs100    *float64 `json:"carbs100"`
}

// Recipe is a dish: metadata, meal-type tags and its ingredients.
type Recipe struct {
	ID           int64        `json:"id"`
	UserID       int64        `json:"user_id"`
	Name         string       `json:"name"`
	Servings     int          `json:"servings"`
	Instructions string       `json:"instructions"`
	MealTypes    []string     `json:"meal_types"`
	Source       string       `json:"source"`
	CreatedAt    time.Time    `json:"created_at"`
	Ingredients  []Ingredient `json:"ingredients,omitempty"`
}

// Macros computes the total БЖУ/calories of the recipe from its ingredients.
func (r Recipe) Macros() Macros {
	ings := make([]IngredientMacro, len(r.Ingredients))
	for i, in := range r.Ingredients {
		ings[i] = IngredientMacro{
			Grams: in.Grams, Kcal100: in.Kcal100, Protein100: in.Protein100,
			Fat100: in.Fat100, Carbs100: in.Carbs100,
		}
	}
	return ComputeMacros(ings)
}

// Store is a PostgreSQL-backed recipe repository.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore wraps a pgx pool.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Create inserts a recipe and its ingredients atomically.
func (s *Store) Create(ctx context.Context, r Recipe) (Recipe, error) {
	if r.Servings <= 0 {
		r.Servings = 1
	}
	if r.Source == "" {
		r.Source = "manual"
	}
	if r.MealTypes == nil {
		r.MealTypes = []string{}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Recipe{}, err
	}
	defer tx.Rollback(ctx)

	if err := assertProductsOwned(ctx, tx, r.UserID, r.Ingredients); err != nil {
		return Recipe{}, err
	}

	var id int64
	err = tx.QueryRow(ctx,
		`INSERT INTO recipes (user_id, name, servings, instructions, meal_types, source)
		 VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
		r.UserID, r.Name, r.Servings, r.Instructions, r.MealTypes, r.Source,
	).Scan(&id)
	if err != nil {
		return Recipe{}, fmt.Errorf("insert recipe: %w", err)
	}

	if err := insertIngredients(ctx, tx, id, r.Ingredients); err != nil {
		return Recipe{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Recipe{}, err
	}
	return s.Get(ctx, r.UserID, id)
}

// Get returns a recipe with its ingredients, or ErrNotFound.
func (s *Store) Get(ctx context.Context, userID, id int64) (Recipe, error) {
	var r Recipe
	err := s.pool.QueryRow(ctx,
		`SELECT id, user_id, name, servings, instructions, meal_types, source, created_at
		 FROM recipes WHERE id = $1 AND user_id = $2`, id, userID,
	).Scan(&r.ID, &r.UserID, &r.Name, &r.Servings, &r.Instructions, &r.MealTypes, &r.Source, &r.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Recipe{}, ErrNotFound
	}
	if err != nil {
		return Recipe{}, fmt.Errorf("get recipe: %w", err)
	}

	rows, err := s.pool.Query(ctx,
		`SELECT ri.id, ri.product_id, p.name, ri.grams, p.kcal100, p.protein100, p.fat100, p.carbs100
		 FROM recipe_ingredients ri JOIN products p ON p.id = ri.product_id
		 WHERE ri.recipe_id = $1 ORDER BY ri.id`, id)
	if err != nil {
		return Recipe{}, fmt.Errorf("get ingredients: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var in Ingredient
		if err := rows.Scan(&in.ID, &in.ProductID, &in.ProductName, &in.Grams,
			&in.Kcal100, &in.Protein100, &in.Fat100, &in.Carbs100); err != nil {
			return Recipe{}, fmt.Errorf("scan ingredient: %w", err)
		}
		r.Ingredients = append(r.Ingredients, in)
	}
	return r, rows.Err()
}

// List returns a user's recipes (metadata only), ordered by name.
func (s *Store) List(ctx context.Context, userID int64) ([]Recipe, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, user_id, name, servings, instructions, meal_types, source, created_at
		 FROM recipes WHERE user_id = $1 ORDER BY name`, userID)
	if err != nil {
		return nil, fmt.Errorf("list recipes: %w", err)
	}
	defer rows.Close()

	var out []Recipe
	for rows.Next() {
		var r Recipe
		if err := rows.Scan(&r.ID, &r.UserID, &r.Name, &r.Servings, &r.Instructions,
			&r.MealTypes, &r.Source, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan recipe: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Update replaces a recipe's fields and ingredients atomically.
func (s *Store) Update(ctx context.Context, r Recipe) (Recipe, error) {
	if r.Servings <= 0 {
		r.Servings = 1
	}
	if r.MealTypes == nil {
		r.MealTypes = []string{}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Recipe{}, err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx,
		`UPDATE recipes SET name=$3, servings=$4, instructions=$5, meal_types=$6
		 WHERE id=$1 AND user_id=$2`,
		r.ID, r.UserID, r.Name, r.Servings, r.Instructions, r.MealTypes)
	if err != nil {
		return Recipe{}, fmt.Errorf("update recipe: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return Recipe{}, ErrNotFound
	}

	if err := assertProductsOwned(ctx, tx, r.UserID, r.Ingredients); err != nil {
		return Recipe{}, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM recipe_ingredients WHERE recipe_id = $1`, r.ID); err != nil {
		return Recipe{}, fmt.Errorf("clear ingredients: %w", err)
	}
	if err := insertIngredients(ctx, tx, r.ID, r.Ingredients); err != nil {
		return Recipe{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Recipe{}, err
	}
	return s.Get(ctx, r.UserID, r.ID)
}

// Delete removes a recipe (and its ingredients via cascade). ErrNotFound if missing.
func (s *Store) Delete(ctx context.Context, userID, id int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM recipes WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return fmt.Errorf("delete recipe: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func insertIngredients(ctx context.Context, tx pgx.Tx, recipeID int64, ings []Ingredient) error {
	for _, in := range ings {
		if _, err := tx.Exec(ctx,
			`INSERT INTO recipe_ingredients (recipe_id, product_id, grams) VALUES ($1,$2,$3)`,
			recipeID, in.ProductID, in.Grams); err != nil {
			return fmt.Errorf("insert ingredient: %w", err)
		}
	}
	return nil
}

// assertProductsOwned verifies every referenced product belongs to the user.
func assertProductsOwned(ctx context.Context, tx pgx.Tx, userID int64, ings []Ingredient) error {
	if len(ings) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(ings))
	seen := make(map[int64]struct{})
	for _, in := range ings {
		if _, ok := seen[in.ProductID]; ok {
			continue
		}
		seen[in.ProductID] = struct{}{}
		ids = append(ids, in.ProductID)
	}

	var count int
	if err := tx.QueryRow(ctx,
		`SELECT count(*) FROM products WHERE user_id = $1 AND id = ANY($2)`, userID, ids,
	).Scan(&count); err != nil {
		return fmt.Errorf("verify products: %w", err)
	}
	if count != len(ids) {
		return ErrInvalidIngredient
	}
	return nil
}
