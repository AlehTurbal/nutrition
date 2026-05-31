package stores

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when no saved match exists for a plan.
var ErrNotFound = errors.New("store match not found")

// SavedMatch is a persisted matching run for a plan.
type SavedMatch struct {
	ID         int64     `json:"id"`
	UserID     int64     `json:"user_id"`
	MealPlanID int64     `json:"meal_plan_id"`
	CreatedAt  time.Time `json:"created_at"`
	Items      []Match   `json:"items"`
}

// Store persists store-matching proposals.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore wraps a pgx pool.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Save writes a matching run for a plan owned by the user. The plan must belong
// to the user (verified by the FK + explicit ownership check).
func (s *Store) Save(ctx context.Context, userID, planID int64, items []Match) (SavedMatch, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return SavedMatch{}, err
	}
	defer tx.Rollback(ctx)

	var owns bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM meal_plans WHERE id=$1 AND user_id=$2)`,
		planID, userID).Scan(&owns); err != nil {
		return SavedMatch{}, fmt.Errorf("verify plan: %w", err)
	}
	if !owns {
		return SavedMatch{}, ErrNotFound
	}

	var m SavedMatch
	err = tx.QueryRow(ctx,
		`INSERT INTO store_matches (user_id, meal_plan_id) VALUES ($1,$2)
		 RETURNING id, user_id, meal_plan_id, created_at`,
		userID, planID,
	).Scan(&m.ID, &m.UserID, &m.MealPlanID, &m.CreatedAt)
	if err != nil {
		return SavedMatch{}, fmt.Errorf("insert store_match: %w", err)
	}

	for _, it := range items {
		if _, err := tx.Exec(ctx,
			`INSERT INTO store_match_items (store_match_id, needed_name, needed_grams, matched_text, found)
			 VALUES ($1,$2,$3,$4,$5)`,
			m.ID, it.Needed, 0.0, it.Matched, it.Found); err != nil {
			return SavedMatch{}, fmt.Errorf("insert item: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return SavedMatch{}, err
	}
	m.Items = items
	return m, nil
}

// Latest returns the most recent saved match for a plan owned by the user.
func (s *Store) Latest(ctx context.Context, userID, planID int64) (SavedMatch, error) {
	var m SavedMatch
	err := s.pool.QueryRow(ctx,
		`SELECT id, user_id, meal_plan_id, created_at FROM store_matches
		 WHERE user_id=$1 AND meal_plan_id=$2 ORDER BY created_at DESC, id DESC LIMIT 1`,
		userID, planID,
	).Scan(&m.ID, &m.UserID, &m.MealPlanID, &m.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return SavedMatch{}, ErrNotFound
	}
	if err != nil {
		return SavedMatch{}, fmt.Errorf("latest match: %w", err)
	}

	rows, err := s.pool.Query(ctx,
		`SELECT needed_name, matched_text, found FROM store_match_items
		 WHERE store_match_id=$1 ORDER BY id`, m.ID)
	if err != nil {
		return SavedMatch{}, fmt.Errorf("load items: %w", err)
	}
	defer rows.Close()
	m.Items = []Match{}
	for rows.Next() {
		var it Match
		if err := rows.Scan(&it.Needed, &it.Matched, &it.Found); err != nil {
			return SavedMatch{}, fmt.Errorf("scan item: %w", err)
		}
		m.Items = append(m.Items, it)
	}
	return m, rows.Err()
}
