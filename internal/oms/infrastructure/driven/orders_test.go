package driven_test

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/oms/infrastructure/driven"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

const account = "sub-1"

// mintedID builds an id through the real constructor rather than as a literal,
// on §25d's finding: a hand-written id that stops parsing after a format change
// turns whichever test depends on it into a test of something else, silently.
func mintedID(t *testing.T, leg int) ids.ClientOrderID {
	t.Helper()
	trade, err := ids.ParseTradeID("018f0c1a-2b3c-7d4e-8f90-1a2b3c4d5e6f")
	if err != nil {
		t.Fatalf("ParseTradeID: %v", err)
	}
	id, err := ids.NewClientOrderID(ids.ClientOrderRef{Trade: trade, Leg: leg})
	if err != nil {
		t.Fatalf("NewClientOrderID: %v", err)
	}
	return id
}

// perp and spot are the SAME symbol on the same venue, differing only in
// Product — the ledger suite's fixture choice, for its reason: a mismatch built
// from two different symbols cannot tell "compares the whole instrument" from
// "compares the symbol", and this is the pair that actually occurs, since a
// cash-and-carry trade holds an order on each at the same instant.
func perp() ids.VenueInstrument {
	return ids.VenueInstrument{
		Venue: "binance", Product: ids.ProductLinearPerp,
		Symbol: "BTCUSDT", Contract: ids.ContractLinear,
	}
}

func spot() ids.VenueInstrument {
	return ids.VenueInstrument{Venue: "binance", Product: ids.ProductSpot, Symbol: "BTCUSDT"}
}

func qty(t *testing.T, s string) fixedpoint.Quantity {
	t.Helper()
	q, err := fixedpoint.ParseQuantity("BTC", s)
	if err != nil {
		t.Fatalf("ParseQuantity(%q): %v", s, err)
	}
	return q
}

// plannedOrder is a valid order, so each test varies exactly the field it is
// about.
func plannedOrder(t *testing.T) domain.Order {
	t.Helper()
	px, err := fixedpoint.ParsePrice("BTC", "USDT", "50000")
	if err != nil {
		t.Fatalf("ParsePrice: %v", err)
	}
	return domain.Order{
		ClientOrderID: mintedID(t, 1),
		Instrument:    perp(),
		Side:          domain.SideSell,
		Type:          domain.OrderTypeLimit,
		TimeInForce:   domain.TimeInForceIOC,
		Quantity:      qty(t, "10"),
		LimitPrice:    px,
	}
}

// created returns a repository holding one planned order at version 1.
func created(t *testing.T) (*driven.MemoryOrderWriteRepository, domain.Order) {
	t.Helper()
	repo := driven.NewMemoryOrderWriteRepository()
	o := plannedOrder(t)
	if _, err := repo.ApplyPlanned(o); err != nil {
		t.Fatalf("ApplyPlanned: %v", err)
	}
	return repo, o
}

// execution builds a fill for o. The instrument is taken FROM the order rather
// than restated, so a test varying the instrument has to say so.
func execution(t *testing.T, o domain.Order, executionID, quantity string) domain.Fill {
	t.Helper()
	px, err := fixedpoint.ParsePrice("BTC", "USDT", "50000")
	if err != nil {
		t.Fatalf("ParsePrice: %v", err)
	}
	fee, err := fixedpoint.ParseMoney("USDT", "0.5")
	if err != nil {
		t.Fatalf("ParseMoney: %v", err)
	}
	return domain.Fill{
		ExecutionID: executionID,
		Account:     account,
		Instrument:  o.Instrument,
		Order:       o.ClientOrderID,
		Quantity:    qty(t, quantity),
		Price:       px,
		Fee:         fee,
		Ts:          time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC),
	}
}

// advance walks the record forward, failing the test on any refusal. It returns
// the version reached, so a caller never hardcodes one it did not compute.
func advance(t *testing.T, repo *driven.MemoryOrderWriteRepository, o domain.Order, states ...domain.OrderState) int64 {
	t.Helper()
	version := int64(1)
	for _, s := range states {
		rec, err := repo.ApplyState(o.ClientOrderID, version, s)
		if err != nil {
			t.Fatalf("ApplyState(%s): %v", s, err)
		}
		version = rec.Version
	}
	return version
}

func TestApplyPlannedCreatesTheRecordAtPlannedAndVersionOne(t *testing.T) {
	t.Parallel()

	repo := driven.NewMemoryOrderWriteRepository()
	o := plannedOrder(t)

	rec, err := repo.ApplyPlanned(o)
	if err != nil {
		t.Fatalf("ApplyPlanned: %v", err)
	}

	if rec.State != domain.StatePlanned {
		t.Errorf("State = %s, want %s — §5.3's recovery rules read a new record as never dispatched",
			rec.State, domain.StatePlanned)
	}
	if rec.Version != 1 {
		t.Errorf("Version = %d, want 1 — zero is the absence of a version, not a version", rec.Version)
	}
	if rec.Fills != 0 {
		t.Errorf("Fills = %d, want 0", rec.Fills)
	}
	if rec.Order != o {
		t.Errorf("Order = %+v, want %+v", rec.Order, o)
	}
}

// The asset tag is checked, not only the number. A cumulative total that starts
// as fixedpoint's untagged zero validates and formats identically, and fails on
// the order's FIRST fill — the one place nobody looks for a construction bug.
func TestApplyPlannedStartsTheCumulativeAtZeroOfTheOrderedAsset(t *testing.T) {
	t.Parallel()

	repo, o := created(t)

	rec, err := repo.Load(o.ClientOrderID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := rec.Cumulative.Asset(); got != o.Quantity.Asset() {
		t.Errorf("Cumulative asset = %q, want %q", got, o.Quantity.Asset())
	}
	if got := rec.Cumulative.Decimal(); got != "0" {
		t.Errorf("Cumulative = %s, want 0", got)
	}
}

func TestApplyPlannedRefusesAnOrderThatDoesNotValidate(t *testing.T) {
	t.Parallel()

	// Each case is wrong in ONE way and reaches a DIFFERENT sentinel, so the
	// test cannot pass on a repository that refuses everything for one reason.
	cases := map[string]struct {
		mutate func(*domain.Order)
		want   error
	}{
		"no client order id": {
			mutate: func(o *domain.Order) { o.ClientOrderID = "" },
			want:   domain.ErrOrderUnset,
		},
		"side is a venue token nobody normalised": {
			mutate: func(o *domain.Order) { o.Side = "SELL" },
			want:   domain.ErrSideInvalid,
		},
		"ordered quantity is zero": {
			mutate: func(o *domain.Order) { o.Quantity = fixedpoint.ZeroQuantity("BTC") },
			want:   domain.ErrOrderedQuantityNotPositive,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			repo := driven.NewMemoryOrderWriteRepository()
			o := plannedOrder(t)
			tc.mutate(&o)

			if _, err := repo.ApplyPlanned(o); !errors.Is(err, tc.want) {
				t.Fatalf("ApplyPlanned error = %v, want %v", err, tc.want)
			}

			// Nothing may be left behind by a refusal. A repository that stored
			// the record and then reported the error would satisfy the line
			// above and hold an order whose quantity is zero.
			if _, err := repo.Load(plannedOrder(t).ClientOrderID); !errors.Is(err, ports.ErrOrderUnknown) {
				t.Errorf("Load after a refusal = %v, want %v", err, ports.ErrOrderUnknown)
			}
		})
	}
}

// The repeat is a VALID order, or "refuses a repeat" collapses into "refuses an
// invalid order" — §25d's item, one slice on.
//
// The second case is the one that matters most: a repeat carrying a DIFFERENT
// instrument is what silently attaches a new order's fills to the old order's
// instrument, and both quantities stay plausible while every net-delta the
// §6.7 classifier computes is wrong.
func TestApplyPlannedRefusesARepeatOfAValidOrder(t *testing.T) {
	t.Parallel()

	cases := map[string]func(domain.Order) domain.Order{
		"the identical order":      func(o domain.Order) domain.Order { return o },
		"the id on another symbol": func(o domain.Order) domain.Order { o.Instrument = spot(); return o },
	}

	for name, repeat := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			repo, o := created(t)

			if _, err := repo.ApplyPlanned(repeat(o)); !errors.Is(err, domain.ErrDuplicateOrder) {
				t.Fatalf("second ApplyPlanned error = %v, want %v", err, domain.ErrDuplicateOrder)
			}

			rec, err := repo.Load(o.ClientOrderID)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if rec.Order != o || rec.Version != 1 {
				t.Errorf("record after the refused repeat = %+v v%d, want the original at v1",
					rec.Order, rec.Version)
			}
		})
	}
}

func TestLoadRefusesAnOrderTheRepositoryNeverCreated(t *testing.T) {
	t.Parallel()

	repo := driven.NewMemoryOrderWriteRepository()

	if _, err := repo.Load(mintedID(t, 1)); !errors.Is(err, ports.ErrOrderUnknown) {
		t.Fatalf("Load error = %v, want %v", err, ports.ErrOrderUnknown)
	}
}

// TWO consecutive advances, deliberately. With one, `version++` and
// `version = 2` are the same implementation and the test cannot tell them
// apart — the fixture pinned to the value that collapses the distinction.
func TestApplyStateAdvancesTheLifecycleAndBumpsTheVersionByOneEachTime(t *testing.T) {
	t.Parallel()

	repo, o := created(t)

	first, err := repo.ApplyState(o.ClientOrderID, 1, domain.StateDispatchAuthorized)
	if err != nil {
		t.Fatalf("first ApplyState: %v", err)
	}
	if first.State != domain.StateDispatchAuthorized || first.Version != 2 {
		t.Fatalf("after the first advance: %s v%d, want %s v2",
			first.State, first.Version, domain.StateDispatchAuthorized)
	}

	second, err := repo.ApplyState(o.ClientOrderID, first.Version, domain.StateSubmitted)
	if err != nil {
		t.Fatalf("second ApplyState: %v", err)
	}
	if second.State != domain.StateSubmitted || second.Version != 3 {
		t.Errorf("after the second advance: %s v%d, want %s v3",
			second.State, second.Version, domain.StateSubmitted)
	}

	// The order itself is untouched by a state write. §5.1 makes the intent
	// immutable, and a repository that rebuilt the record would be the place
	// that quietly stopped being true.
	if second.Order != o {
		t.Errorf("Order = %+v, want %+v", second.Order, o)
	}
}

func TestApplyStateRefusesATransitionThatIsNotAnEdgeOfTheLifecycle(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		to   domain.OrderState
		want error
	}{
		// Two states on from Planned, skipping the crash boundary entirely.
		"a state SM §1 does not reach from here": {
			to: domain.StateFilled, want: domain.ErrTransitionForbidden,
		},
		// Refused rather than tolerated as a harmless identity — see
		// domain.ErrTransitionNoOp for why the terminal claim depends on it.
		"the state it is already in": {
			to: domain.StatePlanned, want: domain.ErrTransitionNoOp,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			repo, o := created(t)

			if _, err := repo.ApplyState(o.ClientOrderID, 1, tc.to); !errors.Is(err, tc.want) {
				t.Fatalf("ApplyState(%s) error = %v, want %v", tc.to, err, tc.want)
			}

			rec, err := repo.Load(o.ClientOrderID)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if rec.State != domain.StatePlanned || rec.Version != 1 {
				t.Errorf("record after the refusal = %s v%d, want %s v1 — a refused write must leave nothing behind",
					rec.State, rec.Version, domain.StatePlanned)
			}
		})
	}
}

// Both directions of wrong, because `!=` is the claim and a mutant can weaken
// it either way: `<` admits every writer that has read ahead, `>` admits every
// writer that is behind — and the second is the lost update this check exists
// for.
func TestApplyStateRefusesAnyVersionThatIsNotTheRecordsOwn(t *testing.T) {
	t.Parallel()

	cases := map[string]int64{
		"the zero of a caller that never loaded": 0,
		"one behind, after another writer moved": 1,
		"one ahead":                              3,
	}

	for name, expected := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			repo, o := created(t)
			// Move to version 2, so that both a stale (1) and an unreached (3)
			// expectation are available and neither is the record's own.
			advance(t, repo, o, domain.StateDispatchAuthorized)

			_, err := repo.ApplyState(o.ClientOrderID, expected, domain.StateSubmitted)
			if !errors.Is(err, ports.ErrVersionConflict) {
				t.Fatalf("ApplyState at expected=%d error = %v, want %v", expected, err, ports.ErrVersionConflict)
			}

			rec, err := repo.Load(o.ClientOrderID)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if rec.State != domain.StateDispatchAuthorized || rec.Version != 2 {
				t.Errorf("record after the refusal = %s v%d, want %s v2",
					rec.State, rec.Version, domain.StateDispatchAuthorized)
			}
		})
	}
}

// The write is wrong in BOTH ways at once, which is the only fixture that can
// see the order of the two checks. A caller holding a stale version has a stale
// idea of the state too, so an illegal edge is the SYMPTOM; reporting it would
// send a reader to SM §1's table to explain a race.
func TestApplyStateReportsTheVersionConflictWhenTheEdgeIsAlsoIllegal(t *testing.T) {
	t.Parallel()

	repo, o := created(t)
	advance(t, repo, o, domain.StateDispatchAuthorized)

	_, err := repo.ApplyState(o.ClientOrderID, 1, domain.StateFilled)
	if !errors.Is(err, ports.ErrVersionConflict) {
		t.Fatalf("error = %v, want %v", err, ports.ErrVersionConflict)
	}
	if errors.Is(err, domain.ErrTransitionForbidden) {
		t.Errorf("error also reports %v — the conflict is the cause, the forbidden edge is its symptom",
			domain.ErrTransitionForbidden)
	}
}

func TestApplyStateRefusesAnOrderTheRepositoryNeverCreated(t *testing.T) {
	t.Parallel()

	repo := driven.NewMemoryOrderWriteRepository()

	_, err := repo.ApplyState(mintedID(t, 1), 1, domain.StateDispatchAuthorized)
	if !errors.Is(err, ports.ErrOrderUnknown) {
		t.Fatalf("ApplyState error = %v, want %v", err, ports.ErrOrderUnknown)
	}
}

// Three executions of DIFFERENT sizes, summing to a total that is none of them
// and not twice any of them: "sums every fill", "keeps the last" and "doubles
// the first" give three different answers.
func TestApplyFillAccumulatesAcrossDistinctExecutions(t *testing.T) {
	t.Parallel()

	repo, o := created(t)

	version := int64(1)
	for _, f := range []domain.Fill{
		execution(t, o, "e-1", "1"),
		execution(t, o, "e-2", "2"),
		execution(t, o, "e-3", "4"),
	} {
		rec, applied, err := repo.ApplyFill(f, version)
		if err != nil {
			t.Fatalf("ApplyFill(%s): %v", f.ExecutionID, err)
		}
		if !applied {
			t.Fatalf("ApplyFill(%s) reported no change", f.ExecutionID)
		}
		version = rec.Version
	}

	rec, err := repo.Load(o.ClientOrderID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := rec.Cumulative.Decimal(); got != "7" {
		t.Errorf("Cumulative = %s, want 7", got)
	}
	if rec.Fills != 3 {
		t.Errorf("Fills = %d, want 3", rec.Fills)
	}
	if rec.Version != 4 {
		t.Errorf("Version = %d, want 4 — one bump per applied fill, from 1", rec.Version)
	}
	if rec.State != domain.StatePlanned {
		t.Errorf("State = %s, want %s — a fill moves quantity, never state", rec.State, domain.StatePlanned)
	}
}

// §4 puts the same execution on two streams and §5.3 has REST recovery re-read
// fills the private stream already delivered, so this is the ordinary path
// several times per fill — not an error, and not a version bump either.
func TestApplyFillReportsADuplicateWithoutMovingTheRecord(t *testing.T) {
	t.Parallel()

	repo, o := created(t)

	first, _, err := repo.ApplyFill(execution(t, o, "e-1", "3"), 1)
	if err != nil {
		t.Fatalf("first ApplyFill: %v", err)
	}

	again, applied, err := repo.ApplyFill(execution(t, o, "e-1", "3"), first.Version)
	if err != nil {
		t.Fatalf("duplicate ApplyFill: %v", err)
	}
	if applied {
		t.Error("duplicate ApplyFill reported a change")
	}
	if again.Version != first.Version {
		t.Errorf("Version = %d, want %d — a no-op that bumps the version invalidates every other reader for nothing",
			again.Version, first.Version)
	}
	if got := again.Cumulative.Decimal(); got != "3" {
		t.Errorf("Cumulative = %s, want 3 — the duplicate was counted", got)
	}
	if again.Fills != 1 {
		t.Errorf("Fills = %d, want 1", again.Fills)
	}
}

// The tempting order is dedupe first: a duplicate cannot lose an update, so a
// stale writer's repeat is arithmetically harmless and answering
// (record, false, nil) costs nothing. What it costs is the evidence — a
// goroutine holding a version nobody else has is the second-writer condition
// rule 6 exists to prevent, and this is the call that can still see it.
func TestApplyFillChecksTheVersionBeforeTheDedupe(t *testing.T) {
	t.Parallel()

	repo, o := created(t)

	if _, _, err := repo.ApplyFill(execution(t, o, "e-1", "3"), 1); err != nil {
		t.Fatalf("first ApplyFill: %v", err)
	}

	// Same execution, replayed by a caller still holding the pre-fill version.
	_, applied, err := repo.ApplyFill(execution(t, o, "e-1", "3"), 1)
	if !errors.Is(err, ports.ErrVersionConflict) {
		t.Fatalf("stale duplicate ApplyFill error = %v, want %v", err, ports.ErrVersionConflict)
	}
	if applied {
		t.Error("a refused ApplyFill reported a change")
	}
}

// §5.3: "a cumulative-execution ledger accepts any deduped fill for a KNOWN
// order regardless of its current state — so a post-cancel or
// post-SubmissionUnknown late fill still applies." The order here is walked all
// the way to a terminal first, which is the state a plausible implementation
// would gate on.
func TestApplyFillIgnoresTheOrdersLifecycleState(t *testing.T) {
	t.Parallel()

	repo, o := created(t)
	version := advance(t, repo, o,
		domain.StateDispatchAuthorized,
		domain.StateSubmitted,
		domain.StateAcknowledged,
		domain.StateCanceled,
	)

	rec, applied, err := repo.ApplyFill(execution(t, o, "e-late", "2"), version)
	if err != nil {
		t.Fatalf("ApplyFill on a canceled order: %v", err)
	}
	if !applied {
		t.Fatal("ApplyFill on a canceled order reported no change")
	}
	if got := rec.Cumulative.Decimal(); got != "2" {
		t.Errorf("Cumulative = %s, want 2", got)
	}
	if rec.State != domain.StateCanceled {
		t.Errorf("State = %s, want %s — admitting a fill must not revive the state",
			rec.State, domain.StateCanceled)
	}
}

func TestApplyFillRefusesAnExecutionOnAnotherInstrument(t *testing.T) {
	t.Parallel()

	repo, o := created(t)

	f := execution(t, o, "e-1", "3")
	f.Instrument = spot() // the other leg of the same cash-and-carry trade

	_, applied, err := repo.ApplyFill(f, 1)
	if !errors.Is(err, domain.ErrFillInstrumentMismatch) {
		t.Fatalf("ApplyFill error = %v, want %v", err, domain.ErrFillInstrumentMismatch)
	}
	if applied {
		t.Error("a refused ApplyFill reported a change")
	}

	rec, err := repo.Load(o.ClientOrderID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if rec.Version != 1 || rec.Fills != 0 {
		t.Errorf("record after the refusal = v%d with %d fills, want v1 with 0", rec.Version, rec.Fills)
	}
}

func TestApplyFillRefusesAnExecutionNamingAnOrderTheRepositoryNeverCreated(t *testing.T) {
	t.Parallel()

	repo := driven.NewMemoryOrderWriteRepository()
	o := plannedOrder(t)

	_, _, err := repo.ApplyFill(execution(t, o, "e-1", "3"), 1)
	if !errors.Is(err, ports.ErrOrderUnknown) {
		t.Fatalf("ApplyFill error = %v, want %v", err, ports.ErrOrderUnknown)
	}
}

// The claim the mutex alone cannot make, and the reason ports.ErrVersionConflict
// exists: every one of these writers is making a LEGAL transition from the
// state it read, so nothing in SM §1 can tell them apart. Exactly one may win.
//
// Bounded by construction — a fixed number of goroutines, a buffered channel
// sized to match, and no receive that a broken implementation could leave
// waiting. A test that hangs is not a test that fails.
func TestConcurrentApplyStateAdmitsExactlyOneWriterAtAVersion(t *testing.T) {
	t.Parallel()

	const writers = 8

	repo, o := created(t)

	// Two different legal targets from StatePlanned would be ideal, but SM §1
	// gives Planned exactly one successor — so the collision is the same edge
	// raced, which is the shape a retrying caller actually produces.
	results := make(chan error, writers)
	var start sync.WaitGroup
	var done sync.WaitGroup
	start.Add(1)
	for range writers {
		done.Add(1)
		go func() {
			defer done.Done()
			start.Wait()
			_, err := repo.ApplyState(o.ClientOrderID, 1, domain.StateDispatchAuthorized)
			results <- err
		}()
	}
	start.Done()
	done.Wait()
	close(results)

	var won int
	for err := range results {
		switch {
		case err == nil:
			won++
		case errors.Is(err, ports.ErrVersionConflict):
		default:
			t.Errorf("loser returned %v, want %v", err, ports.ErrVersionConflict)
		}
	}
	if won != 1 {
		t.Errorf("%d writers succeeded at version 1, want exactly 1", won)
	}

	rec, err := repo.Load(o.ClientOrderID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if rec.Version != 2 {
		t.Errorf("Version = %d, want 2 — %d writers, one applied write", rec.Version, writers)
	}
}

// The memory-safety half, which the version check says nothing about: every
// exported method touches the same two maps, so one that stopped taking the
// lock would be undefined behaviour rather than a wrong answer. Errors are
// ignored on purpose — this test is about the race detector, and asserting on
// outcomes would make it a flaky version test as well as a race test.
//
// Every fixture is built BEFORE the goroutines start. The helpers call
// t.Fatalf, and t.Fatalf outside the test goroutine ends the wrong goroutine —
// which would turn a fixture bug into a hang or a silent pass rather than a
// failure.
func TestConcurrentWritesAndReadsAreFreeOfDataRaces(t *testing.T) {
	t.Parallel()

	const (
		goroutines = 4
		iterations = 50
	)

	repo, o := created(t)

	fills := make([][]domain.Fill, goroutines)
	for g := range goroutines {
		fills[g] = make([]domain.Fill, iterations)
		for i := range iterations {
			fills[g][i] = execution(t, o, fmt.Sprintf("e-%d-%d", g, i), "1")
		}
	}

	var wg sync.WaitGroup
	for g := range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range iterations {
				rec, err := repo.Load(o.ClientOrderID)
				if err != nil {
					continue
				}
				repo.ApplyFill(fills[g][i], rec.Version)
				repo.ApplyState(o.ClientOrderID, rec.Version, domain.StateDispatchAuthorized)
			}
		}()
	}
	wg.Wait()
}
