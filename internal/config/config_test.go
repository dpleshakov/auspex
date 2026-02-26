package config

import (
	"fmt"
	"os"
	"testing"
)

func TestLoadFromFile_Valid(t *testing.T) {
	f := writeTempConfig(t, `
port: 9090
db_path: test.db
refresh_interval: 5
esi:
  client_id: "myid"
  client_secret: "mysecret"
  callback_url: "http://localhost:9090/auth/eve/callback"
`)
	cfg, err := loadFromFile(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != 9090 {
		t.Errorf("port: got %d, want 9090", cfg.Port)
	}
	if cfg.DBPath != "test.db" {
		t.Errorf("db_path: got %q, want %q", cfg.DBPath, "test.db")
	}
	if cfg.RefreshInterval != 5 {
		t.Errorf("refresh_interval: got %d, want 5", cfg.RefreshInterval)
	}
	if cfg.ESI.ClientID != "myid" {
		t.Errorf("client_id: got %q, want %q", cfg.ESI.ClientID, "myid")
	}
}

func TestLoadFromFile_Defaults(t *testing.T) {
	f := writeTempConfig(t, `
esi:
  client_id: "myid"
  client_secret: "mysecret"
  callback_url: "http://localhost:8080/auth/eve/callback"
`)
	cfg, err := loadFromFile(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != 8080 {
		t.Errorf("port: got %d, want 8080 (default)", cfg.Port)
	}
	if cfg.DBPath != "auspex.db" {
		t.Errorf("db_path: got %q, want %q (default)", cfg.DBPath, "auspex.db")
	}
	if cfg.RefreshInterval != 10 {
		t.Errorf("refresh_interval: got %d, want 10 (default)", cfg.RefreshInterval)
	}
}

func TestLoadFromFile_MissingClientID(t *testing.T) {
	f := writeTempConfig(t, `
esi:
  client_secret: "mysecret"
  callback_url: "http://localhost:8080/auth/eve/callback"
`)
	_, err := loadFromFile(f)
	if err == nil {
		t.Fatal("expected error for missing client_id, got nil")
	}
}

func TestLoadFromFile_MissingClientSecret(t *testing.T) {
	f := writeTempConfig(t, `
esi:
  client_id: "myid"
  callback_url: "http://localhost:8080/auth/eve/callback"
`)
	_, err := loadFromFile(f)
	if err == nil {
		t.Fatal("expected error for missing client_secret, got nil")
	}
}

func TestLoadFromFile_MissingCallbackURL(t *testing.T) {
	f := writeTempConfig(t, `
esi:
  client_id: "myid"
  client_secret: "mysecret"
`)
	_, err := loadFromFile(f)
	if err == nil {
		t.Fatal("expected error for missing callback_url, got nil")
	}
}

func TestLoadFromFile_NotFound(t *testing.T) {
	_, err := loadFromFile("nonexistent.yaml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestLoadFromFile_InvalidPort(t *testing.T) {
	for _, port := range []int{0, -1, 65536, 99999} {
		f := writeTempConfig(t, fmt.Sprintf(`
port: %d
esi:
  client_id: "myid"
  client_secret: "mysecret"
  callback_url: "http://localhost:8080/auth/eve/callback"
`, port))
		_, err := loadFromFile(f)
		if err == nil {
			t.Errorf("expected error for port %d, got nil", port)
		}
	}
}

func TestLoadFromFile_InvalidCallbackURL(t *testing.T) {
	for _, bad := range []string{"not-a-url", "ftp://example.com/cb", ":///bad"} {
		f := writeTempConfig(t, fmt.Sprintf(`
esi:
  client_id: "myid"
  client_secret: "mysecret"
  callback_url: %q
`, bad))
		_, err := loadFromFile(f)
		if err == nil {
			t.Errorf("expected error for callback_url %q, got nil", bad)
		}
	}
}

func TestLoadFromFile_InvalidRefreshInterval(t *testing.T) {
	for _, interval := range []int{0, -1, -100} {
		f := writeTempConfig(t, fmt.Sprintf(`
refresh_interval: %d
esi:
  client_id: "myid"
  client_secret: "mysecret"
  callback_url: "http://localhost:8080/auth/eve/callback"
`, interval))
		_, err := loadFromFile(f)
		if err == nil {
			t.Errorf("expected error for refresh_interval %d, got nil", interval)
		}
	}
}

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "auspex-*.yaml")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	_ = f.Close()
	return f.Name()
}
