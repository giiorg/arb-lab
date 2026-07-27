package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

const (
	account = "sub-1"
	order   = ids.ClientOrderID("ab-0123456789ABCDEFGHJKMNPQRS-00-00")
)

// spot and perp are the SAME symbol on the same venue, differing only in
// Product.
//
// That is the fixture choice the collapse checklist forces here. A mismatch
// fixture built from two different symbols cannot tell "compares the whole
// instrument" from "compares the symbol", and the pair that actually occurs is
// this one: §5.1 makes spot BTCUSDT and perp BTCUSDT distinct instruments, a
// cash-and-carry trade holds an order on each at the same instant, and their
// execution-ID namespaces are independent.
func spot() ids.VenueInstrument {
	return ids.VenueInstrument{Venue: "binance", Product: ids.ProductSpot, Symbol: "BTCUSDT"}
}

func perp() ids.VenueInstrument {
	return ids.VenueInstrument{
		Venue: "binance", Product: ids.ProductLinearPerp,
		Symbol: "BTCUSDT", Contract: ids.ContractLinear,
	}
}

func qty(t *testing.T, s string) fixedpoint.Quantity {
	t.Helper()
	q, err := fixedpoint.ParseQuantity("BTC", s)
	if err != nil {
		t.Fatalf("ParseQuantity(%q): %v", s, err)
	}
	return q
}

// fill builds a valid fill, so each test varies exactly the field it is about.
func fill(t *testing.T, executionID, quantity string) domain.Fill {
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
		Instrument:  spot(),
		Order:       order,
		Quantity:    qty(t, quantity),
		Price:       px,
		Fee:         fee,
		Ts:          time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC),
	}
}

// registered returns a ledger holding `order` on spot, placed for 10 BTC.
func registered(t *testing.T) *domain.Ledger {
	t.Helper()
	l := domain.NewLedger()
	if err := l.Register(order, spot(), qty(t, "10")); err != nil {
		t.Fatalf("Register: %v", err)
	}
	return l
}

// TestCumulativeSumsEveryFill uses THREE distinct quantities that are powers of
// two.
//
// One fill collapses sum, first and last into the same answer. Two collapse sum
// with "first + last". Three chosen as 1, 2 and 4 make the total distinct from
// every individual fill and from every pair of them, so no wrong fold produces
// 7.
func TestCumulativeSumsEveryFill(t *testing.T) {
	t.Parallel()

	l := registered(t)
	for i, spec := range []struct{ id, q string }{{"e1", "1"}, {"e2", "2"}, {"e3", "4"}} {
		applied, err := l.Apply(fill(t, spec.id, spec.q))
		if err != nil {
			t.Fatalf("Apply(%s): %v", spec.id, err)
		}
		if !applied {
			t.Fatalf("Apply(%s) reported no change on fill %d", spec.id, i)
		}
	}

	got, err := l.Cumulative(order)
	if err != nil {
		t.Fatalf("Cumulative: %v", err)
	}
	if got.Decimal() != "7" {
		t.Errorf("Cumulative = %s, want 7", got.Decimal())
	}

	// Ordered 10, filled 7 — so remaining is 3, which equals neither the total,
	// nor the ordered quantity, nor zero, nor any single fill. A fixture where
	// ordered were 14 would make remaining equal cumulative and hide a
	// subtraction written backwards.
	remaining, err := l.Remaining(order)
	if err != nil {
		t.Fatalf("Remaining: %v", err)
	}
	if remaining.Decimal() != "3" {
		t.Errorf("Remaining = %s, want 3", remaining.Decimal())
	}

	count, err := l.FillCount(order)
	if err != nil {
		t.Fatalf("FillCount: %v", err)
	}
	if count != 3 {
		t.Errorf("FillCount = %d, want 3", count)
	}
}

// TestDedupeKeyIsNotTheExecutionIDAlone is the §9 chaos case "identical
// spot/perp execution IDs".
//
// The fixture that carries it varies a NON-ExecutionID column while holding the
// execution ID fixed — the opposite of the natural fixture, which varies the
// execution ID and passes against a key made of the execution ID alone.
func TestDedupeKeyIsNotTheExecutionIDAlone(t *testing.T) {
	t.Parallel()

	perpOrder := ids.ClientOrderID("ab-0123456789ABCDEFGHJKMNPQRS-01-00")
	l := registered(t)
	if err := l.Register(perpOrder, perp(), qty(t, "10")); err != nil {
		t.Fatalf("Register perp: %v", err)
	}

	// Same account, same venue, same symbol, SAME execution id — different
	// product. Two real, distinct executions on a venue whose per-instrument
	// sequences collide.
	spotFill := fill(t, "12345", "1")
	perpFill := fill(t, "12345", "1")
	perpFill.Instrument = perp()
	perpFill.Order = perpOrder

	for _, f := range []domain.Fill{spotFill, perpFill} {
		applied, err := l.Apply(f)
		if err != nil {
			t.Fatalf("Apply(%s): %v", f.Instrument.Product, err)
		}
		if !applied {
			t.Fatalf("Apply(%s) deduped against the other product's execution; "+
				"the key has collapsed to the execution ID", f.Instrument.Product)
		}
	}

	// And the account column, held apart from the product one for the same
	// reason: a key that dropped Account would pass everything above.
	other := domain.NewLedger()
	otherOrder := ids.ClientOrderID("ab-0123456789ABCDEFGHJKMNPQRS-02-00")
	if err := other.Register(otherOrder, spot(), qty(t, "10")); err != nil {
		t.Fatalf("Register: %v", err)
	}
	a := fill(t, "12345", "1")
	a.Order = otherOrder
	b := a
	b.Account = "sub-2"
	keyA, err := a.Key()
	if err != nil {
		t.Fatalf("Key: %v", err)
	}
	keyB, err := b.Key()
	if err != nil {
		t.Fatalf("Key: %v", err)
	}
	if keyA == keyB {
		t.Error("two subaccounts' fills share a key; Account has dropped out of it")
	}
}

func TestApplyDedupesARepeatedExecution(t *testing.T) {
	t.Parallel()

	l := registered(t)
	f := fill(t, "e1", "1")

	applied, err := l.Apply(f)
	if err != nil || !applied {
		t.Fatalf("first Apply = (%v, %v), want (true, nil)", applied, err)
	}

	// The ordinary path, several times per fill: §4 puts the same execution on
	// StreamOrders and StreamExecutions, and REST recovery re-reads it again.
	applied, err = l.Apply(f)
	if err != nil {
		t.Errorf("duplicate Apply errored: %v — a duplicate is the ordinary case", err)
	}
	if applied {
		t.Error("duplicate Apply reported a change")
	}

	got, err := l.Cumulative(order)
	if err != nil {
		t.Fatalf("Cumulative: %v", err)
	}
	if got.Decimal() != "1" {
		t.Errorf("Cumulative = %s, want 1 — the duplicate was counted", got.Decimal())
	}
	count, err := l.FillCount(order)
	if err != nil {
		t.Fatalf("FillCount: %v", err)
	}
	if count != 1 {
		t.Errorf("FillCount = %d, want 1", count)
	}
}

// TestApplyReportsMisattributionBeforeDeduping pins the check order.
//
// The fixture is both a duplicate AND on the wrong instrument, which is the only
// input that can tell the two orderings apart: dedupe-first retires it as
// (false, nil) and the mapping bug is never seen again.
func TestApplyReportsMisattributionBeforeDeduping(t *testing.T) {
	t.Parallel()

	l := registered(t)
	f := fill(t, "e1", "1")
	if _, err := l.Apply(f); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	wrong := f
	wrong.Instrument = perp()
	applied, err := l.Apply(wrong)
	if applied {
		t.Error("a misattributed fill was applied")
	}
	if !errors.Is(err, domain.ErrFillInstrumentMismatch) {
		t.Errorf("Apply = %v, want ErrFillInstrumentMismatch — a duplicate that is "+
			"also misattributed must not be retired silently", err)
	}
}

func TestApplyRefusesAFillForAnUnknownOrder(t *testing.T) {
	t.Parallel()

	l := domain.NewLedger()
	f := fill(t, "e1", "1")
	applied, err := l.Apply(f)
	if applied {
		t.Error("a fill for an unregistered order was applied")
	}
	if !errors.Is(err, domain.ErrFillUnknownOrder) {
		t.Errorf("Apply = %v, want ErrFillUnknownOrder", err)
	}
	if !strings.Contains(err.Error(), string(order)) {
		t.Errorf("message %q does not name the order", err)
	}
}

// TestApplyRecordsAnOverFillRatherThanRefusingIt is §3.0's "venue reality wins"
// on the arithmetic.
func TestApplyRecordsAnOverFillRatherThanRefusingIt(t *testing.T) {
	t.Parallel()

	l := domain.NewLedger()
	if err := l.Register(order, spot(), qty(t, "5")); err != nil {
		t.Fatalf("Register: %v", err)
	}
	for _, spec := range []struct{ id, q string }{{"e1", "1"}, {"e2", "2"}, {"e3", "4"}} {
		if _, err := l.Apply(fill(t, spec.id, spec.q)); err != nil {
			t.Fatalf("Apply(%s): %v", spec.id, err)
		}
	}

	remaining, err := l.Remaining(order)
	if err != nil {
		t.Fatalf("Remaining: %v", err)
	}
	if remaining.Decimal() != "-2" {
		t.Errorf("Remaining = %s, want -2 — an over-execution clamped at zero "+
			"reports a package as flat while it holds reverse exposure", remaining.Decimal())
	}
}

// TestCumulativeOfAnUnfilledOrderCarriesTheAsset checks the tag, not only the
// value.
//
// fixedpoint.ZeroQuantity("BTC") and the untagged fixedpoint.Quantity{} both
// render as zero, so a test asserting the number alone passes against the one
// that fails its first Add.
func TestCumulativeOfAnUnfilledOrderCarriesTheAsset(t *testing.T) {
	t.Parallel()

	l := registered(t)
	got, err := l.Cumulative(order)
	if err != nil {
		t.Fatalf("Cumulative: %v", err)
	}
	if err := got.Validate(); err != nil {
		t.Errorf("the zero cumulative does not validate: %v", err)
	}
	if got.Asset() != "BTC" {
		t.Errorf("Cumulative asset = %q, want BTC", got.Asset())
	}
	if got.Decimal() != "0" {
		t.Errorf("Cumulative = %s, want 0", got.Decimal())
	}
}

func TestRegisterRefusesADuplicateAndAnEmptyOrder(t *testing.T) {
	t.Parallel()

	l := registered(t)
	if err := l.Register(order, spot(), qty(t, "10")); !errors.Is(err, domain.ErrDuplicateOrder) {
		t.Errorf("second Register = %v, want ErrDuplicateOrder", err)
	}
	if err := l.Register("", spot(), qty(t, "10")); !errors.Is(err, domain.ErrOrderUnset) {
		t.Errorf(`Register("") = %v, want ErrOrderUnset`, err)
	}
	if err := l.Register("ab-x", spot(), qty(t, "0")); !errors.Is(err, domain.ErrOrderedQuantityNotPositive) {
		t.Errorf("Register with zero quantity = %v, want ErrOrderedQuantityNotPositive", err)
	}
}

// TestRegisterRefusesAnUnvalidatedInstrument is the clause FillKey's doc comment
// says the checksum argument depends on.
//
// {linear_perp, inverse} is a contradiction VenueInstrument.Validate rejects.
// Admitted, it would be a distinct map key from the {linear_perp, linear} naming
// the same real instrument — so the same execution could apply twice, which is
// exactly the hole the wider key looked like it opened.
func TestRegisterRefusesAnUnvalidatedInstrument(t *testing.T) {
	t.Parallel()

	contradiction := ids.VenueInstrument{
		Venue: "binance", Product: ids.ProductLinearPerp,
		Symbol: "BTCUSDT", Contract: ids.ContractInverse,
	}
	l := domain.NewLedger()
	if err := l.Register(order, contradiction, qty(t, "10")); err == nil {
		t.Error("Register accepted an instrument whose contract contradicts its product")
	}

	f := fill(t, "e1", "1")
	f.Instrument = contradiction
	if _, err := f.Key(); err == nil {
		t.Error("Key accepted an instrument whose contract contradicts its product")
	}
}

// TestTheLedgerAdmitsFillsWithNoReferenceToOrderState states §5.3's
// orthogonality where a reader looks for it.
//
// The claim is enforced by the type — no method here takes an OrderState — so
// what this adds is the SEQUENCE: a fill arriving after the point a caller would
// treat the order as canceled, and after the point it would treat it as
// terminal, still applies. §5.3 calls that window unbounded.
func TestTheLedgerAdmitsFillsWithNoReferenceToOrderState(t *testing.T) {
	t.Parallel()

	l := registered(t)
	if _, err := l.Apply(fill(t, "e1", "1")); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// The caller now moves the order PartiallyFilled -> CancelPending ->
	// Canceled. The ledger is not told, because there is nothing to tell it.
	for _, edge := range [][2]domain.OrderState{
		{domain.StatePartiallyFilled, domain.StateCancelPending},
		{domain.StateCancelPending, domain.StateCanceled},
	} {
		if err := domain.CanTransition(edge[0], edge[1]); err != nil {
			t.Fatalf("CanTransition(%s, %s): %v", edge[0], edge[1], err)
		}
	}

	// A post-cancel late fill, which §5.3 and SM §1's chaos list both require to
	// apply.
	applied, err := l.Apply(fill(t, "e2", "2"))
	if err != nil {
		t.Fatalf("post-cancel Apply: %v", err)
	}
	if !applied {
		t.Fatal("a post-cancel late fill was not applied")
	}
	got, err := l.Cumulative(order)
	if err != nil {
		t.Fatalf("Cumulative: %v", err)
	}
	if got.Decimal() != "3" {
		t.Errorf("Cumulative = %s, want 3 — the ledger closed for a terminal order", got.Decimal())
	}
}

func TestFillValidateRefusesEachWayItCanBeUnusable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		mut  func(*domain.Fill)
		want error
	}{
		{"no execution id", func(f *domain.Fill) { f.ExecutionID = "" }, domain.ErrExecutionIDUnset},
		{"no account", func(f *domain.Fill) { f.Account = "" }, domain.ErrAccountUnset},
		{"no order", func(f *domain.Fill) { f.Order = "" }, domain.ErrOrderUnset},
		{"zero quantity", func(f *domain.Fill) { f.Quantity = qty(t, "0") }, domain.ErrFillQuantityNotPositive},
		{"negative quantity", func(f *domain.Fill) { f.Quantity = qty(t, "-1") }, domain.ErrFillQuantityNotPositive},
		{"no timestamp", func(f *domain.Fill) { f.Ts = time.Time{} }, domain.ErrFillTimestampUnset},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := fill(t, "e1", "1")
			tc.mut(&f)
			if err := f.Validate(); !errors.Is(err, tc.want) {
				t.Errorf("Validate = %v, want %v", err, tc.want)
			}
		})
	}

	// The positive control: without it a Validate that objected to everything
	// would score a perfect six above.
	if err := fill(t, "e1", "1").Validate(); err != nil {
		t.Errorf("the unmutated fill does not validate: %v", err)
	}
}

// TestApplyRefusesAFillWhoseAssetDisagrees checks that fixedpoint's refusal
// reaches the caller wrapped, rather than being duplicated here.
func TestApplyRefusesAFillWhoseAssetDisagrees(t *testing.T) {
	t.Parallel()

	l := registered(t)
	f := fill(t, "e1", "1")
	eth, err := fixedpoint.ParseQuantity("ETH", "1")
	if err != nil {
		t.Fatalf("ParseQuantity: %v", err)
	}
	f.Quantity = eth

	applied, err := l.Apply(f)
	if applied {
		t.Error("a fill in another asset was applied")
	}
	if err == nil {
		t.Fatal("Apply accepted an ETH fill against a BTC order")
	}
	if !strings.Contains(err.Error(), "e1") || !strings.Contains(err.Error(), string(order)) {
		t.Errorf("message %q names neither the execution nor the order", err)
	}

	// And nothing was half-applied: the refusal must leave the ledger where it
	// was, including the dedupe set, or a corrected retry would be swallowed.
	if count, err := l.FillCount(order); err != nil || count != 0 {
		t.Errorf("FillCount = (%d, %v) after a refused fill, want (0, nil)", count, err)
	}
	f.Quantity = qty(t, "1")
	applied, err = l.Apply(f)
	if err != nil || !applied {
		t.Errorf("the corrected retry = (%v, %v), want (true, nil) — the refused "+
			"fill consumed its dedupe key", applied, err)
	}
}

func TestKnowsSeparatesARegisteredOrderFromAnAbsentOne(t *testing.T) {
	t.Parallel()

	l := registered(t)
	if !l.Knows(order) {
		t.Error("Knows reports a registered order as absent")
	}
	if l.Knows("ab-0123456789ABCDEFGHJKMNPQRS-99-00") {
		t.Error("Knows reports an unregistered order as held")
	}
}
