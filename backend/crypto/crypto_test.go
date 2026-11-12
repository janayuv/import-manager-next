package crypto_test

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	appcrypto "github.com/janayuv/import-manager-next/backend/crypto"
)

func TestPasswordComplexityAndArgon2idLifecycle(t *testing.T) {
	invalid := []string{"short1!", "alllower1!", "ALLUPPER1!", "NoDigits!!", "NoSymbol123", "With Space1!"}
	for _, p := range invalid {
		if err := appcrypto.ValidatePasswordComplexity(p); err == nil {
			t.Fatalf("expected complexity rejection for %q", p)
		}
	}

	valid := "Str0ng!Pass#2026"
	if err := appcrypto.ValidatePasswordComplexity(valid); err != nil {
		t.Fatalf("unexpected rejection for valid password: %v", err)
	}

	hash, err := appcrypto.HashPassword(valid)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$") {
		t.Fatalf("unexpected hash format: %s", hash)
	}

	ok, rehash, err := appcrypto.VerifyPassword(valid, hash)
	if err != nil || !ok || rehash {
		t.Fatalf("expected match=true, needsRehash=false; got ok=%v rehash=%v err=%v", ok, rehash, err)
	}

	okWrong, _, err := appcrypto.VerifyPassword("Wrong!Pass1", hash)
	if err != nil || okWrong {
		t.Fatalf("expected wrong password rejection")
	}

	// Verify legacy bcrypt hash compatibility flags needsRehash=true.
	bcHash, err := bcrypt.GenerateFromPassword([]byte(valid), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt gen: %v", err)
	}
	okBc, rehashBc, err := appcrypto.VerifyPassword(valid, string(bcHash))
	if err != nil || !okBc || !rehashBc {
		t.Fatalf("expected bcrypt match + rehash=true, got ok=%v rehash=%v err=%v", okBc, rehashBc, err)
	}
}

func TestIMBK2RoundTripAndSidecar(t *testing.T) {
	plain := []byte("SQLCipher snapshot payload bytes 0123456789")
	pass := "Backup!Key#99"

	enc, err := appcrypto.EncryptBackupIMBK2(plain, pass)
	if err != nil {
		t.Fatalf("EncryptBackupIMBK2: %v", err)
	}
	if !bytes.HasPrefix(enc, []byte(appcrypto.BackupMagicV2)) {
		t.Fatalf("missing IMBK2 prefix")
	}

	dec, fmtVer, err := appcrypto.DecryptBackup(enc, pass)
	if err != nil || fmtVer != "IMBK2" || !bytes.Equal(dec, plain) {
		t.Fatalf("DecryptBackup mismatch: fmt=%s err=%v", fmtVer, err)
	}

	if _, _, err := appcrypto.DecryptBackup(enc, "wrong-pass"); err == nil {
		t.Fatalf("expected wrong passphrase failure")
	}

	p := filepath.Join(t.TempDir(), "test.imbk")
	if err := writeDummy(p, enc); err != nil {
		t.Fatalf("write dummy: %v", err)
	}
	expectedHex, err := appcrypto.WriteSidecarSHA256(p, enc)
	if err != nil {
		t.Fatalf("WriteSidecarSHA256: %v", err)
	}
	gotHex, err := appcrypto.VerifyFileSidecarSHA256(p)
	if err != nil || gotHex != expectedHex {
		t.Fatalf("VerifyFileSidecarSHA256 failed: got=%s expected=%s err=%v", gotHex, expectedHex, err)
	}
}
