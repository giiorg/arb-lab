package driven_test

import (
	"context"
	"errors"
	"testing"

	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/infrastructure/driven"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// This file checks the margin-status ACL (task #208).
//
// # What the checklist demanded here
//
//   - The reader RECORDS its urgency, for recordingPositionReader's reason: a
//     §7.6 shed class pinned inside an adapter is invisible from the return
//     value, and the class this one pins is why the read is not shed during the
//     incident it is reserved for.
//   - Every figure in the fixture is a DIFFERENT number, and the liq price is
//     stated. The port type mirrors the engine type field for field, which is
//     precisely the shape where a transposed assignment produces a fully
//     populated reading — so the translation is asserted value by value.
//   - The unstated-liq-price case is its own test, because LiqPriceStated is the
//     one field a transposition-free copy can silently drop while the reading
//     still validates on the engine's side.
//   - The reader's ERROR and its held=false are separate tests asserting
//     OPPOSITE things about the same two return values, because collapsing them
//     is the defect: an unanswered read reported as flat takes §7.1's trigger
//     off a live short.

type recordingMarginReader struct {
	status venue.MarginStatus
	held   bool
	err    error

	instruments []ids.VenueInstrument
	urgencies   []ratelimit.Priority
	assets      []venue.InstrumentAssets
}

func (r *recordingMarginReader) MarginStatus(_ context.Context, instrument ids.VenueInstrument, assets venue.InstrumentAssets, urgency ratelimit.Priority) (venue.MarginStatus, bool, error) {
	r.instruments = append(r.instruments, instrument)
	r.urgencies = append(r.urgencies, urgency)
	r.assets = append(r.assets, assets)
	if r.err != nil {
		return venue.MarginStatus{}, false, r.err
	}
	return r.status, r.held, nil
}

var _ driven.VenueMarginStatusReader = (*recordingMarginReader)(nil)

const (
	marginLiqFixture    = "71234.55"
	marginMaintFixture  = "312.75"
	marginPostedFixture = "1850.25"
)

func liveVenueMarginStatus(t *testing.T) venue.MarginStatus {
	t.Helper()
	liq, err := fixedpoint.ParsePrice("BTC", "USDT", marginLiqFixture)
	if err != nil {
		t.Fatalf("ParsePrice: %v", err)
	}
	maint, err := fixedpoint.ParseMoney("USDT", marginMaintFixture)
	if err != nil {
		t.Fatalf("ParseMoney: %v", err)
	}
	posted, err := fixedpoint.ParseMoney("USDT", marginPostedFixture)
	if err != nil {
		t.Fatalf("ParseMoney: %v", err)
	}
	return venue.MarginStatus{
		Instrument:     perpInstrument(),
		LiqPrice:       liq,
		LiqPriceStated: true,
		Maintenance:    maint,
		PostedMargin:   posted,
	}
}

func newMarginAdapter(t *testing.T, reader *recordingMarginReader) *driven.VenueMarginStatus {
	t.Helper()
	adapter, err := driven.NewVenueMarginStatus(reader, positionAssets())
	if err != nil {
		t.Fatalf("NewVenueMarginStatus: %v", err)
	}
	return adapter
}

func TestVenueMarginStatusTranslatesEveryFieldAndPinsTheSafetyClass(t *testing.T) {
	t.Parallel()

	reader := &recordingMarginReader{status: liveVenueMarginStatus(t), held: true}
	reading, held, err := newMarginAdapter(t, reader).MarginStatus(perpInstrument())
	if err != nil {
		t.Fatalf("MarginStatus: %v", err)
	}
	if !held {
		t.Fatal("held = false on a live position")
	}

	if reading.Instrument != perpInstrument() {
		t.Errorf("instrument = %v, want %v", reading.Instrument, perpInstrument())
	}
	if !reading.LiqPriceStated {
		t.Error("LiqPriceStated = false — the flag is the half consumers branch on, and losing it makes a stated liq price unread")
	}
	if reading.LiqPrice.Decimal() != marginLiqFixture {
		t.Errorf("LiqPrice = %s, want %s", reading.LiqPrice.Decimal(), marginLiqFixture)
	}
	if reading.Maintenance.Decimal() != marginMaintFixture {
		t.Errorf("Maintenance = %s, want %s", reading.Maintenance.Decimal(), marginMaintFixture)
	}
	if reading.PostedMargin.Decimal() != marginPostedFixture {
		t.Errorf("PostedMargin = %s, want %s", reading.PostedMargin.Decimal(), marginPostedFixture)
	}

	// §7.1 names this a Safety read twice: the stale-mark failover polls it
	// "from the reserved rate-limit allocation", and the top-up escalation
	// "re-queries margin + position first". Shed under load, the escalation
	// decides from a stale before-state during exactly the burst that produced
	// the stress.
	if len(reader.urgencies) != 1 || reader.urgencies[0] != ratelimit.PrioritySafety {
		t.Errorf("urgencies = %v, want exactly one Safety", reader.urgencies)
	}
	if len(reader.assets) != 1 || reader.assets[0] != (venue.InstrumentAssets{Base: "BTC", Quote: "USDT"}) {
		t.Errorf("assets = %v, want the declared pair for this instrument", reader.assets)
	}
}

func TestVenueMarginStatusCarriesAnUnstatedLiqPrice(t *testing.T) {
	t.Parallel()

	status := liveVenueMarginStatus(t)
	status.LiqPrice, status.LiqPriceStated = fixedpoint.Price{}, false

	reading, held, err := newMarginAdapter(t, &recordingMarginReader{status: status, held: true}).
		MarginStatus(perpInstrument())
	if err != nil {
		t.Fatalf("MarginStatus: %v", err)
	}
	if !held {
		t.Fatal("held = false — an unstated liq price does not make a live position disappear")
	}
	if reading.LiqPriceStated {
		t.Error("LiqPriceStated = true — the venue stated none, and a false here is NEVER 'far from liquidation'")
	}
	// The rest of the reading is still what a top-up is sized from.
	if reading.Maintenance.Decimal() != marginMaintFixture {
		t.Errorf("Maintenance = %s, want %s", reading.Maintenance.Decimal(), marginMaintFixture)
	}
}

func TestVenueMarginStatusReportsAFlatInstrument(t *testing.T) {
	t.Parallel()

	reading, held, err := newMarginAdapter(t, &recordingMarginReader{held: false}).
		MarginStatus(perpInstrument())
	if err != nil {
		t.Fatalf("MarginStatus: %v — a flat instrument is the ordinary answer, not an error", err)
	}
	if held {
		t.Error("held = true where the venue holds nothing")
	}
	if reading != (ports.MarginReading{}) {
		t.Errorf("reading = %+v beside held=false — a flat instrument must carry no figures", reading)
	}
}

func TestVenueMarginStatusNeverReportsAFailedReadAsFlat(t *testing.T) {
	t.Parallel()

	// The opposite assertion to the test above, on the same two return values.
	// Flat means "this position needs no monitoring"; unanswered means "this
	// position's liq distance is unknown", which is §7.1's stale-input rung.
	// Collapsing them silently stops the trigger watching a live short.
	sentinel := errors.New("venue unreachable")
	_, held, err := newMarginAdapter(t, &recordingMarginReader{err: sentinel}).
		MarginStatus(perpInstrument())
	if !errors.Is(err, sentinel) {
		t.Errorf("err = %v, want the reader's own error unwrapped", err)
	}
	if held {
		t.Error("held = true on a failed read")
	}
}

func TestVenueMarginStatusRefusesAnUndeclaredInstrument(t *testing.T) {
	t.Parallel()

	// Refused, not reported flat. The venue's decimals cannot be typed without
	// this instrument's assets, so the honest answer is "nobody can say" — and
	// reading it as flat takes the trigger off a live short because of a line
	// missing from a config file.
	other := ids.VenueInstrument{
		Venue: "binance", Product: ids.ProductLinearPerp, Symbol: "ETHUSDT", Contract: ids.ContractLinear,
	}
	reader := &recordingMarginReader{status: liveVenueMarginStatus(t), held: true}

	_, held, err := newMarginAdapter(t, reader).MarginStatus(other)
	if !errors.Is(err, driven.ErrMarginInstrumentUndeclared) {
		t.Errorf("err = %v, want ErrMarginInstrumentUndeclared", err)
	}
	if held {
		t.Error("held = true for an instrument nobody declared")
	}
	if len(reader.instruments) != 0 {
		t.Errorf("the venue was called %d times for an undeclared instrument — the refusal must precede the call, which costs §7.6 budget", len(reader.instruments))
	}
}

// TestVenueMarginStatusValidatesWhatItsReaderHandsOver was written after a
// mutation SURVIVED: deleting the Validate call broke no test, because every
// fixture handed the adapter a status the engine would itself have accepted.
//
// The gap was real rather than an equivalent mutant. This adapter's reader is an
// INTERFACE — the real client validates before returning, and nothing about the
// seam requires that of whatever else satisfies it. The field the check earns
// its place on is LiqPriceStated: the translation is field-for-field, so losing
// the flag while keeping the price is the one slip that leaves a reading the
// ENGINE's own status still validates, and it is the flag every consumer
// branches on.
func TestVenueMarginStatusValidatesWhatItsReaderHandsOver(t *testing.T) {
	t.Parallel()

	status := liveVenueMarginStatus(t)
	status.LiqPriceStated = false // the price survives, the flag does not

	_, held, err := newMarginAdapter(t, &recordingMarginReader{status: status, held: true}).
		MarginStatus(perpInstrument())
	if !errors.Is(err, ports.ErrMarginReadingLiqPriceContradicts) {
		t.Errorf("err = %v, want ErrMarginReadingLiqPriceContradicts — the port checks its own contract at its own boundary", err)
	}
	if held {
		t.Error("held = true on a reading the port refuses")
	}
}

func TestVenueMarginStatusRefusesAnAnswerForAnotherInstrument(t *testing.T) {
	t.Parallel()

	// The engine builds the instrument from the one it was given, so a mismatch
	// means the layers disagree about which market answered — the one error here
	// that yields a confident liquidation price for the wrong position.
	status := liveVenueMarginStatus(t)
	status.Instrument = ids.VenueInstrument{
		Venue: "binance", Product: ids.ProductLinearPerp, Symbol: "ETHUSDT", Contract: ids.ContractLinear,
	}

	_, held, err := newMarginAdapter(t, &recordingMarginReader{status: status, held: true}).
		MarginStatus(perpInstrument())
	if !errors.Is(err, driven.ErrMarginStatusOutsideScope) {
		t.Errorf("err = %v, want ErrMarginStatusOutsideScope", err)
	}
	if held {
		t.Error("held = true on an answer for a different instrument")
	}
}

func TestNewVenueMarginStatusRefusesAnUnusableWiring(t *testing.T) {
	t.Parallel()

	if _, err := driven.NewVenueMarginStatus(nil, positionAssets()); !errors.Is(err, driven.ErrVenueMarginStatusReaderMissing) {
		t.Errorf("nil reader = %v, want ErrVenueMarginStatusReaderMissing", err)
	}

	// An empty table refuses every instrument, forever, with every call
	// succeeding at the transport — §7.1 would meet it as "unknown" on every
	// position rather than as a wiring gap.
	if _, err := driven.NewVenueMarginStatus(&recordingMarginReader{}, nil); !errors.Is(err, driven.ErrInstrumentAssetsMissing) {
		t.Errorf("empty table = %v, want ErrInstrumentAssetsMissing", err)
	}

	// A mixed table would let a caller name venue B's instrument against venue
	// A's client and be refused two layers down by route() — a runtime
	// diagnosis for a mistake this constructor can name.
	mixed := positionAssets()
	mixed[ids.VenueInstrument{
		Venue: "bybit", Product: ids.ProductLinearPerp, Symbol: "BTCUSDT", Contract: ids.ContractLinear,
	}] = venue.InstrumentAssets{Base: "BTC", Quote: "USDT"}
	if _, err := driven.NewVenueMarginStatus(&recordingMarginReader{}, mixed); !errors.Is(err, driven.ErrInstrumentVenuesMixed) {
		t.Errorf("mixed table = %v, want ErrInstrumentVenuesMixed", err)
	}
}

// TestMarginReadingValidateIsPositivelyControlled proves the port's own checks
// are not universally objecting — the adapter runs Validate on every
// translation, so a predicate that refused everything would make each refusal
// below read as a pass.
func TestMarginReadingValidateIsPositivelyControlled(t *testing.T) {
	t.Parallel()

	s := liveVenueMarginStatus(t)
	live := ports.MarginReading{
		Instrument:     s.Instrument,
		LiqPrice:       s.LiqPrice,
		LiqPriceStated: s.LiqPriceStated,
		Maintenance:    s.Maintenance,
		PostedMargin:   s.PostedMargin,
	}
	if err := live.Validate(); err != nil {
		t.Fatalf("Validate() on a real reading = %v, want nil", err)
	}

	unstated := live
	unstated.LiqPrice, unstated.LiqPriceStated = fixedpoint.Price{}, false
	if err := unstated.Validate(); err != nil {
		t.Fatalf("Validate() with no stated liq price = %v, want nil", err)
	}

	if err := (ports.MarginReading{}).Validate(); err == nil {
		t.Error("the zero MarginReading validated — an unpopulated maintenance reads as a position requiring nothing")
	}

	spot := live
	spot.Instrument = ids.VenueInstrument{Venue: "binance", Product: ids.ProductSpot, Symbol: "BTCUSDT"}
	if err := spot.Validate(); !errors.Is(err, ports.ErrMarginReadingSurfaceAbsent) {
		t.Errorf("spot reading = %v, want ErrMarginReadingSurfaceAbsent", err)
	}

	contradicting := live
	contradicting.LiqPriceStated = false
	if err := contradicting.Validate(); !errors.Is(err, ports.ErrMarginReadingLiqPriceContradicts) {
		t.Errorf("price beside a cleared flag = %v, want ErrMarginReadingLiqPriceContradicts", err)
	}

	foreign, err := fixedpoint.ParseMoney("USDC", marginPostedFixture)
	if err != nil {
		t.Fatalf("ParseMoney: %v", err)
	}
	mismatched := live
	mismatched.PostedMargin = foreign
	if err := mismatched.Validate(); !errors.Is(err, ports.ErrMarginReadingAssetsDisagree) {
		t.Errorf("mixed assets = %v, want ErrMarginReadingAssetsDisagree", err)
	}
}
