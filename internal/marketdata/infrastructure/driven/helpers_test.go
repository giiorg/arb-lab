package driven_test

import (
	"encoding/json"
	"testing"

	"github.com/giiorg/arb-lab/internal/marketdata/infrastructure/driven"
	"github.com/giiorg/arb-lab/internal/platform/config"
)

// encode writes a schema back out as a document.
//
// config.Duration marshals to the string form it accepts, so a round trip
// through here produces a document this schema can read — which is what makes a
// reload test express its input as a Go value instead of as hand-written JSON
// that could drift from the struct it is meant to populate.
func encode(t *testing.T, cfg driven.FreshnessConfig) []byte {
	t.Helper()
	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	return b
}

func newStore(t *testing.T, cfg driven.FreshnessConfig) *config.Store[driven.FreshnessConfig] {
	t.Helper()
	store, err := config.New[driven.FreshnessConfig]("test", at, encode(t, cfg))
	if err != nil {
		t.Fatalf("config.New: %v", err)
	}
	return store
}

func newBudgets(t *testing.T, cfg driven.FreshnessConfig) *driven.ConfigBudgets {
	t.Helper()
	b, err := driven.NewConfigBudgets(newStore(t, cfg))
	if err != nil {
		t.Fatalf("NewConfigBudgets: %v", err)
	}
	return b
}
