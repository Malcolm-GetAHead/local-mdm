package auth

import (
	"log/slog"
	"net/http"
)

type Middleware struct {
	validator *OIDCValidator
	logger    *slog.Logger
}

func NewMiddleware(validator *OIDCValidator, logger *slog.Logger) *Middleware {
	return &Middleware{
		validator: validator,
		logger:    logger,
	}
}

func (m *Middleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract token
		tokenString, err := ExtractBearerToken(r)
		if err != nil {
			m.logger.Warn("Missing or invalid authorization header", "error", err, "path", r.URL.Path)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		
		// Validate token
		user, err := m.validator.ValidateToken(tokenString)
		if err != nil {
			m.logger.Warn("Token validation failed", "error", err, "path", r.URL.Path)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		
		// Add user to context
		ctx := WithUser(r.Context(), user)
		
		m.logger.Debug("Authenticated request", "user_id", user.ID, "email", user.Email, "roles", user.Roles)
		
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (m *Middleware) RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, err := UserFromContext(r.Context())
			if err != nil {
				m.logger.Warn("No user in context", "path", r.URL.Path)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			
			if !user.HasAnyRole(roles...) {
				m.logger.Warn("Insufficient permissions", "user_id", user.ID, "required_roles", roles, "user_roles", user.Roles)
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			
			next.ServeHTTP(w, r)
		})
	}
}

func (m *Middleware) OptionalAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenString, err := ExtractBearerToken(r)
		if err == nil {
			if user, err := m.validator.ValidateToken(tokenString); err == nil {
				ctx := WithUser(r.Context(), user)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}
		
		// Continue without auth
		next.ServeHTTP(w, r)
	})
}
