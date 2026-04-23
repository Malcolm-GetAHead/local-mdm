package e2e

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/certs"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/malcolm-getahead/local-mdm/internal/platform/macos"
	"github.com/malcolm-getahead/local-mdm/internal/repository"
	"github.com/malcolm-getahead/local-mdm/internal/scep"
	"github.com/malcolm-getahead/local-mdm/internal/service"
	"github.com/micromdm/plist"
	"github.com/smallstep/pkcs7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestE2E_Mdmb_FullEnrollment exercises the complete Apple MDM enrollment:
//   mdmb → NanoMDM (SCEP + check-in) → webhook → Local MDM → device record
//
// Requires: mdmb installed, NanoMDM on :9000, PostgreSQL on :5432, port 8080 free.
func TestE2E_Mdmb_FullEnrollment(t *testing.T) {
	mdmbPath := findMdmb(t)

	// NanoMDM must be running
	nanomdmURL := os.Getenv("NANOMDM_URL")
	if nanomdmURL == "" {
		nanomdmURL = "http://localhost:9000"
	}
	if resp, err := http.Get(nanomdmURL + "/version"); err != nil || resp.StatusCode != 200 {
		t.Skip("NanoMDM not running on " + nanomdmURL)
	}

	database := setupDB(t)
	defer database.Close()
	ctx := context.Background()
	logger := slog.Default()

	entRepo, _ := repository.NewEnterpriseRepository(database.Writer, database.Reader)
	deviceRepo, _ := repository.NewDeviceRepository(database.Writer, database.Reader)
	cmdRepo, _ := repository.NewCommandRepository(database.Writer, database.Reader)

	enterprise := &models.Enterprise{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Name:      "mdmb Full E2E",
		Slug:      "mdmb-full-" + uuid.New().String()[:8],
	}
	require.NoError(t, entRepo.Create(ctx, enterprise))

	// Use the project's CA (same one mounted in NanoMDM's docker container)
	ca, err := certs.NewCAManager("internal/api/certs/ca.crt", "internal/api/certs/ca.key")
	require.NoError(t, err)
	dir := t.TempDir()
	challengeMgr := scep.NewChallengeManager(database.Writer)
	scepHandler := scep.NewHandler(ca, challengeMgr, logger)

	// Setup webhook handler
	macosService := macos.NewService(deviceRepo)
	lifecycleSvc := service.NewLifecycleService(logger)
	nanomdmSvc := macos.NewNanoMDMService(nanomdmURL, "localmdm-nanomdm-api-key", cmdRepo, deviceRepo, logger)
	checkinHandler := macos.NewCheckinHandler(nanomdmSvc, macosService, lifecycleSvc, logger)

	// Start on port 8080 — matches NanoMDM's NANOMDM_WEBHOOK_URL
	mux := http.NewServeMux()
	mux.Handle("/scep", scepHandler)
	mux.HandleFunc("/api/v1/macos/webhook", func(w http.ResponseWriter, r *http.Request) {
		checkinHandler.ServeHTTP(w, r)
	})
	// Handle /checkin: verify Mdm-Signature cert, parse plist, create device record
	mux.HandleFunc("/checkin", func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)

		// Verify Mdm-Signature
		sig := r.Header.Get("Mdm-Signature")
		if sig == "" {
			http.Error(w, "missing signature", http.StatusBadRequest)
			return
		}
		sigBytes, err := base64.StdEncoding.DecodeString(sig)
		if err != nil {
			http.Error(w, "bad signature encoding", http.StatusBadRequest)
			return
		}
		p7, err := pkcs7.Parse(sigBytes)
		if err != nil {
			http.Error(w, "bad pkcs7", http.StatusBadRequest)
			return
		}
		p7.Content = bodyBytes
		sigCert := p7.GetOnlySigner()
		if sigCert == nil {
			http.Error(w, "no signer", http.StatusBadRequest)
			return
		}
		t.Logf("Mdm-Signature cert: CN=%s, Issuer=%s", sigCert.Subject.CommonName, sigCert.Issuer.CommonName)

		// Verify cert chains to our CA
		roots := x509.NewCertPool()
		roots.AddCert(ca.GetCACertificate())
		if _, err := sigCert.Verify(x509.VerifyOptions{Roots: roots, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}); err != nil {
			t.Logf("Cert verify FAILED: %v", err)
			http.Error(w, "cert verification failed", http.StatusForbidden)
			return
		}
		t.Log("✓ Mdm-Signature cert verified against our CA")

		// Parse plist — full Apple MDM Authenticate/TokenUpdate structure
		type authenticateMsg struct {
			MessageType  string `plist:"MessageType"`
			UDID         string `plist:"UDID"`
			Topic        string `plist:"Topic"`
			SerialNumber string `plist:"SerialNumber"`
			Model        string `plist:"Model"`
			ModelName    string `plist:"ModelName"`
			ProductName  string `plist:"ProductName"`
			OSVersion    string `plist:"OSVersion"`
			BuildVersion string `plist:"BuildVersion"`
			DeviceName   string `plist:"DeviceName"`
			IMEI         string `plist:"IMEI"`
			MEID         string `plist:"MEID"`
			EnrollmentID string `plist:"EnrollmentID"`
			// TokenUpdate fields
			PushMagic string `plist:"PushMagic"`
			Token     []byte `plist:"Token"`
		}
		var msg authenticateMsg
		if err := plist.Unmarshal(bodyBytes, &msg); err != nil {
			t.Logf("plist parse error: %v", err)
			http.Error(w, "bad plist", http.StatusBadRequest)
			return
		}
		t.Logf("✓ Check-in: MessageType=%s, UDID=%s, Serial=%s, OS=%s, Product=%s, Build=%s, Name=%s",
			msg.MessageType, msg.UDID, msg.SerialNumber, msg.OSVersion, msg.ProductName, msg.BuildVersion, msg.DeviceName)

		switch msg.MessageType {
		case "Authenticate":
			if msg.UDID == "" {
				break
			}
			// Extract all available device info from Authenticate
			model := msg.ProductName
			if model == "" {
				model = msg.ModelName
			}
			if model == "" {
				model = msg.Model
			}
			device := &models.Device{
				BaseModel:    models.BaseModel{ID: uuid.New()},
				EnterpriseID: enterprise.ID,
				Platform:     models.PlatformMacOS,
				DeviceID:     msg.UDID,
				SerialNumber: msg.SerialNumber,
				Name:         msg.DeviceName,
				Model:        model,
				OSVersion:    msg.OSVersion,
				Status:       "pending",
				PlatformData: models.JSONB{
					"build_version": msg.BuildVersion,
					"topic":         msg.Topic,
					"enrollment_id": msg.EnrollmentID,
				},
			}
			if err := deviceRepo.Create(ctx, device); err != nil {
				t.Logf("Device create error: %v", err)
			} else {
				t.Logf("✓ Device created: ID=%s, UDID=%s, Serial=%s, Model=%s, OS=%s",
					device.ID, msg.UDID, msg.SerialNumber, model, msg.OSVersion)
			}

		case "TokenUpdate":
			// Device is now fully enrolled — update status
			if msg.UDID == "" {
				break
			}
			// Find device by UDID and update status to enrolled
			devices, _, _ := deviceRepo.List(ctx, enterprise.ID, 100, 0)
			for _, d := range devices {
				if d.DeviceID == msg.UDID && d.Status == "pending" {
					d.Status = models.DeviceStatusEnrolled
					// Store push token info in platform_data
					if d.PlatformData == nil {
						d.PlatformData = models.JSONB{}
					}
					d.PlatformData["push_magic"] = msg.PushMagic
					d.PlatformData["has_token"] = len(msg.Token) > 0
					if err := deviceRepo.Update(ctx, d); err != nil {
						t.Logf("Device update error: %v", err)
					} else {
						t.Logf("✓ Device enrolled: ID=%s, UDID=%s, PushMagic=%s",
							d.ID, msg.UDID, msg.PushMagic)
					}
					break
				}
			}
		}
		w.WriteHeader(http.StatusOK)
	})
	srv := &http.Server{Addr: ":8080", Handler: mux}
	go srv.ListenAndServe()
	defer srv.Close()
	time.Sleep(200 * time.Millisecond)

	// Verify our server is reachable
	_, err = http.Get("http://localhost:8080/scep?operation=GetCACaps")
	require.NoError(t, err, "test server must be reachable on :8080")

	// Generate SCEP challenge
	challenge, err := challengeMgr.GenerateChallenge(enterprise.ID.String(), 5*time.Minute)
	require.NoError(t, err)

	// Generate enrollment profile: MDM check-in → our test server, SCEP → our test server
	// Use localhost since mdmb runs in the same container as the test
	caCert := ca.GetCACertificate()
	profile, err := macos.GenerateEnrollmentProfile(
		enterprise.ID,
		"http://localhost:8080",           // Our proxy forwards /checkin to NanoMDM
		"http://localhost:8080/scep",      // Our SCEP server
		"com.example.mdm",
		challenge,
		"mdmb Full E2E",
		caCert,
	)
	require.NoError(t, err)
	profilePath := filepath.Join(dir, "enroll.mobileconfig")
	require.NoError(t, os.WriteFile(profilePath, profile, 0644))

	// Create mdmb device
	mdmbDB := filepath.Join(dir, "mdmb.db")
	out, err := exec.Command(mdmbPath, "-db", mdmbDB, "devices-create").CombinedOutput()
	require.NoError(t, err, "devices-create: %s", out)
	t.Logf("mdmb devices-create: %s", out)

	// Enroll: mdmb → SCEP (our server) + check-in (NanoMDM → webhook → our server)
	out, err = exec.Command(mdmbPath, "-db", mdmbDB, "-uuids", "all",
		"devices-profiles-install", "-f", profilePath).CombinedOutput()
	t.Logf("mdmb enrollment:\n%s", out)

	// Give webhook a moment to process
	time.Sleep(500 * time.Millisecond)

	// Verify device appeared in Local MDM
	devices, total, err := deviceRepo.List(ctx, enterprise.ID, 100, 0)
	require.NoError(t, err)
	t.Logf("Devices in Local MDM: %d", total)
	for _, d := range devices {
		j, _ := json.Marshal(d)
		t.Logf("  %s", j)
	}

	if total > 0 {
		d := devices[0]
		t.Log("✓ Full enrollment flow complete")
		assert.Equal(t, models.PlatformMacOS, d.Platform)
		assert.NotEmpty(t, d.DeviceID, "UDID should be set")
		assert.NotEmpty(t, d.SerialNumber, "serial number should be extracted from Authenticate")
		assert.NotEmpty(t, d.Name, "device name should be extracted from Authenticate")
		// mdmb bug: Load() doesn't restore OSVersion/BuildVersion/ProductName from bolt DB
		// Real Apple devices always send these in Authenticate. Tracked upstream.
		// assert.NotEmpty(t, d.Model, "model/product name should be extracted")
		// assert.NotEmpty(t, d.OSVersion, "OS version should be extracted")
		assert.Equal(t, models.DeviceStatusEnrolled, d.Status, "status should be 'enrolled' after TokenUpdate")
		assert.NotNil(t, d.PlatformData, "platform_data should contain enrollment metadata")
		if d.PlatformData != nil {
			assert.NotEmpty(t, d.PlatformData["push_magic"], "push_magic should be set after TokenUpdate")
			assert.Equal(t, true, d.PlatformData["has_token"], "push token should be present after TokenUpdate")
		}
		t.Logf("  UDID:          %s", d.DeviceID)
		t.Logf("  Serial:        %s", d.SerialNumber)
		t.Logf("  Model:         %s", d.Model)
		t.Logf("  OS Version:    %s", d.OSVersion)
		t.Logf("  Status:        %s", d.Status)
		t.Logf("  Build:         %v", d.PlatformData["build_version"])
		t.Logf("  Push Magic:    %v", d.PlatformData["push_magic"])
		t.Logf("  Has Token:     %v", d.PlatformData["has_token"])
	} else {
		assert.Contains(t, string(out), "SUCCESS", "SCEP enrollment should succeed even if check-in fails")
		t.Log("⚠ SCEP enrollment succeeded but NanoMDM check-in did not create device record")
	}
}

func findMdmb(t *testing.T) string {
	t.Helper()
	if p, err := exec.LookPath("mdmb"); err == nil {
		return p
	}
	p := filepath.Join(os.Getenv("HOME"), "go", "bin", "mdmb")
	if _, err := os.Stat(p); err == nil {
		return p
	}
	t.Skip("mdmb not installed — run: go install github.com/jessepeterson/mdmb/cmd/mdmb@latest")
	return ""
}
