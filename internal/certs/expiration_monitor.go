package certs

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
)

// ExpirationMonitor monitors certificate expiration and logs warnings
type ExpirationMonitor struct {
	db               *sql.DB
	logger           *slog.Logger
	checkInterval    time.Duration
	warningThreshold time.Duration
	ticker           *time.Ticker
	stopChan         chan struct{}
	wg               sync.WaitGroup
	mu               sync.Mutex
	running          bool
	stopped          bool
	expiringCount    int // Number of certificates expiring within threshold
}

// NewExpirationMonitor creates a new certificate expiration monitor
func NewExpirationMonitor(db *sql.DB, logger *slog.Logger, checkInterval, warningThreshold time.Duration) *ExpirationMonitor {
	if checkInterval <= 0 {
		checkInterval = 24 * time.Hour // Default: check daily
	}
	if warningThreshold <= 0 {
		warningThreshold = 30 * 24 * time.Hour // Default: warn 30 days before
	}

	return &ExpirationMonitor{
		db:               db,
		logger:           logger,
		checkInterval:    checkInterval,
		warningThreshold: warningThreshold,
		stopChan:         make(chan struct{}),
	}
}

// Start begins monitoring certificate expiration
func (m *ExpirationMonitor) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.ticker = time.NewTicker(m.checkInterval)
	m.mu.Unlock()

	m.wg.Add(1)
	go m.run()
}

// Stop gracefully stops the monitor
func (m *ExpirationMonitor) Stop() {
	m.mu.Lock()
	if !m.running || m.stopped {
		m.mu.Unlock()
		return
	}
	m.running = false
	m.stopped = true
	ticker := m.ticker
	m.mu.Unlock()

	close(m.stopChan)
	if ticker != nil {
		ticker.Stop()
	}
	m.wg.Wait()
}

// run is the main monitoring loop
func (m *ExpirationMonitor) run() {
	defer m.wg.Done()

	// Check immediately on start
	m.checkExpiringCertificates()

	for {
		select {
		case <-m.ticker.C:
			m.checkExpiringCertificates()
		case <-m.stopChan:
			return
		}
	}
}

// checkExpiringCertificates queries and logs expiring certificates
func (m *ExpirationMonitor) checkExpiringCertificates() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	certs, err := m.getExpiringCertificates(ctx)
	if err != nil {
		if m.logger != nil {
			m.logger.Error("Failed to check expiring certificates", "error", err)
		}
		return
	}

	// Update metric
	m.mu.Lock()
	m.expiringCount = len(certs)
	m.mu.Unlock()

	for _, cert := range certs {
		daysRemaining := int(time.Until(cert.ExpiresAt).Hours() / 24)
		
		if m.logger != nil {
			m.logger.Warn("Certificate expiring soon",
				"certificate_id", cert.ID,
				"device_id", cert.DeviceID,
				"subject", cert.Subject,
				"serial_number", cert.SerialNumber,
				"expires_at", cert.ExpiresAt,
				"days_remaining", daysRemaining,
			)
		}
	}

	if m.logger != nil && len(certs) > 0 {
		m.logger.Info("Certificate expiration check completed",
			"expiring_count", len(certs),
			"threshold_days", int(m.warningThreshold.Hours()/24),
		)
	}
}

// getExpiringCertificates retrieves certificates expiring within threshold
func (m *ExpirationMonitor) getExpiringCertificates(ctx context.Context) ([]*models.Certificate, error) {
	threshold := time.Now().Add(m.warningThreshold)

	query := `
		SELECT id, device_id, cert_type, subject, serial_number, cert_data, 
		       issued_at, expires_at, revoked_at, created_at, updated_at
		FROM certificates
		WHERE expires_at <= $1
		  AND expires_at > NOW()
		  AND revoked_at IS NULL
		ORDER BY expires_at ASC`

	rows, err := m.db.QueryContext(ctx, query, threshold)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var certs []*models.Certificate
	for rows.Next() {
		cert := &models.Certificate{}
		var deviceID sql.NullString

		err := rows.Scan(
			&cert.ID, &deviceID, &cert.CertType, &cert.Subject, &cert.SerialNumber,
			&cert.CertData, &cert.IssuedAt, &cert.ExpiresAt, &cert.RevokedAt,
			&cert.CreatedAt, &cert.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		if deviceID.Valid {
			id, _ := uuid.Parse(deviceID.String)
			cert.DeviceID = &id
		}

		certs = append(certs, cert)
	}

	return certs, rows.Err()
}

// GetExpiringCount returns the current count of expiring certificates
func (m *ExpirationMonitor) GetExpiringCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.expiringCount
}
