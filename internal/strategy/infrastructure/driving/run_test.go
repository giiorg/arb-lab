package driving

// Run's battery: a synthetic world small enough to reason about, real
// enough to drive every stage — the values themselves are the unit
// oracles' business; what THIS battery pins is orchestration and
// governance: the §41e binding refusals with their positive control
// (§41i's named suspect), write-once, the two-invocation determinism
// claim, the H2 enumeration, the sealed promotion identity, and the
// emitted document loading through the production door.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/capture"
	mdomain "github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/platform/events"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
	"github.com/giiorg/arb-lab/internal/strategy/infrastructure/driven"
)

// runWorld builds the synthetic world: committed datasets (400-settlement
// funding grid with recovered dips, 48-hour kline pair), one manifested
// archive day carrying both book streams and a settlement-spanning
// markPrice window, and the real committed reference revision.
func runWorld(t *testing.T) (datasetsDir, archiveRoot, refPath string) {
	t.Helper()
	root := t.TempDir()
	datasetsDir = filepath.Join(root, "datasets")
	archiveRoot = filepath.Join(root, "archive")

	// --- Family 1: the funding identity. 2023-01-01 + 400×8h. Dips to
	// -0.001 every 50th settlement recover next instant (run length 1 <
	// dwell 2): the band's recovered-excursion evidence.
	start := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	var fundingRows []string
	for i := 0; i < 400; i++ {
		// 0.0015/settlement sits ABOVE the frame's ~0.0011 clearing
		// level (24h min hold amortizes the full round trip), so the
		// era enters; the dips fall below it and recover next instant.
		rate := "0.00150000"
		if i%50 == 25 {
			rate = "-0.00100000"
		}
		at := start.Add(time.Duration(i) * 8 * time.Hour)
		fundingRows = append(fundingRows,
			fmt.Sprintf(`{"symbol":"BTCUSDT","fundingTime":%d,"fundingRate":"%s"}`, at.UnixMilli(), rate))
	}
	writeRunDataset(t, datasetsDir, "binance", "funding.TEST.json", map[string]any{
		"venue": "binance", "series": "funding", "symbol": "BTCUSDT",
		"start": "2023-01-01T00:00:00Z", "end": "2023-05-14T00:00:00Z", "interval": "8h0m0s",
		"fetched_at": "2026-08-01T00:00:00Z", "endpoint": "test",
		"pages": []json.RawMessage{json.RawMessage("[" + joinRows(fundingRows) + "]")},
	})

	// --- The kline pair: 48 hourly rows, constant basis (mark 10 over
	// spot) so the adverse move's tail is the declared zero.
	klinePage := func(open string) json.RawMessage {
		var rows []string
		for i := 0; i < 48; i++ {
			at := start.Add(time.Duration(i) * time.Hour)
			rows = append(rows, fmt.Sprintf(`[%d,"%s"]`, at.UnixMilli(), open))
		}
		return json.RawMessage("[" + joinRows(rows) + "]")
	}
	writeRunDataset(t, datasetsDir, "binance", "spot_kline_1h.TEST.json", map[string]any{
		"venue": "binance", "series": "spot_kline_1h", "symbol": "BTCUSDT",
		"start": "2023-01-01T00:00:00Z", "end": "2023-01-02T23:00:00Z", "interval": "1h0m0s",
		"fetched_at": "2026-08-01T00:00:00Z", "endpoint": "test",
		"pages": []json.RawMessage{klinePage("30000.00")},
	})
	writeRunDataset(t, datasetsDir, "binance", "mark_kline_1h.TEST.json", map[string]any{
		"venue": "binance", "series": "mark_kline_1h", "symbol": "BTCUSDT",
		"start": "2023-01-01T00:00:00Z", "end": "2023-01-02T23:00:00Z", "interval": "1h0m0s",
		"fetched_at": "2026-08-01T00:00:00Z", "endpoint": "test",
		"pages": []json.RawMessage{klinePage("30010.00")},
	})

	// --- Family 2: day 2026-07-30, one record pair per 10s sample window
	// (grid−100ms and grid+300ms). The §41i identical-leg suspect is
	// answered by construction: the two LEGS carry different ladders, the
	// perp's completion book VANISHES its best ask (so perp-ask slippage
	// is positive while spot's is zero and perp-bid's is zero — leg AND
	// side keying both visible in values), and each stream ends with a
	// thin-book tail in the "normal" stratum: 500 spot instants (exactly
	// MinEvidence — kept under <, dropped under <=) and 20 perp instants
	// (dropped, the sub-minimum arm).
	var rels []string
	day := time.Date(2026, 7, 30, 10, 0, 0, 0, time.UTC)
	type ladder struct{ bids, asks, thinBids, thinAsks, shiftedAsks string }
	mkBookRecs := func(l ladder, thin int, frame func(uint64, string, string) string) map[time.Time][]capture.Record {
		files := map[time.Time][]capture.Record{}
		for i := 0; i < 505+thin; i++ {
			grid := day.Add(time.Duration(i) * 10 * time.Second)
			for n, off := range []time.Duration{-100 * time.Millisecond, 300 * time.Millisecond} {
				at := grid.Add(off)
				window := at.Truncate(time.Hour)
				seq := uint64(i*2 + n + 1)
				bids, asks := l.bids, l.asks
				if i >= 505 {
					bids, asks = l.thinBids, l.thinAsks
				} else if n == 1 && l.shiftedAsks != "" {
					asks = l.shiftedAsks
				}
				files[window] = append(files[window], dataRec(seq, at, frame(seq, bids, asks)))
			}
		}
		return files
	}
	spotLadder := ladder{
		bids:     `[["29999.90","1.5"],["29999.80","1.0"],["29999.70","0.5"]]`,
		asks:     `[["30000.00","1.5"],["30000.10","1.0"],["30000.20","0.5"]]`,
		thinBids: `[["29999.90","0.4"],["29999.80","0.4"]]`,
		thinAsks: `[["30000.00","0.4"],["30000.10","0.4"]]`,
	}
	perpLadder := ladder{
		bids:        `[["64266.60","3.0"],["64266.50","1.2"],["64266.40","0.8"]]`,
		asks:        `[["64266.70","2.5"],["64266.80","1.5"],["64266.90","1.0"]]`,
		thinBids:    `[["64266.60","0.5"],["64266.50","0.3"]]`,
		thinAsks:    `[["64266.70","0.5"],["64266.80","0.3"]]`,
		shiftedAsks: `[["64266.80","2.5"],["64266.90","1.5"],["64267.00","1.0"]]`,
	}
	spotKey := capture.StreamKey{Source: events.SourceID{Instrument: calibSpotInst, Stream: events.StreamBook}, Topic: "btcusdt@depth20@100ms"}
	for window, recs := range mkBookRecs(spotLadder, 500, func(seq uint64, bids, asks string) string {
		return fmt.Sprintf(`{"lastUpdateId":%d,"bids":%s,"asks":%s}`, seq+1000, bids, asks)
	}) {
		rels = append(rels, writeArchiveFile(t, archiveRoot, spotKey, window, recs))
	}
	perpKey := capture.StreamKey{Source: events.SourceID{Instrument: calibPerpInst, Stream: events.StreamBook}, Topic: "btcusdt@depth20@100ms"}
	for window, recs := range mkBookRecs(perpLadder, 20, func(seq uint64, bids, asks string) string {
		return fmt.Sprintf(`{"e":"depthUpdate","E":%d,"s":"BTCUSDT","u":%d,"b":%s,"a":%s}`,
			time.Date(2026, 7, 30, 10, 0, 0, 0, time.UTC).UnixMilli(), seq+2000, bids, asks)
	}) {
		rels = append(rels, writeArchiveFile(t, archiveRoot, perpKey, window, recs))
	}

	// --- markPrice spanning the 16:00 settlement: 15:00–16:05 at 5s.
	markKey := capture.StreamKey{Source: events.SourceID{Instrument: calibPerpInst, Stream: events.StreamFunding}, Topic: "btcusdt@markPrice@1s"}
	settle := time.Date(2026, 7, 30, 16, 0, 0, 0, time.UTC)
	next := settle.Add(8 * time.Hour)
	markFiles := map[time.Time][]capture.Record{}
	from := time.Date(2026, 7, 30, 15, 0, 0, 0, time.UTC)
	for i := 0; ; i++ {
		at := from.Add(time.Duration(i) * 5 * time.Second)
		if at.After(settle.Add(5 * time.Minute)) {
			break
		}
		target, rate := settle, "0.00025000"
		if !at.Before(settle) {
			target, rate = next, "0.00010000"
		}
		markFiles[at.Truncate(time.Hour)] = append(markFiles[at.Truncate(time.Hour)],
			dataRec(uint64(i+1), at, markFrame(at.Add(-80*time.Millisecond).UnixMilli(), rate, target.UnixMilli())))
	}
	for window, recs := range markFiles {
		rels = append(rels, writeArchiveFile(t, archiveRoot, markKey, window, recs))
	}

	writeManifest(t, archiveRoot, "2026-07-30", rels)
	return datasetsDir, archiveRoot, filepath.Join("..", "..", "..", "..", "configs", "reference.btcusdt.json")
}

func joinRows(rows []string) string {
	out := ""
	for i, r := range rows {
		if i > 0 {
			out += ","
		}
		out += r
	}
	return out
}

func writeRunDataset(t *testing.T, dir, venueName, name string, doc map[string]any) {
	t.Helper()
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal dataset: %v", err)
	}
	full := filepath.Join(dir, venueName, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, data, 0o644); err != nil {
		t.Fatalf("write dataset: %v", err)
	}
}

const runTestExeDigest = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func cleanCalibrator() Calibrator {
	return Calibrator{
		ReadRevision:     func() (string, bool, error) { return "testrev0000", false, nil },
		ExecutableDigest: func() (string, error) { return runTestExeDigest, nil },
	}
}

var runFreeze = time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)

func TestRunEmitsLoadableGovernedArtifact(t *testing.T) {
	datasets, archive, ref := runWorld(t)
	out := filepath.Join(t.TempDir(), "calibration.btcusdt.v1.json")

	res, err := cleanCalibrator().Run(RunConfig{
		DatasetsDir: datasets, ArchiveRoot: archive, ReferencePath: ref,
		OutPath: out, Freeze: runFreeze,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	art, err := driven.LoadArtifactFile(out)
	if err != nil {
		t.Fatalf("load emitted artifact: %v", err)
	}
	if art.Ref.Hash != res.Hash {
		t.Fatalf("hash %s vs result %s", art.Ref.Hash, res.Hash)
	}

	// The promotion identity IS the seal — venue and fork, verbatim.
	if art.Promotion.Venue != SealVenue || art.Promotion.Fork != string(SealFork) {
		t.Fatalf("promotion %s/%s; want the §41h seal %s/%s",
			art.Promotion.Venue, art.Promotion.Fork, SealVenue, SealFork)
	}
	// The procedure identity is the injected binding.
	if art.Doc.Procedure.Commit != "testrev0000" || art.Doc.Procedure.ExecutableSHA256 != runTestExeDigest {
		t.Fatalf("procedure identity %+v not the executing build's", art.Doc.Procedure)
	}
	// The H1 boundary echo is the DECLARED instant, not a recomputation.
	if art.Doc.Datasets.H1Boundary != "2026-02-01T00:00:00Z" {
		t.Fatalf("h1 boundary echo = %q", art.Doc.Datasets.H1Boundary)
	}
	// The H2 enumeration: exactly the one manifested day before the freeze.
	if len(art.Doc.Datasets.Manifests) != 1 || art.Doc.Datasets.Manifests[0].Day != "2026-07-30" {
		t.Fatalf("manifests = %+v; want exactly 2026-07-30", art.Doc.Datasets.Manifests)
	}
	// The history digests are the files' actual bytes.
	for _, h := range art.Doc.Datasets.History {
		data, err := os.ReadFile(filepath.Join(datasets, "..", h.File))
		if err != nil {
			t.Fatalf("history file %s: %v", h.File, err)
		}
		if err := CheckDatasetDigest(h.SHA256, data); err != nil {
			t.Fatalf("history digest %s: %v", h.File, err)
		}
	}
	// 18 cells: calm 2 legs × 2 sides × 3 candidates (12) plus the spot
	// "normal" thin tail (6, at EXACTLY MinEvidence — kept under <,
	// dropped under <=); the perp normal tail (20 observations) is
	// ABSENT. Leg and side keying are visible in VALUES: the perp's
	// completion books vanish their best ask, so perp-ask calm cells are
	// strictly positive while spot-ask and perp-bid stay zero.
	if len(art.Cost.Cells) != 18 {
		t.Fatalf("cells = %d; want 18 (12 calm + 6 spot normal at exactly min evidence)", len(art.Cost.Cells))
	}
	var sawSpotNormal, sawPerpAskPositive, sawPerpBidZero, sawSpotAskZero bool
	for _, c := range art.Cost.Cells {
		if c.Observations < ProcMinEvidence {
			t.Fatalf("cell %+v below min evidence survived", c)
		}
		sign, err := c.P90.Sign()
		if err != nil {
			t.Fatalf("cell p90: %v", err)
		}
		switch {
		case c.Instrument == calibSpotInst && c.Stratum == "normal":
			sawSpotNormal = true
			if c.Observations != ProcMinEvidence {
				t.Fatalf("spot normal cell observations = %d; want exactly %d", c.Observations, ProcMinEvidence)
			}
		case c.Instrument == calibPerpInst && c.Stratum == "normal":
			t.Fatalf("perp normal cell survived with %d observations", c.Observations)
		case c.Instrument == calibPerpInst && c.Stratum == "calm" && c.Side == "ask":
			sawPerpAskPositive = sign > 0
			if sign <= 0 {
				t.Fatalf("perp calm ask p90 = %s; want positive (the vanished best)", c.P90)
			}
		case c.Instrument == calibPerpInst && c.Stratum == "calm" && c.Side == "bid":
			sawPerpBidZero = sign == 0
		case c.Instrument == calibSpotInst && c.Stratum == "calm" && c.Side == "ask":
			sawSpotAskZero = sign == 0
		}
	}
	if !sawSpotNormal || !sawPerpAskPositive || !sawPerpBidZero || !sawSpotAskZero {
		t.Fatalf("keying assertions incomplete: spotNormal=%v perpAsk+=%v perpBid0=%v spotAsk0=%v",
			sawSpotNormal, sawPerpAskPositive, sawPerpBidZero, sawSpotAskZero)
	}
	// The sample start is the first UTC day after the freeze.
	if !art.Promotion.SampleStart.Equal(time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("sample start %v; want 2026-08-01T00:00:00Z", art.Promotion.SampleStart)
	}
}

// The §41i determinism suspect: TWO invocations of the run over the same
// world produce byte-identical artifacts.
func TestRunDeterministicAcrossInvocations(t *testing.T) {
	datasets, archive, ref := runWorld(t)
	out1 := filepath.Join(t.TempDir(), "calibration.btcusdt.v1.json")
	out2 := filepath.Join(t.TempDir(), "calibration.btcusdt.v1.json")

	r1, err := cleanCalibrator().Run(RunConfig{DatasetsDir: datasets, ArchiveRoot: archive, ReferencePath: ref, OutPath: out1, Freeze: runFreeze})
	if err != nil {
		t.Fatalf("run 1: %v", err)
	}
	r2, err := cleanCalibrator().Run(RunConfig{DatasetsDir: datasets, ArchiveRoot: archive, ReferencePath: ref, OutPath: out2, Freeze: runFreeze})
	if err != nil {
		t.Fatalf("run 2: %v", err)
	}
	if !bytes.Equal(r1.Bytes, r2.Bytes) {
		t.Fatal("two invocations over one world emitted different bytes")
	}
	if r1.Hash != r2.Hash {
		t.Fatalf("hashes differ: %s vs %s", r1.Hash, r2.Hash)
	}
}

// §41e's binding refusals AND the clean positive control (§41i's
// execution-binding suspect: a fixture that only exercises the clean tree
// proves nothing).
func TestRunExecutionBindingRefusals(t *testing.T) {
	datasets, archive, ref := runWorld(t)
	base := RunConfig{DatasetsDir: datasets, ArchiveRoot: archive, ReferencePath: ref, Freeze: runFreeze}

	dirty := cleanCalibrator()
	dirty.ReadRevision = func() (string, bool, error) { return "testrev0000", true, nil }
	base.OutPath = filepath.Join(t.TempDir(), "a.json")
	if _, err := dirty.Run(base); !errors.Is(err, ErrTreeDirty) {
		t.Fatalf("dirty tree: %v, want ErrTreeDirty", err)
	}

	unstamped := cleanCalibrator()
	unstamped.ReadRevision = func() (string, bool, error) { return "", false, nil }
	base.OutPath = filepath.Join(t.TempDir(), "b.json")
	if _, err := unstamped.Run(base); !errors.Is(err, ErrRevisionUnavailable) {
		t.Fatalf("no revision: %v, want ErrRevisionUnavailable", err)
	}

	// A malformed executable digest refuses before assembly.
	badExe := cleanCalibrator()
	badExe.ExecutableDigest = func() (string, error) { return "NOT-A-DIGEST", nil }
	base.OutPath = filepath.Join(t.TempDir(), "c.json")
	if _, err := badExe.Run(base); !errors.Is(err, driven.ErrArtifactHashFormat) {
		t.Fatalf("bad digest: %v, want ErrArtifactHashFormat", err)
	}

	// The positive control: the clean binding emits (proved throughout
	// this file, but named here so the refusal suite cannot pass by
	// refusing everything).
	base.OutPath = filepath.Join(t.TempDir(), "d.json")
	if _, err := cleanCalibrator().Run(base); err != nil {
		t.Fatalf("clean binding refused: %v", err)
	}
}

// Write-once: a second run at the SAME path refuses — versions are
// append-only (§41d).
func TestRunRefusesExistingArtifact(t *testing.T) {
	datasets, archive, ref := runWorld(t)
	out := filepath.Join(t.TempDir(), "calibration.btcusdt.v1.json")
	cfg := RunConfig{DatasetsDir: datasets, ArchiveRoot: archive, ReferencePath: ref, OutPath: out, Freeze: runFreeze}
	if _, err := cleanCalibrator().Run(cfg); err != nil {
		t.Fatalf("first run: %v", err)
	}
	if _, err := cleanCalibrator().Run(cfg); !errors.Is(err, ErrArtifactExists) {
		t.Fatalf("second run: %v, want ErrArtifactExists", err)
	}
}

// checkDriftTail directly: the invariant compares RESULTING DRIFTS, not
// raw shifts — two non-negative tails both freeze zero and pass however
// they order; a merged tail freezing a LARGER margin than family 1's
// refuses.
func TestCheckDriftTailInvariant(t *testing.T) {
	qp := func(q, s string) domain.QuantilePoint {
		return domain.QuantilePoint{Q: procTestRate(t, q), Shift: procTestRate(t, s)}
	}
	tail := procTestRate(t, "0.05")
	grid := func(shift string) []domain.QuantilePoint {
		return []domain.QuantilePoint{qp("0.05", shift), qp("0.5", "0")}
	}

	// Family 1 more adverse than merged would ever be: fine.
	if err := checkDriftTail(grid("-0.002"), grid("-0.001"), tail, 8*time.Hour); err != nil {
		t.Fatalf("family-1-dominant tails refused: %v", err)
	}
	// Equal: fine.
	if err := checkDriftTail(grid("-0.002"), grid("-0.002"), tail, 8*time.Hour); err != nil {
		t.Fatalf("equal tails refused: %v", err)
	}
	// Both non-negative (one positive, one zero): both freeze ZERO drift —
	// equivalent, must pass (the synthetic world's own case).
	if err := checkDriftTail(grid("0.000002"), grid("0"), tail, 8*time.Hour); err != nil {
		t.Fatalf("two zero-drift tails refused: %v", err)
	}
	// Merged strictly more adverse AND negative: the pinning would
	// understate — refused by name.
	if err := checkDriftTail(grid("-0.001"), grid("-0.002"), tail, 8*time.Hour); !errors.Is(err, ErrDriftTailDominated) {
		t.Fatalf("dominated tail: %v, want ErrDriftTailDominated", err)
	}
}

// frameCellP90: no cell for the declared (stratum, candidate) refuses by
// name; with both sides present the WORSE side is drawn.
func TestFrameCellSelection(t *testing.T) {
	mkCell := func(side string, p90 string) driven.CostCell {
		return driven.CostCell{
			Instrument: calibSpotInst, Side: mdomain.BookSide(side), Stratum: ProcSurrogateStratum,
			MaxQty: ProcCandidateQty, P90: procTestRate(t, p90),
			Observations: ProcMinEvidence, Method: "m",
		}
	}
	if _, err := frameCellP90(calibSpotInst, nil); !errors.Is(err, ErrFrameUnpriced) {
		t.Fatalf("no cells: %v, want ErrFrameUnpriced", err)
	}
	got, err := frameCellP90(calibSpotInst, []driven.CostCell{mkCell("bid", "0.0001"), mkCell("ask", "0.0003")})
	if err != nil {
		t.Fatalf("selection: %v", err)
	}
	if got != procTestRate(t, "0.0003") {
		t.Fatalf("frame p90 = %s; want the worse side's 0.0003", got)
	}
}

// The procedure roster's internal coherence — the constraints that make
// the frozen parameters CONSUMABLE: the drift tail and the decision
// quantile must be grid members (EntryDriftFrom and ShiftAt both refuse
// otherwise, but at derivation time, not commit time — this test moves
// the refusal to the commit), the family-2 procedure must validate, and
// the classifier's last rule must be the catch-all so every runtime book
// classifies.
func TestProcedureRosterCoherence(t *testing.T) {
	if err := ProcFamily2.Validate(); err != nil {
		t.Fatalf("family-2 procedure: %v", err)
	}
	if err := ProcImminent.Validate(); err != nil {
		t.Fatalf("imminent procedure: %v", err)
	}
	inGrid := func(q string) bool {
		want := procTestRate(t, q)
		for _, g := range ProcResidualGrid {
			if g == want {
				return true
			}
		}
		return false
	}
	if !inGrid(ProcEntryDriftTail.Decimal()) {
		t.Fatalf("drift tail %s is not a grid member", ProcEntryDriftTail)
	}
	if !inGrid(ProcDecisionQuantile.Decimal()) {
		t.Fatalf("decision quantile %s is not a grid member", ProcDecisionQuantile)
	}
	cl := ProcClassifier()
	if err := cl.Validate(); err != nil {
		t.Fatalf("classifier: %v", err)
	}
	last := cl.Rules[len(cl.Rules)-1]
	if last.Spread.Lo.Kind != domain.BoundUnbounded || last.Spread.Hi.Kind != domain.BoundUnbounded ||
		last.Depth.Lo.Kind != domain.BoundUnbounded || last.Depth.Hi.Kind != domain.BoundUnbounded {
		t.Fatalf("last rule %q is not the catch-all; a runtime book could be unclassifiable", last.Name)
	}
	if ProcFamily2.Latency != ProcLatencyAssumedBound {
		t.Fatal("the counterfactual walk-ahead and the declared latency bound have diverged")
	}
}

func procTestRate(t *testing.T, s string) fixedpoint.Rate {
	t.Helper()
	return procRate(s)
}
