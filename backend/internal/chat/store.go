// Package chat persists the assistant's conversation: threads and the visible
// user/assistant messages. Everything is scoped to a user through
// chat_threads.user_id.
package chat

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a thread does not exist for the user.
var ErrNotFound = errors.New("not found")

// Thread is a single conversation.
type Thread struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
}

// Message is one visible turn in a thread.
type Message struct {
	ID        int64     `json:"id"`
	ThreadID  int64     `json:"thread_id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// Store is a PostgreSQL-backed repository for chat threads and messages.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore wraps a pgx pool.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// CreateThread starts a new thread for the user.
func (s *Store) CreateThread(ctx context.Context, userID int64, title string) (Thread, error) {
	var t Thread
	err := s.pool.QueryRow(ctx,
		`INSERT INTO chat_threads (user_id, title)
		 VALUES ($1, $2)
		 RETURNING id, user_id, title, created_at`,
		userID, title,
	).Scan(&t.ID, &t.UserID, &t.Title, &t.CreatedAt)
	if err != nil {
		return Thread{}, fmt.Errorf("create thread: %w", err)
	}
	return t, nil
}

// ListThreads returns the user's threads, newest first.
func (s *Store) ListThreads(ctx context.Context, userID int64) ([]Thread, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, user_id, title, created_at
		 FROM chat_threads WHERE user_id = $1 ORDER BY created_at DESC`, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list threads: %w", err)
	}
	defer rows.Close()

	var out []Thread
	for rows.Next() {
		var t Thread
		if err := rows.Scan(&t.ID, &t.UserID, &t.Title, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan thread: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// GetThread returns one of the user's threads or ErrNotFound. Used for ownership
// checks before reading or appending messages.
func (s *Store) GetThread(ctx context.Context, userID, id int64) (Thread, error) {
	var t Thread
	err := s.pool.QueryRow(ctx,
		`SELECT id, user_id, title, created_at
		 FROM chat_threads WHERE id = $1 AND user_id = $2`, id, userID,
	).Scan(&t.ID, &t.UserID, &t.Title, &t.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Thread{}, ErrNotFound
	}
	if err != nil {
		return Thread{}, fmt.Errorf("get thread: %w", err)
	}
	return t, nil
}

// DeleteThread removes one of the user's threads (messages cascade). Returns
// ErrNotFound if the thread is missing or not owned by the user.
func (s *Store) DeleteThread(ctx context.Context, userID, id int64) error {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM chat_threads WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return fmt.Errorf("delete thread: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListMessages returns a thread's messages oldest-first. Ownership must be
// verified by the caller (via GetThread).
func (s *Store) ListMessages(ctx context.Context, threadID int64) ([]Message, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, thread_id, role, content, created_at
		 FROM chat_messages WHERE thread_id = $1 ORDER BY created_at, id`, threadID,
	)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	defer rows.Close()

	var out []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.ThreadID, &m.Role, &m.Content, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// AddMessage appends a message to a thread. Ownership must be verified by the
// caller (via GetThread).
func (s *Store) AddMessage(ctx context.Context, threadID int64, role, content string) (Message, error) {
	var m Message
	err := s.pool.QueryRow(ctx,
		`INSERT INTO chat_messages (thread_id, role, content)
		 VALUES ($1, $2, $3)
		 RETURNING id, thread_id, role, content, created_at`,
		threadID, role, content,
	).Scan(&m.ID, &m.ThreadID, &m.Role, &m.Content, &m.CreatedAt)
	if err != nil {
		return Message{}, fmt.Errorf("add message: %w", err)
	}
	return m, nil
}
