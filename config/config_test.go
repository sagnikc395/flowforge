package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sagnikc395/anchora/config"
)

func write(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAsyncDefaults(t *testing.T) {
	cfg, err := config.Load(write(t, "server:\n  address: \":9000\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Async.Lease(); got != 30*time.Second {
		t.Fatalf("lease = %s, want 30s", got)
	}
	if got := cfg.Async.Heartbeat(); got != 10*time.Second {
		t.Fatalf("heartbeat = %s, want a third of the lease", got)
	}
	if got := cfg.Async.WorkerTTL(); got != 2*time.Minute {
		t.Fatalf("worker TTL = %s, want four leases", got)
	}
	if !cfg.Async.ReaperEnabled() {
		t.Fatal("the reaper must default to on so a single node recovers its own work")
	}
}

func TestAsyncRejectsHeartbeatLongerThanLease(t *testing.T) {
	_, err := config.Load(write(t, "async:\n  lease_ms: 1000\n  heartbeat_ms: 5000\n"))
	if err == nil {
		t.Fatal("expected an error: a heartbeat slower than the lease guarantees lease loss")
	}
}

func TestAsyncEnabledRequiresBackendURLs(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("REDIS_URL", "")
	if _, err := config.Load(write(t, "async:\n  enabled: true\n")); err == nil {
		t.Fatal("expected async to fail fast when the backend URLs are unset")
	}
	t.Setenv("DATABASE_URL", "postgres://localhost/anchora")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	if _, err := config.Load(write(t, "async:\n  enabled: true\n")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReaperCanBeDisabled(t *testing.T) {
	cfg, err := config.Load(write(t, "async:\n  reaper: false\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Async.ReaperEnabled() {
		t.Fatal("reaper: false must disable the sweep")
	}
}
