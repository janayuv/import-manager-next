package updater

import (
	"strings"
	"testing"
)

func TestParseSHA256Sums_StripsUTF8BOMAndCRLF(t *testing.T) {
	exeHash := strings.Repeat("a", 64)
	installerHash := strings.Repeat("B", 64)
	input := "\ufeff" + exeHash + "  import-manager.exe\r\n" +
		installerHash + " *Import_Manager-amd64-installer.exe\r\n"

	sums, err := parseSHA256Sums(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parseSHA256Sums: %v", err)
	}
	if got := sums["import-manager.exe"]; got != exeHash {
		t.Fatalf("first entry after BOM: got %q, want %q", got, exeHash)
	}
	if got := sums["Import_Manager-amd64-installer.exe"]; got != strings.ToLower(installerHash) {
		t.Fatalf("installer entry: got %q, want %q", got, strings.ToLower(installerHash))
	}
}

func TestValidateUpdateURL_PinsConfiguredRepository(t *testing.T) {
	s := &Service{repo: "janayuv/import-manager-next", allowedTestHosts: map[string]bool{}}

	cases := []struct {
		url         string
		redirect    bool
		wantAllowed bool
	}{
		{"https://github.com/janayuv/import-manager-next/releases/download/v2.4.0/Import_Manager-amd64-installer.exe", false, true},
		{"https://github.com/JanaYuv/Import-Manager-Next/releases/download/v2.4.0/SHA256SUMS.txt", false, true},
		{"https://github.com/attacker/fake-repo/releases/download/v2.4.0/installer.exe", false, false},
		{"https://github.com/janayuv/import-manager-next/releases/download/v2.4.0/../../../../attacker/x/releases/download/v1/a.exe", false, false},
		{"https://github.com/janayuv/import-manager-next/releases/download/v2.4.0/sub/dir/a.exe", false, false},
		{"https://github.com/janayuv/import-manager-next/releases/download/a.exe", false, false},
		{"https://user:pass@github.com/janayuv/import-manager-next/releases/download/v2.4.0/a.exe", false, false},
		{"http://github.com/janayuv/import-manager-next/releases/download/v2.4.0/a.exe", false, false},
		{"https://release-assets.githubusercontent.com/github-production-release-asset/123/abc", false, false},
		{"https://release-assets.githubusercontent.com/github-production-release-asset/123/abc", true, true},
		{"https://objects.githubusercontent.com/github-production-release-asset-2e65be/123", true, true},
		{"https://github.com/attacker/fake-repo/releases/download/v1/a.exe", true, false},
		{"https://evil.example.com/a.exe", true, false},
	}
	for _, tc := range cases {
		err := s.validateUpdateURL(tc.url, tc.redirect)
		if (err == nil) != tc.wantAllowed {
			t.Errorf("validateUpdateURL(%q, redirect=%v): allowed=%v, want %v (err=%v)", tc.url, tc.redirect, err == nil, tc.wantAllowed, err)
		}
	}
}
