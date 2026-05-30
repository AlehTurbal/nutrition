package auth

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ErrInvalidToken indicates a missing, malformed or expired token.
var ErrInvalidToken = errors.New("invalid token")

// Manager issues and verifies HS256 JWTs carrying the user ID as subject.
type Manager struct {
	secret []byte
	ttl    time.Duration
}

// NewManager builds a token manager from a secret and token lifetime.
func NewManager(secret string, ttl time.Duration) *Manager {
	return &Manager{secret: []byte(secret), ttl: ttl}
}

// Generate returns a signed token for the given user.
func (m *Manager) Generate(userID int64) (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Subject:   strconv.FormatInt(userID, 10),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// Parse verifies a token and returns the user ID it identifies.
func (m *Manager) Parse(tokenStr string) (int64, error) {
	claims := &jwt.RegisteredClaims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("%w: unexpected signing method", ErrInvalidToken)
		}
		return m.secret, nil
	})
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	userID, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: bad subject", ErrInvalidToken)
	}
	return userID, nil
}
