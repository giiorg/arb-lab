package application_test

import (
	"errors"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// This file checks the spot half of §7.5's net-delta invariant — assertSpot's
// cell arithmetic against a recorded baseline (task #179, record §70c/§70h).
//
// # The collapse checklist, run before the tests were written
//
//   - Baseline ≠ 0 AND ours ≠ 0 on the main fixture (§70h item 3), so
//     `B == ours` and `B − baseline == ours` are different predicates.
//   - A fee in the BASE asset on a buy, a fee in a NON-base asset on the same
//     order, and a fee on a DIFFERENT pair debiting the baselined asset
//     (§70h items 1–2) — dropping the fee term, widening its scope to all
//     assets, or narrowing it to own-pair rows each moves the answer.
//   - BOTH delta signs on their own fixtures (§70h item 4).
//   - The absent-asset fixture's report holds TWO rows, neither the baselined
//     asset (§70h item 6) — "the row for this asset" and "the first row" must
//     differ, and absence must reach the subtraction as zero, not as a skip.
//   - TWO spot pairs sharing one base (§70h item 9), with different
//     cumulatives, so a per-instrument fold and the per-asset cell disagree.
//
// The records are PLANTED with their cumulative and fees rather than joined
// through the venue fakes: Assert reads the projection, and the sweep→ledger
// path that fills it is module_test's and record_test's subject. What this
// file owns is the arithmetic.

// fakeSpotBalances is ports.SpotBalances at the port, answering per venue and
// counting calls — the single-read-per-venue claim is a count.
type fakeSpotBalances struct {
	reports map[ids.Venue]ports.BalanceReport
	err     error
	calls   int
}

func newFakeSpotBalances() *fakeSpotBalances {
	return &fakeSpotBalances{reports: make(map[ids.Venue]ports.BalanceReport)}
}

func (f *fakeSpotBalances) Balances(s ports.SpotBalanceScope) (ports.BalanceReport, error) {
	f.calls++
	if f.err != nil {
		return ports.BalanceReport{}, f.err
	}
	return f.reports[s.Venue], nil
}

var _ ports.SpotBalances = (*fakeSpotBalances)(nil)

func spotUSDC() ids.VenueInstrument {
	return ids.VenueInstrument{Venue: "binance", Product: ids.ProductSpot, Symbol: "BTCUSDC"}
}

func ethSpot() ids.VenueInstrument {
	return ids.VenueInstrument{Venue: "binance", Product: ids.ProductSpot, Symbol: "ETHUSDT"}
}

func assetQty(t *testing.T, asset, v string) fixedpoint.Quantity {
	t.Helper()
	q, err := fixedpoint.ParseQuantity(fixedpoint.Asset(asset), v)
	if err != nil {
		t.Fatalf("ParseQuantity(%s, %q): %v", asset, v, err)
	}
	return q
}

func assetFee(t *testing.T, asset, v string) fixedpoint.Money {
	t.Helper()
	m, err := fixedpoint.ParseMoney(fixedpoint.Asset(asset), v)
	if err != nil {
		t.Fatalf("ParseMoney(%s, %q): %v", asset, v, err)
	}
	return m
}

// spotRoster is the one-pair roster most fixtures use.
func spotRoster() []application.SpotInstrument {
	return []application.SpotInstrument{{Instrument: spot(), Base: "BTC", Quote: "USDT"}}
}

func btcBaseline(t *testing.T, amount string) application.SpotBaseline {
	t.Helper()
	return application.SpotBaseline{Venue: "binance", Asset: "BTC", Amount: assetQty(t, "BTC", amount)}
}

// balanceRow is one pinned asset row; the words ride along because the port
// promises them and an empty-words row is a shape the adapter refuses.
func balanceRow(t *testing.T, asset, total string) ports.AssetBalance {
	t.Helper()
	return ports.AssetBalance{
		Asset: fixedpoint.Asset(asset),
		Total: assetQty(t, asset, total),
		Words: []ports.VerbatimField{{Name: "free", Value: total}, {Name: "locked", Value: "0"}},
	}
}

func balanceReport(t *testing.T, rows ...ports.AssetBalance) ports.BalanceReport {
	t.Helper()
	at, err := clock.NewServerTime(joinNow, testUncertainty)
	if err != nil {
		t.Fatalf("NewServerTime: %v", err)
	}
	// AbsenceIsZero by default: most fixtures state every asset they mean,
	// and the conclusive kind is the one whose silences are also answers.
	// Tests about the INCONCLUSIVE kind set the field themselves (§70i).
	return ports.BalanceReport{ObservedAt: at, Rows: rows, Absence: ports.AbsenceIsZero}
}

// newSpotJoiner mirrors newJoiner with a wired spot leg. No position scope:
// these fixtures are about the leg with no position surface.
func newSpotJoiner(t *testing.T, leg application.SpotLeg) (*application.Joiner, *fakeOrders) {
	t.Helper()
	orders := newFakeOrders()
	clk, err := clock.NewTestClock(joinNow, []clock.Calibration{
		{Venue: perp().Venue, Offset: testVenueOffset, Uncertainty: testUncertainty},
	})
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	j, err := application.NewJoiner(newFakeJoinVenue(), newFakePositions(), orders, clk, joinAccount, leg)
	if err != nil {
		t.Fatalf("NewJoiner: %v", err)
	}
	return j, orders
}

// plantAssertable puts a TERMINAL order in the repository with the cumulative
// and fees the projection would hold after a concluded join — the state
// Assert reads.
func plantAssertable(t *testing.T, r *fakeOrders, o domain.Order, cumulative string, fees ...fixedpoint.Money) {
	t.Helper()
	at, err := clock.NewServerTime(joinNow.Add(-time.Hour), testUncertainty)
	if err != nil {
		t.Fatalf("NewServerTime: %v", err)
	}
	r.records[o.ClientOrderID] = ports.OrderRecord{
		Order: o, State: domain.StateFilled, Version: 7, AuthorizedAt: at,
		Cumulative: assetQty(t, string(o.Quantity.Asset()), cumulative),
		Fees:       fees,
	}
}

func spotDeltaEquals(t *testing.T, got application.AssetDelta, asset, expected, observed, unexplained string) {
	t.Helper()
	if got.Venue != "binance" || string(got.Asset) != asset {
		t.Errorf("delta cell = %s/%s, want binance/%s", got.Venue, got.Asset, asset)
	}
	for _, cmp := range []struct {
		label string
		got   fixedpoint.Quantity
		want  string
	}{
		{"Expected", got.Expected, expected},
		{"Observed", got.Observed, observed},
		{"Unexplained", got.Unexplained, unexplained},
	} {
		c, err := cmp.got.Cmp(assetQty(t, asset, cmp.want))
		if err != nil {
			t.Fatalf("%s: Cmp: %v", cmp.label, err)
		}
		if c != 0 {
			t.Errorf("%s = %s, want %s", cmp.label, cmp.got, cmp.want)
		}
	}
}

// TestAssertSpotExplainsTheWalletThroughTheBaseline is the positive control
// and §70h items 1–3 in one: baseline and ours BOTH nonzero, a base-asset fee
// on a buy, a non-base fee on the same order, another pair's order whose fee
// debits the baselined asset, and a PERP order whose futures-wallet fee is
// charged in the same asset — every fee-scope mutation moves this fixture's
// answer off zero, in one direction each: dropping the fee term, narrowing it
// to own-pair rows, and widening it to derivative orders (whose fees debit the
// futures wallet the spot read never sees) are three different wrong numbers.
//
// ours(BTC) = 0.5 − 0.2 − 0.0005 (own base fee) − 0.0001 (the ETH pair's BTC
// fee) = 0.2994; B(BTC) = 0.5994; baseline = 0.3 → observed 0.2994 → clean.
// The USDT fee and the perp's BTC fee must move nothing.
func TestAssertSpotExplainsTheWalletThroughTheBaseline(t *testing.T) {
	t.Parallel()

	balances := newFakeSpotBalances()
	balances.reports["binance"] = balanceReport(t,
		balanceRow(t, "BTC", "0.5994"), balanceRow(t, "ETH", "1.3"))
	leg := application.SpotLeg{
		Balances: balances,
		Roster: []application.SpotInstrument{
			{Instrument: spot(), Base: "BTC", Quote: "USDT"},
			{Instrument: ethSpot(), Base: "ETH", Quote: "USDT"},
		},
		Baselines: []application.SpotBaseline{
			btcBaseline(t, "0.3"),
			{Venue: "binance", Asset: "ETH", Amount: assetQty(t, "ETH", "0.3")},
		},
	}
	// Built inline rather than through newSpotJoiner: this fixture carries the
	// perp leg, so it needs the scope and the position surface answering for it.
	orders := newFakeOrders()
	positions := newFakePositions()
	clk, err := clock.NewTestClock(joinNow, []clock.Calibration{
		{Venue: perp().Venue, Offset: testVenueOffset, Uncertainty: testUncertainty},
	})
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	j, err := application.NewJoiner(newFakeJoinVenue(), positions, orders, clk, joinAccount, leg, perpScopeFor())
	if err != nil {
		t.Fatalf("NewJoiner: %v", err)
	}

	buy := joinOrder(t, 1, spot(), domain.SideBuy, "0.5")
	sell := joinOrder(t, 2, spot(), domain.SideSell, "0.2")
	ethBuy := domain.Order{
		ClientOrderID: mintedID(t, 3), Instrument: ethSpot(),
		Side: domain.SideBuy, Type: domain.OrderTypeLimit, TimeInForce: domain.TimeInForceIOC,
		Quantity: assetQty(t, "ETH", "1"), LimitPrice: price(t, "100"),
	}
	perpSell := joinOrder(t, 4, perp(), domain.SideSell, "0.5")
	plantAssertable(t, orders, buy, "0.5", assetFee(t, "BTC", "0.0005"), assetFee(t, "USDT", "1.2"))
	plantAssertable(t, orders, sell, "0.2")
	plantAssertable(t, orders, ethBuy, "1", assetFee(t, "BTC", "0.0001"))
	plantAssertable(t, orders, perpSell, "0.5", assetFee(t, "BTC", "0.01"))
	positions.reports[perpScopeFor()] = ports.PositionReport{
		Positions: []ports.VenuePosition{{Instrument: perp(), Net: signed(t, "-0.5")}},
	}

	got, err := j.Assert([]ids.ClientOrderID{
		buy.ClientOrderID, sell.ClientOrderID, ethBuy.ClientOrderID, perpSell.ClientOrderID,
	})
	if err != nil {
		t.Fatalf("Assert: %v", err)
	}
	if len(got.Unexplained) != 0 {
		t.Errorf("Unexplained = %+v, want none — the perp leg is clean by its own surface", got.Unexplained)
	}
	if len(got.SpotUnexplained) != 0 {
		t.Errorf("SpotUnexplained = %+v, want none — the baseline explains the wallet exactly", got.SpotUnexplained)
	}
	if len(got.SpotUnconcluded) != 0 || len(got.Unasserted) != 0 {
		t.Errorf("unconcluded/unasserted = %+v/%+v, want none", got.SpotUnconcluded, got.Unasserted)
	}
	if got.Holds() {
		t.Errorf("assertion = %+v, want clean", got)
	}
}

// TestAssertSpotLatchesBothDeltaSigns is §70h item 4: a surplus the journal
// cannot explain and coins the journal says we hold that the venue does not —
// each sign on its OWN fixture, with the literal numbers asserted so a
// subtraction run backwards cannot flip both sides together.
func TestAssertSpotLatchesBothDeltaSigns(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name                            string
		balance                         string
		expected, observed, unexplained string
	}{
		{"venue above: unexplained surplus", "1", "0.5", "0.7", "0.2"},
		{"venue below: missing coins", "0.6", "0.5", "0.3", "-0.2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			balances := newFakeSpotBalances()
			balances.reports["binance"] = balanceReport(t, balanceRow(t, "BTC", tc.balance))
			j, orders := newSpotJoiner(t, application.SpotLeg{
				Balances:  balances,
				Roster:    spotRoster(),
				Baselines: []application.SpotBaseline{btcBaseline(t, "0.3")},
			})
			buy := joinOrder(t, 1, spot(), domain.SideBuy, "0.5")
			plantAssertable(t, orders, buy, "0.5")

			got, err := j.Assert([]ids.ClientOrderID{buy.ClientOrderID})
			if err != nil {
				t.Fatalf("Assert: %v", err)
			}
			if len(got.SpotUnexplained) != 1 {
				t.Fatalf("SpotUnexplained = %+v, want exactly one BTC delta", got.SpotUnexplained)
			}
			spotDeltaEquals(t, got.SpotUnexplained[0], "BTC", tc.expected, tc.observed, tc.unexplained)
			if !got.Holds() {
				t.Error("Holds() = false — a spot delta is §7.5's operator-clearance finding")
			}
		})
	}
}

// TestAssertSpotAnAbsentAssetIsAZeroBalance is §70h item 6 under the
// CONCLUSIVE enumeration kind (AbsenceIsZero — Bybit's, whose coin-less call
// omits zero coins by the venue's own sentence): the baselined asset is
// missing from a report that holds TWO other rows, and the absence must reach
// the subtraction as a zero balance — observed = 0 − baseline — never as a
// skipped cell. The inconclusive kind is the test below (§70i).
func TestAssertSpotAnAbsentAssetIsAZeroBalance(t *testing.T) {
	t.Parallel()

	balances := newFakeSpotBalances()
	balances.reports["binance"] = balanceReport(t,
		balanceRow(t, "ETH", "1"), balanceRow(t, "USDT", "5"))
	j, _ := newSpotJoiner(t, application.SpotLeg{
		Balances:  balances,
		Roster:    spotRoster(),
		Baselines: []application.SpotBaseline{btcBaseline(t, "0.3")},
	})

	got, err := j.Assert(nil)
	if err != nil {
		t.Fatalf("Assert: %v", err)
	}
	if len(got.SpotUnexplained) != 1 {
		t.Fatalf("SpotUnexplained = %+v, want the baselined asset's missing coins", got.SpotUnexplained)
	}
	spotDeltaEquals(t, got.SpotUnexplained[0], "BTC", "0", "-0.3", "-0.3")
	if !got.Holds() {
		t.Error("Holds() = false — a wallet short of its own baseline is exposure with the sign flipped")
	}
}

// TestAssertSpotAnInconclusiveAbsenceConcludesNothing is §70i: the same
// missing asset under the INCONCLUSIVE kind (Binance's, whose account read
// returns every asset with omitZeroBalances unset, so a missing one is an
// anomaly the documented shape rules out). The assertion must neither read
// zero — a quantity minted from missing evidence — nor latch: the asset is
// reported unconcluded, the same non-latching non-pass an unpinned row earns.
func TestAssertSpotAnInconclusiveAbsenceConcludesNothing(t *testing.T) {
	t.Parallel()

	balances := newFakeSpotBalances()
	report := balanceReport(t, balanceRow(t, "ETH", "1"), balanceRow(t, "USDT", "5"))
	report.Absence = ports.AbsenceInconclusive
	balances.reports["binance"] = report
	j, _ := newSpotJoiner(t, application.SpotLeg{
		Balances:  balances,
		Roster:    spotRoster(),
		Baselines: []application.SpotBaseline{btcBaseline(t, "0.3")},
	})

	got, err := j.Assert(nil)
	if err != nil {
		t.Fatalf("Assert: %v", err)
	}
	if len(got.SpotUnexplained) != 0 {
		t.Errorf("SpotUnexplained = %+v, want none — an inconclusive absence is not a delta", got.SpotUnexplained)
	}
	want := application.UnconcludedBalance{Venue: "binance", Asset: "BTC",
		Reason: ports.UnpinnedAbsenceInconclusive}
	if len(got.SpotUnconcluded) != 1 || got.SpotUnconcluded[0] != want {
		t.Errorf("SpotUnconcluded = %+v, want [%+v]", got.SpotUnconcluded, want)
	}
	if got.Holds() {
		t.Error("Holds() = true — refusing to conclude must not latch an operator incident")
	}
}

// TestAssertSpotRefusesAReportWithNoAbsenceSemantics: the zero value is a
// report that never stated what its silences mean, and it is refused on the
// boot where nothing happens to be absent — not first on the boot where
// something is (§70i).
func TestAssertSpotRefusesAReportWithNoAbsenceSemantics(t *testing.T) {
	t.Parallel()

	balances := newFakeSpotBalances()
	report := balanceReport(t, balanceRow(t, "BTC", "0.3"))
	report.Absence = ports.AbsenceSemanticsUnspecified
	balances.reports["binance"] = report
	j, _ := newSpotJoiner(t, application.SpotLeg{
		Balances:  balances,
		Roster:    spotRoster(),
		Baselines: []application.SpotBaseline{btcBaseline(t, "0.3")},
	})

	if _, err := j.Assert(nil); !errors.Is(err, ports.ErrAbsenceSemanticsInvalid) {
		t.Errorf("Assert = %v, want ErrAbsenceSemanticsInvalid", err)
	}
}

// TestAssertSpotVirginRosterReportsEveryInstrument is §70h item 17: configured
// spot instruments, an EMPTY dispatched set, no baseline. The coverage is
// ROSTER-driven — a journal-derived set would be empty and the boot would
// pass having compared nothing — every instrument reports
// UnassertedBaselineUnrecorded, the posture holds, and the wallet is never
// read, because there is no baseline to compare it against.
func TestAssertSpotVirginRosterReportsEveryInstrument(t *testing.T) {
	t.Parallel()

	balances := newFakeSpotBalances()
	j, _ := newSpotJoiner(t, application.SpotLeg{
		Balances: balances,
		Roster: []application.SpotInstrument{
			{Instrument: spot(), Base: "BTC", Quote: "USDT"},
			{Instrument: ethSpot(), Base: "ETH", Quote: "USDT"},
		},
	})

	got, err := j.Assert(nil)
	if err != nil {
		t.Fatalf("Assert: %v", err)
	}
	want := []application.UnassertedInstrument{
		{Instrument: spot(), Reason: application.UnassertedBaselineUnrecorded},
		{Instrument: ethSpot(), Reason: application.UnassertedBaselineUnrecorded},
	}
	if len(got.Unasserted) != len(want) {
		t.Fatalf("Unasserted = %+v, want %+v", got.Unasserted, want)
	}
	for i := range want {
		if got.Unasserted[i] != want[i] {
			t.Errorf("Unasserted[%d] = %+v, want %+v", i, got.Unasserted[i], want[i])
		}
	}
	if !got.Holds() {
		t.Error("Holds() = false — an unrecorded baseline is a wallet the invariant never compared")
	}
	if balances.calls != 0 {
		t.Errorf("balance surface saw %d calls with no baseline recorded, want 0", balances.calls)
	}
}

// TestAssertSpotTwoPairsShareTheAssetCell is §70h item 9: BTCUSDT and BTCUSDC
// share one BTC balance, so the cell sums BOTH pairs' cumulatives and the
// venue is read ONCE. The two cumulatives differ (0.5 and 0.2), so a fold
// that dropped either pair reports 0.3 or 0.6 instead of the 0.1 asserted.
func TestAssertSpotTwoPairsShareTheAssetCell(t *testing.T) {
	t.Parallel()

	balances := newFakeSpotBalances()
	balances.reports["binance"] = balanceReport(t, balanceRow(t, "BTC", "1.8"))
	j, orders := newSpotJoiner(t, application.SpotLeg{
		Balances: balances,
		Roster: []application.SpotInstrument{
			{Instrument: spot(), Base: "BTC", Quote: "USDT"},
			{Instrument: spotUSDC(), Base: "BTC", Quote: "USDC"},
		},
		Baselines: []application.SpotBaseline{btcBaseline(t, "1")},
	})
	usdtBuy := joinOrder(t, 1, spot(), domain.SideBuy, "0.5")
	usdcBuy := joinOrder(t, 2, spotUSDC(), domain.SideBuy, "0.2")
	plantAssertable(t, orders, usdtBuy, "0.5")
	plantAssertable(t, orders, usdcBuy, "0.2")

	got, err := j.Assert([]ids.ClientOrderID{usdtBuy.ClientOrderID, usdcBuy.ClientOrderID})
	if err != nil {
		t.Fatalf("Assert: %v", err)
	}
	if len(got.SpotUnexplained) != 1 {
		t.Fatalf("SpotUnexplained = %+v, want ONE cell for the shared base", got.SpotUnexplained)
	}
	spotDeltaEquals(t, got.SpotUnexplained[0], "BTC", "0.7", "0.8", "0.1")
	if balances.calls != 1 {
		t.Errorf("balance surface saw %d calls, want exactly 1 — one venue, one read", balances.calls)
	}
}

// TestAssertSpotUnpinnedRowConcludesNothing is §70h item 5 at the port: an
// unpinned baselined asset is SpotUnconcluded — reason carried, no delta
// computed, and NOT latching (Holds stays false) — while a second baselined
// asset on the same read still concludes, so "unconcluded" and "skipped the
// whole wallet" are different answers.
func TestAssertSpotUnpinnedRowConcludesNothing(t *testing.T) {
	t.Parallel()

	balances := newFakeSpotBalances()
	report := balanceReport(t, balanceRow(t, "ETH", "1.3"))
	report.Unpinned = []ports.UnpinnedBalance{{Asset: "BTC", Reason: ports.UnpinnedBorrowOutstanding}}
	balances.reports["binance"] = report
	j, _ := newSpotJoiner(t, application.SpotLeg{
		Balances: balances,
		Roster: []application.SpotInstrument{
			{Instrument: spot(), Base: "BTC", Quote: "USDT"},
			{Instrument: ethSpot(), Base: "ETH", Quote: "USDT"},
		},
		Baselines: []application.SpotBaseline{
			btcBaseline(t, "0.3"),
			{Venue: "binance", Asset: "ETH", Amount: assetQty(t, "ETH", "1.3")},
		},
	})

	got, err := j.Assert(nil)
	if err != nil {
		t.Fatalf("Assert: %v", err)
	}
	want := []application.UnconcludedBalance{{Venue: "binance", Asset: "BTC", Reason: ports.UnpinnedBorrowOutstanding}}
	if len(got.SpotUnconcluded) != 1 || got.SpotUnconcluded[0] != want[0] {
		t.Fatalf("SpotUnconcluded = %+v, want %+v", got.SpotUnconcluded, want)
	}
	if len(got.SpotUnexplained) != 0 {
		t.Errorf("SpotUnexplained = %+v — an unpinned row must not become a delta, and the ETH cell is clean", got.SpotUnexplained)
	}
	if got.Holds() {
		t.Error("Holds() = true — unconcluded is re-reportable work, not an operator latch")
	}
}

// TestAssertSpotRefusesADuplicateBalanceRow: two rows for one asset would make
// the cell's total a declaration-order coin flip, so the adapter-contract
// violation is an ERROR here rather than a wrong number downstream.
func TestAssertSpotRefusesADuplicateBalanceRow(t *testing.T) {
	t.Parallel()

	balances := newFakeSpotBalances()
	balances.reports["binance"] = balanceReport(t,
		balanceRow(t, "BTC", "1"), balanceRow(t, "BTC", "0.5"))
	j, _ := newSpotJoiner(t, application.SpotLeg{
		Balances:  balances,
		Roster:    spotRoster(),
		Baselines: []application.SpotBaseline{btcBaseline(t, "0.3")},
	})

	if _, err := j.Assert(nil); !errors.Is(err, application.ErrBalanceRowDuplicate) {
		t.Fatalf("Assert = %v, want ErrBalanceRowDuplicate", err)
	}
}

// TestNewJoinerSpotLegRefusals walks validateSpotLeg's refusals, §70h item 24
// among them: a baselined asset that is another pair's QUOTE is refused at
// construction — `ours` counts base flows and same-asset fees, so a pair
// spending the baselined asset as consideration would move the wallet where
// the formula cannot see — while the SAME roster without the baseline
// constructs, because the deny is about the baseline, not the pair.
func TestNewJoinerSpotLegRefusals(t *testing.T) {
	t.Parallel()

	ethBTC := ids.VenueInstrument{Venue: "binance", Product: ids.ProductSpot, Symbol: "ETHBTC"}
	overlap := []application.SpotInstrument{
		{Instrument: spot(), Base: "BTC", Quote: "USDT"},
		{Instrument: ethBTC, Base: "ETH", Quote: "BTC"},
	}

	cases := []struct {
		name string
		leg  application.SpotLeg
		want error
	}{
		{
			"a baselined asset that is a pair's quote",
			application.SpotLeg{Balances: newFakeSpotBalances(), Roster: overlap,
				Baselines: []application.SpotBaseline{btcBaseline(t, "0.3")}},
			application.ErrSpotBaselineIsQuote,
		},
		{
			"a baseline no roster base consumes",
			application.SpotLeg{Balances: newFakeSpotBalances(), Roster: spotRoster(),
				Baselines: []application.SpotBaseline{{Venue: "binance", Asset: "XRP", Amount: assetQty(t, "XRP", "1")}}},
			application.ErrSpotBaselineUnconsumed,
		},
		{
			"baselines with no balance surface",
			application.SpotLeg{Roster: spotRoster(),
				Baselines: []application.SpotBaseline{btcBaseline(t, "0.3")}},
			application.ErrSpotBalancesMissing,
		},
		{
			"a duplicate roster instrument",
			application.SpotLeg{Roster: append(spotRoster(), spotRoster()...)},
			application.ErrSpotRosterDuplicate,
		},
		{
			"a duplicate baseline cell",
			application.SpotLeg{Balances: newFakeSpotBalances(), Roster: spotRoster(),
				Baselines: []application.SpotBaseline{btcBaseline(t, "0.3"), btcBaseline(t, "0.4")}},
			application.ErrSpotBaselineDuplicate,
		},
		{
			"a derivative in the spot roster",
			application.SpotLeg{Roster: []application.SpotInstrument{{Instrument: perp(), Base: "BTC", Quote: "USDT"}}},
			application.ErrSpotRosterNotSpot,
		},
		{
			"a baseline amount tagged with another asset",
			application.SpotLeg{Balances: newFakeSpotBalances(), Roster: spotRoster(),
				Baselines: []application.SpotBaseline{{Venue: "binance", Asset: "BTC", Amount: assetQty(t, "ETH", "1")}}},
			application.ErrSpotBaselineAssetMismatch,
		},
	}
	clk, err := clock.NewTestClock(joinNow, []clock.Calibration{
		{Venue: perp().Venue, Offset: testVenueOffset, Uncertainty: testUncertainty},
	})
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := application.NewJoiner(newFakeJoinVenue(), newFakePositions(), newFakeOrders(), clk, joinAccount, tc.leg)
			if !errors.Is(err, tc.want) {
				t.Errorf("NewJoiner = %v, want %v", err, tc.want)
			}
		})
	}

	// The positive control for item 24's sharper half: the SAME overlapping
	// roster with no baseline constructs — the deny is the baseline's.
	if _, err := application.NewJoiner(newFakeJoinVenue(), newFakePositions(), newFakeOrders(), clk, joinAccount,
		application.SpotLeg{Roster: overlap}); err != nil {
		t.Errorf("NewJoiner(overlapping roster, no baseline) = %v, want nil", err)
	}
}
