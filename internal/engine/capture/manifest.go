package capture

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// §32e's per-day manifest, slice F (§38a):
//
//	<root>/v1/_manifest/2026-07-30.json — every CLOSED file of that day, with
//	its digest, record count and gap count.
//
// The entry is written by the writer inside the close path, after the sidecar
// and only then — close-time is when the digest and the counts are facts being
// created rather than facts being re-derived. That ordering is what makes the
// crash-window artifact one-sided: sidecar present + entry absent is the
// expected shape of a crash between the two writes (repairable by a LABELED
// backfill, §38b); entry present + sidecar absent would mean the outer claim
// was written before the inner one existed, and this file refuses to produce
// that state.
//
// What the manifest buys over the sidecars it summarizes: the entry OUTLIVES
// the file. A deleted archive file takes its sidecar with it and nothing on
// disk contradicts anything — the copy verifies clean while missing an hour.
// The manifest is the piece that turns "everything present is intact" into
// "everything is present" (§38e), which is why it lives INSIDE the version
// root with root-relative paths: it travels with every copy and testifies
// against it.

const (
	// manifestDirName sits directly under the version dir, beside the venue
	// trees. `_` is in path.go's unreserved set, so this name is NOT protected
	// by the leading-dot escape the way `.writer.lock` is — what protects it is
	// position: a venue named "_manifest" would collide, and ids.Venue's own
	// validation plus the two venues this project trades make that a theoretical
	// rather than a realistic accident. discover() treats the subtree as
	// format-owned, not as stray files.
	manifestDirName = "_manifest"

	// manifestLockName serializes read-modify-write of the day files. One lock
	// for the whole directory rather than per day: contention is eight writers
	// rotating at the same hour boundary, the critical section is a
	// milliseconds-long local read+rename, and a per-day lock would litter the
	// directory with one dotfile per day forever.
	manifestLockName = ".manifest.lock"

	// manifestFormat is the file's self-description. Refused on read when
	// unknown, for ErrFormatVersion's reason: a later format may mean something
	// this build would misread, and "unreadable version" beats a plausible
	// misreading.
	manifestFormat = "arbcap-manifest/1"

	// manifestDayLayout names a day file: 2026-07-30.json.
	manifestDayLayout = "2006-01-02"
)

// ManifestSource says how an entry came to exist, and it exists so a repair can
// never impersonate the real thing (§38b). It crosses a persistence boundary,
// so it takes §3.6 rule 8's string shape.
type ManifestSource string

const (
	// ManifestSourceUnspecified is the zero value and names no provenance.
	ManifestSourceUnspecified ManifestSource = ""

	// ManifestSourceClose is an entry written by the writer at the moment the
	// file closed — the digest and counts are the close's own facts.
	ManifestSourceClose ManifestSource = "close"

	// ManifestSourceBackfill is an entry derived later from the sidecar and the
	// file's own records (§38b). The digest is still close-time truth — it is
	// COPIED from the sidecar, never recomputed from a re-read — but the entry
	// was not there when the file closed, and it says so forever.
	ManifestSourceBackfill ManifestSource = "backfill"
)

// Validate reports whether s is a declared source, excluding the zero value.
func (s ManifestSource) Validate() error {
	switch s {
	case ManifestSourceUnspecified:
		return ErrManifestSourceUnspecified
	case ManifestSourceClose, ManifestSourceBackfill:
		return nil
	}
	return fmt.Errorf("source %q: %w", string(s), ErrManifestSourceUnknown)
}

// String implements fmt.Stringer.
func (s ManifestSource) String() string { return string(s) }

// ManifestEntry is one closed file's row: §32e's {digest, record count, gap
// count}, plus the fields that make the row checkable without the file
// (window, last_seq) and honest about itself (closed_at, source) — §38a.
type ManifestEntry struct {
	// Path is relative to the archive root, slash-separated, because the
	// manifest travels with the copy and an absolute path is a fact about one
	// host.
	Path string `json:"path"`

	// Window is the file's rotation window start, RFC3339 UTC. The entry's DAY
	// is derived from it — not from ClosedAt, because the 23:00 window's close
	// legitimately happens in the next day and the manifest answers "what does
	// day D's archive hold", not "what closed during day D".
	Window string `json:"window"`

	// SHA256 is the sidecar's value: hex over the complete file, Close record
	// included.
	SHA256 string `json:"sha256"`

	// Records counts every record in the file, Close included — what a full
	// walk of the file finds, which is what a checker will compare against.
	Records uint64 `json:"records"`

	// Gaps counts the file's Gap records.
	Gaps uint64 `json:"gaps"`

	// LastSeq is the last Data CaptureSeq in the file; zero for a file that
	// captured no frames.
	LastSeq uint64 `json:"last_seq"`

	// ClosedAt is when the Close record was written (RFC3339Nano, from the
	// injected clock) — for a close entry. A backfill entry carries the
	// backfill instant, which is the honest value: the close instant was a fact
	// the crash destroyed.
	ClosedAt string `json:"closed_at"`

	Source ManifestSource `json:"source"`
}

// Validate reports whether the entry could describe a closed file. The zero
// value fails, per the platform rule: an unpopulated entry would read as a
// zero-record file with no digest, upserted as fact.
func (e ManifestEntry) Validate() error {
	if e.Path == "" {
		return ErrManifestPathUnset
	}
	if filepath.IsAbs(e.Path) || strings.Contains(e.Path, `\`) {
		return fmt.Errorf("%q: %w", e.Path, ErrManifestPathShape)
	}
	if _, err := e.day(); err != nil {
		return err
	}
	if len(e.SHA256) != 64 {
		return fmt.Errorf("%d hex digits, want 64: %w", len(e.SHA256), ErrManifestDigestShape)
	}
	if _, err := hex.DecodeString(e.SHA256); err != nil {
		return fmt.Errorf("%q: %w", e.SHA256, ErrManifestDigestShape)
	}
	// A cleanly closed file holds at least its Open and Close records, so zero
	// is an unpopulated field rather than a small file.
	if e.Records == 0 {
		return ErrManifestRecordsZero
	}
	if e.ClosedAt == "" {
		return ErrManifestClosedAtUnset
	}
	if _, err := time.Parse(time.RFC3339Nano, e.ClosedAt); err != nil {
		return fmt.Errorf("closed_at %q: %w: %w", e.ClosedAt, ErrManifestTimeShape, err)
	}
	return e.Source.Validate()
}

// day is the manifest day the entry belongs to, derived from its window.
func (e ManifestEntry) day() (string, error) {
	w, err := time.Parse(time.RFC3339, e.Window)
	if err != nil {
		return "", fmt.Errorf("window %q: %w: %w", e.Window, ErrManifestTimeShape, err)
	}
	return w.UTC().Format(manifestDayLayout), nil
}

// Manifest is one day's file.
type Manifest struct {
	Format  string          `json:"format"`
	Day     string          `json:"day"`
	Entries []ManifestEntry `json:"entries"`
}

// entry returns the index of the entry for path, or -1.
func (m *Manifest) entry(path string) int {
	return slices.IndexFunc(m.Entries, func(e ManifestEntry) bool { return e.Path == path })
}

func manifestDir(root string) string {
	return filepath.Join(root, versionDir(), manifestDirName)
}

func manifestPath(root, day string) string {
	return filepath.Join(manifestDir(root), day+".json")
}

// loadManifest reads one day's manifest. A missing file is (nil, nil) — absence
// is a fact the caller classifies, not an error. An unparseable or wrong-format
// file is ErrManifestCorrupt: the caller must not read through it, and nothing
// here repairs it, because overwriting it would destroy the evidence verify
// exists to report (§38c's manifest_unreadable).
func loadManifest(root, day string) (*Manifest, error) {
	body, err := os.ReadFile(manifestPath(root, day))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("capture: read manifest: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, fmt.Errorf("capture: manifest %s: %w: %w", day, ErrManifestCorrupt, err)
	}
	if m.Format != manifestFormat {
		return nil, fmt.Errorf("capture: manifest %s declares format %q, this build reads %q: %w",
			day, m.Format, manifestFormat, ErrManifestCorrupt)
	}
	if m.Day != day {
		return nil, fmt.Errorf("capture: manifest file %s declares day %q: %w",
			day, m.Day, ErrManifestCorrupt)
	}
	return &m, nil
}

// upsertManifestEntry lands e in its day's manifest, replacing any existing
// entry for the same path. Used by the writer's close path, where replacement
// is correct: a continued hour file (restart, append, re-close) has a new
// digest, and the latest close is the whole-file truth.
func upsertManifestEntry(root string, e ManifestEntry) error {
	return updateManifest(root, e, func(m *Manifest) bool {
		if i := m.entry(e.Path); i >= 0 {
			m.Entries[i] = e
			return true
		}
		m.Entries = append(m.Entries, e)
		return true
	})
}

// insertManifestEntryIfAbsent lands e only when no entry for its path exists —
// the backfill rule (§38b): a repair must never overwrite close-time truth.
// Reports whether it inserted.
func insertManifestEntryIfAbsent(root string, e ManifestEntry) (bool, error) {
	inserted := false
	err := updateManifest(root, e, func(m *Manifest) bool {
		if m.entry(e.Path) >= 0 {
			return false
		}
		m.Entries = append(m.Entries, e)
		inserted = true
		return true
	})
	return inserted, err
}

// updateManifest is the locked read-modify-write both mutations share.
//
// The lock is a BLOCKING flock, and blocking is the right direction here where
// the stream lock's is non-blocking: that one refuses a second long-lived
// writer, where waiting would be the silent hang §31g rule 4 forbids; this one
// serializes eight writers that all rotate at the same hour boundary through a
// milliseconds-long critical section, where refusing would drop a manifest
// entry to avoid a wait nobody would notice. A holder that dies is released by
// the kernel; a holder that hangs is a disk that has hung, and every writer is
// already stuck behind the same disk.
//
// The write is temp-file + fsync + rename, so a crash leaves either the old
// manifest or the new one, never a torn half — a torn manifest would be
// ErrManifestCorrupt forever after, manufactured by the machinery that exists
// to prevent that class of loss.
func updateManifest(root string, e ManifestEntry, mutate func(*Manifest) bool) error {
	if err := e.Validate(); err != nil {
		return fmt.Errorf("capture: manifest entry for %s: %w", e.Path, err)
	}
	day, err := e.day()
	if err != nil {
		return err
	}
	dir := manifestDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("capture: create manifest directory: %w", err)
	}
	lockFile, err := os.OpenFile(filepath.Join(dir, manifestLockName), os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return fmt.Errorf("capture: open manifest lock: %w", err)
	}
	defer func() { _ = lockFile.Close() }()
	if err := flockExclusiveBlocking(lockFile); err != nil {
		return fmt.Errorf("capture: lock manifest: %w", err)
	}
	defer func() { _ = funlockFile(lockFile) }()

	m, err := loadManifest(root, day)
	if err != nil {
		return err
	}
	if m == nil {
		m = &Manifest{Format: manifestFormat, Day: day}
	}
	if !mutate(m) {
		return nil
	}
	slices.SortFunc(m.Entries, func(a, b ManifestEntry) int {
		return strings.Compare(a.Path, b.Path)
	})

	body, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("capture: encode manifest %s: %w", day, err)
	}
	tmp := filepath.Join(dir, fmt.Sprintf(".%s.json.tmp.%d", day, os.Getpid()))
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("capture: create manifest temp file: %w", err)
	}
	_, writeErr := f.Write(append(body, '\n'))
	syncErr := f.Sync()
	closeErr := f.Close()
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("capture: write manifest %s: %w", day, err)
	}
	if err := os.Rename(tmp, manifestPath(root, day)); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("capture: land manifest %s: %w", day, err)
	}
	return nil
}

// BackfillManifestDay derives entries for every closed file of one UTC day
// that has a sidecar and no manifest entry, and inserts them labeled
// ManifestSourceBackfill. It reports how many entries it added.
//
// Two standing jobs (§38b): the transition — every file closed before the
// manifest layer existed — and §38a's crash window. It refuses nothing it
// merely skips: a file without a sidecar has no close-time truth to copy, so
// it is left for verify to report rather than manufactured into an entry.
//
// The digest comes from the SIDECAR, never from re-hashing the file — bit rot
// between close and backfill must fail the check, not be hashed into the
// authority. The counts come from a walk of the file's own records, which is
// safe for the same reason: records are CRC-covered close-time statements, and
// a walk that hits corruption aborts the entry.
func BackfillManifestDay(root string, day time.Time, now time.Time) (int, error) {
	dayStr := day.UTC().Format(manifestDayLayout)
	existing, err := loadManifest(root, dayStr)
	if err != nil {
		return 0, err
	}
	manifested := make(map[string]bool)
	if existing != nil {
		for _, e := range existing.Entries {
			manifested[e.Path] = true
		}
	}

	added := 0
	walkErr := filepath.WalkDir(filepath.Join(root, versionDir()), func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == manifestDirName {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, archiveExt) {
			return nil
		}
		_, window, err := ParseFilePath(root, p)
		if err != nil {
			return nil // not an archive file; verify's stray-file pass owns it
		}
		if window.UTC().Format(manifestDayLayout) != dayStr {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		relSlash := filepath.ToSlash(rel)
		if manifested[relSlash] {
			return nil
		}

		sidecarDigest, err := readSidecarDigest(p)
		if err != nil {
			return nil // no sidecar, or an unreadable one: verify's finding, not ours
		}
		counts, err := walkFileCounts(p)
		if err != nil || !counts.sawClose {
			return nil // crashed or corrupt: no close-time truth to backfill
		}
		inserted, err := insertManifestEntryIfAbsent(root, ManifestEntry{
			Path:     relSlash,
			Window:   window.Format(time.RFC3339),
			SHA256:   sidecarDigest,
			Records:  counts.records,
			Gaps:     counts.gaps,
			LastSeq:  counts.lastSeq,
			ClosedAt: now.UTC().Format(time.RFC3339Nano),
			Source:   ManifestSourceBackfill,
		})
		if err != nil {
			return err
		}
		if inserted {
			added++
		}
		return nil
	})
	return added, walkErr
}

// readSidecarDigest reads the hex digest from a file's `.sha256` sidecar,
// which writer.go writes in sha256sum's own format: "<hex>  <basename>\n".
func readSidecarDigest(archivePath string) (string, error) {
	body, err := os.ReadFile(archivePath + digestExt)
	if err != nil {
		return "", err
	}
	digest, _, ok := strings.Cut(strings.TrimSpace(string(body)), " ")
	if !ok || len(digest) != 64 {
		return "", fmt.Errorf("capture: sidecar %s%s is not sha256sum format", archivePath, digestExt)
	}
	if _, err := hex.DecodeString(digest); err != nil {
		return "", fmt.Errorf("capture: sidecar %s%s: %w", archivePath, digestExt, err)
	}
	return digest, nil
}

// fileCounts is what one full walk of a closed file's records reports.
type fileCounts struct {
	records  uint64
	gaps     uint64
	lastSeq  uint64
	sawClose bool
}

// walkFileCounts decodes every record in an archive file. Any refusal aborts:
// a backfill entry derived from a partial walk would claim counts for a file
// it did not finish reading.
func walkFileCounts(p string) (fileCounts, error) {
	f, err := os.Open(p)
	if err != nil {
		return fileCounts{}, err
	}
	defer func() { _ = f.Close() }()
	res, err := scanArchive(f)
	if err != nil {
		return fileCounts{}, err
	}
	if res.tornBytes > 0 {
		return fileCounts{}, fmt.Errorf("capture: %s has a torn tail", p)
	}
	return fileCounts{
		records:  res.records,
		gaps:     res.gaps,
		lastSeq:  res.lastSeq,
		sawClose: res.hadClose,
	}, nil
}
