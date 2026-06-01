// Package users provides storage and retrieval of accounts, body profiles and
// weight history. All domain data is scoped to a user.
package users

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	// ErrNotFound is returned when a requested row does not exist.
	ErrNotFound = errors.New("not found")
	// ErrEmailTaken is returned when registering an already-used email.
	ErrEmailTaken = errors.New("email already registered")
	// ErrUserMissing is returned when an operation references a user_id that no
	// longer exists (e.g. a still-valid token after the DB was reset).
	ErrUserMissing = errors.New("user does not exist")
)

// User is an account record.
type User struct {
	ID           int64     `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

// Profile holds the body parameters used to compute nutrition targets.
type Profile struct {
	UserID        int64     `json:"user_id"`
	Sex           string    `json:"sex"`
	HeightCm      float64   `json:"height_cm"`
	Age           int       `json:"age"`
	ActivityLevel string    `json:"activity_level"`
	Goal          string    `json:"goal"`
	ProteinPerKg  float64   `json:"protein_per_kg"`
	FatPct        float64   `json:"fat_pct"`
	MealSlots     []string  `json:"meal_slots"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// WeightEntry is a single dated weight measurement.
type WeightEntry struct {
	ID         int64     `json:"id"`
	UserID     int64     `json:"user_id"`
	WeightKg   float64   `json:"weight_kg"`
	RecordedAt time.Time `json:"recorded_at"`
}

// Store is a PostgreSQL-backed repository for users and their data.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore wraps a pgx pool.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// CreateUser inserts a new account. Returns ErrEmailTaken on a duplicate email.
func (s *Store) CreateUser(ctx context.Context, email, passwordHash string) (User, error) {
	var u User
	err := s.pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash)
		 VALUES ($1, $2)
		 RETURNING id, email, password_hash, created_at`,
		email, passwordHash,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return User{}, ErrEmailTaken
		}
		return User{}, fmt.Errorf("create user: %w", err)
	}
	return u, nil
}

// GetUserByEmail looks up an account by email. Returns ErrNotFound if missing.
func (s *Store) GetUserByEmail(ctx context.Context, email string) (User, error) {
	var u User
	err := s.pool.QueryRow(ctx,
		`SELECT id, email, password_hash, created_at FROM users WHERE email = $1`, email,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("get user by email: %w", err)
	}
	return u, nil
}

// UpsertProfile creates or replaces a user's body profile.
func (s *Store) UpsertProfile(ctx context.Context, p Profile) (Profile, error) {
	if p.MealSlots == nil {
		p.MealSlots = []string{}
	}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO profiles
		   (user_id, sex, height_cm, age, activity_level, goal, protein_per_kg, fat_pct, meal_slots, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, now())
		 ON CONFLICT (user_id) DO UPDATE SET
		   sex = EXCLUDED.sex,
		   height_cm = EXCLUDED.height_cm,
		   age = EXCLUDED.age,
		   activity_level = EXCLUDED.activity_level,
		   goal = EXCLUDED.goal,
		   protein_per_kg = EXCLUDED.protein_per_kg,
		   fat_pct = EXCLUDED.fat_pct,
		   meal_slots = EXCLUDED.meal_slots,
		   updated_at = now()
		 RETURNING user_id, sex, height_cm, age, activity_level, goal, protein_per_kg, fat_pct, meal_slots, updated_at`,
		p.UserID, p.Sex, p.HeightCm, p.Age, p.ActivityLevel, p.Goal, p.ProteinPerKg, p.FatPct, p.MealSlots,
	).Scan(&p.UserID, &p.Sex, &p.HeightCm, &p.Age, &p.ActivityLevel, &p.Goal, &p.ProteinPerKg, &p.FatPct, &p.MealSlots, &p.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" { // foreign_key_violation
			return Profile{}, ErrUserMissing
		}
		return Profile{}, fmt.Errorf("upsert profile: %w", err)
	}
	return p, nil
}

// GetProfile returns a user's profile or ErrNotFound.
func (s *Store) GetProfile(ctx context.Context, userID int64) (Profile, error) {
	var p Profile
	err := s.pool.QueryRow(ctx,
		`SELECT user_id, sex, height_cm, age, activity_level, goal, protein_per_kg, fat_pct, meal_slots, updated_at
		 FROM profiles WHERE user_id = $1`, userID,
	).Scan(&p.UserID, &p.Sex, &p.HeightCm, &p.Age, &p.ActivityLevel, &p.Goal, &p.ProteinPerKg, &p.FatPct, &p.MealSlots, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Profile{}, ErrNotFound
	}
	if err != nil {
		return Profile{}, fmt.Errorf("get profile: %w", err)
	}
	return p, nil
}

// AddWeight records a new weight measurement (defaults recorded_at to now).
func (s *Store) AddWeight(ctx context.Context, userID int64, weightKg float64) (WeightEntry, error) {
	var w WeightEntry
	err := s.pool.QueryRow(ctx,
		`INSERT INTO weight_entries (user_id, weight_kg)
		 VALUES ($1, $2)
		 RETURNING id, user_id, weight_kg, recorded_at`,
		userID, weightKg,
	).Scan(&w.ID, &w.UserID, &w.WeightKg, &w.RecordedAt)
	if err != nil {
		return WeightEntry{}, fmt.Errorf("add weight: %w", err)
	}
	return w, nil
}

// ListWeights returns a user's weight history, newest first.
func (s *Store) ListWeights(ctx context.Context, userID int64) ([]WeightEntry, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, user_id, weight_kg, recorded_at
		 FROM weight_entries WHERE user_id = $1 ORDER BY recorded_at DESC`, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list weights: %w", err)
	}
	defer rows.Close()

	var out []WeightEntry
	for rows.Next() {
		var w WeightEntry
		if err := rows.Scan(&w.ID, &w.UserID, &w.WeightKg, &w.RecordedAt); err != nil {
			return nil, fmt.Errorf("scan weight: %w", err)
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// LatestWeight returns the most recent weight entry or ErrNotFound.
func (s *Store) LatestWeight(ctx context.Context, userID int64) (WeightEntry, error) {
	var w WeightEntry
	err := s.pool.QueryRow(ctx,
		`SELECT id, user_id, weight_kg, recorded_at
		 FROM weight_entries WHERE user_id = $1 ORDER BY recorded_at DESC LIMIT 1`, userID,
	).Scan(&w.ID, &w.UserID, &w.WeightKg, &w.RecordedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return WeightEntry{}, ErrNotFound
	}
	if err != nil {
		return WeightEntry{}, fmt.Errorf("latest weight: %w", err)
	}
	return w, nil
}
