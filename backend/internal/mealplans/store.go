// Package mealplans stores weekly meal plans laid out as a day × meal-slot grid
// and derives the aggregated shopping need (ingredients to buy) plus per-slot БЖУ.
package mealplans

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	// ErrNotFound is returned when a plan or item does not exist for the user.
	ErrNotFound = errors.New("meal plan not found")
	// ErrInvalidRecipe is returned when an item references a recipe the user
	// does not own.
	ErrInvalidRecipe = errors.New("item references unknown recipe")
	// ErrInvalidProduct is returned when an item references a product the user
	// does not own.
	ErrInvalidProduct = errors.New("item references unknown product")
)

// dateLayout is the wire format for DATE values (YYYY-MM-DD).
const dateLayout = "2006-01-02"

// Date is a calendar date that marshals as YYYY-MM-DD.
type Date struct{ time.Time }

// MarshalJSON renders the date as "2006-01-02".
func (d Date) MarshalJSON() ([]byte, error) {
	return []byte(`"` + d.Format(dateLayout) + `"`), nil
}

// UnmarshalJSON parses a "2006-01-02" string.
func (d *Date) UnmarshalJSON(b []byte) error {
	s := string(b)
	if len(s) < 2 {
		return fmt.Errorf("invalid date %q", s)
	}
	t, err := time.Parse(dateLayout, s[1:len(s)-1])
	if err != nil {
		return err
	}
	d.Time = t
	return nil
}

// Plan is a meal plan covering [StartDate, EndDate].
type Plan struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Name      string    `json:"name"`
	StartDate Date      `json:"start_date"`
	EndDate   Date      `json:"end_date"`
	CreatedAt time.Time `json:"created_at"`
	Items     []Item    `json:"items"`
}

// Item places a recipe or a raw product into one day × meal-slot cell of a plan.
// Exactly one side is set: a recipe (RecipeID/RecipeName/Servings) or a product
// (ProductID/ProductName/Grams).
type Item struct {
	ID          int64    `json:"id"`
	MealPlanID  int64    `json:"meal_plan_id"`
	DayDate     Date     `json:"day_date"`
	MealSlot    string   `json:"meal_slot"`
	RecipeID    int64    `json:"recipe_id"`
	RecipeName  string   `json:"recipe_name"`
	Servings    float64  `json:"servings"`
	ProductID   *int64   `json:"product_id,omitempty"`
	ProductName string   `json:"product_name,omitempty"`
	Grams       *float64 `json:"grams,omitempty"`
}

// Store is a PostgreSQL-backed meal-plan repository.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore wraps a pgx pool.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// CreatePlan inserts an empty plan.
func (s *Store) CreatePlan(ctx context.Context, p Plan) (Plan, error) {
	err := s.pool.QueryRow(ctx,
		`INSERT INTO meal_plans (user_id, name, start_date, end_date)
		 VALUES ($1,$2,$3,$4)
		 RETURNING id, user_id, name, start_date, end_date, created_at`,
		p.UserID, p.Name, p.StartDate.Time, p.EndDate.Time,
	).Scan(&p.ID, &p.UserID, &p.Name, &p.StartDate.Time, &p.EndDate.Time, &p.CreatedAt)
	if err != nil {
		return Plan{}, fmt.Errorf("create plan: %w", err)
	}
	p.Items = []Item{}
	return p, nil
}

// GetPlan returns a plan with its items, or ErrNotFound.
func (s *Store) GetPlan(ctx context.Context, userID, id int64) (Plan, error) {
	var p Plan
	err := s.pool.QueryRow(ctx,
		`SELECT id, user_id, name, start_date, end_date, created_at
		 FROM meal_plans WHERE id = $1 AND user_id = $2`, id, userID,
	).Scan(&p.ID, &p.UserID, &p.Name, &p.StartDate.Time, &p.EndDate.Time, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Plan{}, ErrNotFound
	}
	if err != nil {
		return Plan{}, fmt.Errorf("get plan: %w", err)
	}

	rows, err := s.pool.Query(ctx,
		`SELECT mpi.id, mpi.meal_plan_id, mpi.day_date, mpi.meal_slot,
		        mpi.recipe_id, r.name, mpi.servings,
		        mpi.product_id, p.name, mpi.grams
		 FROM meal_plan_items mpi
		 LEFT JOIN recipes  r ON r.id = mpi.recipe_id
		 LEFT JOIN products p ON p.id = mpi.product_id
		 WHERE mpi.meal_plan_id = $1
		 ORDER BY mpi.day_date, mpi.meal_slot, mpi.id`, id)
	if err != nil {
		return Plan{}, fmt.Errorf("get items: %w", err)
	}
	defer rows.Close()
	p.Items = []Item{}
	for rows.Next() {
		var it Item
		var recipeID *int64
		var recipeName, productName *string
		if err := rows.Scan(&it.ID, &it.MealPlanID, &it.DayDate.Time, &it.MealSlot,
			&recipeID, &recipeName, &it.Servings,
			&it.ProductID, &productName, &it.Grams); err != nil {
			return Plan{}, fmt.Errorf("scan item: %w", err)
		}
		if recipeID != nil {
			it.RecipeID = *recipeID
		}
		if recipeName != nil {
			it.RecipeName = *recipeName
		}
		if productName != nil {
			it.ProductName = *productName
		}
		p.Items = append(p.Items, it)
	}
	return p, rows.Err()
}

// ListPlans returns a user's plans (without items), newest start first.
func (s *Store) ListPlans(ctx context.Context, userID int64) ([]Plan, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, user_id, name, start_date, end_date, created_at
		 FROM meal_plans WHERE user_id = $1 ORDER BY start_date DESC, id DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list plans: %w", err)
	}
	defer rows.Close()
	var out []Plan
	for rows.Next() {
		var p Plan
		if err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.StartDate.Time, &p.EndDate.Time, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan plan: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// DeletePlan removes a plan (and its items via cascade). ErrNotFound if missing.
func (s *Store) DeletePlan(ctx context.Context, userID, id int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM meal_plans WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return fmt.Errorf("delete plan: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// AddItem places a recipe or a raw product into a plan cell. The plan and the
// referenced recipe/product must belong to the user (ErrNotFound /
// ErrInvalidRecipe / ErrInvalidProduct otherwise).
func (s *Store) AddItem(ctx context.Context, userID int64, it Item) (Item, error) {
	isProduct := it.ProductID != nil
	if !isProduct && it.Servings <= 0 {
		it.Servings = 1
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Item{}, err
	}
	defer tx.Rollback(ctx)

	var owns bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM meal_plans WHERE id = $1 AND user_id = $2)`,
		it.MealPlanID, userID).Scan(&owns); err != nil {
		return Item{}, fmt.Errorf("verify plan: %w", err)
	}
	if !owns {
		return Item{}, ErrNotFound
	}

	if isProduct {
		var productOK bool
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM products WHERE id = $1 AND user_id = $2)`,
			*it.ProductID, userID).Scan(&productOK); err != nil {
			return Item{}, fmt.Errorf("verify product: %w", err)
		}
		if !productOK {
			return Item{}, ErrInvalidProduct
		}
		err = tx.QueryRow(ctx,
			`INSERT INTO meal_plan_items (meal_plan_id, day_date, meal_slot, product_id, grams)
			 VALUES ($1,$2,$3,$4,$5) RETURNING id`,
			it.MealPlanID, it.DayDate.Time, it.MealSlot, *it.ProductID, it.Grams,
		).Scan(&it.ID)
		if err != nil {
			return Item{}, fmt.Errorf("insert product item: %w", err)
		}
		if err := tx.QueryRow(ctx, `SELECT name FROM products WHERE id = $1`, *it.ProductID).Scan(&it.ProductName); err != nil {
			return Item{}, fmt.Errorf("load product name: %w", err)
		}
	} else {
		var recipeOK bool
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM recipes WHERE id = $1 AND user_id = $2)`,
			it.RecipeID, userID).Scan(&recipeOK); err != nil {
			return Item{}, fmt.Errorf("verify recipe: %w", err)
		}
		if !recipeOK {
			return Item{}, ErrInvalidRecipe
		}
		err = tx.QueryRow(ctx,
			`INSERT INTO meal_plan_items (meal_plan_id, day_date, meal_slot, recipe_id, servings)
			 VALUES ($1,$2,$3,$4,$5) RETURNING id`,
			it.MealPlanID, it.DayDate.Time, it.MealSlot, it.RecipeID, it.Servings,
		).Scan(&it.ID)
		if err != nil {
			return Item{}, fmt.Errorf("insert item: %w", err)
		}
		if err := tx.QueryRow(ctx, `SELECT name FROM recipes WHERE id = $1`, it.RecipeID).Scan(&it.RecipeName); err != nil {
			return Item{}, fmt.Errorf("load recipe name: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return Item{}, err
	}
	return it, nil
}

// CopyDay replaces the items of each target day with copies of the source day's
// items, within one plan owned by the user. Targets equal to the source are
// skipped. Returns the newly created items. ErrNotFound if the plan is missing.
func (s *Store) CopyDay(ctx context.Context, userID, planID int64, source Date, targets []Date) ([]Item, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var owns bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM meal_plans WHERE id = $1 AND user_id = $2)`,
		planID, userID).Scan(&owns); err != nil {
		return nil, fmt.Errorf("verify plan: %w", err)
	}
	if !owns {
		return nil, ErrNotFound
	}

	// Load the source day's items (recipe or product).
	rows, err := tx.Query(ctx,
		`SELECT mpi.meal_slot, mpi.recipe_id, r.name, mpi.servings,
		        mpi.product_id, p.name, mpi.grams
		 FROM meal_plan_items mpi
		 LEFT JOIN recipes  r ON r.id = mpi.recipe_id
		 LEFT JOIN products p ON p.id = mpi.product_id
		 WHERE mpi.meal_plan_id = $1 AND mpi.day_date = $2
		 ORDER BY mpi.meal_slot, mpi.id`, planID, source.Time)
	if err != nil {
		return nil, fmt.Errorf("load source items: %w", err)
	}
	type srcItem struct {
		slot        string
		recipeID    *int64
		recipeName  *string
		servings    float64
		productID   *int64
		productName *string
		grams       *float64
	}
	var src []srcItem
	for rows.Next() {
		var it srcItem
		if err := rows.Scan(&it.slot, &it.recipeID, &it.recipeName, &it.servings,
			&it.productID, &it.productName, &it.grams); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan source item: %w", err)
		}
		src = append(src, it)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := []Item{}
	for _, target := range targets {
		if target.Time.Equal(source.Time) {
			continue
		}
		// Replace: clear the target day first.
		if _, err := tx.Exec(ctx,
			`DELETE FROM meal_plan_items WHERE meal_plan_id = $1 AND day_date = $2`,
			planID, target.Time); err != nil {
			return nil, fmt.Errorf("clear target day: %w", err)
		}
		for _, it := range src {
			var newID int64
			copied := Item{MealPlanID: planID, DayDate: target, MealSlot: it.slot}
			if it.productID != nil {
				if err := tx.QueryRow(ctx,
					`INSERT INTO meal_plan_items (meal_plan_id, day_date, meal_slot, product_id, grams)
					 VALUES ($1,$2,$3,$4,$5) RETURNING id`,
					planID, target.Time, it.slot, *it.productID, it.grams,
				).Scan(&newID); err != nil {
					return nil, fmt.Errorf("insert copied item: %w", err)
				}
				copied.ProductID = it.productID
				copied.Grams = it.grams
				if it.productName != nil {
					copied.ProductName = *it.productName
				}
			} else {
				if err := tx.QueryRow(ctx,
					`INSERT INTO meal_plan_items (meal_plan_id, day_date, meal_slot, recipe_id, servings)
					 VALUES ($1,$2,$3,$4,$5) RETURNING id`,
					planID, target.Time, it.slot, *it.recipeID, it.servings,
				).Scan(&newID); err != nil {
					return nil, fmt.Errorf("insert copied item: %w", err)
				}
				if it.recipeID != nil {
					copied.RecipeID = *it.recipeID
				}
				if it.recipeName != nil {
					copied.RecipeName = *it.recipeName
				}
				copied.Servings = it.servings
			}
			copied.ID = newID
			out = append(out, copied)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateItem changes the quantity of one item in a plan owned by the user:
// servings for a recipe item or grams for a product item. Exactly one of
// servings/grams is applied, matching the item's kind. Returns the refreshed
// item (with joined recipe/product name). ErrNotFound if the item is missing or
// not owned by the user.
func (s *Store) UpdateItem(ctx context.Context, userID, planID, itemID int64, servings, grams *float64) (Item, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Item{}, err
	}
	defer tx.Rollback(ctx)

	// Load the item, scoped to the user's plan, to learn its kind.
	var it Item
	var curServings float64
	var productID *int64
	err = tx.QueryRow(ctx,
		`SELECT mpi.id, mpi.product_id, mpi.servings
		 FROM meal_plan_items mpi
		 JOIN meal_plans mp ON mp.id = mpi.meal_plan_id
		 WHERE mpi.id = $1 AND mpi.meal_plan_id = $2 AND mp.user_id = $3`,
		itemID, planID, userID).Scan(&it.ID, &productID, &curServings)
	if errors.Is(err, pgx.ErrNoRows) {
		return Item{}, ErrNotFound
	}
	if err != nil {
		return Item{}, fmt.Errorf("load item: %w", err)
	}

	if productID != nil {
		if grams == nil {
			return Item{}, ErrInvalidProduct
		}
		if _, err := tx.Exec(ctx,
			`UPDATE meal_plan_items SET grams = $1 WHERE id = $2`, *grams, itemID); err != nil {
			return Item{}, fmt.Errorf("update grams: %w", err)
		}
	} else {
		if servings == nil {
			return Item{}, ErrInvalidRecipe
		}
		if _, err := tx.Exec(ctx,
			`UPDATE meal_plan_items SET servings = $1 WHERE id = $2`, *servings, itemID); err != nil {
			return Item{}, fmt.Errorf("update servings: %w", err)
		}
	}

	// Re-select the full row with joined names.
	var recipeID *int64
	var recipeName, productName *string
	err = tx.QueryRow(ctx,
		`SELECT mpi.id, mpi.meal_plan_id, mpi.day_date, mpi.meal_slot,
		        mpi.recipe_id, r.name, mpi.servings,
		        mpi.product_id, p.name, mpi.grams
		 FROM meal_plan_items mpi
		 LEFT JOIN recipes  r ON r.id = mpi.recipe_id
		 LEFT JOIN products p ON p.id = mpi.product_id
		 WHERE mpi.id = $1`, itemID).Scan(
		&it.ID, &it.MealPlanID, &it.DayDate.Time, &it.MealSlot,
		&recipeID, &recipeName, &it.Servings,
		&it.ProductID, &productName, &it.Grams)
	if err != nil {
		return Item{}, fmt.Errorf("reload item: %w", err)
	}
	if recipeID != nil {
		it.RecipeID = *recipeID
	}
	if recipeName != nil {
		it.RecipeName = *recipeName
	}
	if productName != nil {
		it.ProductName = *productName
	}

	if err := tx.Commit(ctx); err != nil {
		return Item{}, err
	}
	return it, nil
}

// DeleteItem removes one item from a plan owned by the user. ErrNotFound if missing.
func (s *Store) DeleteItem(ctx context.Context, userID, planID, itemID int64) error {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM meal_plan_items mpi
		 USING meal_plans mp
		 WHERE mpi.id = $1 AND mpi.meal_plan_id = $2 AND mp.id = mpi.meal_plan_id AND mp.user_id = $3`,
		itemID, planID, userID)
	if err != nil {
		return fmt.Errorf("delete item: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
