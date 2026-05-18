package backend

import (
	"testing"

	"github.com/robertDouglass/claude-log-analyzer/internal/localstore"
)

func TestNewAPIStoreDefaultsToLocal(t *testing.T) {
	t.Setenv("CLAUDE_ANALYZER_BACKEND", "")
	t.Setenv("CLAUDE_ANALYZER_DATA_DIR", t.TempDir())
	store, err := NewAPIStore()
	if err != nil {
		t.Fatalf("NewAPIStore failed: %v", err)
	}
	if _, ok := store.(*localstore.Store); !ok {
		t.Fatalf("expected local store, got %T", store)
	}
}

func TestNewAPIStoreRejectsUnknownBackend(t *testing.T) {
	t.Setenv("CLAUDE_ANALYZER_BACKEND", "nope")
	if _, err := NewAPIStore(); err == nil {
		t.Fatal("expected unknown backend error")
	}
}

func TestNewAPIStoreAWSRequiresConfig(t *testing.T) {
	t.Setenv("CLAUDE_ANALYZER_BACKEND", "aws")
	t.Setenv("AWS_REGION", "us-east-1")
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	if _, err := NewAPIStore(); err == nil {
		t.Fatal("expected AWS configuration error")
	}
}
