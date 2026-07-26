package config_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/giiorg/arb-lab/internal/platform/config"
)

// TestReloadAppliesAHotChange is the positive control for the whole default-deny
// table below. An implementation that refused every reload would pass all nine
// rejection cases and fail here.
func TestReloadAppliesAHotChange(t *testing.T) {
	s := mustStore(t)

	ch, err := s.Reload("configs/test.json", testAt, []byte(withField(fMaxOpen, "5")))
	if err != nil {
		t.Fatalf("Reload = %v, want nil", err)
	}
	if !ch.Applied {
		t.Error("Change.Applied = false, want true")
	}
	if got := s.Current().Value.Risk.MaxTotalOpen; got != 5 {
		t.Errorf("MaxTotalOpen after reload = %d, want 5", got)
	}
	if !slices.Equal(ch.Fields, []string{"risk.max_total_open"}) {
		t.Errorf("Change.Fields = %v, want [risk.max_total_open]", ch.Fields)
	}
}

// TestReloadRejectsAChangeToAnUntaggedField is default-deny stated once per
// construct the walk can reach. Each row is a field that a running process must
// not have moved underneath it, and each names WHY in its case name.
func TestReloadRejectsAChangeToAnUntaggedField(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		path string
	}{
		{
			name: "a top-level leaf: changing the journal path abandons the WAL",
			doc:  withField(fJournal, `"/var/lib/arblab/other.db"`),
			path: "journal",
		},
		{
			name: "a leaf inside a non-hot struct: the §7.1 escalation action",
			doc:  withField(fBlindAction, `"close"`),
			path: "risk.mark_blind_action",
		},
		{
			name: "a leaf inside a non-hot map entry: a venue endpoint",
			doc:  withField(fBinanceEndpoint, `"https://testnet.binancefuture.com"`),
			path: `venues["binance"].endpoint`,
		},
		{
			name: "a self-decoding type inside a map entry: a rotated key ref",
			doc:  withField(fBinanceKey, `"env:BINANCE_API_KEY_V2"`),
			path: `venues["binance"].key_ref`,
		},
		{
			name: "a leaf behind a pointer",
			doc:  withField(fStrategy, `{"min_funding_apr": "0.05", "unused": 1}`),
			path: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := mustStore(t)
			before := s.Current()

			ch, err := s.Reload("configs/test.json", testAt, []byte(tc.doc))
			if err == nil {
				t.Fatalf("Reload accepted a change to an untagged field; Change = %+v", ch)
			}
			if tc.path != "" {
				if !errors.Is(err, config.ErrNotReloadable) {
					t.Fatalf("Reload = %v, want ErrNotReloadable", err)
				}
				if !strings.Contains(err.Error(), tc.path) {
					t.Errorf("error %q does not name the path %q", err, tc.path)
				}
			}

			// The rejected reload changed nothing.
			after := s.Current()
			if after.Rev != before.Rev {
				t.Errorf("a rejected reload minted a revision: %v -> %v", before.Rev, after.Rev)
			}
		})
	}
}

// TestReloadRejectsAStructuralChangeToAnUntaggedComposite covers what recursion
// cannot express.
//
// Adding or removing a venue is not "a field changed" — there is no field on
// both sides to compare — so the permission has to sit on the map itself, which
// carries no hot tag. That asymmetry is the design: editing a venue's hot
// rate-limit budget is absorbable, introducing a venue means wiring a client.
func TestReloadRejectsAStructuralChangeToAnUntaggedComposite(t *testing.T) {
	s := mustStore(t)

	dropped := strings.Replace(baseDoc, `,
    "bybit": {
      "endpoint": "https://api.bybit.com",
      "key_ref": "file:/run/secrets/bybit.key",
      "rate_limit": 600
    }`, "", 1)
	if dropped == baseDoc {
		t.Fatal("fixture edit did not apply — the test would prove nothing")
	}

	if _, err := s.Reload("configs/test.json", testAt, []byte(dropped)); !errors.Is(err, config.ErrNotReloadable) {
		t.Fatalf("Reload = %v, want ErrNotReloadable for a removed venue", err)
	}

	// ADDING an entry is the asymmetric case, and it needs its own row.
	//
	// Removal is caught by the per-key presence walk: a key in the live config
	// is missing from the candidate. An addition is invisible to that walk —
	// every live key is still there — so only the SIZE comparison catches it. A
	// mutant that deleted the size check survived the removal case above while
	// silently admitting a whole new venue, which is a client nobody wired.
	const added = `{
  "journal": "/var/lib/arblab/oms.db",
  "risk": {"max_total_open": 3, "t_blind": "90s", "mark_blind_action": "delever"},
  "venues": {
    "binance": {"endpoint": "https://fapi.binance.com", "key_ref": "env:BINANCE_API_KEY", "rate_limit": 1200},
    "bybit":   {"endpoint": "https://api.bybit.com", "key_ref": "file:/run/secrets/bybit.key", "rate_limit": 600},
    "okx":     {"endpoint": "https://www.okx.com", "key_ref": "env:OKX_API_KEY", "rate_limit": 100}
  },
  "symbols": ["BTCUSDT"],
  "strategy": {"min_funding_apr": "0.05"}
}`
	if _, err := config.Decode[testConfig]([]byte(added)); err != nil {
		t.Fatalf("the added-venue fixture is not a valid document (%v); it would be "+
			"refused for the wrong reason and the test would prove nothing", err)
	}

	if _, err := s.Reload("configs/test.json", testAt, []byte(added)); !errors.Is(err, config.ErrNotReloadable) {
		t.Fatalf("Reload = %v, want ErrNotReloadable for an added venue", err)
	}

	// And a pointer going present -> absent is the same class of change.
	if _, err := s.Reload("configs/test.json", testAt, []byte(withField(fStrategy, "null"))); !errors.Is(err, config.ErrNotReloadable) {
		t.Fatalf("Reload = %v, want ErrNotReloadable for a removed strategy", err)
	}
}

// TestReloadAcceptsAHotFieldInsideANonHotMapEntry is the case that justifies
// recursing into composites at all.
//
// A wholesale DeepEqual at the map would refuse this: the map carries no tag, so
// any difference anywhere under it would be a policy violation, and a per-venue
// rate-limit budget — which §5.2 puts exactly here — could never be tuned
// without a restart. The path in Change.Fields has to name the venue, or an
// audit record cannot say which one moved.
func TestReloadAcceptsAHotFieldInsideANonHotMapEntry(t *testing.T) {
	s := mustStore(t)

	ch, err := s.Reload("configs/test.json", testAt, []byte(withField(fBinanceRate, "2400")))
	if err != nil {
		t.Fatalf("Reload = %v, want nil", err)
	}
	if !slices.Equal(ch.Fields, []string{`venues["binance"].rate_limit`}) {
		t.Fatalf("Change.Fields = %v, want the binance entry named", ch.Fields)
	}
	if got := s.Current().Value.Venues["binance"].RateLimit; got != 2400 {
		t.Errorf("binance rate_limit = %d, want 2400", got)
	}
	if got := s.Current().Value.Venues["bybit"].RateLimit; got != 600 {
		t.Errorf("bybit rate_limit = %d, want 600 (untouched)", got)
	}
}

// TestReloadTaggingAStructMakesItsSubtreeHot pins tag inheritance. Symbols is a
// tagged slice, so its contents may change wholesale; Strategy is a non-hot
// pointer to a struct whose one field IS tagged, so that field may change
// through it.
func TestReloadTaggingAStructMakesItsSubtreeHot(t *testing.T) {
	s := mustStore(t)

	ch, err := s.Reload("configs/test.json", testAt, []byte(withField(fSymbols, `["BTCUSDT","ETHUSDT"]`)))
	if err != nil {
		t.Fatalf("Reload of a hot slice = %v, want nil", err)
	}
	if !slices.Equal(ch.Fields, []string{"symbols"}) {
		t.Errorf("Change.Fields = %v, want [symbols]", ch.Fields)
	}

	// A fresh store: the reload above left symbols at two entries, so reloading
	// a document built from baseFields would move symbols BACK and the second
	// assertion would be about two changes rather than one.
	s = mustStore(t)
	ch, err = s.Reload("configs/test.json", testAt, []byte(withField(fStrategy, `{"min_funding_apr": "0.08"}`)))
	if err != nil {
		t.Fatalf("Reload of a hot field behind a non-hot pointer = %v, want nil", err)
	}
	if !slices.Equal(ch.Fields, []string{"strategy.min_funding_apr"}) {
		t.Errorf("Change.Fields = %v, want [strategy.min_funding_apr]", ch.Fields)
	}
}

// TestReloadReportsEveryHotFieldThatMoved checks the audit record is complete
// rather than first-wins. §5.2 requires every change be logged, and a record
// naming one of three edits is a record that hides two.
func TestReloadReportsEveryHotFieldThatMoved(t *testing.T) {
	s := mustStore(t)

	multi := doc(baseFields[0], "7", `"120s"`, baseFields[3],
		baseFields[4], baseFields[5], "999",
		`["BTCUSDT","SOLUSDT"]`, baseFields[8])

	ch, err := s.Reload("configs/test.json", testAt, []byte(multi))
	if err != nil {
		t.Fatalf("Reload = %v, want nil", err)
	}

	want := []string{
		"risk.max_total_open",
		"risk.t_blind",
		"symbols",
		`venues["binance"].rate_limit`,
	}
	if !slices.Equal(ch.Fields, want) {
		t.Errorf("Change.Fields = %v, want %v", ch.Fields, want)
	}
}

// TestReloadFieldsAreSorted makes the audit record deterministic. Go's map
// iteration order is randomised per run, so without the sort the same edit
// produces different records on different runs and two audit lines for one event
// cannot be compared.
//
// The fixture is built so an unsorted walk would disagree: three venues whose
// map order is random, and whose names sort in the opposite direction from the
// order they appear in the document.
func TestReloadFieldsAreSorted(t *testing.T) {
	const three = `{
  "journal": "/j", "risk": {"max_total_open": 1, "t_blind": "1s", "mark_blind_action": "close"},
  "venues": {
    "zeta":  {"endpoint": "z", "key_ref": "env:Z", "rate_limit": 1},
    "alpha": {"endpoint": "a", "key_ref": "env:A", "rate_limit": 1},
    "mid":   {"endpoint": "m", "key_ref": "env:M", "rate_limit": 1}
  },
  "symbols": [], "strategy": null}`
	bumped := strings.ReplaceAll(three, `"rate_limit": 1`, `"rate_limit": 2`)

	want := []string{`venues["alpha"].rate_limit`, `venues["mid"].rate_limit`, `venues["zeta"].rate_limit`}

	// Repeated, because map order is randomised per iteration: one run agreeing
	// with the sorted order by luck is not evidence. §9i's lesson — a coin flip
	// tested once is a coin flip untested.
	for i := range 50 {
		s, err := config.New[testConfig]("configs/test.json", testAt, []byte(three))
		if err != nil {
			t.Fatalf("New = %v", err)
		}
		ch, err := s.Reload("configs/test.json", testAt, []byte(bumped))
		if err != nil {
			t.Fatalf("Reload = %v", err)
		}
		if !slices.Equal(ch.Fields, want) {
			t.Fatalf("trial %d: Change.Fields = %v, want %v", i, ch.Fields, want)
		}
	}
}

// TestReloadRejectsOnTheFirstViolationDeterministically guards the same coin
// flip on the error path. A document that changes an untagged field in each of
// three venues must always name the same one, or an operator fixing what the
// error named gets a different error next time and cannot tell whether they
// helped.
func TestReloadRejectsOnTheFirstViolationDeterministically(t *testing.T) {
	const three = `{
  "journal": "/j", "risk": {"max_total_open": 1, "t_blind": "1s", "mark_blind_action": "close"},
  "venues": {
    "zeta":  {"endpoint": "z", "key_ref": "env:Z", "rate_limit": 1},
    "alpha": {"endpoint": "a", "key_ref": "env:A", "rate_limit": 1},
    "mid":   {"endpoint": "m", "key_ref": "env:M", "rate_limit": 1}
  },
  "symbols": [], "strategy": null}`
	moved := strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(three,
		`"endpoint": "z"`, `"endpoint": "z2"`),
		`"endpoint": "a"`, `"endpoint": "a2"`),
		`"endpoint": "m"`, `"endpoint": "m2"`)

	for i := range 50 {
		s, err := config.New[testConfig]("configs/test.json", testAt, []byte(three))
		if err != nil {
			t.Fatalf("New = %v", err)
		}
		_, err = s.Reload("configs/test.json", testAt, []byte(moved))
		if !errors.Is(err, config.ErrNotReloadable) {
			t.Fatalf("trial %d: Reload = %v, want ErrNotReloadable", i, err)
		}
		if !strings.Contains(err.Error(), `venues["alpha"].endpoint`) {
			t.Fatalf("trial %d: error %q, want the lowest-sorting venue named", i, err)
		}
	}
}

// TestReloadNamesTheFieldNotTheValue is the package's scoped
// no-values-in-messages invariant, checked where it matters most: the reload
// policy, whose error is produced from a live configuration and a candidate one
// and therefore has both values in hand.
//
// The sentinels are chosen to be findable rather than realistic. What they stand
// in for is real: a §5.2 schema holds venue endpoints and — if somebody declined
// SecretRef — credentials, and ErrNotReloadable travels to whatever alerts on a
// failed hot-reload.
func TestReloadNamesTheFieldNotTheValue(t *testing.T) {
	const oldSentinel = "OLD-SENTINEL-4f2a9c"
	const newSentinel = "NEW-SENTINEL-b71e30"

	s := mustStore(t)
	if _, err := s.Reload("configs/test.json", testAt, []byte(withField(fJournal, `"`+oldSentinel+`"`))); err == nil {
		t.Fatal("setup: expected the journal change to be refused")
	}

	_, err := s.Reload("configs/test.json", testAt, []byte(withField(fJournal, `"`+newSentinel+`"`)))
	if !errors.Is(err, config.ErrNotReloadable) {
		t.Fatalf("Reload = %v, want ErrNotReloadable", err)
	}
	for _, leaked := range []string{oldSentinel, newSentinel, "/var/lib/arblab/oms.db"} {
		if strings.Contains(err.Error(), leaked) {
			t.Errorf("error %q contains a configuration value (%q)", err, leaked)
		}
	}
	if !strings.Contains(err.Error(), "journal") {
		t.Errorf("error %q does not name the field", err)
	}
}
