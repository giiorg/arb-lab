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
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	dcv "github.com/giiorg/arb-lab/internal/strategy/domain/crossvenue"
)

func simFixtureRoot(t *testing.T, includeCloseBooks bool) (string, string) {
	t.Helper()
	root := t.TempDir()
	rest := t.TempDir()

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
		bnF = append(bnF, dataAt(at, markFrame(at, f.rate)))
	}
	writeFixture(t, filepath_Join(root, add(bnFundingRel("2026-07-30T10.arbcap"))), false, bnF...)

	// Bybit ticker: snapshot 0.0001 at :03, churn at :30 to advance the
	// grid — every instant reads the held 0.0001 within the wide slack.
	snapAt := sampleBase.Add(3 * time.Second)
	churnAt := sampleBase.Add(30 * time.Second)
	writeFixture(t, filepath_Join(root, add(byFundingRel("2026-07-30T10.arbcap"))), false,
		dataAt(snapAt, tickerSnapshot(snapAt, "0.0001")),
		dataAt(churnAt, tickerChurn(churnAt)),
	)

	// Binance books: A at the open boundary, then a 38-sample B tail;
	// dropping the tail starves the close boundary for the refusal case.
	aB, aA := bookAFixture()
	bB, bA := bookBFixture()
	f1 := sampleBase.Add(8 * time.Second)
	bnB := []fixRec{dataAt(f1, bnDepthFrame(f1, 100, aB, aA))}
	if includeCloseBooks {
		for k := 0; k <= 38; k++ {
			at := sampleBase.Add(time.Duration(18+5*k) * time.Second)
			bnB = append(bnB, dataAt(at, bnDepthFrame(at, 110+uint64(k), bB, bA)))
		}
	}
	writeFixture(t, filepath_Join(root, add(bnBookRel("2026-07-30T10.arbcap"))), false, bnB...)

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
		dataAt(s1, byBookSnapFrame(s1, 100, pBids, pAsks)),
		dataAt(d1, byBookDeltaFrame(d1, 101, nil, nil)),
		dataAt(s2, byBookSnapFrame(s2, 200, qBids, qAsks)),
	}
	for k := 0; k <= 37; k++ {
		at := sampleBase.Add(time.Duration(27+5*k) * time.Second)
		byB = append(byB, dataAt(at, byBookDeltaFrame(at, 201+uint64(k), nil, nil)))
	}
	writeFixture(t, filepath_Join(root, add(byBookRel("2026-07-30T10.arbcap"))), false, byB...)

	writeManifest(t, root, sampleDay, rel...)
	// A post-cutoff manifest day: the enumeration bound must leave it
	// unread, which the report's Days list makes visible.
	writeManifest(t, root, "2026-08-02")

	// Settled prints: both venues at :15 (inside the window, hedged),
	// plus a flanking pre-window row each, plus one POST-CUTOFF Binance
	// row the seal must exclude and count.
	pre := sampleBase.Add(-4 * time.Hour)
	in := sampleBase.Add(15 * time.Second)
	sealed := SettledCutoff.Add(2 * time.Hour)
	storeRaw(t, rest, "binance", "ABCUSDT", "2026-08-05",
		[2]string{ms(pre), "0.0005"}, [2]string{ms(in), "0.001"}, [2]string{ms(sealed), "0.009"})
	storeRaw(t, rest, "bybit", "ABCUSDT", "2026-08-05",
		[2]string{ms(pre), "0.0001"}, [2]string{ms(in), "0.0002"})
	return root, rest
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
		Symbols:     []SymbolSpec{{Symbol: "ABCUSDT", Qty: abcQty(t, "2")}},
		Sample:      SampleParams{Every: 5 * time.Second, Slack: 60 * time.Second},
		Entry:       r("0.0004"),
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

func TestRunSimLabelRequired(t *testing.T) {
	p := simParamsFixture(t)
	p.Label = "  "
	if _, err := RunSim("", "", p); !errors.Is(err, ErrLabelUnset) {
		t.Fatalf("err %v, want ErrLabelUnset", err)
	}
}
