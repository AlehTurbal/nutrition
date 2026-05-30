package auth

import (
	"context"
	"errors"
	"strings"

	"github.com/alehturbal/nutrition/backend/internal/users"
)

// ErrInvalidCredentials is returned when login fails.
var ErrInvalidCredentials = errors.New("invalid credentials")

// ErrWeakPassword is returned when a registration password is too short.
var ErrWeakPassword = errors.New("password must be at least 8 characters")

// ErrInvalidEmail is returned for an empty/malformed email.
var ErrInvalidEmail = errors.New("invalid email")

// Service implements registration and login on top of the user store.
type Service struct {
	store  *users.Store
	tokens *Manager
}

// NewService wires the user store and token manager together.
func NewService(store *users.Store, tokens *Manager) *Service {
	return &Service{store: store, tokens: tokens}
}

// Register creates an account and returns a freshly issued token.
func (s *Service) Register(ctx context.Context, email, password string) (string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" || !strings.Contains(email, "@") {
		return "", ErrInvalidEmail
	}
	if len(password) < 8 {
		return "", ErrWeakPassword
	}

	hash, err := HashPassword(password)
	if err != nil {
		return "", err
	}

	user, err := s.store.CreateUser(ctx, email, hash)
	if err != nil {
		return "", err
	}
	return s.tokens.Generate(user.ID)
}

// Login verifies credentials and returns a token, or ErrInvalidCredentials.
func (s *Service) Login(ctx context.Context, email, password string) (string, error) {
	email = strings.TrimSpace(strings.ToLower(email))

	user, err := s.store.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, users.ErrNotFound) {
			return "", ErrInvalidCredentials
		}
		return "", err
	}

	if err := CheckPassword(user.PasswordHash, password); err != nil {
		return "", ErrInvalidCredentials
	}
	return s.tokens.Generate(user.ID)
}
