package backup_test

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/pbkdf2"

	"github.com/janayuv/import-manager-next/backend/app/bootstrap"
	"github.com/janayuv/import-manager-next/backend/models"
)

func setupBackupEnv(t *testing.T) *bootstrap.Runtime {
	t.Helper()
	t.Setenv("IMPORT_MANAGER_DB_KEY", "backup-test-key-0123456789abcdef0123")
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
	return rt
}

func encryptLegacyIMBK1(passphrase string, plain []byte) []byte {
	salt := make([]byte, 16)
	_, _ = rand.Read(salt)
	nonce := make([]byte, 12)
	_, _ = rand.Read(nonce)
	key := pbkdf2.Key([]byte(passphrase), salt, 100_000, 32, sha256.New)
	blk, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCM(blk)
	ct := gcm.Seal(nil, nonce, plain, nil)
	out := append([]byte("IMBK1"), salt...)
	out = append(out, nonce...)
	out = append(out, ct...)
	return out
}

func TestBackupIMBK2IMKeyRestoreSafetyAndSchedules(t *testing.T) {
	rt := setupBackupEnv(t)

	baselineList, err := rt.Suppliers.List(models.SupplierPageRequest{Page: 1, PageSize: 100})
	if err != nil {
		t.Fatalf("list baseline suppliers: %v", err)
	}
	baselineCount := baselineList.TotalCount

	s1, err := rt.Suppliers.Create(models.SupplierInput{
		SupplierName: "Before Snapshot Exports",
		Country:      "Germany",
		Email:        "snap@before.de",
		IsActive:     true,
	})
	if err != nil {
		t.Fatalf("create supplier: %v", err)
	}

	// 1. Create IMBK2 encrypted backup + save passphrase to isolated keyring
	rec, err := rt.Backup.CreateBackup(models.CreateBackupRequest{
		Passphrase:    "BackupPass#2026",
		SavePassToKey: true,
		Notes:         "Pre-upgrade snapshot",
	})
	if err != nil {
		t.Fatalf("CreateBackup failed: %v", err)
	}
	if !rec.SidecarExists || rec.SizeBytes <= 0 || len(rec.SHA256) != 64 {
		t.Fatalf("unexpected backup record: %+v", rec)
	}
	hasPass, err := rt.Backup.HasSavedBackupPassphrase()
	if err != nil || !hasPass {
		t.Fatalf("expected saved backup passphrase in keyring, got %v (err=%v)", hasPass, err)
	}

	// 2. Export & re-import .imkey bundle
	imkeyPath := filepath.Join(t.TempDir(), "vault.imkey")
	expRes, err := rt.Backup.ExportIMKey(imkeyPath, "", "TransportKey#99")
	if err != nil || !expRes.HasBackupPass {
		t.Fatalf("ExportIMKey failed: %+v err=%v", expRes, err)
	}
	if err := rt.Backup.ClearBackupPassphrase(); err != nil {
		t.Fatalf("ClearBackupPassphrase failed: %v", err)
	}
	impRes, err := rt.Backup.ImportIMKey(imkeyPath, "TransportKey#99")
	if err != nil || !impRes.StoredInKeyring {
		t.Fatalf("ImportIMKey failed: %+v err=%v", impRes, err)
	}

	// 3. Dry-run verification succeeds using imported passphrase from keyring
	drySummary, err := rt.Backup.RestoreBackup(models.RestoreBackupRequest{
		BackupPath:    rec.Path,
		DryRun:        true,
		VerifySidecar: true,
	})
	if err != nil {
		t.Fatalf("DryRun restore failed: %v", err)
	}
	if !drySummary.DryRun || !drySummary.SidecarVerified || !drySummary.ForeignKeysOK || drySummary.BackupFormat != "IMBK2" {
		t.Fatalf("unexpected dry-run summary: %+v", drySummary)
	}
	if drySummary.TableCounts["suppliers"] != int64(baselineCount+1) {
		t.Fatalf("expected %d suppliers in dry-run counts, got %d", baselineCount+1, drySummary.TableCounts["suppliers"])
	}

	// 4. Corrupt sidecar and confirm restore rejects tampered hash
	origSidecar, _ := os.ReadFile(rec.Path + ".sha256")
	_ = os.WriteFile(rec.Path+".sha256", []byte(strings.Repeat("0", 64)+"\n"), 0o600)
	if _, err := rt.Backup.RestoreBackup(models.RestoreBackupRequest{
		BackupPath:    rec.Path,
		DryRun:        true,
		VerifySidecar: true,
	}); err == nil {
		t.Fatal("expected sidecar corruption error, got nil")
	}
	_ = os.WriteFile(rec.Path+".sha256", origSidecar, 0o600)

	// 5. Mutate DB after snapshot
	if _, err := rt.Suppliers.Create(models.SupplierInput{
		SupplierName: "Post Snapshot Supplier",
		Country:      "Japan",
		Email:        "after@post.jp",
		IsActive:     true,
	}); err != nil {
		t.Fatalf("create second supplier: %v", err)
	}

	// 6. Execute live restore and verify database rolls back to s1 only + creates safety backup
	restored, err := rt.Backup.RestoreBackup(models.RestoreBackupRequest{
		BackupPath:    rec.Path,
		DryRun:        false,
		VerifySidecar: true,
	})
	if err != nil {
		t.Fatalf("live RestoreBackup failed: %v", err)
	}
	if restored.SafetyBackupPath == "" || !restored.ForeignKeysOK {
		t.Fatalf("expected safety backup path & FK OK, got %+v", restored)
	}
	if _, err := os.Stat(restored.SafetyBackupPath); err != nil {
		t.Fatalf("safety backup file missing on disk: %v", err)
	}

	list, err := rt.Suppliers.List(models.SupplierPageRequest{Page: 1, PageSize: 100})
	if err != nil {
		t.Fatalf("List post-restore failed: %v", err)
	}
	if list.TotalCount != baselineCount+1 {
		t.Fatalf("expected %d suppliers after restore (rolling back Post Snapshot Supplier), got total=%d", baselineCount+1, list.TotalCount)
	}
	foundS1 := false
	for _, item := range list.Items {
		if item.ID == s1.ID {
			foundS1 = true
		}
		if item.SupplierName == "Post Snapshot Supplier" {
			t.Fatalf("Post Snapshot Supplier should have been rolled back by restore")
		}
	}
	if !foundS1 {
		t.Fatalf("expected pre-snapshot supplier %s after restore", s1.ID)
	}

	// 7. Verify legacy IMBK1 (100k PBKDF2) backup decrypt & dry-run compatibility
	snapPlain := filepath.Join(t.TempDir(), "raw_legacy.db")
	if _, err := rt.DB.DB().Exec(`VACUUM INTO ?`, snapPlain); err != nil {
		t.Fatalf("vacuum raw snap: %v", err)
	}
	rawSnapBytes, _ := os.ReadFile(snapPlain)
	imbk1Bytes := encryptLegacyIMBK1("LegacyPass#1", rawSnapBytes)
	imbk1Path := filepath.Join(t.TempDir(), "legacy.imbk1")
	_ = os.WriteFile(imbk1Path, imbk1Bytes, 0o600)

	imbk1Res, err := rt.Backup.RestoreBackup(models.RestoreBackupRequest{
		BackupPath: imbk1Path,
		Passphrase: "LegacyPass#1",
		DryRun:     true,
	})
	if err != nil || imbk1Res.BackupFormat != "IMBK1" || !imbk1Res.ForeignKeysOK {
		t.Fatalf("IMBK1 compatibility dry-run failed: %+v err=%v", imbk1Res, err)
	}

	// 8. Schedules CRUD
	sched, err := rt.Backup.SaveBackupSchedule(models.SaveBackupScheduleInput{
		Name:           "Nightly Local Vault",
		CronExpr:       "0 2 * * *",
		Destination:    "local",
		RetentionCount: 7,
		RetentionDays:  30,
		Enabled:        true,
	})
	if err != nil || sched.ID == 0 {
		t.Fatalf("SaveBackupSchedule failed: %+v err=%v", sched, err)
	}
	scheds, err := rt.Backup.ListBackupSchedules()
	if err != nil || len(scheds) != 1 {
		t.Fatalf("ListBackupSchedules expected 1, got %d err=%v", len(scheds), err)
	}
	if err := rt.Backup.DeleteBackupSchedule(sched.ID); err != nil {
		t.Fatalf("DeleteBackupSchedule failed: %v", err)
	}

	// 9. DeleteBackupRecord and DeleteBackupFile
	bks, err := rt.Backup.ListBackups()
	if err != nil || len(bks) == 0 {
		t.Fatalf("ListBackups expected at least 1 backup, got len=%d err=%v", len(bks), err)
	}
	targetBk := bks[0]
	if _, err := os.Stat(targetBk.Path); err != nil {
		t.Fatalf("expected backup file to exist before delete: %v", err)
	}
	if err := rt.Backup.DeleteBackupRecord(targetBk.ID, true); err != nil {
		t.Fatalf("DeleteBackupRecord failed: %v", err)
	}
	if _, err := os.Stat(targetBk.Path); !os.IsNotExist(err) {
		t.Fatalf("expected backup file to be removed from disk, got err=%v", err)
	}
	if _, err := os.Stat(targetBk.Path + ".sha256"); !os.IsNotExist(err) {
		t.Fatalf("expected sidecar file to be removed from disk, got err=%v", err)
	}
	bksAfter, err := rt.Backup.ListBackups()
	if err != nil {
		t.Fatalf("ListBackups after delete failed: %v", err)
	}
	for _, b := range bksAfter {
		if b.ID == targetBk.ID {
			t.Fatalf("expected backup record %d to be deleted from table", targetBk.ID)
		}
	}
}

func TestBackupRestoreCrossMachinePortable(t *testing.T) {
	// 1. Simulate Machine A with Key A
	t.Setenv("IMPORT_MANAGER_DB_KEY", "machine-a-key-11111111111111111111111111111111")
	t.Setenv("IMPORT_MANAGER_KEYRING_FILE", filepath.Join(t.TempDir(), "keyring_a.json"))
	dirA := filepath.Join(t.TempDir(), "appdata_a")
	rtA, err := bootstrap.Initialize(nil, dirA)
	if err != nil {
		t.Fatalf("bootstrap A failed: %v", err)
	}
	defer func() { _ = rtA.Shutdown() }()

	if _, err := rtA.Security.CompleteAdminSetup("admin", "AdminPassA#123"); err != nil {
		t.Fatalf("admin setup A failed: %v", err)
	}

	// Insert distinct data on Machine A
	sup, err := rtA.Suppliers.Create(models.SupplierInput{
		SupplierName: "Machine A Global Supplier",
		Country:      "Germany",
		Email:        "global@machine-a.de",
		IsActive:     true,
	})
	if err != nil {
		t.Fatalf("create supplier on A: %v", err)
	}

	backupRec, err := rtA.Backup.CreateBackup(models.CreateBackupRequest{
		Passphrase: "UserCrossMachinePass#1",
		Notes:      "Exported for Machine B",
	})
	if err != nil {
		t.Fatalf("create backup on A failed: %v", err)
	}

	// 2. Simulate Machine B with a completely different database encryption key Key B
	t.Setenv("IMPORT_MANAGER_DB_KEY", "machine-b-key-22222222222222222222222222222222")
	t.Setenv("IMPORT_MANAGER_KEYRING_FILE", filepath.Join(t.TempDir(), "keyring_b.json"))
	dirB := filepath.Join(t.TempDir(), "appdata_b")
	rtB, err := bootstrap.Initialize(nil, dirB)
	if err != nil {
		t.Fatalf("bootstrap B failed: %v", err)
	}
	defer func() { _ = rtB.Shutdown() }()

	if _, err := rtB.Security.CompleteAdminSetup("admin", "AdminPassB#456"); err != nil {
		t.Fatalf("admin setup B failed: %v", err)
	}

	// 3. Restore backup on Machine B using ONLY the passphrase
	res, err := rtB.Backup.RestoreBackup(models.RestoreBackupRequest{
		BackupPath:    backupRec.Path,
		Passphrase:    "UserCrossMachinePass#1",
		DryRun:        false,
		VerifySidecar: true,
	})
	if err != nil {
		t.Fatalf("restore on Machine B failed: %v", err)
	}

	if !res.ForeignKeysOK || res.IntegrityCheck != "ok" {
		t.Fatalf("restore on B verification failed: %+v", res)
	}

	// 4. Confirm Machine B now has Machine A's data
	listB, err := rtB.Suppliers.List(models.SupplierPageRequest{Page: 1, PageSize: 100})
	if err != nil {
		t.Fatalf("list on B failed: %v", err)
	}
	found := false
	for _, s := range listB.Items {
		if s.ID == sup.ID && s.SupplierName == "Machine A Global Supplier" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Machine B did not find restored supplier %s", sup.ID)
	}
}
