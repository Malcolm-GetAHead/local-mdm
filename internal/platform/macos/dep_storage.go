package macos

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"database/sql"

	"github.com/malcolm-getahead/local-mdm/internal/apperrors"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"

	"github.com/micromdm/nanodep/client"
)

// DEPStorage provides encrypted storage for DEP tokens and configuration.
// It implements the nanoDEP storage interfaces with pgcrypto encryption.
type DEPStorage struct {
	db            *sql.DB
	encryptionKey string
}

// NewDEPStorage creates a new encrypted DEP storage.
func NewDEPStorage(db *sql.DB, encryptionKey string) *DEPStorage {
	return &DEPStorage{db: db, encryptionKey: encryptionKey}
}

// --- AuthTokens (nanoDEP client.AuthTokensRetriever) ---

func (s *DEPStorage) RetrieveAuthTokens(ctx context.Context, name string) (*client.OAuth1Tokens, error) {
	query := `
		SELECT
			pgp_sym_decrypt(consumer_key, $2),
			pgp_sym_decrypt(consumer_secret, $2),
			pgp_sym_decrypt(access_token, $2),
			pgp_sym_decrypt(access_secret, $2),
			access_token_expiry
		FROM dep_names WHERE name = $1`

	var tokens client.OAuth1Tokens
	var ck, cs, at, as sql.NullString
	var expiry sql.NullTime

	err := s.db.QueryRowContext(ctx, query, name, s.encryptionKey).Scan(&ck, &cs, &at, &as, &expiry)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("DEP name not found: %s: %w", name, apperrors.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve auth tokens: %w", err)
	}

	tokens.ConsumerKey = ck.String
	tokens.ConsumerSecret = cs.String
	tokens.AccessToken = at.String
	tokens.AccessSecret = as.String
	if expiry.Valid {
		tokens.AccessTokenExpiry = expiry.Time
	}

	return &tokens, nil
}

// StoreAuthTokens stores encrypted OAuth tokens for a DEP name.
func (s *DEPStorage) StoreAuthTokens(ctx context.Context, name string, tokens *client.OAuth1Tokens) error {
	query := `
		INSERT INTO dep_names (name, consumer_key, consumer_secret, access_token, access_secret, access_token_expiry)
		VALUES ($1, pgp_sym_encrypt($2, $7), pgp_sym_encrypt($3, $7), pgp_sym_encrypt($4, $7), pgp_sym_encrypt($5, $7), $6)
		ON CONFLICT (name) DO UPDATE SET
			consumer_key = pgp_sym_encrypt($2, $7),
			consumer_secret = pgp_sym_encrypt($3, $7),
			access_token = pgp_sym_encrypt($4, $7),
			access_secret = pgp_sym_encrypt($5, $7),
			access_token_expiry = $6`

	_, err := s.db.ExecContext(ctx, query,
		name, tokens.ConsumerKey, tokens.ConsumerSecret,
		tokens.AccessToken, tokens.AccessSecret,
		tokens.AccessTokenExpiry, s.encryptionKey,
	)
	return err
}

// --- Config (nanoDEP client.ConfigRetriever) ---

func (s *DEPStorage) RetrieveConfig(ctx context.Context, name string) (*client.Config, error) {
	query := `SELECT config_base_url FROM dep_names WHERE name = $1`
	var baseURL sql.NullString
	err := s.db.QueryRowContext(ctx, query, name).Scan(&baseURL)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("DEP name not found: %s: %w", name, apperrors.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve config: %w", err)
	}
	cfg := &client.Config{}
	if baseURL.Valid {
		cfg.BaseURL = baseURL.String
	}
	return cfg, nil
}

// StoreConfig stores the DEP configuration for a name.
func (s *DEPStorage) StoreConfig(ctx context.Context, name string, cfg *client.Config) error {
	query := `
		INSERT INTO dep_names (name, config_base_url) VALUES ($1, $2)
		ON CONFLICT (name) DO UPDATE SET config_base_url = $2`
	_, err := s.db.ExecContext(ctx, query, name, cfg.BaseURL)
	return err
}

// --- Cursor (nanoDEP sync.CursorStorage) ---

func (s *DEPStorage) RetrieveCursor(ctx context.Context, name string) (string, error) {
	query := `SELECT syncer_cursor FROM dep_names WHERE name = $1`
	var cursor sql.NullString
	err := s.db.QueryRowContext(ctx, query, name).Scan(&cursor)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to retrieve cursor: %w", err)
	}
	return cursor.String, nil
}

func (s *DEPStorage) StoreCursor(ctx context.Context, name string, cursor string) error {
	query := `UPDATE dep_names SET syncer_cursor = $2 WHERE name = $1`
	_, err := s.db.ExecContext(ctx, query, name, cursor)
	return err
}

// --- Assigner Profile (nanoDEP sync.AssignerProfileRetriever) ---

func (s *DEPStorage) RetrieveAssignerProfile(ctx context.Context, name string) (string, time.Time, error) {
	query := `SELECT assigner_profile_uuid, assigner_profile_uuid_at FROM dep_names WHERE name = $1`
	var profileUUID sql.NullString
	var modTime sql.NullTime
	err := s.db.QueryRowContext(ctx, query, name).Scan(&profileUUID, &modTime)
	if err == sql.ErrNoRows {
		return "", time.Time{}, nil
	}
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to retrieve assigner profile: %w", err)
	}
	t := time.Time{}
	if modTime.Valid {
		t = modTime.Time
	}
	return profileUUID.String, t, nil
}

// StoreAssignerProfile stores the assigner profile UUID for a DEP name.
func (s *DEPStorage) StoreAssignerProfile(ctx context.Context, name string, profileUUID string) error {
	query := `UPDATE dep_names SET assigner_profile_uuid = $2, assigner_profile_uuid_at = NOW() WHERE name = $1`
	_, err := s.db.ExecContext(ctx, query, name, profileUUID)
	return err
}

// --- Token PKI ---

// GenerateTokenPKI generates a new keypair and stores it as staging.
func (s *DEPStorage) GenerateTokenPKI(ctx context.Context, name string, cn string, validityDays int) ([]byte, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate RSA key: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Duration(validityDays) * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})

	query := `
		INSERT INTO dep_names (name, tokenpki_staging_cert_pem, tokenpki_staging_key_pem)
		VALUES ($1, $2, pgp_sym_encrypt($3, $4))
		ON CONFLICT (name) DO UPDATE SET
			tokenpki_staging_cert_pem = $2,
			tokenpki_staging_key_pem = pgp_sym_encrypt($3, $4)`

	_, err = s.db.ExecContext(ctx, query, name, string(certPEM), string(keyPEM), s.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to store token PKI: %w", err)
	}

	return certPEM, nil
}

// RetrieveCurrentTokenPKI retrieves the current (non-staging) token PKI cert and key.
func (s *DEPStorage) RetrieveCurrentTokenPKI(ctx context.Context, name string) ([]byte, []byte, error) {
	query := `SELECT tokenpki_cert_pem, pgp_sym_decrypt(tokenpki_key_pem, $2) FROM dep_names WHERE name = $1`
	var certPEM, keyPEM sql.NullString
	err := s.db.QueryRowContext(ctx, query, name, s.encryptionKey).Scan(&certPEM, &keyPEM)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to retrieve token PKI: %w", err)
	}
	return []byte(certPEM.String), []byte(keyPEM.String), nil
}

// RetrieveStagingTokenPKI retrieves the staging token PKI cert and key.
func (s *DEPStorage) RetrieveStagingTokenPKI(ctx context.Context, name string) ([]byte, []byte, error) {
	query := `SELECT tokenpki_staging_cert_pem, pgp_sym_decrypt(tokenpki_staging_key_pem, $2) FROM dep_names WHERE name = $1`
	var certPEM, keyPEM sql.NullString
	err := s.db.QueryRowContext(ctx, query, name, s.encryptionKey).Scan(&certPEM, &keyPEM)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to retrieve staging token PKI: %w", err)
	}
	return []byte(certPEM.String), []byte(keyPEM.String), nil
}

// UpstageTokenPKI promotes staging PKI to current.
func (s *DEPStorage) UpstageTokenPKI(ctx context.Context, name string) error {
	query := `
		UPDATE dep_names SET
			tokenpki_cert_pem = tokenpki_staging_cert_pem,
			tokenpki_key_pem = tokenpki_staging_key_pem,
			tokenpki_staging_cert_pem = NULL,
			tokenpki_staging_key_pem = NULL
		WHERE name = $1`
	_, err := s.db.ExecContext(ctx, query, name)
	return err
}

// --- DEP Devices ---

// StoreSyncedDevice stores or updates a device from a DEP sync.
func (s *DEPStorage) StoreSyncedDevice(ctx context.Context, depName, serialNumber string, deviceData map[string]interface{}) error {
	dataJSON, err := json.Marshal(deviceData)
	if err != nil {
		return fmt.Errorf("failed to marshal device data: %w", err)
	}
	query := `
		INSERT INTO dep_devices (serial_number, dep_name, device_data, synced_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (serial_number, dep_name) DO UPDATE SET
			device_data = $3, synced_at = NOW()`
	_, err = s.db.ExecContext(ctx, query, serialNumber, depName, dataJSON)
	return err
}

// ListDEPDevices lists synced DEP devices for a DEP name.
func (s *DEPStorage) ListDEPDevices(ctx context.Context, depName string, limit, offset int) ([]DEPDevice, int, error) {
	countQuery := `SELECT COUNT(*) FROM dep_devices WHERE dep_name = $1`
	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, depName).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `
		SELECT serial_number, dep_name, profile_uuid, profile_status, device_data, synced_at, assigned_at
		FROM dep_devices WHERE dep_name = $1
		ORDER BY synced_at DESC LIMIT $2 OFFSET $3`

	rows, err := s.db.QueryContext(ctx, query, depName, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var devices []DEPDevice
	for rows.Next() {
		var d DEPDevice
		var dataBytes []byte
		if err := rows.Scan(&d.SerialNumber, &d.DEPName, &d.ProfileUUID, &d.ProfileStatus, &dataBytes, &d.SyncedAt, &d.AssignedAt); err != nil {
			return nil, 0, err
		}
		if len(dataBytes) > 0 {
			if err := json.Unmarshal(dataBytes, &d.DeviceData); err != nil {
				return nil, 0, fmt.Errorf("failed to unmarshal device data: %w", err)
			}
		}
		devices = append(devices, d)
	}
	return devices, total, rows.Err()
}

// DEPDevice represents a device synced from Apple DEP.
type DEPDevice struct {
	SerialNumber  string                 `json:"serial_number"`
	DEPName       string                 `json:"dep_name"`
	ProfileUUID   *string                `json:"profile_uuid,omitempty"`
	ProfileStatus string                 `json:"profile_status"`
	DeviceData    map[string]interface{} `json:"device_data"`
	SyncedAt      time.Time              `json:"synced_at"`
	AssignedAt    *time.Time             `json:"assigned_at,omitempty"`
}
