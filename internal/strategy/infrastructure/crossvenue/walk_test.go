package crossvenue

// Walker battery. Fixture discipline: the manifest-day bound fixture holds
// a day EQUAL to the bound's UTC day — strictly-before and at-or-before
// differ only there; the unvouched-file fixture vouches the day's OTHER
// file, so Vouches is exercised against a populated map rather than a
// vacuously empty one.

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/capture"
	"github.com/giiorg/arb-lab/internal/platform/events"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// fixRec is one record of a fixture archive file.
type fixRec struct {
	kind    capture.RecordKind
	recv    time.Time
	payload string
}

func dataAt(recv time.Time, payload string) fixRec {
	return fixRec{kind: capture.RecordKindData, recv: recv, payload: payload}
}

func gapAt(recv time.Time) fixRec {
	return fixRec{kind: capture.RecordKindGap, recv: recv, payload: `{}`}
}

// writeFixture writes one archive file from explicit records — recv
// instants are the walkers' grid inputs, so the fixture controls them.
func writeFixture(t *testing.T, path string, torn bool, recs ...fixRec) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	buf := capture.AppendFileHeader(nil)
	var err error
	for i, r := range recs {
		buf, err = capture.AppendRecord(buf, capture.Record{
			Kind:       r.kind,
			CaptureSeq: uint64(i + 1),
			RecvTime:   r.recv,
			Mono:       time.Duration(i + 1),
			Payload:    []byte(r.payload),
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if torn {
		whole := len(buf)
		buf, err = capture.AppendRecord(buf, capture.Record{
			Kind: capture.RecordKindData, CaptureSeq: uint64(len(recs) + 1),
			RecvTime: recs[len(recs)-1].recv.Add(time.Second), Mono: time.Hour, Payload: []byte(`{}`),
		})
		if err != nil {
			t.Fatal(err)
		}
		buf = buf[:whole+7]
	}
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeManifest writes one day's manifest vouching exactly the given
// root-relative paths.
func writeManifest(t *testing.T, root, day string, rels ...string) {
	t.Helper()
	m := capture.Manifest{Format: "arb-capture-manifest/v1", Day: day}
	for _, rel := range rels {
		m.Entries = append(m.Entries, capture.ManifestEntry{Path: rel, Window: day + "T00:00:00Z"})
	}
	dir := filepath.Join(root, "v1", "_manifest")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, day+".json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// walkInst / walkKey name the fixture stream: a Bybit linear book stream,
// arbitrary for the walker (scanStream reads paths, not wire bytes).
var walkInst = ids.VenueInstrument{Venue: "bybit", Product: ids.ProductLinearPerp, Contract: ids.ContractLinear, Symbol: "ABCUSDT"}
var walkKey = capture.StreamKey{Source: events.SourceID{Instrument: walkInst, Stream: events.StreamBook}, Topic: "orderbook.50.ABCUSDT"}

// streamRel returns the stream file's root-relative path for a day/hour.
func streamRel(day, name string) string {
	return "v1/bybit/linear_perp/linear/ABCUSDT/book/orderbook.50.ABCUSDT/" +
		day[0:4] + "/" + day[5:7] + "/" + day[8:10] + "/" + name
}

func streamAbs(root, day, name string) string {
	return filepath.Join(root, filepath.FromSlash(streamRel(day, name)))
}

// The bound day itself is EXCLUDED: its manifest is still gaining hours
// while capture runs (family2's determinism rule), and here it is also the
// seal — a day equal to the cutoff's day is gate evidence.
func TestLoadManifestDaysExcludesBoundDay(t *testing.T) {
	root := t.TempDir()
	for _, day := range []string{"2026-07-31", "2026-07-30", "2026-08-01", "2026-08-02"} {
		writeManifest(t, root, day)
	}
	days, err := LoadManifestDays(root, time.Date(2026, 8, 1, 11, 13, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(days) != 2 || days[0].Day != "2026-07-30" || days[1].Day != "2026-07-31" {
		t.Fatalf("days %+v, want ascending 07-30, 07-31", days)
	}
	for _, d := range days {
		if d.SHA256 == "" {
			t.Fatalf("day %s missing digest", d.Day)
		}
	}
}

func TestLoadManifestDaysRefusesMalformed(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "v1", "_manifest")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "2026-07-30.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadManifestDays(root, BooksCutoff); !errors.Is(err, ErrManifestMalformed) {
		t.Fatalf("err %v, want ErrManifestMalformed", err)
	}
}

// A manifest whose body claims a different day than its filename is a
// mislabeled vouching document, refused.
func TestLoadManifestDaysRefusesDayMismatch(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "2026-07-30")
	src := filepath.Join(root, "v1", "_manifest", "2026-07-30.json")
	dst := filepath.Join(root, "v1", "_manifest", "2026-07-29.json")
	if err := os.Rename(src, dst); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadManifestDays(root, BooksCutoff); !errors.Is(err, ErrManifestMalformed) {
		t.Fatalf("err %v, want ErrManifestMalformed", err)
	}
}

func TestLoadManifestDaysEmptyRefuses(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "2026-08-02") // nothing before the bound
	if _, err := LoadManifestDays(root, BooksCutoff); !errors.Is(err, ErrNoManifestDays) {
		t.Fatalf("err %v, want ErrNoManifestDays", err)
	}
}

// A file on disk the day's manifest does not vouch for refuses BY NAME —
// and the manifest DOES vouch the day's other file, so the check reads a
// populated map.
func TestScanStreamRefusesUnvouchedFile(t *testing.T) {
	root := t.TempDir()
	day := "2026-07-30"
	at := time.Date(2026, 7, 30, 10, 0, 1, 0, time.UTC)
	writeFixture(t, streamAbs(root, day, "2026-07-30T10.arbcap"), false, dataAt(at, `{}`))
	writeFixture(t, streamAbs(root, day, "2026-07-30T11.arbcap"), false, dataAt(at.Add(time.Hour), `{}`))
	writeManifest(t, root, day, streamRel(day, "2026-07-30T10.arbcap")) // T11 unvouched
	days, err := LoadManifestDays(root, BooksCutoff)
	if err != nil {
		t.Fatal(err)
	}
	err = scanStream(root, walkKey, days, nil, func(capture.Record) error { return nil })
	if !errors.Is(err, ErrDayUnmanifested) {
		t.Fatalf("err %v, want ErrDayUnmanifested", err)
	}
}

// A vouched file that ends mid-record is integrity damage, not a tail to
// measure around (§45b's torn-tail policy).
func TestScanStreamRefusesTornFile(t *testing.T) {
	root := t.TempDir()
	day := "2026-07-30"
	at := time.Date(2026, 7, 30, 10, 0, 1, 0, time.UTC)
	writeFixture(t, streamAbs(root, day, "2026-07-30T10.arbcap"), true, dataAt(at, `{}`))
	writeManifest(t, root, day, streamRel(day, "2026-07-30T10.arbcap"))
	days, err := LoadManifestDays(root, BooksCutoff)
	if err != nil {
		t.Fatal(err)
	}
	err = scanStream(root, walkKey, days, nil, func(capture.Record) error { return nil })
	if !errors.Is(err, ErrArchiveCorrupt) {
		t.Fatalf("err %v, want ErrArchiveCorrupt", err)
	}
}

// A day with no directory for this stream is coverage, not integrity: the
// scan continues into the days that do hold files, and onFile fires once
// per FILE — the stateful consumers' cold-start seam.
func TestScanStreamSkipsAbsentDayAndSignalsFiles(t *testing.T) {
	root := t.TempDir()
	at := time.Date(2026, 7, 31, 10, 0, 1, 0, time.UTC)
	writeFixture(t, streamAbs(root, "2026-07-31", "2026-07-31T10.arbcap"), false, dataAt(at, `a`))
	writeFixture(t, streamAbs(root, "2026-07-31", "2026-07-31T11.arbcap"), false, dataAt(at.Add(time.Hour), `b`))
	writeManifest(t, root, "2026-07-30") // manifested day, no stream files
	writeManifest(t, root, "2026-07-31",
		streamRel("2026-07-31", "2026-07-31T10.arbcap"),
		streamRel("2026-07-31", "2026-07-31T11.arbcap"))
	days, err := LoadManifestDays(root, BooksCutoff)
	if err != nil {
		t.Fatal(err)
	}
	var trace []string
	err = scanStream(root, walkKey, days, func() { trace = append(trace, "file") }, func(rec capture.Record) error {
		if rec.Kind == capture.RecordKindData {
			trace = append(trace, string(rec.Payload))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	want := []string{"file", "a", "file", "b"}
	if len(trace) != len(want) {
		t.Fatalf("trace %v, want %v", trace, want)
	}
	for i := range want {
		if trace[i] != want[i] {
			t.Fatalf("trace %v, want %v", trace, want)
		}
	}
}
