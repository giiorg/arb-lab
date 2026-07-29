package capture

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// errorIs is errors.Is with a name short enough to read inside a table.
func errorIs(err, target error) bool { return errors.Is(err, target) }

// fakeClock is slice B's TimeSource double — §32m says the writer is "driven by
// a FAKE source under TestClock", and this is the clock half of that.
//
// It models the three readings SEPARATELY rather than deriving them from one
// field, which is the whole reason the port has three methods. In particular
// Mono keeps advancing across a backstep: a double that derived Mono from Now
// would satisfy the interface, pass every test, and hide the exact defect §32i
// exists about — a run of records whose RecvTimes are identical because the
// clamp engaged, with nothing in the archive saying so.
//
// Concurrency-safe because the queue tests run a producer goroutine against it
// under -race.
type fakeClock struct {
	mu        sync.Mutex
	now       time.Time
	mono      time.Duration
	backCount uint64
	backTotal time.Duration
}

func newFakeClock(start time.Time) *fakeClock { return &fakeClock{now: start} }

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Mono() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.mono
}

func (c *fakeClock) Backsteps() (uint64, time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.backCount, c.backTotal
}

// Advance moves both readings forward, which is the ordinary case.
func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
	c.mono += d
}

// Backstep models what platform/clock.Wall does with a backwards system-clock
// step: it absorbs it, so Now does NOT move, and reports it through Backsteps.
// Mono advances because real time passed.
func (c *fakeClock) Backstep(d time.Duration, elapsed time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.backCount++
	c.backTotal += d
	c.mono += elapsed
}

// testConfig is a writer config with every field populated and a temp root.
//
// The two intervals are deliberately COPRIME with the rotation period — §32l's
// table names `alive_interval == rotation period` as a collapse, because a tick
// that only ever fires at a boundary makes "wrote a tick" and "rotated"
// indistinguishable. 7 minutes into a 60-minute hour gives 8 ticks per window,
// none of them on the boundary.
func testConfig(t *testing.T, key StreamKey, c *fakeClock) WriterConfig {
	t.Helper()
	return WriterConfig{
		Root:           t.TempDir(),
		Key:            key,
		Clock:          c,
		Provenance:     Provenance{Tool: "capture-test", GitSHA: "deadbeef", ConfigHash: "cafe"},
		RotationPeriod: time.Hour,
		AliveInterval:  7 * time.Minute,
		SyncInterval:   5 * time.Second,
	}
}

// dataRecord builds a Data record the way a reader would stamp it (§32d).
func dataRecord(seq uint64, c *fakeClock, payload string) Record {
	return Record{
		Kind:       RecordKindData,
		CaptureSeq: seq,
		RecvTime:   c.Now(),
		Mono:       c.Mono(),
		Payload:    []byte(payload),
	}
}

// readAllRecords decodes an entire archive file, so an assertion can be made
// about what is ON DISK rather than about what the writer believes it wrote.
//
// That distinction is the reason this helper exists rather than a Stats check: a
// writer whose counters were correct and whose bytes never reached the file
// would pass every Stats assertion in this package.
func readAllRecords(t *testing.T, path string) []Record {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	n, err := ParseFileHeader(body)
	if err != nil {
		t.Fatalf("%s: file header: %v", path, err)
	}
	var out []Record
	for rest := body[n:]; len(rest) > 0; {
		rec, used, err := DecodeRecord(rest)
		if err != nil {
			t.Fatalf("%s: record %d at offset %d: %v", path, len(out), len(body)-len(rest), err)
		}
		out = append(out, rec)
		rest = rest[used:]
	}
	return out
}

// kindsOf reduces a record list to its kinds, which is what most structural
// assertions are actually about.
func kindsOf(recs []Record) []RecordKind {
	out := make([]RecordKind, len(recs))
	for i, r := range recs {
		out[i] = r.Kind
	}
	return out
}

// countKind counts records of one kind.
func countKind(recs []Record, k RecordKind) int {
	n := 0
	for _, r := range recs {
		if r.Kind == k {
			n++
		}
	}
	return n
}

// sha256Hex is the digest of a byte slice, written independently of the writer's
// own rolling hash so the two can disagree.
//
// The writer feeds a streaming hash.Hash as it writes; this hashes a finished
// buffer in one call. A test that recomputed the digest through the writer's own
// path would confirm the code agrees with itself.
func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// frameLenOfLastRecord reports the byte length of the final record in a complete
// archive file, by walking it from the start.
//
// Walking rather than reading the trailing length field: the frame's length sits
// at its FRONT, so there is no way to find the last record from the end of the
// file, and a helper that pretended otherwise would be asserting a format
// property this format does not have (§33c).
func frameLenOfLastRecord(body []byte) (int, error) {
	n, err := ParseFileHeader(body)
	if err != nil {
		return 0, err
	}
	last := 0
	for rest := body[n:]; len(rest) > 0; {
		_, used, err := DecodeRecord(rest)
		if err != nil {
			return 0, err
		}
		last = used
		rest = rest[used:]
	}
	if last == 0 {
		return 0, errors.New("the file holds no records")
	}
	return last, nil
}

// archiveFiles lists every .arbcap file under a root, sorted, so a rotation test
// can assert how many windows produced a file — including the windows that
// produced one with no Data in it at all.
func archiveFiles(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && filepath.Ext(p) == archiveExt {
			out = append(out, p)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return out
}
