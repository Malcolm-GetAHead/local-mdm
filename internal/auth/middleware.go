package auth

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/malcolm-getahead/local-mdm/internal/audit"
)

// TokenValidator validates API tokens and returns the associated user.
type TokenValidator interface {
	Validate(ctx context.Context, plaintext string) (*AuthUser, error)
}

// Middleware provides HTTP middleware for authentication and authorization.
type Middleware struct {
	validator      *OIDCValidator
	tokenValidator TokenValidator
	logger         *slog.Logger
	auditLogger    audit.AuditLogger
}

// NewMiddleware creates a new authentication middleware instance.
func NewMiddleware(validator *OIDCValidator, logger *slog.Logger) *Middleware {
	return &Middleware{
		validator: validator,
		logger:    logger,
	}
}

// SetTokenValidator sets the API token validator for dual auth support.
func (m *Middleware) SetTokenValidator(tv TokenValidator) {
	m.tokenValidator = tv
}

// ValidateTokenDirect validates a token string and returns the AuthUser.
// Used by the web dashboard OIDC callback to validate tokens outside of HTTP middleware.
func (m *Middleware) ValidateTokenDirect(token string) (*AuthUser, error) {
	return m.validator.ValidateToken(token)
}

// SetAuditLogger sets the audit logger for the middleware.
// This should be called after creating the middleware to enable audit logging.
func (m *Middleware) SetAuditLogger(auditLogger audit.AuditLogger) {
	m.auditLogger = auditLogger
}

func (m *Middleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := GetRequestID(r.Context())
		
		// Extract token
		tokenString, err := ExtractBearerToken(r)
		if err != nil {
			m.logger.Warn("Missing or invalid authorization header", "error", err, "path", r.URL.Path, "request_id", requestID)
			if m.auditLogger != nil {
				_ = m.auditLogger.Log(r.Context(), audit.Event{
					Action: "auth.failure", ResourceType: "user",
					Details:   map[string]interface{}{"reason": "missing_token", "path": r.URL.Path},
					IPAddress: getIP(r), UserAgent: r.UserAgent(),
				})
			}
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		
		// Try API token auth first if it looks like an lmdm_ token
		if m.tokenValidator != nil && strings.HasPrefix(tokenString, "lmdm_") {
			user, err := m.tokenValidator.Validate(r.Context(), tokenString)
			if err == nil {
				ctx := WithUser(r.Context(), user)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			m.logger.Warn("API token validation failed", "error", err, "request_id", requestID)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		
		// Fall through to OIDC validation
		user, err := m.validator.ValidateToken(tokenString)
		if err != nil {
			m.logger.Warn("Token validation failed", "error", err, "path", r.URL.Path, "request_id", requestID)
			if m.auditLogger != nil {
				_ = m.auditLogger.Log(r.Context(), audit.Event{
					Action: "auth.failure", ResourceType: "user",
					Details:   map[string]interface{}{"reason": "invalid_token", "path": r.URL.Path},
					IPAddress: getIP(r), UserAgent: r.UserAgent(),
				})
			}
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		
		ctx := WithUser(r.Context(), user)
		m.logger.Debug("Authenticated request", "user_id", user.ID, "email", user.Email, "roles", user.Roles, "request_id", requestID)
		
		if m.auditLogger != nil {
			_ = m.auditLogger.Log(ctx, audit.Event{
				Action: "auth.success", ResourceType: "user",
				Details:   map[string]interface{}{"user_id": user.ID, "email": user.Email, "path": r.URL.Path},
				IPAddress: getIP(r), UserAgent: r.UserAgent(),
			})
		}
		
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (m *Middleware) RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := GetRequestID(r.Context())
			user, err := UserFromContext(r.Context())
			if err != nil {
				m.logger.Warn("No user in context", "path", r.URL.Path, "request_id", requestID)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			
			if !user.HasAnyRole(roles...) {
				m.logger.Warn("Insufficient permissions", "user_id", user.ID, "required_roles", roles, "user_roles", user.Roles, "request_id", requestID)
				
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

// GetRequestID extracts the request ID from the context
func GetRequestID(ctx context.Context) string {
	if requestID, ok := ctx.Value(requestIDKey).(string); ok {
		return requestID
	}
	return ""
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
