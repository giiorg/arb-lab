package capture

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// Slice F's manifest (§38a–§38c). The fixture-collapse checklist shaped these
// before any mutation ran, and the choices it forced are stated where they
// bite: the day-boundary fixture uses a 23:00 window so "day from window" and
// "day from close instant" disagree; the gap count is 2 so 0/1 collapses
// cannot hide a dropped counter; records, gaps and lastSeq are pairwise
// distinct so a swapped field cannot pass; and the mismatch table wrongs ONE
// field at a time, because a fixture that wrongs all four cannot say which
// comparison exists.

// planted is a valid, deliberately wrong 64-hex digest, distinct from anything
// SHA-256 of these fixtures produces.
const planted = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"

// editManifest rewrites one day's manifest through a mutation, the way a test
// plants a stale or corrupt entry.
func editManifest(t *testing.T, root, day string, mutate func(*Manifest)) {
	t.Helper()
	m, err := loadManifest(root, day)
	if err != nil {
		t.Fatalf("load manifest %s: %v", day, err)
	}
	if m == nil {
		t.Fatalf("no manifest for %s to edit", day)
	}
	mutate(m)
	body, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatalf("encode manifest: %v", err)
	}
	if err := os.WriteFile(manifestPath(root, day), body, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}

// copyTree copies an archive root, the way an off-host sync would (§38e).
func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		body, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, body, 0o644)
	})
	if err != nil {
		t.Fatalf("copy %s to %s: %v", src, dst, err)
	}
}

// TestWriterClosesLandManifestEntries drives a REAL writer across a UTC day
// boundary: a 23:00 window whose rotation and close happen after midnight.
// That window is the fixture the day rule needs — the entry must land under
// the WINDOW's day while the close instant's day disagrees (§38a), and any
// window would pass for an implementation that keyed on either.
func TestWriterClosesLandManifestEntries(t *testing.T) {
	c := newFakeClock(time.Date(2026, 7, 29, 23, 5, 0, 0, time.UTC))
	cfg := testConfig(t, perpKey(), c)
	w, err := OpenWriter(cfg)
	if err != nil {
		t.Fatalf("OpenWriter: %v", err)
	}

	for seq := uint64(1); seq <= 2; seq++ {
		if err := w.Data(dataRecord(seq, c, `{"p":1}`)); err != nil {
			t.Fatalf("Data %d: %v", seq, err)
		}
		c.Advance(time.Minute)
	}
	// Two gaps, not one: with a single gap "the counter increments" and "the
	// counter is pinned to 1" are the same claim.
	if err := w.Gap(Gap{Cause: CauseOverflow, From: 3, To: 5, Count: 3}); err != nil {
		t.Fatalf("Gap: %v", err)
	}
	if err := w.Gap(Gap{Cause: CauseReconnect, Detail: "socket dropped"}); err != nil {
		t.Fatalf("Gap: %v", err)
	}
	if err := w.Data(dataRecord(6, c, `{"p":2}`)); err != nil {
		t.Fatalf("Data 6: %v", err)
	}

	// Cross midnight: this Data record's RecvTime rotates the writer, closing
	// the 23:00 file at an instant inside 2026-07-30.
	c.Advance(58 * time.Minute)
	if err := w.Data(dataRecord(7, c, `{"p":3}`)); err != nil {
		t.Fatalf("Data 7: %v", err)
	}
	pathT23 := perpKey().FilePath(cfg.Root, time.Date(2026, 7, 29, 23, 0, 0, 0, time.UTC), cfg.RotationPeriod)
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	m, err := loadManifest(cfg.Root, "2026-07-29")
	if err != nil {
		t.Fatalf("load 2026-07-29: %v", err)
	}
	if m == nil {
		t.Fatal("the 23:00 window's entry did not land under its WINDOW's day")
	}
	if len(m.Entries) != 1 {
		t.Fatalf("2026-07-29 manifest holds %d entries, want 1", len(m.Entries))
	}
	e := m.Entries[0]
	if err := e.Validate(); err != nil {
		t.Errorf("entry does not validate: %v", err)
	}
	if e.Source != ManifestSourceClose {
		t.Errorf("Source = %q, want %q", e.Source, ManifestSourceClose)
	}
	if want := uint64(len(readAllRecords(t, pathT23))); e.Records != want {
		t.Errorf("Records = %d, a walk of the file finds %d", e.Records, want)
	}
	if e.Gaps != 2 {
		t.Errorf("Gaps = %d, want 2", e.Gaps)
	}
	if e.LastSeq != 6 {
		t.Errorf("LastSeq = %d, want 6", e.LastSeq)
	}
	sidecar, err := readSidecarDigest(pathT23)
	if err != nil {
		t.Fatalf("sidecar: %v", err)
	}
	if e.SHA256 != sidecar {
		t.Errorf("entry digest %s is not the sidecar's %s", e.SHA256, sidecar)
	}
	if e.Window != "2026-07-29T23:00:00Z" {
		t.Errorf("Window = %q, want the window start", e.Window)
	}
	if filepath.IsAbs(e.Path) || strings.Contains(e.Path, `\`) {
		t.Errorf("Path %q is not root-relative slash form", e.Path)
	}
	if got := filepath.Join(cfg.Root, filepath.FromSlash(e.Path)); got != pathT23 {
		t.Errorf("Path resolves to %q, want %q", got, pathT23)
	}

	next, err := loadManifest(cfg.Root, "2026-07-30")
	if err != nil {
		t.Fatalf("load 2026-07-30: %v", err)
	}
	if next == nil || len(next.Entries) != 1 {
		t.Fatalf("2026-07-30 manifest = %+v, want the post-midnight file's entry", next)
	}
	if next.Entries[0].LastSeq != 7 {
		t.Errorf("next day LastSeq = %d, want 7", next.Entries[0].LastSeq)
	}
}

// TestManifestUpsertReplacesAndBackfillDoesNot pins the two write policies to
// their callers (§38a vs §38b): a re-close legitimately replaces its entry; a
// repair must never displace close-time truth.
func TestManifestUpsertReplacesAndBackfillDoesNot(t *testing.T) {
	root := t.TempDir()
	entry := func(path, sha string, src ManifestSource) ManifestEntry {
		return ManifestEntry{
			Path: path, Window: "2026-07-29T14:00:00Z", SHA256: sha,
			Records: 5, Gaps: 1, LastSeq: 3,
			ClosedAt: "2026-07-29T15:00:00.5Z", Source: src,
		}
	}
	first := entry("v1/a/file.arbcap", strings.Repeat("a", 64), ManifestSourceClose)
	if err := upsertManifestEntry(root, first); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	second := entry("v1/a/file.arbcap", strings.Repeat("b", 64), ManifestSourceClose)
	if err := upsertManifestEntry(root, second); err != nil {
		t.Fatalf("upsert again: %v", err)
	}
	m, err := loadManifest(root, "2026-07-29")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(m.Entries) != 1 || m.Entries[0].SHA256 != second.SHA256 {
		t.Fatalf("after re-close: %+v, want one entry carrying the second digest", m.Entries)
	}

	repair := entry("v1/a/file.arbcap", planted, ManifestSourceBackfill)
	inserted, err := insertManifestEntryIfAbsent(root, repair)
	if err != nil {
		t.Fatalf("insert-if-absent: %v", err)
	}
	if inserted {
		t.Error("a backfill displaced a close-time entry")
	}
	other := entry("v1/b/file.arbcap", planted, ManifestSourceBackfill)
	inserted, err = insertManifestEntryIfAbsent(root, other)
	if err != nil {
		t.Fatalf("insert-if-absent new path: %v", err)
	}
	if !inserted {
		t.Error("a backfill for a new path was refused")
	}
	m, err = loadManifest(root, "2026-07-29")
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(m.Entries) != 2 || m.Entries[0].SHA256 != second.SHA256 {
		t.Fatalf("final entries %+v: the close entry must survive with its own digest", m.Entries)
	}
	if m.Entries[0].Path > m.Entries[1].Path {
		t.Errorf("entries are not sorted by path: %q, %q", m.Entries[0].Path, m.Entries[1].Path)
	}
}

// TestManifestEntryRefusesTheZeroValue is the platform rule worn by the new
// type: every unusable shape is refused with the sentinel that names it.
func TestManifestEntryRefusesTheZeroValue(t *testing.T) {
	valid := ManifestEntry{
		Path: "v1/a/file.arbcap", Window: "2026-07-29T14:00:00Z",
		SHA256: strings.Repeat("a", 64), Records: 2, LastSeq: 1,
		ClosedAt: "2026-07-29T15:00:00Z", Source: ManifestSourceClose,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("the valid control fails: %v", err)
	}
	cases := []struct {
		name   string
		mutate func(*ManifestEntry)
		want   error
	}{
		{"zero value", func(e *ManifestEntry) { *e = ManifestEntry{} }, ErrManifestPathUnset},
		{"absolute path", func(e *ManifestEntry) { e.Path = "/v1/a/file.arbcap" }, ErrManifestPathShape},
		{"backslash path", func(e *ManifestEntry) { e.Path = `v1\a\file.arbcap` }, ErrManifestPathShape},
		{"short digest", func(e *ManifestEntry) { e.SHA256 = "abc123" }, ErrManifestDigestShape},
		{"non-hex digest", func(e *ManifestEntry) { e.SHA256 = strings.Repeat("z", 64) }, ErrManifestDigestShape},
		{"zero records", func(e *ManifestEntry) { e.Records = 0 }, ErrManifestRecordsZero},
		{"unset closed_at", func(e *ManifestEntry) { e.ClosedAt = "" }, ErrManifestClosedAtUnset},
		{"unparseable closed_at", func(e *ManifestEntry) { e.ClosedAt = "yesterday" }, ErrManifestTimeShape},
		{"unparseable window", func(e *ManifestEntry) { e.Window = "hour 14" }, ErrManifestTimeShape},
		{"unset source", func(e *ManifestEntry) { e.Source = ManifestSourceUnspecified }, ErrManifestSourceUnspecified},
		{"unknown source", func(e *ManifestEntry) { e.Source = "sidecar" }, ErrManifestSourceUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := valid
			tc.mutate(&e)
			if err := e.Validate(); !errorIs(err, tc.want) {
				t.Errorf("Validate() = %v, want %v", err, tc.want)
			}
		})
	}
}

// TestBackfillDerivesFromSidecarNotBytes plants a sidecar digest the bytes do
// not hash to: the backfilled entry must carry the PLANTED value (§38b —
// close-time truth is copied, never recomputed), and an implementation that
// re-hashed the file would pass every fixture whose sidecar happens to agree.
// The crashed and sidecar-less neighbours pin the two refusals in the same
// walk.
func TestBackfillDerivesFromSidecarNotBytes(t *testing.T) {
	root := t.TempDir()
	w1, w2, w3 := fixtureWindow(t, 1), fixtureWindow(t, 2), fixtureWindow(t, 3)

	sealed := newArchive(t, root, perpKey(), w1)
	sealed.open(0)
	sealed.data(1)
	sealed.data(2)
	sealed.gap(Gap{Cause: CauseOverflow, From: 3, To: 4, Count: 2})
	sealed.gap(Gap{Cause: CauseReconnect, Detail: "dropped"})
	sealed.data(5)
	sealed.seal()
	sealedPath := sealed.write()

	crashed := newArchive(t, root, perpKey(), w2)
	crashed.open(0)
	crashed.data(6)
	crashed.write() // sidecar, no Close: no close-time truth to backfill

	bare := newArchive(t, root, perpKey(), w3)
	bare.open(0)
	bare.data(7)
	bare.seal()
	barePath := bare.write()
	if err := os.Remove(barePath + digestExt); err != nil {
		t.Fatalf("remove sidecar: %v", err)
	}

	// The builder manifested the sealed and bare files; strip the manifest so
	// backfill starts from the pre-slice-F state, and plant the wrong sidecar.
	if err := os.RemoveAll(manifestDir(root)); err != nil {
		t.Fatalf("remove manifests: %v", err)
	}
	sidecarBody := planted + "  " + filepath.Base(sealedPath) + "\n"
	if err := os.WriteFile(sealedPath+digestExt, []byte(sidecarBody), 0o644); err != nil {
		t.Fatalf("plant sidecar: %v", err)
	}

	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	added, err := BackfillManifestDay(root, w1, now)
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if added != 1 {
		t.Fatalf("backfill added %d entries, want 1 (crashed and sidecar-less skipped)", added)
	}
	m, err := loadManifest(root, w1.Format(manifestDayLayout))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(m.Entries) != 1 {
		t.Fatalf("%d entries, want 1", len(m.Entries))
	}
	e := m.Entries[0]
	if e.SHA256 != planted {
		t.Errorf("digest %s: the entry must COPY the sidecar, not re-hash the file", e.SHA256)
	}
	if e.Source != ManifestSourceBackfill {
		t.Errorf("Source = %q, want %q — a repair must say so forever", e.Source, ManifestSourceBackfill)
	}
	if e.Records != 7 || e.Gaps != 2 || e.LastSeq != 5 {
		t.Errorf("counts {records %d, gaps %d, lastSeq %d}, want {7, 2, 5} from the walk",
			e.Records, e.Gaps, e.LastSeq)
	}
	if e.ClosedAt != now.Format(time.RFC3339Nano) {
		t.Errorf("ClosedAt = %q, want the backfill instant — the close instant is a fact the crash destroyed", e.ClosedAt)
	}

	again, err := BackfillManifestDay(root, w1, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("second backfill: %v", err)
	}
	if again != 0 {
		t.Errorf("second run added %d entries, want 0 — backfill is idempotent", again)
	}
}

// TestManifestUpsertsAreSerializedAcrossWriters is the blocking flock under
// -race: eight writers rotating at one boundary is the NORMAL case (§38a),
// and a lost entry here would be a manifest lying by omission.
func TestManifestUpsertsAreSerializedAcrossWriters(t *testing.T) {
	root := t.TempDir()
	var wg sync.WaitGroup
	errs := make([]error, 8)
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = upsertManifestEntry(root, ManifestEntry{
				Path:   "v1/stream" + string(rune('a'+i)) + "/f.arbcap",
				Window: "2026-07-29T14:00:00Z", SHA256: strings.Repeat("a", 64),
				Records: 2, LastSeq: 1, ClosedAt: "2026-07-29T15:00:00Z",
				Source: ManifestSourceClose,
			})
		}()
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("writer %d: %v", i, err)
		}
	}
	m, err := loadManifest(root, "2026-07-29")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(m.Entries) != 8 {
		t.Fatalf("%d entries survived 8 concurrent closes, want 8", len(m.Entries))
	}
}

// twoWindowCfg is a verify request spanning windows w and w+1.
func twoWindowCfg(root string, w time.Time) VerifyConfig {
	cfg := builtCfg(root, w)
	cfg.To = w.Add(2 * verifyPeriod)
	return cfg
}

// TestVerifyManifestCrossChecks is §38c's finding roster, one distinguishing
// fixture per row.
func TestVerifyManifestCrossChecks(t *testing.T) {
	w1, w2 := fixtureWindow(t, 1), fixtureWindow(t, 2)
	day := w1.Format(manifestDayLayout)

	// build seals one archive per window under a fresh root.
	build := func(t *testing.T, windows ...time.Time) string {
		t.Helper()
		root := t.TempDir()
		seq := uint64(0)
		for _, w := range windows {
			b := newArchive(t, root, perpKey(), w)
			b.open(seq)
			for range 3 {
				seq++
				b.data(seq)
			}
			b.fillAlive(builtCfg(root, w))
			b.seal()
			b.write()
		}
		return root
	}

	t.Run("a deleted file is reported by the entry that outlives it", func(t *testing.T) {
		root := build(t, w1, w2)
		gone := perpKey().FilePath(root, w1, verifyPeriod)
		if err := os.Remove(gone); err != nil {
			t.Fatalf("remove: %v", err)
		}
		if err := os.Remove(gone + digestExt); err != nil {
			t.Fatalf("remove sidecar: %v", err)
		}
		rep := verifyOK(t, twoWindowCfg(root, w1))
		hits := findingsFor(rep, FindingManifestMissingFile)
		if len(hits) != 1 {
			t.Fatalf("manifest_missing_file ×%d, want 1: %s", len(hits), dumpFindings(rep))
		}
		if hits[0].Window != w1 || hits[0].Stream != perpKey() {
			t.Errorf("finding points at (%v, %v), want (%v, %v)",
				hits[0].Stream, hits[0].Window, perpKey(), w1)
		}
		if got := rep.ExitClass(); got != ClassIntegrity {
			t.Errorf("ExitClass = %q, want integrity — the archive lost a file it vouched for", got)
		}
	})

	t.Run("a stale entry is caught one wrong field at a time", func(t *testing.T) {
		// One field per case: a fixture that wrongs all four cannot say which
		// comparison exists, and a mutant dropping any single one survives it.
		cases := []struct {
			name   string
			mutate func(*ManifestEntry)
		}{
			{"digest", func(e *ManifestEntry) { e.SHA256 = planted }},
			{"records", func(e *ManifestEntry) { e.Records++ }},
			{"gaps", func(e *ManifestEntry) { e.Gaps++ }},
			{"last_seq", func(e *ManifestEntry) { e.LastSeq++ }},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				root := build(t, w1)
				editManifest(t, root, day, func(m *Manifest) { tc.mutate(&m.Entries[0]) })
				rep := verifyOK(t, builtCfg(root, w1))
				if got := findingKinds(rep); len(got) != 1 || got[0] != FindingManifestMismatch {
					t.Fatalf("findings %v, want only manifest_mismatch: %s", got, dumpFindings(rep))
				}
				if got := rep.ExitClass(); got != ClassIntegrity {
					t.Errorf("ExitClass = %q, want integrity", got)
				}
			})
		}
	})

	t.Run("a file changed after close is reported once, not twice", func(t *testing.T) {
		root := build(t, w1)
		path := perpKey().FilePath(root, w1, verifyPeriod)
		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		if _, err := f.Write([]byte{0xFF}); err != nil {
			t.Fatalf("append: %v", err)
		}
		if err := f.Close(); err != nil {
			t.Fatalf("close: %v", err)
		}
		rep := verifyOK(t, builtCfg(root, w1))
		kinds := findingKinds(rep)
		if len(findingsFor(rep, FindingDigestMismatch)) == 0 {
			t.Errorf("the sidecar mismatch is not reported: %v", kinds)
		}
		if n := len(findingsFor(rep, FindingManifestMismatch)); n != 0 {
			t.Errorf("manifest_mismatch ×%d beside digest_mismatch: one changed file, two findings", n)
		}
	})

	t.Run("a closed file missing from an existing manifest is the crash window", func(t *testing.T) {
		root := build(t, w1, w2)
		removed := ""
		editManifest(t, root, day, func(m *Manifest) {
			removed = m.Entries[0].Path
			m.Entries = m.Entries[1:]
		})
		rep := verifyOK(t, twoWindowCfg(root, w1))
		hits := findingsFor(rep, FindingUnmanifested)
		if len(hits) != 1 {
			t.Fatalf("unmanifested ×%d, want exactly the removed file: %s", len(hits), dumpFindings(rep))
		}
		if !strings.HasSuffix(filepath.ToSlash(hits[0].Path), removed) {
			t.Errorf("finding names %q, the removed entry was %q", hits[0].Path, removed)
		}
		if got := hits[0].Class(); got != ClassCoverage {
			t.Errorf("class %q, want coverage — the file itself is intact and checkable", got)
		}
	})

	t.Run("a day with no manifest earns one finding, not one per file", func(t *testing.T) {
		root := build(t, w1, w2)
		if err := os.RemoveAll(manifestDir(root)); err != nil {
			t.Fatalf("remove manifests: %v", err)
		}
		rep := verifyOK(t, twoWindowCfg(root, w1))
		hits := findingsFor(rep, FindingManifestAbsent)
		if len(hits) != 1 {
			t.Fatalf("manifest_absent ×%d for one day of two files, want 1: %s",
				len(hits), dumpFindings(rep))
		}
		if got := hits[0].Class(); got != ClassCoverage {
			t.Errorf("class %q, want coverage — the expected case is a pre-manifest archive", got)
		}

		// The operational loop closes: backfill, and the finding is gone.
		if _, err := BackfillManifestDay(root, w1, w2.Add(24*time.Hour)); err != nil {
			t.Fatalf("backfill: %v", err)
		}
		rep = verifyOK(t, twoWindowCfg(root, w1))
		manifestKinds := []FindingKind{
			FindingManifestAbsent, FindingUnmanifested, FindingManifestMismatch,
			FindingManifestMissingFile, FindingManifestUnreadable,
		}
		for _, k := range findingKinds(rep) {
			if slices.Contains(manifestKinds, k) {
				t.Errorf("finding %s survives a backfill of an intact tree: %s", k, dumpFindings(rep))
			}
		}
	})

	t.Run("a day with no closed files makes no manifest claim", func(t *testing.T) {
		root := t.TempDir()
		b := newArchive(t, root, perpKey(), w1)
		b.open(0)
		b.data(1)
		b.fillAlive(builtCfg(root, w1))
		b.write() // no Close: crashed, nothing a manifest should list
		rep := verifyOK(t, builtCfg(root, w1))
		if n := len(findingsFor(rep, FindingManifestAbsent)); n != 0 {
			t.Errorf("manifest_absent ×%d for a day with nothing to manifest", n)
		}
	})

	t.Run("an unreadable manifest is integrity, in each of its shapes", func(t *testing.T) {
		cases := []struct {
			name string
			body string
		}{
			{"not json", `{"format": "arbcap-manifest/1",`},
			{"unknown format", `{"format": "arbcap-manifest/9", "day": "` + day + `", "entries": []}`},
			{"day disagrees with its file name", `{"format": "arbcap-manifest/1", "day": "1999-01-01", "entries": []}`},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				root := build(t, w1)
				if err := os.WriteFile(manifestPath(root, day), []byte(tc.body), 0o644); err != nil {
					t.Fatalf("plant manifest: %v", err)
				}
				rep := verifyOK(t, builtCfg(root, w1))
				if n := len(findingsFor(rep, FindingManifestUnreadable)); n != 1 {
					t.Fatalf("manifest_unreadable ×%d, want 1: %s", n, dumpFindings(rep))
				}
				if got := rep.ExitClass(); got != ClassIntegrity {
					t.Errorf("ExitClass = %q, want integrity — the completeness check for the day did not run", got)
				}
			})
		}
	})
}

// TestVerifyProvesACopyCompleteByManifest is the slice's done-when as a test
// (§38e): the manifest travels inside the tree, so the copy is verified
// without asking the source — and the positive control shows the completeness
// claim BITES, which a clean run alone cannot show.
func TestVerifyProvesACopyCompleteByManifest(t *testing.T) {
	w1, w2 := fixtureWindow(t, 1), fixtureWindow(t, 2)
	src := t.TempDir()
	seq := uint64(0)
	for _, w := range []time.Time{w1, w2} {
		b := newArchive(t, src, perpKey(), w)
		b.open(seq)
		for range 3 {
			seq++
			b.data(seq)
		}
		b.fillAlive(builtCfg(src, w))
		b.seal()
		b.write()
	}

	copy1 := t.TempDir()
	copyTree(t, src, copy1)
	rep := verifyOK(t, twoWindowCfg(copy1, w1))
	if got := rep.ExitClass(); got != ClassUnspecified {
		t.Fatalf("a faithful copy verifies %q, want clean: %s", got, dumpFindings(rep))
	}

	// The partial copy: one whole closed hour absent, sidecar and all. Without
	// the manifest nothing on the copy's disk contradicts anything.
	copy2 := t.TempDir()
	copyTree(t, src, copy2)
	gone := perpKey().FilePath(copy2, w2, verifyPeriod)
	if err := os.Remove(gone); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if err := os.Remove(gone + digestExt); err != nil {
		t.Fatalf("remove sidecar: %v", err)
	}
	rep = verifyOK(t, twoWindowCfg(copy2, w1))
	if n := len(findingsFor(rep, FindingManifestMissingFile)); n != 1 {
		t.Fatalf("manifest_missing_file ×%d on a partial copy, want 1: %s", n, dumpFindings(rep))
	}
	if got := rep.ExitClass(); got != ClassIntegrity {
		t.Errorf("a partial copy verifies %q, want integrity", got)
	}
}
