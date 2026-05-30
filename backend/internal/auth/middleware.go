package auth

import (
	"context"
	"net/http"
	"strings"
)

type ctxKey int

const userIDKey ctxKey = iota

// Middleware authenticates requests via a Bearer token and stores the user ID in
// the request context. Unauthenticated requests receive 401.
func (m *Manager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		token, ok := strings.CutPrefix(header, "Bearer ")
		if !ok || token == "" {
			http.Error(w, `{"error":"missing bearer token"}`, http.StatusUnauthorized)
			return
		}

		userID, err := m.Parse(token)
		if err != nil {
			http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), userIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// UserIDFromContext returns the authenticated user ID set by Middleware.
func UserIDFromContext(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(userIDKey).(int64)
	return id, ok
}
