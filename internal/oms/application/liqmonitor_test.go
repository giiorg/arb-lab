package application_test

import (
	"errors"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	riskdomain "github.com/giiorg/arb-lab/internal/risk/domain"
)

// §7.1's liq-distance trigger (task #210) — the first consumer of both read
// ports.
//
// Four fixture decisions here came from the collapse checklist and each closes a
// distinction the code makes that a natural fixture would have hidden:
//
//   - TWO venues with DIFFERENT mark budgets, so "this venue's max age" and "the
//     only max age" are different values.
//   - Age and DataAge always DIFFERENT, with one case stale on each clock alone,
//     so `Age > b || DataAge > b` is distinguishable from either half.
//   - A FLAT instrument with a stale mark, so "the mark is screened
//     unconditionally" is distinguishable from "the mark is screened for held
//     positions".
//   - A held position whose margin read AND mark are both broken, so the
//     precedence between the two reasons is pinned rather than incidental.

const (
	liqBinanceMaxAge = 5 * time.Second
	liqBybitMaxAge   = 30 * time.Second
)

func liqPerp() ids.VenueInstrument {
	return ids.VenueInstrument{
		Venue: "binance", Product: ids.ProductLinearPerp,
		Symbol: "BTCUSDT", Contract: ids.ContractLinear,
	}
}

// liqAltPerp is on the OTHER venue and carries the other budget. Two venues is
// what makes the per-venue lookup observable at all.
func liqAltPerp() ids.VenueInstrument {
	return ids.VenueInstrument{
		Venue: "bybit", Product: ids.ProductLinearPerp,
		Symbol: "SOLUSDT", Contract: ids.ContractLinear,
	}
}

// fakeMarkRisk is ports.MarkRiskReader with a scripted answer per instrument,
// recording the instants it was asked about.
type fakeMarkRisk struct {
	readings map[ids.VenueInstrument]ports.MarkReading
	observed map[ids.VenueInstrument]bool
	err      error
	gotNow   time.Time
	gotSrv   time.Time
	calls    int
}

var _ ports.MarkRiskReader = (*fakeMarkRisk)(nil)

func (f *fakeMarkRisk) Mark(vi ids.VenueInstrument, now, serverNow time.Time) (ports.MarkReading, bool, error) {
	f.calls++
	f.gotNow, f.gotSrv = now, serverNow
	if f.err != nil {
		return ports.MarkReading{}, false, f.err
	}
	r, ok := f.readings[vi]
	if !ok || !f.observed[vi] {
		return ports.MarkReading{}, false, nil
	}
	return r, true, nil
}

// fakeMargin is ports.VenueMarginStatus with a scripted answer per instrument.
type fakeMargin struct {
	statuses map[ids.VenueInstrument]ports.MarginReading
	held     map[ids.VenueInstrument]bool
	err      error

	// reads records every instrument this port was asked about, in order.
	//
	// The dispatch path's claim needs it and no earlier test did (task #215): a
	// dispatcher that ran §7.1's evidence contract after a successful call would
	// produce the same verdict on this venue and a DIFFERENT one on a venue with
	// an act history, so the observable difference is the second read rather
	// than the answer.
	reads []ids.VenueInstrument
}

var _ ports.VenueMarginStatus = (*fakeMargin)(nil)

func (f *fakeMargin) MarginStatus(vi ids.VenueInstrument) (ports.MarginReading, bool, error) {
	f.reads = append(f.reads, vi)
	if f.err != nil {
		return ports.MarginReading{}, false, f.err
	}
	return f.statuses[vi], f.held[vi], nil
}

func liqRate(t *testing.T, s string) fixedpoint.Rate {
	t.Helper()
	r, err := fixedpoint.ParseRate(s)
	if err != nil {
		t.Fatalf("ParseRate(%q): %v", s, err)
	}
	return r
}

func liqPrice(t *testing.T, vi ids.VenueInstrument, s string) fixedpoint.Price {
	t.Helper()
	base := fixedpoint.Asset("BTC")
	if vi.Symbol == "SOLUSDT" {
		base = "SOL"
	}
	p, err := fixedpoint.ParsePrice(base, "USDT", s)
	if err != nil {
		t.Fatalf("ParsePrice(%q): %v", s, err)
	}
	return p
}

func liqPolicy(t *testing.T) riskdomain.LiqDistancePolicy {
	t.Helper()
	return riskdomain.LiqDistancePolicy{
		Open:      liqRate(t, "0.30"),
		Alert:     liqRate(t, "0.25"),
		Rebalance: liqRate(t, "0.20"),
		Emergency: liqRate(t, "0.15"),
	}
}

func liqPolicies(t *testing.T) riskdomain.LiqDistancePolicies {
	t.Helper()
	alt := riskdomain.LiqDistancePolicy{
		Open:      liqRate(t, "0.50"),
		Alert:     liqRate(t, "0.45"),
		Rebalance: liqRate(t, "0.40"),
		Emergency: liqRate(t, "0.35"),
	}
	return riskdomain.LiqDistancePolicies{liqPerp(): liqPolicy(t), liqAltPerp(): alt}
}

func liqBudgets() map[ids.Venue]time.Duration {
	return map[ids.Venue]time.Duration{"binance": liqBinanceMaxAge, "bybit": liqBybitMaxAge}
}

func liqClock(t *testing.T) *clock.TestClock {
	t.Helper()
	clk, err := clock.NewTestClock(testNow, []clock.Calibration{
		{Venue: "binance", Offset: 250 * time.Millisecond, Uncertainty: 40 * time.Millisecond},
		{Venue: "bybit", Offset: -100 * time.Millisecond, Uncertainty: 60 * time.Millisecond},
	})
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	return clk
}

// freshMark is a reading well inside both venues' budgets, with the two ages
// DIFFERENT so that neither can stand in for the other.
func freshMark(t *testing.T, vi ids.VenueInstrument, mark string) ports.MarkReading {
	t.Helper()
	return ports.MarkReading{
		Instrument: vi, Price: liqPrice(t, vi, mark),
		Age: time.Second, DataAge: 2 * time.Second,
	}
}

type liqFixture struct {
	monitor *application.LiqMonitor
	mark    *fakeMarkRisk
	margin  *fakeMargin
}

func newLiqMonitor(t *testing.T) liqFixture {
	t.Helper()
	mark := &fakeMarkRisk{
		readings: map[ids.VenueInstrument]ports.MarkReading{},
		observed: map[ids.VenueInstrument]bool{},
	}
	margin := &fakeMargin{
		statuses: map[ids.VenueInstrument]ports.MarginReading{},
		held:     map[ids.VenueInstrument]bool{},
	}
	m, err := application.NewLiqMonitor(mark, margin, liqPolicies(t), liqBudgets(), liqClock(t))
	if err != nil {
		t.Fatalf("NewLiqMonitor: %v", err)
	}
	return liqFixture{monitor: m, mark: mark, margin: margin}
}

// hold scripts a held position with a stated liquidation price and a fresh mark.
func (f liqFixture) hold(t *testing.T, vi ids.VenueInstrument, mark, liq string) {
	t.Helper()
	f.mark.readings[vi] = freshMark(t, vi, mark)
	f.mark.observed[vi] = true
	f.margin.held[vi] = true
	f.margin.statuses[vi] = ports.MarginReading{
		Instrument: vi, LiqPrice: liqPrice(t, vi, liq), LiqPriceStated: true,
	}
}

func TestAssessPlacesAHeldShortOnTheLadder(t *testing.T) {
	t.Parallel()

	// The mark is fixed at 100 and only the liquidation price moves, so every
	// row differs in exactly the quantity §7.1's trigger is a function of.
	for _, tc := range []struct {
		name     string
		liq      string
		wantD    string
		wantBand riskdomain.LiqBand
		wantWith bool
	}{
		{"far clear — openings permitted", "140", "0.4", riskdomain.LiqBandClear, false},
		{"exactly at the open threshold", "130", "0.3", riskdomain.LiqBandClear, false},
		// The region the two-axis design exists for: clear, and closed.
		{"below open, above alert", "127", "0.27", riskdomain.LiqBandClear, true},
		{"alerting", "122", "0.22", riskdomain.LiqBandAlert, true},
		{"rebalance — §7.1's top-up rung", "118", "0.18", riskdomain.LiqBandRebalance, true},
		{"emergency", "110", "0.1", riskdomain.LiqBandEmergency, true},
		// The mark has crossed the liquidation price. The most urgent state on
		// the book, and it must not read as headroom.
		{"already breached", "90", "-0.1", riskdomain.LiqBandEmergency, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			f := newLiqMonitor(t)
			f.hold(t, liqPerp(), "100", tc.liq)

			a, err := f.monitor.Assess(liqPerp(), riskdomain.PositionSideShort)
			if err != nil {
				t.Fatalf("Assess: %v", err)
			}
			if err := a.Validate(); err != nil {
				t.Fatalf("the assessment does not validate: %v", err)
			}
			if a.Reason != application.LiqInputsComplete {
				t.Fatalf("reason = %s, want complete", a.Reason)
			}
			if !a.Held {
				t.Error("Held = false on a position the venue reported")
			}
			if a.Distance.Decimal() != tc.wantD {
				t.Errorf("distance = %s, want %s", a.Distance.Decimal(), tc.wantD)
			}
			if a.Band != tc.wantBand {
				t.Errorf("band = %s, want %s", a.Band, tc.wantBand)
			}
			if a.OpeningWithheld != tc.wantWith {
				t.Errorf("OpeningWithheld = %v, want %v", a.OpeningWithheld, tc.wantWith)
			}
			// The two ages are carried through separately. Equal values here
			// would make a swap invisible.
			if a.MarkAge != time.Second || a.MarkDataAge != 2*time.Second {
				t.Errorf("ages = (%v, %v), want (1s, 2s)", a.MarkAge, a.MarkDataAge)
			}
		})
	}
}

func TestAssessReadsThePolicyOfTheInstrumentItWasAsked(t *testing.T) {
	t.Parallel()

	// One distance, two instruments, two policies, two bands. With a one-row
	// table this passes against a monitor that ignores the key entirely.
	f := newLiqMonitor(t)
	f.hold(t, liqPerp(), "100", "138")    // 38% — clear under BTC's ladder
	f.hold(t, liqAltPerp(), "100", "138") // 38% — rebalance under the alt's

	btc, err := f.monitor.Assess(liqPerp(), riskdomain.PositionSideShort)
	if err != nil {
		t.Fatalf("Assess: %v", err)
	}
	alt, err := f.monitor.Assess(liqAltPerp(), riskdomain.PositionSideShort)
	if err != nil {
		t.Fatalf("Assess: %v", err)
	}
	if btc.Distance.Decimal() != alt.Distance.Decimal() {
		t.Fatalf("the two distances differ (%s vs %s) — then the bands could differ for that reason instead",
			btc.Distance, alt.Distance)
	}
	if btc.Band != riskdomain.LiqBandClear {
		t.Errorf("BTC band = %s, want clear", btc.Band)
	}
	if alt.Band != riskdomain.LiqBandRebalance {
		t.Errorf("alt band = %s, want rebalance", alt.Band)
	}
	if btc.OpeningWithheld {
		t.Error("38%% withheld openings under a 30%% open threshold")
	}
	if !alt.OpeningWithheld {
		t.Error("38%% permitted openings under a 50%% open threshold")
	}
}

func TestAssessJudgesTheMarkAgainstItsOwnVenuesBudgetAndBothClocks(t *testing.T) {
	t.Parallel()

	// Binance's budget is 5s and Bybit's is 30s. The reading below is 20s old on
	// the transport clock, which is stale on one venue and fresh on the other —
	// so a monitor reading "a" max age rather than "this venue's" gets one of
	// the two rows wrong whichever constant it picked.
	for _, tc := range []struct {
		name         string
		vi           ids.VenueInstrument
		age, dataAge time.Duration
		wantReason   application.LiqUnknownReason
	}{
		{"fresh on both clocks", liqPerp(), time.Second, 2 * time.Second, application.LiqInputsComplete},
		{"20s on the 5s venue", liqPerp(), 20 * time.Second, 2 * time.Second, application.LiqMarkStale},
		{"20s on the 30s venue", liqAltPerp(), 20 * time.Second, 2 * time.Second, application.LiqInputsComplete},
		// Each clock alone condemns the reading. Without BOTH rows, one of the
		// two comparisons could be deleted with nothing failing — and the
		// data-age row is the venue-side stall, where frames keep arriving on
		// schedule with a frozen SourceTime.
		{"transport stale, data fresh", liqPerp(), 6 * time.Second, time.Second, application.LiqMarkStale},
		{"transport fresh, data stale", liqPerp(), time.Second, 6 * time.Second, application.LiqMarkStale},
		// Exactly at the budget is still fresh: a budget is a permission to
		// trust up to and including its bound (marketdata's inclusive-fresh
		// boundary, restated one context over).
		{"exactly at the budget on both clocks", liqPerp(), liqBinanceMaxAge, liqBinanceMaxAge, application.LiqInputsComplete},
		{"one nanosecond past the budget", liqPerp(), liqBinanceMaxAge + 1, time.Second, application.LiqMarkStale},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			f := newLiqMonitor(t)
			f.hold(t, tc.vi, "100", "140")
			r := f.mark.readings[tc.vi]
			r.Age, r.DataAge = tc.age, tc.dataAge
			f.mark.readings[tc.vi] = r

			a, err := f.monitor.Assess(tc.vi, riskdomain.PositionSideShort)
			if err != nil {
				t.Fatalf("Assess: %v", err)
			}
			if a.Reason != tc.wantReason {
				t.Fatalf("reason = %s, want %s", a.Reason, tc.wantReason)
			}
			if tc.wantReason == application.LiqMarkStale {
				if a.Band != riskdomain.LiqBandUnknown {
					t.Errorf("band = %s on a stale mark, want unknown", a.Band)
				}
				if !a.OpeningWithheld {
					t.Error("a stale mark did not block openings — §7.1 says it must")
				}
				if a.Distance != (fixedpoint.Rate{}) {
					t.Errorf("a distance of %s survived a stale mark", a.Distance)
				}
			}
		})
	}
}

func TestAssessScreensTheMarkEvenOnAFlatInstrument(t *testing.T) {
	t.Parallel()

	// §7.1 blocks openings on the FEED's health, not on whether a position
	// exists — and the mark stream is public, so it is answerable for an
	// instrument this system holds nothing on. A monitor that returned early on
	// flat would make the block conditional on already holding the exposure it
	// exists to prevent acquiring.
	f := newLiqMonitor(t)
	f.mark.readings[liqPerp()] = ports.MarkReading{
		Instrument: liqPerp(), Price: liqPrice(t, liqPerp(), "100"),
		Age: 20 * time.Second, DataAge: time.Second,
	}
	f.mark.observed[liqPerp()] = true
	f.margin.held[liqPerp()] = false

	a, err := f.monitor.Assess(liqPerp(), riskdomain.PositionSideShort)
	if err != nil {
		t.Fatalf("Assess: %v", err)
	}
	if a.Reason != application.LiqMarkStale {
		t.Errorf("reason = %s on a flat instrument with a stale mark, want mark-stale", a.Reason)
	}
	if !a.OpeningWithheld {
		t.Error("a stale mark on a flat instrument did not block openings")
	}
	if a.Held {
		t.Error("Held = true on a flat instrument")
	}
	if f.mark.calls == 0 {
		t.Error("the mark port was never read")
	}
}

func TestAFlatInstrumentWithAHealthyMarkWithholdsNothing(t *testing.T) {
	t.Parallel()

	// The deadlock this ruling avoids: a flat instrument has no liquidation
	// price, so treating "no distance" uniformly would veto the opening — and
	// the evidence that would lift the veto is produced only BY opening. The
	// permitted region would be unreachable from a flat book and the system
	// would never trade, with every individual refusal reading as the gate
	// working.
	f := newLiqMonitor(t)
	f.mark.readings[liqPerp()] = freshMark(t, liqPerp(), "100")
	f.mark.observed[liqPerp()] = true
	f.margin.held[liqPerp()] = false

	a, err := f.monitor.Assess(liqPerp(), riskdomain.PositionSideShort)
	if err != nil {
		t.Fatalf("Assess: %v", err)
	}
	if err := a.Validate(); err != nil {
		t.Fatalf("the assessment does not validate: %v", err)
	}
	if a.Reason != application.LiqPositionFlat {
		t.Errorf("reason = %s, want position-flat", a.Reason)
	}
	if a.OpeningWithheld {
		t.Error("a flat instrument with a healthy mark blocked its own first opening")
	}
	if a.Band != riskdomain.LiqBandUnknown {
		t.Errorf("band = %s on a flat instrument, want unknown", a.Band)
	}
}

func TestAnUnstatedLiquidationPriceWithholdsWithoutEscalating(t *testing.T) {
	t.Parallel()

	// TASK #209's ruling, as a test. Bybit documents `liqPrice: ""` when the
	// price falls outside the symbol's band — the WELL-MARGINED case — so the
	// absence arrives on the healthy path. Two things must hold and they pull in
	// opposite directions:
	//
	//   it is never "far from liquidation"      → openings are withheld
	//   it is never an escalation trigger        → the band is Unknown, not Emergency
	//
	// The second is the one that is easy to get wrong in the "conservative"
	// direction: escalating here would de-lever, which RAISES headroom, which
	// pushes the liq price further outside the band, which makes the venue
	// withhold it again — a trigger whose remedy strengthens it, whose only
	// fixed point is flat.
	f := newLiqMonitor(t)
	f.mark.readings[liqAltPerp()] = freshMark(t, liqAltPerp(), "100")
	f.mark.observed[liqAltPerp()] = true
	f.margin.held[liqAltPerp()] = true
	f.margin.statuses[liqAltPerp()] = ports.MarginReading{Instrument: liqAltPerp()}

	a, err := f.monitor.Assess(liqAltPerp(), riskdomain.PositionSideShort)
	if err != nil {
		t.Fatalf("Assess: %v", err)
	}
	// The ruling's own two assertions come FIRST, before Validate. Validate
	// happens to refuse a band beside an incomplete reason, so running it first
	// would make it the thing that catches an escalating mutant — and the
	// consistency rule and the #209 ruling are different claims that must each
	// be seen to bite.
	if a.Reason != application.LiqLiquidationPriceUnstated {
		t.Fatalf("reason = %s, want liq-price-unstated", a.Reason)
	}
	if !a.OpeningWithheld {
		t.Error("an unstated liq price permitted an opening — absence is never evidence of headroom")
	}
	if a.Band != riskdomain.LiqBandUnknown {
		t.Errorf("band = %s — an unstated liq price must not drive the escalation ladder (#209); "+
			"the de-lever it would fire RAISES headroom, which is what makes the venue withhold the price again", a.Band)
	}
	if !a.Held {
		t.Error("Held = false — the venue reported a position; only its liq price is missing")
	}
	if err := a.Validate(); err != nil {
		t.Fatalf("the assessment does not validate: %v", err)
	}
}

func TestAnUnreadableMarginReadIsNeverFlatAndOutranksTheMark(t *testing.T) {
	t.Parallel()

	// Two claims in one fixture because they are the same decision. §5.3 forbids
	// re-deriving an absence from an unanswered query, so a failed margin read
	// must NOT report the instrument flat — which would silently stop the
	// trigger watching a live short. And when the mark is broken too, the margin
	// failure is the one reported, because it is the one that leaves this system
	// unable to say whether it holds anything at all.
	f := newLiqMonitor(t)
	f.margin.err = errors.New("venue timeout")
	f.mark.err = errors.New("clock inversion")

	a, err := f.monitor.Assess(liqPerp(), riskdomain.PositionSideShort)
	if err != nil {
		t.Fatalf("Assess returned an error for a venue-side failure: %v", err)
	}
	if a.Reason != application.LiqMarginUnreadable {
		t.Errorf("reason = %s, want margin-unreadable (it outranks the mark verdict)", a.Reason)
	}
	if a.Reason == application.LiqPositionFlat {
		t.Error("an unanswered margin read was reported as a flat instrument")
	}
	if !a.OpeningWithheld {
		t.Error("an unreadable margin read permitted an opening")
	}
	if a.Held {
		t.Error("Held = true from a read that failed")
	}
}

func TestAMissingOrUnreadableMarkAreDifferentAnswers(t *testing.T) {
	t.Parallel()

	// "Never observed" and "observed and undatable" call for different operator
	// responses — boot versus a clock-calibration fault on this host — and the
	// port's own contract refuses to collapse them, so neither does this.
	for _, tc := range []struct {
		name   string
		script func(*liqFixture)
		want   application.LiqUnknownReason
	}{
		{"never observed", func(f *liqFixture) { f.mark.observed[liqPerp()] = false }, application.LiqMarkAbsent},
		{"port error", func(f *liqFixture) { f.mark.err = errors.New("clock inversion") }, application.LiqMarkUnreadable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			f := newLiqMonitor(t)
			f.hold(t, liqPerp(), "100", "140")
			tc.script(&f)

			a, err := f.monitor.Assess(liqPerp(), riskdomain.PositionSideShort)
			if err != nil {
				t.Fatalf("Assess: %v", err)
			}
			if a.Reason != tc.want {
				t.Errorf("reason = %s, want %s", a.Reason, tc.want)
			}
			if !a.OpeningWithheld {
				t.Error("openings were permitted with no usable mark")
			}
			if a.MarkAge != 0 || a.MarkDataAge != 0 {
				t.Errorf("ages (%v, %v) survived an unusable mark", a.MarkAge, a.MarkDataAge)
			}
		})
	}
}

func TestAssessMeasuresTheMarkAgainstTwoDIFFERENTClocks(t *testing.T) {
	t.Parallel()

	// The port takes engine time for the transport age and the VENUE's time for
	// the data age, and a monitor passing the same instant twice would satisfy
	// the signature while making a venue-side stall unmeasurable. The clock's
	// binance calibration is +250ms, so the two instants must differ by exactly
	// that.
	f := newLiqMonitor(t)
	f.hold(t, liqPerp(), "100", "140")
	if _, err := f.monitor.Assess(liqPerp(), riskdomain.PositionSideShort); err != nil {
		t.Fatalf("Assess: %v", err)
	}
	if !f.mark.gotNow.Equal(testNow) {
		t.Errorf("engine instant = %v, want %v", f.mark.gotNow, testNow)
	}
	want := testNow.Add(250 * time.Millisecond)
	if !f.mark.gotSrv.Equal(want) {
		t.Errorf("venue instant = %v, want %v (the calibrated offset)", f.mark.gotSrv, want)
	}
	if f.mark.gotNow.Equal(f.mark.gotSrv) {
		t.Fatal("both instants are the same — then the two ages measure the same thing")
	}
}

func TestAssessRefusesWiringDefectsRatherThanReportingThem(t *testing.T) {
	t.Parallel()

	// The other side of the error contract: conditions no venue can cause and no
	// retry can clear come back as errors, so they cannot be mistaken for an
	// outage the ladder is handling.
	f := newLiqMonitor(t)
	f.hold(t, liqPerp(), "100", "140")

	unlisted := ids.VenueInstrument{
		Venue: "binance", Product: ids.ProductLinearPerp,
		Symbol: "ETHUSDT", Contract: ids.ContractLinear,
	}
	if _, err := f.monitor.Assess(unlisted, riskdomain.PositionSideShort); !errors.Is(err, riskdomain.ErrLiqPolicyUnconfigured) {
		t.Errorf("unconfigured instrument: %v, want ErrLiqPolicyUnconfigured", err)
	}
	if _, err := f.monitor.Assess(liqPerp(), riskdomain.PositionSideUnspecified); !errors.Is(err, riskdomain.ErrPositionSideInvalid) {
		t.Errorf("unstated side: %v, want ErrPositionSideInvalid", err)
	}
	if _, err := f.monitor.Assess(ids.VenueInstrument{}, riskdomain.PositionSideShort); err == nil {
		t.Error("an unset instrument was assessed")
	}
}

func TestNewLiqMonitorRefusesAMonitorThatCouldNeverJudge(t *testing.T) {
	t.Parallel()

	mark := &fakeMarkRisk{}
	margin := &fakeMargin{}

	// One omission at a time, each with its own sentinel: together they would
	// pass against a constructor that refused everything for one reason.
	for _, tc := range []struct {
		name     string
		mark     ports.MarkRiskReader
		margin   ports.VenueMarginStatus
		policies riskdomain.LiqDistancePolicies
		budgets  map[ids.Venue]time.Duration
		clk      clock.Clock
		want     error
	}{
		{"no mark port", nil, margin, liqPolicies(t), liqBudgets(), liqClock(t), application.ErrMarkRiskReaderNil},
		{"no margin port", mark, nil, liqPolicies(t), liqBudgets(), liqClock(t), application.ErrVenueMarginStatusNil},
		{"no clock", mark, margin, liqPolicies(t), liqBudgets(), nil, application.ErrClockMissing},
		{"empty policy table", mark, margin, riskdomain.LiqDistancePolicies{}, liqBudgets(), liqClock(t), riskdomain.ErrLiqPoliciesEmpty},
		// A budget table covering ONE of the two venues. The gap it forecloses
		// is the hardest kind to see: distances keep being computed for the
		// covered venue while the other's staleness rung silently never fires.
		{"a venue with no budget", mark, margin, liqPolicies(t),
			map[ids.Venue]time.Duration{"binance": liqBinanceMaxAge}, liqClock(t), application.ErrMarkMaxAgeUnconfigured},
		{"a budget of zero", mark, margin, liqPolicies(t),
			map[ids.Venue]time.Duration{"binance": liqBinanceMaxAge, "bybit": 0}, liqClock(t), application.ErrMarkMaxAgeNotPositive},
		{"a negative budget", mark, margin, liqPolicies(t),
			map[ids.Venue]time.Duration{"binance": liqBinanceMaxAge, "bybit": -time.Second}, liqClock(t), application.ErrMarkMaxAgeNotPositive},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := application.NewLiqMonitor(tc.mark, tc.margin, tc.policies, tc.budgets, tc.clk)
			if !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}

	// An uncalibrated venue: the data age would be measured against the zero
	// instant, and every mark on that venue would read as permanently unusable
	// on a venue that is in fact healthy. Refused at construction, by name.
	half, err := clock.NewTestClock(testNow, []clock.Calibration{
		{Venue: "binance", Offset: 250 * time.Millisecond, Uncertainty: 40 * time.Millisecond},
	})
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	if _, err := application.NewLiqMonitor(mark, margin, liqPolicies(t), liqBudgets(), half); !errors.Is(err, application.ErrVenueUncalibrated) {
		t.Errorf("uncalibrated venue: %v, want ErrVenueUncalibrated", err)
	}

	// The positive control: the fully-wired monitor builds, so every refusal
	// above is measuring an omission rather than a constructor that objects to
	// everything.
	if _, err := application.NewLiqMonitor(mark, margin, liqPolicies(t), liqBudgets(), liqClock(t)); err != nil {
		t.Fatalf("the fully-wired monitor was refused: %v", err)
	}
}

func TestLiqMonitorDoesNotAliasTheTablesItWasGiven(t *testing.T) {
	t.Parallel()

	policies := liqPolicies(t)
	budgets := liqBudgets()
	mark := &fakeMarkRisk{
		readings: map[ids.VenueInstrument]ports.MarkReading{liqPerp(): freshMark(t, liqPerp(), "100")},
		observed: map[ids.VenueInstrument]bool{liqPerp(): true},
	}
	margin := &fakeMargin{
		statuses: map[ids.VenueInstrument]ports.MarginReading{
			liqPerp(): {Instrument: liqPerp(), LiqPrice: liqPrice(t, liqPerp(), "140"), LiqPriceStated: true},
		},
		held: map[ids.VenueInstrument]bool{liqPerp(): true},
	}
	m, err := application.NewLiqMonitor(mark, margin, policies, budgets, liqClock(t))
	if err != nil {
		t.Fatalf("NewLiqMonitor: %v", err)
	}

	// A root that keeps its maps and edits them later must not be able to
	// retune §7.1's thresholds behind a constructed monitor's back — §5.2 makes
	// this policy restart-required, and an aliased map is a hot reload nobody
	// declared.
	delete(policies, liqPerp())
	budgets["binance"] = time.Nanosecond

	a, err := m.Assess(liqPerp(), riskdomain.PositionSideShort)
	if err != nil {
		t.Fatalf("Assess after the caller edited its maps: %v", err)
	}
	if a.Reason != application.LiqInputsComplete {
		t.Errorf("reason = %s, want complete — the monitor is reading the caller's mutated tables", a.Reason)
	}
	if a.Band != riskdomain.LiqBandClear {
		t.Errorf("band = %s, want clear", a.Band)
	}
}

func TestAssessmentValidateRefusesStatesAssessCannotProduce(t *testing.T) {
	t.Parallel()

	good := application.LiqAssessment{
		Instrument: liqPerp(), Side: riskdomain.PositionSideShort, Held: true,
		Band: riskdomain.LiqBandAlert, Distance: liqRate(t, "0.22"),
		Reason: application.LiqInputsComplete,
	}
	if err := good.Validate(); err != nil {
		t.Fatalf("a well-formed assessment was refused: %v", err)
	}

	// Every case is a literal a consumer's fixture could write and no path here
	// produces — which is exactly the set that would let B3's and B4's tests
	// pass against states that cannot occur.
	for _, tc := range []struct {
		name   string
		mutate func(application.LiqAssessment) application.LiqAssessment
	}{
		{"unassessed reason", func(a application.LiqAssessment) application.LiqAssessment {
			a.Reason = application.LiqInputsUnassessed
			return a
		}},
		{"undeclared reason", func(a application.LiqAssessment) application.LiqAssessment {
			a.Reason = application.LiqUnknownReason(99)
			return a
		}},
		{"unset side", func(a application.LiqAssessment) application.LiqAssessment {
			a.Side = riskdomain.PositionSideUnspecified
			return a
		}},
		{"complete beside an unknown band", func(a application.LiqAssessment) application.LiqAssessment {
			a.Band, a.Distance = riskdomain.LiqBandUnknown, fixedpoint.Rate{}
			return a
		}},
		{"a band beside an incomplete reason", func(a application.LiqAssessment) application.LiqAssessment {
			a.Reason, a.Distance = application.LiqMarkStale, fixedpoint.Rate{}
			return a
		}},
		{"a distance beside an incomplete reason", func(a application.LiqAssessment) application.LiqAssessment {
			a.Reason, a.Band = application.LiqMarkStale, riskdomain.LiqBandUnknown
			return a
		}},
		{"a computed distance for an unheld position", func(a application.LiqAssessment) application.LiqAssessment {
			a.Held = false
			return a
		}},
		{"flat and held at once", func(a application.LiqAssessment) application.LiqAssessment {
			a.Reason, a.Band, a.Distance = application.LiqPositionFlat, riskdomain.LiqBandUnknown, fixedpoint.Rate{}
			return a
		}},
		{"unset instrument", func(a application.LiqAssessment) application.LiqAssessment {
			a.Instrument = ids.VenueInstrument{}
			return a
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if err := tc.mutate(good).Validate(); err == nil {
				t.Error("Validate accepted a state Assess cannot produce")
			}
		})
	}
}

func TestLiqUnknownReasonRendersEveryMemberDistinctly(t *testing.T) {
	t.Parallel()

	seen := map[string]bool{}
	for _, r := range []application.LiqUnknownReason{
		application.LiqInputsUnassessed, application.LiqInputsComplete, application.LiqPositionFlat,
		application.LiqMarginUnreadable, application.LiqMarkAbsent, application.LiqMarkStale,
		application.LiqMarkUnreadable, application.LiqLiquidationPriceUnstated,
	} {
		s := r.String()
		if s == "" {
			t.Errorf("reason %d renders empty", uint8(r))
		}
		if seen[s] {
			t.Errorf("two reasons render as %q — they would collide as metric labels", s)
		}
		seen[s] = true
	}
	if application.LiqUnknownReason(99).String() == application.LiqMarkStale.String() {
		t.Error("an undeclared reason renders as a declared one")
	}
}
