package parity_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/worldsignal/backend/internal/parity"
)

// TestMain skips the entire differential suite when the legacy TypeScript backend
// is absent (i.e. after cutover). The suite verifies Go-vs-TS parity and is only
// runnable while the reference implementation exists; its passing results are
// recorded in git history and MIGRATION_PLAN.md.
func TestMain(m *testing.M) {
	marker := filepath.Join(parity.RepoRoot(), "backend", "scripts", "stage.ts")
	if _, err := os.Stat(marker); err != nil {
		fmt.Println("legacy TypeScript backend absent — differential parity suite skipped (verified pre-cutover)")
		os.Exit(0)
	}
	os.Exit(m.Run())
}
