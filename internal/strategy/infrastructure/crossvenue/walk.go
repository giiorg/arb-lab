package crossvenue

// The gate's archive walker (§45b): a manifest-gated statistical walk in
// family2's shape, deliberately NOT that file — family2's H1/H2 guards and
// Binance-only roster are v1's frozen procedure identity, and the gate has
// its own guards (the post-cutoff seal instead of H1). What is shared is
// the capture package's decode API and the file-granular manifest rule.
//
// Torn-tail policy follows family2, not WalkRecords' recovery default: the
// walker consumes manifested days only, refuses an unmanifested file, and
// treats a torn manifested file as ErrArchiveCorrupt — gate evidence is
// vouched-complete files, and measuring around damage silently thins the
// sample.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/capture"
)

// Walker sentinels.
var (
	// ErrArchiveCorrupt reports a manifested file that does not decode end
	// to end. A closed, vouched file is a complete record chain by §32's
	// contract; a torn one here is integrity damage.
	ErrArchiveCorrupt = fmt.Errorf("strategy/crossvenue: manifested archive file does not decode")

	// ErrManifestMalformed reports a day manifest that does not parse as
	// capture's own schema.
	ErrManifestMalformed = fmt.Errorf("strategy/crossvenue: day manifest does not parse")

	// ErrNoManifestDays reports an empty walk universe: nothing manifested
	// strictly before the bound. An empty enumeration must refuse rather
	// than let a wrong -root read as a market with no evidence.
	ErrNoManifestDays = fmt.Errorf("strategy/crossvenue: no manifested archive day precedes the bound")

	// ErrDayUnmanifested reports an archive file the day's manifest does
	// not vouch for — the file-granular H2 rule.
	ErrDayUnmanifested = fmt.Errorf("strategy/crossvenue: archive file not vouched by its day manifest")
)

// ManifestDay is one walkable archive day: its manifest digest (the
// identifier a procedure artifact enumerates) and the root-relative paths
// that manifest vouches for.
type ManifestDay struct {
	Day    string
	SHA256 string

	vouched map[string]bool
}

// Vouches reports whether the day's manifest lists the root-relative path.
func (d ManifestDay) Vouches(rel string) bool { return d.vouched[rel] }

// LoadManifestDays enumerates every day manifest under the archive root
// whose day is STRICTLY BEFORE the bound's UTC day, sorted ascending. The
// strict day bound is family2's determinism rule: the bound day's own
// manifest is still gaining hours while capture runs, so two invocations
// hours apart would read two different universes. The sim passes the
// §43b books cutoff here, which is what makes the post-cutoff seal
// structural rather than remembered (§45f-2).
func LoadManifestDays(root string, before time.Time) ([]ManifestDay, error) {
	if before.IsZero() {
		return nil, fmt.Errorf("strategy/crossvenue: manifest bound instant unset")
	}
	boundDay := before.UTC().Format("2006-01-02")
	dir := filepath.Join(root, "v1", "_manifest")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("manifest dir: %w", err)
	}
	var days []ManifestDay
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}
		day := strings.TrimSuffix(name, ".json")
		if day >= boundDay {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("manifest %s: %w", day, err)
		}
		var m capture.Manifest
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("manifest %s: %w: %w", day, ErrManifestMalformed, err)
		}
		if m.Day != day {
			return nil, fmt.Errorf("manifest file %s claims day %s: %w", day, m.Day, ErrManifestMalformed)
		}
		vouched := make(map[string]bool, len(m.Entries))
		for _, entry := range m.Entries {
			vouched[entry.Path] = true
		}
		sum := sha256.Sum256(data)
		days = append(days, ManifestDay{Day: day, SHA256: hex.EncodeToString(sum[:]), vouched: vouched})
	}
	if len(days) == 0 {
		return nil, fmt.Errorf("before %s: %w", boundDay, ErrNoManifestDays)
	}
	sort.Slice(days, func(i, j int) bool { return days[i].Day < days[j].Day })
	return days, nil
}

// scanStream walks one stream's hour files across the manifested days in
// path order, decoding every record and handing it to fn. A stream
// directory absent for a day is coverage, not integrity — the verify layer
// owns that ledger — so it is skipped; a file the day's manifest does not
// vouch for, or a vouched file that fails to decode, refuses by name.
//
// onFile, when non-nil, fires before each file's first record with the
// boundary's continuity verdict. cont is true iff the file's first record
// is an Open whose non-zero PrevLastSeq equals the previous file's last
// observed CaptureSeq — a living writer's rotation (§45n-1), the one file
// boundary the framing records prove is NOT a hole, so a stateful
// consumer (the ticker state, the book accumulator) may carry state
// across it. Everything else is cold (cont=false): the walk's first
// file, a restart's zero claim, a sequence mismatch, or a first record
// that is not an Open. The verdict is computed from the archive's own
// evidence, never from file adjacency — an absent hour between two
// files fails the sequence check by itself. Mid-file discontinuities
// stay the consumer's job: every one is a Gap record, which consumers
// reset on (§39p-2's rule, narrowed by §45n-1 to the boundaries that
// are actually holes).
func scanStream(root string, key capture.StreamKey, days []ManifestDay, onFile func(cont bool), fn func(capture.Record) error) error {
	dir := key.Dir(root)
	var lastSeq uint64
	for _, d := range days {
		dayDir := filepath.Join(dir, d.Day[0:4], d.Day[5:7], d.Day[8:10])
		entries, err := os.ReadDir(dayDir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("day %s: %w", d.Day, err)
		}
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".arbcap") {
				names = append(names, e.Name())
			}
		}
		sort.Strings(names)
		for _, name := range names {
			full := filepath.Join(dayDir, name)
			rel, err := filepath.Rel(root, full)
			if err != nil {
				return fmt.Errorf("file %s: %w", full, err)
			}
			rel = filepath.ToSlash(rel)
			if !d.Vouches(rel) {
				return fmt.Errorf("file %s: %w", rel, ErrDayUnmanifested)
			}
			first := true
			err = scanFile(full, func(rec capture.Record) error {
				if first {
					first = false
					cont := false
					if rec.Kind == capture.RecordKindOpen {
						prev, perr := capture.OpenPrevLastSeq(rec.Payload)
						if perr != nil {
							return fmt.Errorf("%s: %w: %w", rel, ErrArchiveCorrupt, perr)
						}
						// Zero is the writer's documented "no predecessor"
						// sentinel — a fresh process cannot know where the
						// previous hour ended — and on the walk's first file
						// (lastSeq still 0) the guard is also what keeps an
						// explicit zero claim from vacuously matching.
						cont = prev != 0 && prev == lastSeq
					}
					if onFile != nil {
						onFile(cont)
					}
				}
				lastSeq = rec.CaptureSeq
				return fn(rec)
			})
			if err != nil {
				return err
			}
			if first && onFile != nil {
				// A vouched file with zero records: signal it cold so no
				// consumer carries state across an anomalous boundary.
				onFile(false)
			}
		}
	}
	return nil
}

// scanFile decodes one archive file end to end, refusing a torn tail.
func scanFile(path string, fn func(capture.Record) error) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("archive file: %w", err)
	}
	off, err := capture.ParseFileHeader(data)
	if err != nil {
		return fmt.Errorf("%s: %w: %w", path, ErrArchiveCorrupt, err)
	}
	rest := data[off:]
	for len(rest) > 0 {
		rec, n, err := capture.DecodeRecord(rest)
		if err != nil {
			return fmt.Errorf("%s at offset %d: %w: %w", path, len(data)-len(rest), ErrArchiveCorrupt, err)
		}
		rest = rest[n:]
		if err := fn(rec); err != nil {
			return err
		}
	}
	return nil
}
