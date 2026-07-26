package config_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/config"
)

// The fixture schema is shaped like §5.2's real configuration rather than like a
// minimal test case, because most of what this package does only has a meaning
// against that shape. A hot field nested inside a NON-hot map is what the
// per-venue rate-limit budget looks like; a non-hot leaf beside it is what the
// endpoint looks like; and the difference between them is the entire reload
// policy.
//
// Read the tags as the claim under test:
//
//	testConfig.Journal                 not hot   — changing the journal path
//	                                             mid-run abandons the WAL
//	testConfig.Risk.MaxTotalOpen       hot       — a cap the operator tightens
//	testConfig.Risk.BlindAction        not hot   — §7.1 escalation policy
//	testConfig.Venues[v].RateLimit     hot       — inside a non-hot map
//	testConfig.Venues[v].KeyRef        not hot   — rotating a key needs a restart
//	testConfig.Symbols                 hot       — a whole slice, tagged
//	testConfig.Strategy                not hot   — a pointer, absent or present
type testConfig struct {
	Journal  string               `json:"journal"`
	Risk     testRisk             `json:"risk"`
	Venues   map[string]testVenue `json:"venues"`
	Symbols  []string             `json:"symbols" config:"hot"`
	Strategy *testStrategy        `json:"strategy"`
}

type testRisk struct {
	MaxTotalOpen int             `json:"max_total_open" config:"hot"`
	TBlind       config.Duration `json:"t_blind" config:"hot"`
	BlindAction  string          `json:"mark_blind_action"`
}

type testVenue struct {
	Endpoint  string           `json:"endpoint"`
	KeyRef    config.SecretRef `json:"key_ref"`
	RateLimit int              `json:"rate_limit" config:"hot"`
}

type testStrategy struct {
	MinFundingAPR string `json:"min_funding_apr" config:"hot"`
}

// Validate is a schema's own check, deliberately minimal here: this package's
// tests are about the machinery, and a schema that validated elaborately would
// make it ambiguous which layer rejected a document.
func (c testConfig) Validate() error {
	if c.Journal == "" {
		return errors.New("journal path unset")
	}
	if c.Risk.MaxTotalOpen <= 0 {
		return errors.New("max_total_open must be positive")
	}
	switch c.Risk.BlindAction {
	case "delever", "close":
	default:
		return fmt.Errorf("mark_blind_action %q is not delever or close", c.Risk.BlindAction)
	}
	if err := c.Risk.TBlind.Validate(); err != nil {
		return err
	}
	for name, v := range c.Venues {
		if err := v.KeyRef.Validate(); err != nil {
			return fmt.Errorf("venue %s: %w", name, err)
		}
	}
	return nil
}

// baseDoc is a valid document for testConfig. Tests mutate one thing about it at
// a time, which is what makes a failure name a cause.
const baseDoc = `{
  "journal": "/var/lib/arblab/oms.db",
  "risk": {
    "max_total_open": 3,
    "t_blind": "90s",
    "mark_blind_action": "delever"
  },
  "venues": {
    "binance": {
      "endpoint": "https://fapi.binance.com",
      "key_ref": "env:BINANCE_API_KEY",
      "rate_limit": 1200
    },
    "bybit": {
      "endpoint": "https://api.bybit.com",
      "key_ref": "file:/run/secrets/bybit.key",
      "rate_limit": 600
    }
  },
  "symbols": ["BTCUSDT"],
  "strategy": {"min_funding_apr": "0.05"}
}`

// testAt is the load instant every test supplies, standing in for a Clock read.
// Fixed rather than time.Now() so a revision is reproducible.
var testAt = time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)

// mustStore builds a store over baseDoc, failing the test if the fixture itself
// is broken. Almost every test needs one, and a fixture that stopped decoding
// would otherwise show up as a confusing failure in whatever ran first.
func mustStore(t *testing.T) *config.Store[testConfig] {
	t.Helper()
	s, err := config.New[testConfig]("configs/test.json", testAt, []byte(baseDoc))
	if err != nil {
		t.Fatalf("fixture baseDoc does not load: %v", err)
	}
	return s
}

// doc renders a testConfig-shaped document with the given substitutions applied
// to baseDoc's fields. Written as a format string rather than by marshalling a
// struct on purpose: marshalling would test this package against its own encoder
// and could never produce a duplicate key, an unknown field, or a bare number
// where a duration belongs — which is most of what there is to test.
func doc(journal, maxOpen, tBlind, blindAction, binanceEndpoint, binanceKey, binanceRate, symbols, strategy string) string {
	return fmt.Sprintf(`{
  "journal": %s,
  "risk": {
    "max_total_open": %s,
    "t_blind": %s,
    "mark_blind_action": %s
  },
  "venues": {
    "binance": {
      "endpoint": %s,
      "key_ref": %s,
      "rate_limit": %s
    },
    "bybit": {
      "endpoint": "https://api.bybit.com",
      "key_ref": "file:/run/secrets/bybit.key",
      "rate_limit": 600
    }
  },
  "symbols": %s,
  "strategy": %s
}`, journal, maxOpen, tBlind, blindAction, binanceEndpoint, binanceKey, binanceRate, symbols, strategy)
}

// baseFields are baseDoc's values as doc() arguments, so a test can vary one.
var baseFields = []string{
	`"/var/lib/arblab/oms.db"`, `3`, `"90s"`, `"delever"`,
	`"https://fapi.binance.com"`, `"env:BINANCE_API_KEY"`, `1200`,
	`["BTCUSDT"]`, `{"min_funding_apr": "0.05"}`,
}

// withField renders baseDoc with argument i replaced.
func withField(i int, v string) string {
	f := append([]string(nil), baseFields...)
	f[i] = v
	return doc(f[0], f[1], f[2], f[3], f[4], f[5], f[6], f[7], f[8])
}

// jsonUnmarshalStrict decodes the way this package would if it made only the
// check the standard library offers — DisallowUnknownFields and nothing else.
//
// It exists so the duplicate-key and trailing-data tests can MEASURE what their
// guards are worth rather than assert that the guards exist: they run the same
// bytes through this and show that encoding/json accepts them silently. A guard
// whose absence changes nothing is a guard nobody can justify keeping.
func jsonUnmarshalStrict(document string, dst any) error {
	dec := json.NewDecoder(strings.NewReader(document))
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

// Field indices for withField, named so a test reads as a claim about a field.
const (
	fJournal = iota
	fMaxOpen
	fTBlind
	fBlindAction
	fBinanceEndpoint
	fBinanceKey
	fBinanceRate
	fSymbols
	fStrategy
)
