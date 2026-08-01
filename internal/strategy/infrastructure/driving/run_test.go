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
	"github.com/giiorg/arb-lab/internal/platform/events"
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
	klinePage := func(base float64) json.RawMessage {
		var rows []string
		for i := 0; i < 48; i++ {
			at := start.Add(time.Duration(i) * time.Hour)
			rows = append(rows, fmt.Sprintf(`[%d,"%0.2f"]`, at.UnixMilli(), base))
		}
		return json.RawMessage("[" + joinRows(rows) + "]")
	}
	writeRunDataset(t, datasetsDir, "binance", "spot_kline_1h.TEST.json", map[string]any{
		"venue": "binance", "series": "spot_kline_1h", "symbol": "BTCUSDT",
		"start": "2023-01-01T00:00:00Z", "end": "2023-01-02T23:00:00Z", "interval": "1h0m0s",
		"fetched_at": "2026-08-01T00:00:00Z", "endpoint": "test",
		"pages": []json.RawMessage{klinePage(30000)},
	})
	writeRunDataset(t, datasetsDir, "binance", "mark_kline_1h.TEST.json", map[string]any{
		"venue": "binance", "series": "mark_kline_1h", "symbol": "BTCUSDT",
		"start": "2023-01-01T00:00:00Z", "end": "2023-01-02T23:00:00Z", "interval": "1h0m0s",
		"fetched_at": "2026-08-01T00:00:00Z", "endpoint": "test",
		"pages": []json.RawMessage{klinePage(30010)},
	})

	// --- Family 2: day 2026-07-30. Books: 505 sample windows × 2 records
	// (one at grid−100ms, the completion at grid+300ms), calm stratum
	// (3 BTC/side, one-tick spread), starting 10:00.
	var rels []string
	day := time.Date(2026, 7, 30, 10, 0, 0, 0, time.UTC)
	mkBookRecs := func(frame func(uint64, string, string) string) map[time.Time][]capture.Record {
		files := map[time.Time][]capture.Record{}
		for i := 0; i < 505; i++ {
			grid := day.Add(time.Duration(i) * 10 * time.Second)
			for n, off := range []time.Duration{-100 * time.Millisecond, 300 * time.Millisecond} {
				at := grid.Add(off)
				window := at.Truncate(time.Hour)
				seq := uint64(i*2 + n + 1)
				bids := fmt.Sprintf(`[["29999.90","1.5"],["29999.80","1.0"],["29999.70","0.5"]]`)
				asks := fmt.Sprintf(`[["30000.00","1.5"],["30000.10","1.0"],["30000.20","0.5"]]`)
				files[window] = append(files[window], dataRec(seq, at, frame(seq, bids, asks)))
			}
		}
		return files
	}
	spotKey := capture.StreamKey{Source: events.SourceID{Instrument: calibSpotInst, Stream: events.StreamBook}, Topic: "btcusdt@depth20@100ms"}
	for window, recs := range mkBookRecs(func(seq uint64, bids, asks string) string {
		return fmt.Sprintf(`{"lastUpdateId":%d,"bids":%s,"asks":%s}`, seq+1000, bids, asks)
	}) {
		rels = append(rels, writeArchiveFile(t, archiveRoot, spotKey, window, recs))
	}
	perpKey := capture.StreamKey{Source: events.SourceID{Instrument: calibPerpInst, Stream: events.StreamBook}, Topic: "btcusdt@depth20@100ms"}
	for window, recs := range mkBookRecs(func(seq uint64, bids, asks string) string {
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
	// 12 cells: 2 legs × 2 sides × 3 candidates, all calm, all ≥ 500.
	if len(art.Cost.Cells) != 12 {
		t.Fatalf("cells = %d; want 12", len(art.Cost.Cells))
	}
	for _, c := range art.Cost.Cells {
		if c.Stratum != "calm" || c.Observations < ProcMinEvidence {
			t.Fatalf("cell %+v: stratum/observations off", c)
		}
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
