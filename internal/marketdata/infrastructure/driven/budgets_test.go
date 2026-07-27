package driven_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/marketdata/infrastructure/driven"
	"github.com/giiorg/arb-lab/internal/platform/config"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// Two venues, and their budgets DIFFER in both fields.
//
// The fixture-dimension rule this codebase runs on: with one venue configured,
// "look the key up" and "return the only entry" are the same answer, and with
// two identical budgets, returning the wrong entry still returns the right
// numbers. Both distinctions are the ones this adapter exists to make.
const (
	binance = ids.Venue("binance")
	bybit   = ids.Venue("bybit")
	unknown = ids.Venue("okx")
)

var (
	binanceBudget = domain.Budget{MaxAge: 30 * time.Second, Blind: 5 * time.Minute}
	bybitBudget   = domain.Budget{MaxAge: 45 * time.Second, Blind: 9 * time.Minute}

	// The BOOK budgets are sub-second, per §3.4, and they are here for the same
	// fixture-dimension reason the two venues are: with book and funding sharing
	// numbers, "read the book section" and "read the funding section" give the
	// same answer, and BookBudget could be reading Funding with nothing failing.
	// The whole point of the split is that these two are not interchangeable.
	binanceBookBudget = domain.Budget{MaxAge: 250 * time.Millisecond, Blind: 2 * time.Second}
	bybitBookBudget   = domain.Budget{MaxAge: 400 * time.Millisecond, Blind: 3 * time.Second}
)

// books is the book half of a valid fixture section, so that a test aimed at
// the FUNDING half does not have to restate it.
func books() map[ids.Venue]driven.BudgetSpec {
	return map[ids.Venue]driven.BudgetSpec{
		binance: spec(binanceBookBudget),
		bybit:   spec(bybitBookBudget),
	}
}

// at is the instant configuration is loaded at. Not the zero time — config.New
// refuses that, so a bug that dropped it would fail for the wrong reason.
var at = time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)

func spec(b domain.Budget) driven.BudgetSpec {
	return driven.BudgetSpec{
		MaxAge: config.NewDuration(b.MaxAge),
		Blind:  config.NewDuration(b.Blind),
	}
}

func twoVenues() driven.FreshnessConfig {
	return driven.FreshnessConfig{Funding: map[ids.Venue]driven.BudgetSpec{
		binance: spec(binanceBudget),
		bybit:   spec(bybitBudget),
	}, Book: books()}
}

// ---------------------------------------------------------------------------
// The schema
// ---------------------------------------------------------------------------

func TestFreshnessConfigRefusesAnEmptySection(t *testing.T) {
	// Nil and empty are separate cases because they are separate mistakes — a
	// section nobody wrote versus one written with no entries — and a length
	// check catches both only if both are tried.
	cases := map[string]driven.FreshnessConfig{
		"nil map":   {},
		"empty map": {Funding: map[ids.Venue]driven.BudgetSpec{}},
	}
	for name, cfg := range cases {
		t.Run(name, func(t *testing.T) {
			err := cfg.Validate()
			if !errors.Is(err, driven.ErrNoVenues) {
				t.Fatalf("Validate() = %v, want ErrNoVenues", err)
			}
		})
	}
}

func TestFreshnessConfigDefersTheBudgetRuleToTheDomain(t *testing.T) {
	// MaxAge == Blind makes the FreshnessStale rung unreachable. The domain
	// owns that rule; this asserts the schema calls it rather than restating
	// it, by checking for the DOMAIN's sentinel.
	cfg := driven.FreshnessConfig{Funding: map[ids.Venue]driven.BudgetSpec{
		binance: spec(binanceBudget),
		bybit:   spec(domain.Budget{MaxAge: time.Minute, Blind: time.Minute}),
	}, Book: books()}

	err := cfg.Validate()
	if !errors.Is(err, domain.ErrBudgetUnreachable) {
		t.Fatalf("Validate() = %v, want domain.ErrBudgetUnreachable", err)
	}
	// The refusal has to name which venue, or an operator holding a file with
	// two venues in it learns only that one of them is wrong.
	if !strings.Contains(err.Error(), string(bybit)) {
		t.Errorf("Validate() = %q, want it to name venue %q", err, bybit)
	}
}

func TestFreshnessConfigRefusesAnUnnamedVenue(t *testing.T) {
	cfg := driven.FreshnessConfig{Funding: map[ids.Venue]driven.BudgetSpec{
		binance: spec(binanceBudget),
		"":      spec(bybitBudget),
	}, Book: books()}

	if err := cfg.Validate(); !errors.Is(err, ids.ErrVenueUnset) {
		t.Fatalf("Validate() = %v, want ids.ErrVenueUnset", err)
	}
}

func TestFreshnessConfigNamesTheSAMEVenueOnEveryRun(t *testing.T) {
	// Go randomises map iteration, so a validator that stopped at "the first"
	// bad entry would name a different venue on different runs — two boot logs
	// from one bad file that disagree about what is wrong with it. Keys are
	// sorted for this reason, and the loop is what proves it.
	cfg := driven.FreshnessConfig{Funding: map[ids.Venue]driven.BudgetSpec{
		binance: spec(domain.Budget{MaxAge: time.Minute, Blind: time.Second}),
		bybit:   spec(domain.Budget{MaxAge: time.Minute, Blind: time.Second}),
	}, Book: books()}

	first := cfg.Validate()
	if first == nil {
		t.Fatal("Validate() = nil, want a refusal")
	}
	for i := range 64 {
		if got := cfg.Validate(); got.Error() != first.Error() {
			t.Fatalf("run %d: Validate() = %q, want the stable %q", i, got, first)
		}
	}
	if !strings.Contains(first.Error(), string(binance)) {
		t.Errorf("Validate() = %q, want the sorted-first venue %q", first, binance)
	}
}

// ---------------------------------------------------------------------------
// The document, which is where the units live
// ---------------------------------------------------------------------------

func TestFreshnessConfigReadsDurationsAsStrings(t *testing.T) {
	doc := []byte(`{` +
		`"funding":{"binance":{"max_age":"30s","blind":"5m"},"bybit":{"max_age":"45s","blind":"9m"}},` +
		`"book":{"binance":{"max_age":"250ms","blind":"2s"},"bybit":{"max_age":"400ms","blind":"3s"}}}`)

	store, err := config.New[driven.FreshnessConfig]("test", at, doc)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	b, err := driven.NewConfigBudgets(store)
	if err != nil {
		t.Fatalf("NewConfigBudgets: %v", err)
	}

	got, ok := b.FundingBudget(binance)
	if !ok {
		t.Fatal("FundingBudget(binance) reported no budget")
	}
	if got != binanceBudget {
		t.Errorf("FundingBudget(binance) = %+v, want %+v", got, binanceBudget)
	}

	// The discriminating half: the two sections carry different numbers for the
	// same venue, so a BookBudget that read the funding map would return a
	// perfectly valid budget and fail only here. Sub-second on purpose, per
	// §3.4 — the funding value is three orders of magnitude too loose for a book
	// that gates execution, and nothing at runtime would say so.
	gotBook, ok := b.BookBudget(binance)
	if !ok {
		t.Fatal("BookBudget(binance) reported no budget")
	}
	if gotBook != binanceBookBudget {
		t.Errorf("BookBudget(binance) = %+v, want %+v — it must read the BOOK section", gotBook, binanceBookBudget)
	}
}

func TestFreshnessConfigRefusesABareNumberForADuration(t *testing.T) {
	// 300 meaning five minutes decodes to 300 NANOSECONDS, which is a blind
	// budget that has expired before it is first checked. config.Duration
	// refuses it; this is the assertion that this schema uses that type and
	// not a plain time.Duration.
	doc := []byte(`{"funding":{"binance":{"max_age":30,"blind":300}}}`)

	if _, err := config.New[driven.FreshnessConfig]("test", at, doc); err == nil {
		t.Fatal("New accepted a numeric duration; 300 would mean 300ns")
	}
}

// ---------------------------------------------------------------------------
// The adapter
// ---------------------------------------------------------------------------

func TestNewConfigBudgetsRefusesANilStore(t *testing.T) {
	if _, err := driven.NewConfigBudgets(nil); !errors.Is(err, driven.ErrStoreNil) {
		t.Fatalf("NewConfigBudgets(nil) = %v, want ErrStoreNil", err)
	}
}

func TestConfigBudgetsAnswersPerVenue(t *testing.T) {
	b := newBudgets(t, twoVenues())

	for venue, want := range map[ids.Venue]domain.Budget{
		binance: binanceBudget,
		bybit:   bybitBudget,
	} {
		got, ok := b.FundingBudget(venue)
		if !ok {
			t.Fatalf("FundingBudget(%s) reported no budget", venue)
		}
		if got != want {
			t.Errorf("FundingBudget(%s) = %+v, want %+v", venue, got, want)
		}
	}
}

func TestConfigBudgetsReportsAnUnconfiguredVenue(t *testing.T) {
	b := newBudgets(t, twoVenues())

	got, ok := b.FundingBudget(unknown)
	if ok {
		t.Fatalf("FundingBudget(%s) = %+v, true; want a miss", unknown, got)
	}
	// The zero Budget is unusable — domain.Budget.Validate refuses it — so a
	// caller that ignored the bool still cannot trade on it.
	if err := got.Validate(); err == nil {
		t.Error("the missed budget validates; a caller ignoring ok would use it")
	}
}

func TestConfigBudgetsSeesAHotReload(t *testing.T) {
	// The port's whole reason for being a port: a budget edited during an
	// incident must reach a use case constructed at boot. An adapter that
	// snapshotted config at construction passes every other test in this file.
	store := newStore(t, twoVenues())
	b, err := driven.NewConfigBudgets(store)
	if err != nil {
		t.Fatalf("NewConfigBudgets: %v", err)
	}

	widened := domain.Budget{MaxAge: 2 * time.Minute, Blind: 20 * time.Minute}
	next := driven.FreshnessConfig{Funding: map[ids.Venue]driven.BudgetSpec{
		binance: spec(widened),
		bybit:   spec(bybitBudget),
	}, Book: books()}
	if _, err := store.Reload("test", at.Add(time.Hour), encode(t, next)); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	got, ok := b.FundingBudget(binance)
	if !ok {
		t.Fatal("FundingBudget(binance) reported no budget after reload")
	}
	if got != widened {
		t.Errorf("FundingBudget(binance) = %+v, want the reloaded %+v", got, widened)
	}
	// The venue that did NOT change must still answer, or "sees the reload"
	// and "lost everything else" would be the same observation.
	if got, ok := b.FundingBudget(bybit); !ok || got != bybitBudget {
		t.Errorf("FundingBudget(bybit) = %+v, %t; want the unchanged %+v", got, ok, bybitBudget)
	}
}

func TestConfigBudgetsSeesAVenueAddedByReload(t *testing.T) {
	// A key-set change is a STRUCTURAL change in config's reload policy, which
	// is refused outright unless the field is tagged hot. So this is the test
	// that the tag is present: without it, onboarding a venue needs a restart,
	// and nothing about the code would say so.
	store := newStore(t, driven.FreshnessConfig{Funding: map[ids.Venue]driven.BudgetSpec{
		binance: spec(binanceBudget),
	}, Book: books()})
	b, err := driven.NewConfigBudgets(store)
	if err != nil {
		t.Fatalf("NewConfigBudgets: %v", err)
	}
	if _, ok := b.FundingBudget(bybit); ok {
		t.Fatal("FundingBudget(bybit) answered before bybit was configured")
	}

	if _, err := store.Reload("test", at.Add(time.Hour), encode(t, twoVenues())); err != nil {
		t.Fatalf("Reload adding a venue: %v", err)
	}

	got, ok := b.FundingBudget(bybit)
	if !ok {
		t.Fatal("FundingBudget(bybit) reported no budget after the venue was added")
	}
	if got != bybitBudget {
		t.Errorf("FundingBudget(bybit) = %+v, want %+v", got, bybitBudget)
	}
}
