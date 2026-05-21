package backend

import (
	"fmt"
	"os"

	"github.com/priivacy-ai/agent-log-analyzer/internal/app"
	"github.com/priivacy-ai/agent-log-analyzer/internal/awsstore"
	"github.com/priivacy-ai/agent-log-analyzer/internal/localstore"
)

func NewAPIStore() (app.APIStore, error) {
	switch backend := getenv("CLAUDE_ANALYZER_BACKEND", "local"); backend {
	case "local":
		return localstore.New(getenv("CLAUDE_ANALYZER_DATA_DIR", "/tmp/agent-log-analyzer"))
	case "aws":
		return awsstore.NewFromEnv()
	default:
		return nil, fmt.Errorf("unsupported backend %q", backend)
	}
}

func NewWorkerStore() (app.WorkerStore, error) {
	switch backend := getenv("CLAUDE_ANALYZER_BACKEND", "local"); backend {
	case "local":
		return localstore.New(getenv("CLAUDE_ANALYZER_DATA_DIR", "/tmp/agent-log-analyzer"))
	case "aws":
		return awsstore.NewFromEnv()
	default:
		return nil, fmt.Errorf("unsupported backend %q", backend)
	}
}

func NewSweeperStore() (app.SweeperStore, error) {
	switch backend := getenv("CLAUDE_ANALYZER_BACKEND", "local"); backend {
	case "local":
		return localstore.New(getenv("CLAUDE_ANALYZER_DATA_DIR", "/tmp/agent-log-analyzer"))
	case "aws":
		return awsstore.NewFromEnv()
	default:
		return nil, fmt.Errorf("unsupported backend %q", backend)
	}
}

func NewEmailOperationsStore() (app.EmailOperationsStore, error) {
	switch backend := getenv("CLAUDE_ANALYZER_BACKEND", "local"); backend {
	case "local":
		return localstore.New(getenv("CLAUDE_ANALYZER_DATA_DIR", "/tmp/agent-log-analyzer"))
	case "aws":
		return awsstore.NewFromEnv()
	default:
		return nil, fmt.Errorf("unsupported backend %q", backend)
	}
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
