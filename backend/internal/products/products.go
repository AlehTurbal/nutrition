// Package products stores a user's editable database of food products and their
// per-100g nutrition (БЖУ). БЖУ values are optional and may be filled in later.
package products

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a product does not exist for the user.
var ErrNotFound = errors.New("product not found")

// ErrInUse is returned when a product cannot be deleted because it is still
// referenced (e.g. used as a recipe ingredient).
var ErrInUse = errors.New("product is in use")

// Product is one food item with optional per-100g nutrition.
type Product struct {
	ID         int64    `json:"id"`
	UserID     int64    `json:"user_id"`
	Name       string   `json:"name"`
	Category   string   `json:"category"`
	Brand      string   `json:"brand"`
	Kcal100    *float64 `json:"kcal100"`
	Protein100 *float64 `json:"protein100"`
	Fat100     *float64 `json:"fat100"`
	Carbs100   *float64 `json:"carbs100"`
	// Fiber100 is dietary fibre per 100g (клетчатка). Like the other per-100g
	// fields it scales with portion size. Optional.
	Fiber100 *float64 `json:"fiber100"`
	// GlycemicIndex is the intrinsic 0–100 GI of the food. Unlike the per-100g
	// БЖУ fields it does not scale with portion size. Optional.
	GlycemicIndex *float64  `json:"glycemic_index"`
	Source        string    `json:"source"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Store is a PostgreSQL-backed product repository.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore wraps a pgx pool.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

const cols = `id, user_id, name, category, brand, kcal100, protein100, fat100, carbs100, fiber100, glycemic_index, source, created_at, updated_at`

func scan(row pgx.Row) (Product, error) {
	var p Product
	err := row.Scan(&p.ID, &p.UserID, &p.Name, &p.Category, &p.Brand,
		&p.Kcal100, &p.Protein100, &p.Fat100, &p.Carbs100, &p.Fiber100, &p.GlycemicIndex, &p.Source, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

// Create inserts a new product owned by p.UserID.
func (s *Store) Create(ctx context.Context, p Product) (Product, error) {
	if p.Source == "" {
		p.Source = "manual"
	}
	out, err := scan(s.pool.QueryRow(ctx,
		`INSERT INTO products (user_id, name, category, brand, kcal100, protein100, fat100, carbs100, fiber100, glycemic_index, source)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		 RETURNING `+cols,
		p.UserID, p.Name, p.Category, p.Brand, p.Kcal100, p.Protein100, p.Fat100, p.Carbs100, p.Fiber100, p.GlycemicIndex, p.Source))
	if err != nil {
		return Product{}, fmt.Errorf("create product: %w", err)
	}
	return out, nil
}

// Get returns a single product owned by the user, or ErrNotFound.
func (s *Store) Get(ctx context.Context, userID, id int64) (Product, error) {
	p, err := scan(s.pool.QueryRow(ctx,
		`SELECT `+cols+` FROM products WHERE id = $1 AND user_id = $2`, id, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Product{}, ErrNotFound
	}
	if err != nil {
		return Product{}, fmt.Errorf("get product: %w", err)
	}
	return p, nil
}

// List returns all of a user's products, ordered by name.
func (s *Store) List(ctx context.Context, userID int64) ([]Product, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+cols+` FROM products WHERE user_id = $1 ORDER BY name`, userID)
	if err != nil {
		return nil, fmt.Errorf("list products: %w", err)
	}
	defer rows.Close()

	var out []Product
	for rows.Next() {
		p, err := scan(rows)
		if err != nil {
			return nil, fmt.Errorf("scan product: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Update replaces a product's editable fields. Returns ErrNotFound if the
// product does not belong to the user.
func (s *Store) Update(ctx context.Context, p Product) (Product, error) {
	out, err := scan(s.pool.QueryRow(ctx,
		`UPDATE products SET
		   name = $3, category = $4, brand = $5,
		   kcal100 = $6, protein100 = $7, fat100 = $8, carbs100 = $9,
		   fiber100 = $10, glycemic_index = $11, source = $12, updated_at = now()
		 WHERE id = $1 AND user_id = $2
		 RETURNING `+cols,
		p.ID, p.UserID, p.Name, p.Category, p.Brand,
		p.Kcal100, p.Protein100, p.Fat100, p.Carbs100, p.Fiber100, p.GlycemicIndex, p.Source))
	if errors.Is(err, pgx.ErrNoRows) {
		return Product{}, ErrNotFound
	}
	if err != nil {
		return Product{}, fmt.Errorf("update product: %w", err)
	}
	return out, nil
}

// Delete removes a product owned by the user. Returns ErrNotFound if missing.
func (s *Store) Delete(ctx context.Context, userID, id int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM products WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" { // foreign_key_violation
			return ErrInUse
		}
		return fmt.Errorf("delete product: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
