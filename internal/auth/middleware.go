package auth

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/malcolm-getahead/local-mdm/internal/audit"
)

type Middleware struct {
	validator   *OIDCValidator
	logger      *slog.Logger
	auditLogger *audit.Logger
}

func NewMiddleware(validator *OIDCValidator, logger *slog.Logger) *Middleware {
	return &Middleware{
		validator: validator,
		logger:    logger,
	}
}

func (m *Middleware) SetAuditLogger(auditLogger *audit.Logger) {
	m.auditLogger = auditLogger
}

func (m *Middleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract token
		tokenString, err := ExtractBearerToken(r)
		if err != nil {
			m.logger.Warn("Missing or invalid authorization header", "error", err, "path", r.URL.Path)
			
			// Log authentication failure
			if m.auditLogger != nil {
				_ = m.auditLogger.Log(r.Context(), audit.Event{
					Action:       "auth.failure",
					ResourceType: "user",
					Details: map[string]interface{}{
						"reason": "missing_token",
						"path":   r.URL.Path,
					},
					IPAddress: getIP(r),
					UserAgent: r.UserAgent(),
				})
			}
			
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		
		// Validate token
		user, err := m.validator.ValidateToken(tokenString)
		if err != nil {
			m.logger.Warn("Token validation failed", "error", err, "path", r.URL.Path)
			
			// Log authentication failure
			if m.auditLogger != nil {
				_ = m.auditLogger.Log(r.Context(), audit.Event{
					Action:       "auth.failure",
					ResourceType: "user",
					Details: map[string]interface{}{
						"reason": "invalid_token",
						"path":   r.URL.Path,
					},
					IPAddress: getIP(r),
					UserAgent: r.UserAgent(),
				})
			}
			
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		
		// Add user to context
		ctx := WithUser(r.Context(), user)
		
		m.logger.Debug("Authenticated request", "user_id", user.ID, "email", user.Email, "roles", user.Roles)
		
		// Log successful authentication
		if m.auditLogger != nil {
			_ = m.auditLogger.Log(ctx, audit.Event{
				Action:       "auth.success",
				ResourceType: "user",
				Details: map[string]interface{}{
					"user_id": user.ID,
					"email":   user.Email,
					"path":    r.URL.Path,
				},
				IPAddress: getIP(r),
				UserAgent: r.UserAgent(),
			})
		}
		
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
				
				// Log authorization failure
				if m.auditLogger != nil {
					_ = m.auditLogger.Log(r.Context(), audit.Event{
						Action:       "auth.access_denied",
						ResourceType: "user",
						Details: map[string]interface{}{
							"user_id":        user.ID,
							"email":          user.Email,
							"required_roles": roles,
							"user_roles":     user.Roles,
							"path":           r.URL.Path,
						},
						IPAddress: getIP(r),
						UserAgent: r.UserAgent(),
					})
				}
				
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

// HealthCheck verifies Keycloak connectivity
func (m *Middleware) HealthCheck(ctx context.Context) error {
	return m.validator.HealthCheck(ctx)
}

// getIP extracts the client IP address from the request
func getIP(r *http.Request) net.IP {
	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			ip := strings.TrimSpace(ips[0])
			if parsed := net.ParseIP(ip); parsed != nil {
				return parsed
			}
		}
	}
	
	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		if parsed := net.ParseIP(xri); parsed != nil {
			return parsed
		}
	}
	
	// Fall back to RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return nil
	}
	return net.ParseIP(host)
}
