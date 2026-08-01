package crossvenue

// Sim assembly battery: one fully hand-computed episode end to end, the
// named-refusal path, the seal exclusion, and the label rule.
//
// The worked scenario (all instants on 2026-07-30, grid 5s, slack 60s):
// the decision signal (binance markPrice minus bybit ticker estimate)
// prints 0.0001, 0.0005, 0.0005, 0.0001, 0.0001 at :05..:25 — entry
// 0.0004 opens at :10, exit 0.0002 closes at :20 (2 counted points).
//
// The cost fixture is built so every selection in the composition has a
// real choice to make, with the two venues DISAGREEING about the worse
// side (§45g), and every distribution's tail long enough that the p90
// sits BELOW the boundary walks on the walked-pinned sides — at four
// samples the upper statistic IS the maximum and the walked arm can
// never win, which is the fixture collapse the checklist exists to find:
//
//   binance: book A at the open boundary walks 0.05 bid / 0.005 ask;
//   book B from :18 on walks 0.02 / 0.015 across a 38-sample tail, so
//   p90 = 0.02 bid / 0.015 ask. Bid cell 0.05 comes from the BOUNDARY
//   WALK (worse-of boundaries beats p90); venue cell = worse side 0.05.
//
//   bybit: book P (boundaries) walks 0.04 bid / 0.015 ask; book Q (the
//   38-sample tail) walks 0.01 / 0.06, so p90 = 0.01 bid / 0.06 ask.
//   Bid cell 0.04 (walked); ask cell 0.06 comes from the P90 ARM; venue
//   cell = worse side 0.06 — the ASK side, where binance's was the bid.
//
// Round trip: 2×(0.0004+0.05) + 2×(0.00055+0.06) = 0.2219. Settled
// prints inside (:10,:20], both venues at :15 (hedged — the phase term
// is zero by the atomic same-instant rule): net = 0.001 − 0.0002 =
// 0.0008. Episode return 0.0008 − 0.2219 = −0.2211, attributed to the
// 10-second period starting at :20 — one grid step past the OPEN, so a
// close-vs-open attribution swap is visible in the period key.

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	dcv "github.com/giiorg/arb-lab/internal/strategy/domain/crossvenue"
)

func simFixtureRoot(t *testing.T, includeCloseBooks bool) (string, string) {
	return simFixtureRootOpt(t, includeCloseBooks, true)
}

// simFixtureRootOpt adds the balanced/imbalanced settlement choice: with
// balanced=false the bybit in-window print is dropped, so the window ends
// one settlement out of step — the §45n-3 void's subject.
func simFixtureRootOpt(t *testing.T, includeCloseBooks, balanced bool) (string, string) {
	t.Helper()
	root := t.TempDir()
	rest := t.TempDir()
	rel := writeSimStreams(t, root, "ABCUSDT", includeCloseBooks)
	writeManifest(t, root, sampleDay, rel...)
	// A post-cutoff manifest day: the enumeration bound must leave it
	// unread, which the report's Days list makes visible.
	writeManifest(t, root, "2026-08-02")
	writeSimSettled(t, rest, "ABCUSDT", balanced)
	return root, rest
}

// simWorldFor plants a SECOND symbol's identical world into an existing
// fixture — same instants, same rates, same books — so a multi-symbol
// walk varies nothing but the roster entry each spec declares.
func simWorldFor(t *testing.T, root, rest, sym string) {
	t.Helper()
	rel := writeSimStreams(t, root, sym, true)
	appendManifest(t, root, sampleDay, rel...)
	writeSimSettled(t, rest, sym, true)
}

// writeSimStreams writes one symbol's four streams and returns their
// root-relative paths. The scenario is the file header's; the symbol is
// the only thing that varies.
func writeSimStreams(t *testing.T, root, sym string, includeCloseBooks bool) []string {
	t.Helper()
	rel := []string{}
	add := func(r string) string { rel = append(rel, r); return r }

	// Binance markPrice: sampled 0.0002 at :05, 0.0006 at :10/:15,
	// 0.0002 at :20/:25.
	bnF := []fixRec{}
	for _, f := range []struct {
		off  int
		rate string
	}{{4, "0.0002"}, {9, "0.0006"}, {19, "0.0002"}, {30, "0.0002"}} {
		at := sampleBase.Add(time.Duration(f.off) * time.Second)
		bnF = append(bnF, dataAt(at, markFrameSym(sym, at, f.rate)))
	}
	writeFixture(t, filepath_Join(root, add(bnFundingRelSym(sym, "2026-07-30T10.arbcap"))), false, bnF...)

	// Bybit ticker: snapshot 0.0001 at :03, churn at :30 to advance the
	// grid — every instant reads the held 0.0001 within the wide slack.
	snapAt := sampleBase.Add(3 * time.Second)
	churnAt := sampleBase.Add(30 * time.Second)
	writeFixture(t, filepath_Join(root, add(byFundingRelSym(sym, "2026-07-30T10.arbcap"))), false,
		dataAt(snapAt, tickerSnapshotSym(sym, snapAt, "0.0001")),
		dataAt(churnAt, tickerChurnSym(sym, churnAt)),
	)

	// Binance books: A at the open boundary, then a 38-sample B tail;
	// dropping the tail starves the close boundary for the refusal case.
	aB, aA := bookAFixture()
	bB, bA := bookBFixture()
	f1 := sampleBase.Add(8 * time.Second)
	bnB := []fixRec{dataAt(f1, bnDepthFrameSym(sym, f1, 100, aB, aA))}
	if includeCloseBooks {
		for k := 0; k <= 38; k++ {
			at := sampleBase.Add(time.Duration(18+5*k) * time.Second)
			bnB = append(bnB, dataAt(at, bnDepthFrameSym(sym, at, 110+uint64(k), bB, bA)))
		}
	}
	writeFixture(t, filepath_Join(root, add(bnBookRelSym(sym, "2026-07-30T10.arbcap"))), false, bnB...)

	// Bybit books: P anchors the boundaries (bid walk 0.04, ask 0.015);
	// Q re-anchors at :22 and its heartbeat tail (bid 0.01, ask 0.06)
	// dominates the distribution.
	pBids := [][2]string{{"9.90", "1"}, {"9.108", "1"}}
	pAsks := [][2]string{{"10.00", "1"}, {"10.30", "1"}}
	qBids := [][2]string{{"9.90", "1"}, {"9.702", "1"}}
	qAsks := [][2]string{{"10.00", "1"}, {"11.20", "1"}}
	s1 := sampleBase.Add(7 * time.Second)
	d1 := sampleBase.Add(17 * time.Second)
	s2 := sampleBase.Add(22 * time.Second)
	byB := []fixRec{
		dataAt(s1, byBookSnapFrameSym(sym, s1, 100, pBids, pAsks)),
		dataAt(d1, byBookDeltaFrameSym(sym, d1, 101, nil, nil)),
		dataAt(s2, byBookSnapFrameSym(sym, s2, 200, qBids, qAsks)),
	}
	for k := 0; k <= 37; k++ {
		at := sampleBase.Add(time.Duration(27+5*k) * time.Second)
		byB = append(byB, dataAt(at, byBookDeltaFrameSym(sym, at, 201+uint64(k), nil, nil)))
	}
	writeFixture(t, filepath_Join(root, add(byBookRelSym(sym, "2026-07-30T10.arbcap"))), false, byB...)
	return rel
}

// writeSimSettled stores one symbol's settled prints: both venues at :15
// (inside the window, hedged), a flanking pre-window row each, and one
// POST-CUTOFF Binance row the seal must exclude and count.
func writeSimSettled(t *testing.T, rest, sym string, balanced bool) {
	t.Helper()
	pre := sampleBase.Add(-4 * time.Hour)
	in := sampleBase.Add(15 * time.Second)
	sealed := SettledCutoff.Add(2 * time.Hour)
	storeRaw(t, rest, "binance", sym, "2026-08-05",
		[2]string{ms(pre), "0.0005"}, [2]string{ms(in), "0.001"}, [2]string{ms(sealed), "0.009"})
	byRows := [][2]string{{ms(pre), "0.0001"}, {ms(in), "0.0002"}}
	if !balanced {
		byRows = byRows[:1] // no in-window bybit print: the legs end out of step
	}
	storeRaw(t, rest, "bybit", sym, "2026-08-05", byRows...)
}

func simParamsFixture(t *testing.T) SimParams {
	t.Helper()
	r := func(s string) fixedpoint.Rate {
		rate, err := fixedpoint.ParseRate(s)
		if err != nil {
			t.Fatal(err)
		}
		return rate
	}
	return SimParams{
		Symbols:     []SymbolSpec{{Symbol: "ABCUSDT", Qty: abcQty(t, "2"), Entry: r("0.0004")}},
		Sample:      SampleParams{Every: 5 * time.Second, Slack: 60 * time.Second},
		ExitDivisor: 2,
		MaxGap:      10 * time.Minute,
		Round:       time.Minute,
		BinanceFee:  r("0.0004"),
		BybitFee:    r("0.00055"),
		P90:         r("0.9"),
		Period:      10 * time.Second,
		Policy:      []dcv.Allocation{{Symbol: "ABCUSDT", Weight: r("1")}},
		MeanBlock:   1,
		Resamples:   200,
		Confidence:  r("0.9"),
		Seed:        7,
		Label:       "sim-battery",
	}
}

func TestRunSimWorkedEpisode(t *testing.T) {
	root, rest := simFixtureRoot(t, true)
	rep, err := RunSim(root, rest, simParamsFixture(t))
	if err != nil {
		t.Fatalf("sim: %v", err)
	}
	if len(rep.Days) != 1 || rep.Days[0] != "2026-07-30" {
		t.Fatalf("days %v", rep.Days)
	}
	if len(rep.Symbols) != 1 {
		t.Fatalf("symbols %d", len(rep.Symbols))
	}
	s := rep.Symbols[0]
	if s.Merge.Joined != 5 {
		t.Fatalf("joined %d, want 5", s.Merge.Joined)
	}
	if s.Scored != 1 || s.RefusedN != 0 || len(s.Episodes) != 1 {
		t.Fatalf("scored %d refused %d episodes %d", s.Scored, s.RefusedN, len(s.Episodes))
	}
	ep := s.Episodes[0]
	if !ep.OpenAt.Equal(sampleBase.Add(10*time.Second)) || !ep.CloseAt.Equal(sampleBase.Add(20*time.Second)) {
		t.Fatalf("window %s..%s", ep.OpenAt, ep.CloseAt)
	}
	if ep.Sign != 1 || ep.Points != 2 || ep.EndImbalance != 0 {
		t.Fatalf("episode %+v", ep)
	}
	if ep.Net != "0.0008" || ep.Cost != "0.2219" || ep.Return != "-0.2211" {
		t.Fatalf("economics net %s cost %s return %s", ep.Net, ep.Cost, ep.Return)
	}
	if s.BnP90Bid != "0.02" || s.BnP90Ask != "0.015" || s.ByP90Bid != "0.01" || s.ByP90Ask != "0.06" {
		t.Fatalf("p90s bn %s/%s by %s/%s", s.BnP90Bid, s.BnP90Ask, s.ByP90Bid, s.ByP90Ask)
	}
	if s.SealedExcluded != 1 {
		t.Fatalf("sealed excluded %d, want 1", s.SealedExcluded)
	}
	if s.Pairing.Matched != 2 {
		t.Fatalf("matched %d, want 2 (the sealed row pairs with nothing)", s.Pairing.Matched)
	}
	if rep.Periods != 8640 {
		t.Fatalf("periods %d, want 8640 (24h at 10s)", rep.Periods)
	}
	// The return lands in the period STARTING at the close instant :20 —
	// one grid step past the open, so a close-vs-open attribution swap
	// changes this key.
	if len(s.NonZeroReturns) != 1 || s.NonZeroReturns["2026-07-30T10:00:20Z"] != "-0.2211" {
		t.Fatalf("non-zero returns %+v, want {2026-07-30T10:00:20Z: -0.2211}", s.NonZeroReturns)
	}
	if rep.PortfolioLCB == "" {
		t.Fatal("no LCB")
	}
}

// Starving the close boundary of Binance books refuses the episode BY
// NAME: nothing scored, the refusal counted and worded, the series zero.
func TestRunSimRefusesUnmeasuredBoundary(t *testing.T) {
	root, rest := simFixtureRoot(t, false)
	rep, err := RunSim(root, rest, simParamsFixture(t))
	if err != nil {
		t.Fatalf("sim: %v", err)
	}
	s := rep.Symbols[0]
	if s.Scored != 0 || s.RefusedN != 1 || len(s.Episodes) != 1 {
		t.Fatalf("scored %d refused %d episodes %d", s.Scored, s.RefusedN, len(s.Episodes))
	}
	if s.Episodes[0].Refused == "" {
		t.Fatal("refusal carries no reason")
	}
	if s.Episodes[0].Return != "" {
		t.Fatalf("refused episode carries a return %q", s.Episodes[0].Return)
	}
}

// Ruling §45n-3, both directions from ONE fixture — which is what makes
// the flag's polarity checkable: the same imbalanced world scores under
// report-only and refuses by name under the void rule. Asserting the
// report-only arm also proves the fixture IS imbalanced; a balanced one
// would pass the void assertion vacuously.
// Ruling §45n-4's claim is per-SYMBOL, so it is only checkable with more
// than one symbol in the walk: a single-symbol fixture makes "this spec's
// entry" and "the roster's first entry" the same value, and the mutation
// aimed at that distinction SURVIVED against exactly such a fixture until
// this test existed. The second symbol's world is byte-identical in
// signal to the first — only its declared entry level differs, and only
// the per-spec read explains it opening no episode.
func TestRunSimEntryIsPerSymbol(t *testing.T) {
	root, rest := simFixtureRoot(t, true)
	simWorldFor(t, root, rest, "DEFUSDT")

	p := simParamsFixture(t)
	r := func(s string) fixedpoint.Rate {
		rate, err := fixedpoint.ParseRate(s)
		if err != nil {
			t.Fatal(err)
		}
		return rate
	}
	// The same world twice; ABCUSDT's level admits the 0.0005 signal,
	// DEFUSDT's sits above it.
	p.Symbols = append(p.Symbols, SymbolSpec{
		Symbol: "DEFUSDT", Qty: defQty(t, "2"), Entry: r("0.0009")})
	p.Policy = []dcv.Allocation{
		{Symbol: "ABCUSDT", Weight: r("0.5")}, {Symbol: "DEFUSDT", Weight: r("0.5")}}

	rep, err := RunSim(root, rest, p)
	if err != nil {
		t.Fatalf("sim: %v", err)
	}
	if len(rep.Symbols) != 2 {
		t.Fatalf("symbols %d, want 2", len(rep.Symbols))
	}
	first, second := rep.Symbols[0], rep.Symbols[1]
	if first.Symbol != "ABCUSDT" || second.Symbol != "DEFUSDT" {
		t.Fatalf("roster order %s, %s", first.Symbol, second.Symbol)
	}
	// The shared world is what makes the comparison a control: both
	// symbols see the same signal, so a difference in episodes can only
	// come from the level each spec declares.
	if second.Merge.Joined != first.Merge.Joined {
		t.Fatalf("joined %d vs %d — the two worlds differ, so entry is not the only variable",
			first.Merge.Joined, second.Merge.Joined)
	}
	if len(first.Episodes) != 1 {
		t.Fatalf("first symbol episodes %d, want 1", len(first.Episodes))
	}
	if len(second.Episodes) != 0 {
		t.Fatalf("second symbol episodes %+v, want none at its own higher entry", second.Episodes)
	}
}

func TestRunSimVoidImbalanceBothWays(t *testing.T) {
	root, rest := simFixtureRootOpt(t, true, false)

	reportOnly := simParamsFixture(t)
	rep, err := RunSim(root, rest, reportOnly)
	if err != nil {
		t.Fatalf("sim: %v", err)
	}
	s := rep.Symbols[0]
	if s.Scored != 1 || s.RefusedN != 0 {
		t.Fatalf("report-only: scored %d refused %d, want 1 and 0", s.Scored, s.RefusedN)
	}
	if s.Episodes[0].EndImbalance == 0 {
		t.Fatal("fixture is balanced — the void rule would pass vacuously")
	}

	void := simParamsFixture(t)
	void.VoidImbalance = true
	rep, err = RunSim(root, rest, void)
	if err != nil {
		t.Fatalf("sim: %v", err)
	}
	s = rep.Symbols[0]
	if s.Scored != 0 || s.RefusedN != 1 {
		t.Fatalf("void: scored %d refused %d, want 0 and 1", s.Scored, s.RefusedN)
	}
	if !strings.Contains(s.Episodes[0].Refused, "imbalance") {
		t.Fatalf("refusal %q does not name the imbalance", s.Episodes[0].Refused)
	}
	if s.Episodes[0].Return != "" || len(s.NonZeroReturns) != 0 {
		t.Fatalf("voided episode still contributed: return %q series %+v",
			s.Episodes[0].Return, s.NonZeroReturns)
	}
}

// The void rule leaves a BALANCED episode alone — the guard reads the
// imbalance, not the flag.
func TestRunSimVoidImbalanceKeepsBalanced(t *testing.T) {
	root, rest := simFixtureRoot(t, true)
	p := simParamsFixture(t)
	p.VoidImbalance = true
	rep, err := RunSim(root, rest, p)
	if err != nil {
		t.Fatalf("sim: %v", err)
	}
	if s := rep.Symbols[0]; s.Scored != 1 || s.RefusedN != 0 {
		t.Fatalf("scored %d refused %d, want the balanced episode scored", s.Scored, s.RefusedN)
	}
}

// Entry is per symbol (§45n-4): raising THIS symbol's level above the
// signal's peak opens no episode, which a shared-level reading could not
// express per symbol.
func TestRunSimPerSymbolEntry(t *testing.T) {
	root, rest := simFixtureRoot(t, true)
	p := simParamsFixture(t)
	r, err := fixedpoint.ParseRate("0.0009") // the fixture's peak signal is 0.0005
	if err != nil {
		t.Fatal(err)
	}
	p.Symbols[0].Entry = r
	rep, err := RunSim(root, rest, p)
	if err != nil {
		t.Fatalf("sim: %v", err)
	}
	if s := rep.Symbols[0]; len(s.Episodes) != 0 {
		t.Fatalf("episodes %+v, want none above the raised entry", s.Episodes)
	}
}

func TestRunSimLabelRequired(t *testing.T) {
	p := simParamsFixture(t)
	p.Label = "  "
	if _, err := RunSim("", "", p); !errors.Is(err, ErrLabelUnset) {
		t.Fatalf("err %v, want ErrLabelUnset", err)
	}
}
