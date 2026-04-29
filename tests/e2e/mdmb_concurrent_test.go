package e2e

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/certs"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/malcolm-getahead/local-mdm/internal/platform/macos"
	"github.com/malcolm-getahead/local-mdm/internal/repository"
	"github.com/malcolm-getahead/local-mdm/internal/scep"
	"github.com/malcolm-getahead/local-mdm/internal/testutil"
	"github.com/micromdm/plist"
	"github.com/smallstep/pkcs7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestE2E_Mdmb_ConcurrentEnrollment enrolls 5 simulated Apple devices
// concurrently and verifies each gets a unique device record.
func TestE2E_Mdmb_ConcurrentEnrollment(t *testing.T) {
	mdmbPath := findMdmb(t)

	database := testutil.ConnectDB(t)
	ctx := context.Background()
	logger := slog.Default()

	entRepo, _ := repository.NewEnterpriseRepository(database.Writer, database.Reader)
	deviceRepo, _ := repository.NewDeviceRepository(database.Writer, database.Reader)

	enterprise := &models.Enterprise{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Name:      "Concurrent Enrollment Test",
		Slug:      "concurrent-" + uuid.New().String()[:8],
	}
	require.NoError(t, entRepo.Create(ctx, enterprise))
	t.Cleanup(func() { database.Writer.Exec("DELETE FROM enterprises WHERE id = $1", enterprise.ID) })

	ca, err := certs.NewCAManager(
		projectPath(t, "internal/api/certs/ca.crt"),
		projectPath(t, "internal/api/certs/ca.key"),
	)
	require.NoError(t, err)
	challengeMgr := scep.NewChallengeManager(database.Writer)
	scepHandler := scep.NewHandler(ca, challengeMgr, logger)

	// Shared mutex for device creation (DB writes)
	var mu sync.Mutex

	mux := http.NewServeMux()
	mux.Handle("/scep", scepHandler)
	mux.HandleFunc("/checkin", func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		sig := r.Header.Get("Mdm-Signature")
		if sig == "" {
			http.Error(w, "missing signature", http.StatusBadRequest)
			return
		}
		sigBytes, _ := base64.StdEncoding.DecodeString(sig)
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
		roots := x509.NewCertPool()
		roots.AddCert(ca.GetCACertificate())
		if _, err := sigCert.Verify(x509.VerifyOptions{Roots: roots, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}); err != nil {
			http.Error(w, "cert verify failed", http.StatusForbidden)
			return
		}
		type checkinMsg struct {
			MessageType string `plist:"MessageType"`
			UDID        string `plist:"UDID"`
		}
		var msg checkinMsg
		if err := plist.Unmarshal(bodyBytes, &msg); err != nil {
			http.Error(w, "bad plist", http.StatusBadRequest)
			return
		}
		if msg.MessageType == "Authenticate" && msg.UDID != "" {
			mu.Lock()
			defer mu.Unlock()
			device := &models.Device{
				BaseModel:    models.BaseModel{ID: uuid.New()},
				EnterpriseID: enterprise.ID,
				Platform:     models.PlatformMacOS,
				DeviceID:     msg.UDID,
				Status:       "pending",
			}
			if err := deviceRepo.Create(ctx, device); err != nil {
				t.Logf("Device create error for %s: %v", msg.UDID, err)
			}
		}
		w.WriteHeader(http.StatusOK)
	})

	srv := &http.Server{Addr: ":8080", Handler: mux}
	go srv.ListenAndServe()
	defer srv.Close()
	time.Sleep(200 * time.Millisecond)

	const numDevices = 5
	dir := t.TempDir()

	// Generate per-device challenges and profiles
	type deviceSetup struct {
		profilePath string
		mdmbDB      string
	}
	setups := make([]deviceSetup, numDevices)
	for i := 0; i < numDevices; i++ {
		challenge, err := challengeMgr.GenerateChallenge(fmt.Sprintf("device-%d", i), 5*time.Minute)
		require.NoError(t, err)
		profile, err := macos.GenerateEnrollmentProfile(
			enterprise.ID, "http://localhost:8080", "http://localhost:8080/scep",
			"com.example.mdm", challenge, "Concurrent Test", ca.GetCACertificate())
		require.NoError(t, err)
		pPath := filepath.Join(dir, fmt.Sprintf("enroll-%d.mobileconfig", i))
		require.NoError(t, os.WriteFile(pPath, profile, 0644))
		setups[i] = deviceSetup{profilePath: pPath, mdmbDB: filepath.Join(dir, fmt.Sprintf("mdmb-%d.db", i))}
	}

	// Enroll all 5 devices concurrently
	var wg sync.WaitGroup
	results := make([]string, numDevices)
	errors := make([]error, numDevices)

	for i := 0; i < numDevices; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			s := setups[idx]
			// Create device
			out, err := exec.Command(mdmbPath, "-db", s.mdmbDB, "devices-create").CombinedOutput()
			if err != nil {
				errors[idx] = fmt.Errorf("create: %s", out)
				return
			}
			// Enroll
			out, err = exec.Command(mdmbPath, "-db", s.mdmbDB, "-uuids", "all",
				"devices-profiles-install", "-f", s.profilePath).CombinedOutput()
			results[idx] = string(out)
			if err != nil {
				errors[idx] = fmt.Errorf("enroll: %s", out)
			}
		}(i)
	}
	wg.Wait()
	time.Sleep(500 * time.Millisecond)

	// Report results
	for i := 0; i < numDevices; i++ {
		if errors[i] != nil {
			t.Logf("Device %d: ERROR: %v", i, errors[i])
		} else {
			t.Logf("Device %d: OK", i)
		}
	}

	// Verify all 5 devices exist with unique UDIDs
	devices, total, err := deviceRepo.List(ctx, enterprise.ID, 100, 0)
	require.NoError(t, err)
	t.Logf("Total devices enrolled: %d", total)

	assert.Equal(t, numDevices, total, "expected %d devices, got %d", numDevices, total)

	// Verify uniqueness
	udids := make(map[string]bool)
	for _, d := range devices {
		assert.Equal(t, models.PlatformMacOS, d.Platform)
		assert.NotEmpty(t, d.DeviceID)
		if udids[d.DeviceID] {
			t.Errorf("duplicate UDID: %s", d.DeviceID)
		}
		udids[d.DeviceID] = true
		t.Logf("  ✓ Device %s (UDID: %s)", d.ID, d.DeviceID)
	}
	assert.Len(t, udids, numDevices, "all UDIDs should be unique")
	t.Logf("✓ %d devices enrolled concurrently, all unique", numDevices)
}
