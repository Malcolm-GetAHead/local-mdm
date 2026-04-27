package constants

// Size constants
const (
	// OneMB is 1 megabyte in bytes
	OneMB = 1 << 20 // 1,048,576 bytes
	
	// MaxRequestBodySize is the maximum size for HTTP request bodies
	MaxRequestBodySize = OneMB
	
	// MaxJWKSResponseSize is the maximum size for JWKS responses
	MaxJWKSResponseSize = OneMB
)

// Timeout constants
const (
	// DefaultQueryTimeout is the default database query timeout
	DefaultQueryTimeout = 30 // seconds
	
	// DefaultRequestTimeout is the default HTTP request timeout
	DefaultRequestTimeout = 30 // seconds
)

// Limit constants
const (
	// MaxDatabaseConnections is the maximum number of database connections
	// This matches PostgreSQL's default connection limit
	MaxDatabaseConnections = 100
	
	// DefaultRateLimit is the default rate limit for API endpoints
	DefaultRateLimit = 100 // requests per window
	
	// MaxActionLength is the maximum length for audit log action strings
	MaxActionLength = 100 // characters
	
	// MaxRateLimiterEntries is the maximum number of IPs tracked by rate limiter
	MaxRateLimiterEntries = 10000
)

// Pagination constants
const (
	// MaxPageSize is the maximum number of records per page
	MaxPageSize = 1000
	
	// DefaultPageSize is the default number of records per page
	DefaultPageSize = 100

	// MaxBatchSize is the maximum number of records to fetch in a single batch operation
	// (e.g., compliance evaluation across all devices in a group/enterprise)
	MaxBatchSize = 10000
)
