package cbic_rates_test

import (
	"path/filepath"
	"testing"

	"github.com/janayuv/import-manager-next/backend/app/bootstrap"
	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/services/cbic_rates"
)

func setupCBICEnv(t *testing.T) (*bootstrap.Runtime, *cbic_rates.Service) {
	t.Helper()
	t.Setenv("IMPORT_MANAGER_DB_KEY", "cbic-test-key-0123456789abcdef0123")
	t.Setenv("IMPORT_MANAGER_KEYRING_FILE", filepath.Join(t.TempDir(), "keyring.json"))
	dir := filepath.Join(t.TempDir(), "appdata")
	rt, err := bootstrap.Initialize(nil, dir)
	if err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}
	t.Cleanup(func() { _ = rt.Shutdown() })

	if _, err := rt.Security.CompleteAdminSetup("admin", "StrongPass1!"); err != nil {
		t.Fatalf("admin setup failed: %v", err)
	}

	svc := cbic_rates.NewService(rt.DB.DB(), rt.Security, rt.Audit)
	return rt, svc
}

func TestCBICExchangeRateServiceLifecycle(t *testing.T) {
	_, svc := setupCBICEnv(t)

	// 1. Verify seeded rates exist
	list, err := svc.List("USD")
	if err != nil {
		t.Fatalf("list USD rates: %v", err)
	}
	if len(list) == 0 {
		t.Fatalf("expected seeded USD rate, got 0")
	}
	if list[0].ExchangeRate != 84.15 {
		t.Fatalf("expected 84.15, got %f", list[0].ExchangeRate)
	}

	// 2. Query effective rate on a target date
	rate, err := svc.GetEffectiveRate("USD", "2026-05-15")
	if err != nil {
		t.Fatalf("get effective rate: %v", err)
	}
	if rate.ExchangeRate != 84.15 || rate.CurrencyCode != "USD" {
		t.Fatalf("unexpected rate: %+v", rate)
	}

	// 3. Create a newer rate notification effective 2026-06-01
	effTo := "2026-06-30"
	created, err := svc.Create(models.CBICExchangeRateInput{
		CurrencyCode:       "USD",
		ExchangeRate:       84.60,
		EffectiveFrom:      "2026-06-01",
		EffectiveTo:        &effTo,
		NotificationNumber: "Notification No. 45/2026-Customs (N.T.)",
		SourceDocument:     "Official Gazette Notification",
		VerificationStatus: "Verified",
		Notes:              "Mid-year tariff adjustment",
	})
	if err != nil {
		t.Fatalf("create rate failed: %v", err)
	}
	if created.ID == 0 || created.ExchangeRate != 84.60 {
		t.Fatalf("unexpected created rate: %+v", created)
	}

	// 4. Test date range routing
	// Prior to 2026-06-01 should return baseline 84.15
	rateMay, err := svc.GetEffectiveRate("USD", "2026-05-20")
	if err != nil || rateMay.ExchangeRate != 84.15 {
		t.Fatalf("expected 84.15 for May, got %f (err=%v)", rateMay.ExchangeRate, err)
	}

	// On 2026-06-15 should return new 84.60
	rateJune, err := svc.GetEffectiveRate("USD", "2026-06-15")
	if err != nil || rateJune.ExchangeRate != 84.60 {
		t.Fatalf("expected 84.60 for June, got %f (err=%v)", rateJune.ExchangeRate, err)
	}

	// 5. Delete newly created rate
	if err := svc.Delete(created.ID); err != nil {
		t.Fatalf("delete rate failed: %v", err)
	}
	rateAfterDelete, err := svc.GetEffectiveRate("USD", "2026-06-15")
	if err != nil || rateAfterDelete.ExchangeRate != 84.15 {
		t.Fatalf("expected fallback to 84.15 after delete, got %f", rateAfterDelete.ExchangeRate)
	}
}
