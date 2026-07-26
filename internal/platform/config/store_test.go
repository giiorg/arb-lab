package config_test

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/config"
)

func TestNewServesRevisionOne(t *testing.T) {
	s := mustStore(t)
	snap := s.Current()

	if err := snap.Rev.Validate(); err != nil {
		t.Fatalf("the first revision does not validate: %v", err)
	}
	if snap.Rev.Num != 1 {
		t.Errorf("Rev.Num = %d, want 1 (numbering from 1 is what makes the zero Revision invalid)", snap.Rev.Num)
	}
	if snap.Rev.Source != "configs/test.json" {
		t.Errorf("Rev.Source = %q", snap.Rev.Source)
	}
	if !snap.Rev.LoadedAt.Equal(testAt) {
		t.Errorf("Rev.LoadedAt = %v, want the instant supplied", snap.Rev.LoadedAt)
	}
}

func TestNewRejectsMissingProvenance(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		at      time.Time
		wantErr error
	}{
		{"no source name", "", testAt, config.ErrSourceUnset},
		{"the zero instant", "configs/test.json", time.Time{}, config.ErrInstantUnset},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, err := config.New[testConfig](tc.source, tc.at, []byte(baseDoc))
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("New = %v, want %v", err, tc.wantErr)
			}
			if s != nil {
				t.Error("New returned a Store alongside an error; there is no partial success")
			}
		})
	}
}

// TestReloadNumbersRevisionsDensely checks the replay coordinate. §3.5 replays
// "the configuration in force at revision N", which is only answerable if the
// numbers are dense and ordered.
func TestReloadNumbersRevisionsDensely(t *testing.T) {
	s := mustStore(t)

	for i, open := range []string{"4", "5", "6"} {
		ch, err := s.Reload("configs/test.json", testAt, []byte(withField(fMaxOpen, open)))
		if err != nil {
			t.Fatalf("Reload %d = %v", i, err)
		}
		want := uint64(i + 2)
		if ch.Next.Num != want {
			t.Errorf("Reload %d minted rev %d, want %d", i, ch.Next.Num, want)
		}
		if ch.Prev.Num != want-1 {
			t.Errorf("Reload %d reports Prev %d, want %d", i, ch.Prev.Num, want-1)
		}
		if got := s.Current().Rev.Num; got != want {
			t.Errorf("Current().Rev.Num = %d, want %d", got, want)
		}
	}
}

// TestReloadOfIdenticalBytesIsANoOp is the file-watcher case. An editor's
// write-then-rename, an inotify event pair, a poller re-reading on a timer: all
// deliver the same bytes twice, and spending a revision number on each would
// make the replay log claim the operator acted when they did not.
func TestReloadOfIdenticalBytesIsANoOp(t *testing.T) {
	s := mustStore(t)
	before := s.Current()

	for range 3 {
		ch, err := s.Reload("configs/test.json", testAt.Add(time.Hour), []byte(baseDoc))
		if err != nil {
			t.Fatalf("Reload = %v, want nil", err)
		}
		if ch.Applied {
			t.Error("Change.Applied = true for byte-identical input")
		}
		if ch.Prev != ch.Next {
			t.Errorf("a no-op reload moved the revision: %v -> %v", ch.Prev, ch.Next)
		}
		if len(ch.Fields) != 0 {
			t.Errorf("Change.Fields = %v, want empty", ch.Fields)
		}
	}

	if s.Current().Rev != before.Rev {
		t.Errorf("revision moved across three no-op reloads: %v -> %v", before.Rev, s.Current().Rev)
	}
}

// TestReloadOfReformattedBytesIsANewRevision is the deliberate other half of the
// rule above, and the two together are what makes the hash the identity.
//
// The document below decodes to exactly the same configuration — only whitespace
// and key order differ — and it still mints a revision, with no fields changed.
// That is the honest record: somebody edited the file. Suppressing it would mean
// the audit trail decided on the operator's behalf that their edit did not
// count.
func TestReloadOfReformattedBytesIsANewRevision(t *testing.T) {
	s := mustStore(t)

	reformatted := strings.ReplaceAll(baseDoc, "\n  ", "\n    ") + "\n"
	if reformatted == baseDoc {
		t.Fatal("fixture edit did not apply")
	}

	ch, err := s.Reload("configs/test.json", testAt, []byte(reformatted))
	if err != nil {
		t.Fatalf("Reload = %v, want nil", err)
	}
	if !ch.Applied {
		t.Error("Change.Applied = false; a changed file is a new revision even when it decodes identically")
	}
	if ch.Next.Num != 2 {
		t.Errorf("Rev.Num = %d, want 2", ch.Next.Num)
	}
	if len(ch.Fields) != 0 {
		t.Errorf("Change.Fields = %v, want empty: nothing about the configuration moved", ch.Fields)
	}
	if ch.Next.Hash == ch.Prev.Hash {
		t.Error("the hash did not change for different bytes")
	}
}

// TestAFailedReloadLeavesThePreviousConfigurationInForce is the safety property
// hot-reload exists to keep. Whatever the reason for the refusal, the process
// must never be briefly serving a configuration that failed its own checks.
//
// Each row fails at a DIFFERENT step of Reload, which is the point: a check that
// happened to swap before validating would pass the first row and fail the last.
func TestAFailedReloadLeavesThePreviousConfigurationInForce(t *testing.T) {
	tests := []struct {
		name string
		doc  string
	}{
		{"malformed JSON", `{"journal": `},
		{"an empty document", ""},
		{"a duplicate key", strings.Replace(baseDoc, `"max_total_open": 3,`, `"max_total_open": 3,"max_total_open": 9,`, 1)},
		{"trailing data", baseDoc + "{}"},
		{"an unknown key", strings.Replace(baseDoc, `"journal"`, `"journal_path"`, 1)},
		{"a schema Validate failure", withField(fMaxOpen, "0")},
		{"a policy violation", withField(fJournal, `"/elsewhere.db"`)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := mustStore(t)
			before := s.Current()

			ch, err := s.Reload("configs/test.json", testAt.Add(time.Hour), []byte(tc.doc))
			if err == nil {
				t.Fatalf("Reload accepted a document it should refuse; Change = %+v", ch)
			}
			if ch.Applied || ch.Next.Num != 0 {
				t.Errorf("a failed Reload returned a non-zero Change: %+v", ch)
			}

			after := s.Current()
			if after.Rev != before.Rev {
				t.Errorf("revision moved on a failed reload: %v -> %v", before.Rev, after.Rev)
			}
			if after.Value.Journal != before.Value.Journal ||
				after.Value.Risk.MaxTotalOpen != before.Value.Risk.MaxTotalOpen {
				t.Errorf("configuration moved on a failed reload: %+v -> %+v", before.Value, after.Value)
			}
		})
	}
}

// TestConcurrentReadsAndReloads runs under -race, which is what the whole test
// is for: Current is an atomic load precisely so risk.Gate can call it per
// decision (§6.3) without queueing behind an operator's editor.
//
// Every reader also asserts INTERNAL CONSISTENCY, not just absence of a race. A
// snapshot must never pair one revision's number with another's value, which is
// what a non-atomic two-field read would eventually produce and what a plain
// race check would miss on a run where the interleaving did not happen to fault.
func TestConcurrentReadsAndReloads(t *testing.T) {
	s := mustStore(t)

	const (
		readers = 8
		reloads = 200
	)

	stop := make(chan struct{})
	var wg sync.WaitGroup

	// Distinct revisions each reader observed. Counted rather than assumed:
	// eight readers that all happened to finish before the first reload would
	// produce an identical green run while checking nothing about concurrency,
	// and a clean run and a vacuous run look the same from the outside.
	seen := make([]map[uint64]bool, readers)

	for r := range readers {
		seen[r] = make(map[uint64]bool)
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				snap := s.Current()
				seen[r][snap.Rev.Num] = true
				if err := snap.Rev.Validate(); err != nil {
					t.Errorf("a reader observed an invalid revision: %v", err)
					return
				}
				// max_total_open is written as rev+2 by the loop below, so the
				// value and the revision that carries it must agree.
				if want := int(snap.Rev.Num) + 2; snap.Value.Risk.MaxTotalOpen != want {
					t.Errorf("torn snapshot: rev %d carries max_total_open %d, want %d",
						snap.Rev.Num, snap.Value.Risk.MaxTotalOpen, want)
					return
				}
			}
		}()
	}

	// Revision 1 must satisfy the reader's invariant too.
	if s.Current().Value.Risk.MaxTotalOpen != 3 {
		t.Fatal("fixture: baseDoc must carry max_total_open 3 for the rev+2 invariant")
	}

	for i := range reloads {
		open := i + 4 // revision i+2 carries max_total_open i+4
		if _, err := s.Reload("configs/test.json", testAt, []byte(withField(fMaxOpen, itoa(open)))); err != nil {
			t.Fatalf("Reload %d = %v", i, err)
		}
	}

	close(stop)
	wg.Wait()

	if got := s.Current().Rev.Num; got != reloads+1 {
		t.Errorf("final revision = %d, want %d", got, reloads+1)
	}

	// The measurement. Each reader owns its own map and it is read only after
	// wg.Wait(), so counting them costs no synchronisation of its own.
	distinct := make(map[uint64]bool)
	for _, m := range seen {
		if len(m) < 2 {
			t.Errorf("a reader observed %d distinct revision(s): it never straddled a reload, "+
				"so it checked nothing this test claims to check", len(m))
		}
		for n := range m {
			distinct[n] = true
		}
	}
	t.Logf("readers observed %d of %d revisions", len(distinct), reloads+1)
}

// TestConcurrentReloadsSerialise checks the writer side. Reload is the only
// mutator and it takes a mutex, so N concurrent reloads of N distinct documents
// must leave the revision at exactly N+1 — no lost update, no double increment.
func TestConcurrentReloadsSerialise(t *testing.T) {
	s := mustStore(t)

	const writers = 16
	var wg sync.WaitGroup
	for i := range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.Reload("configs/test.json", testAt, []byte(withField(fMaxOpen, itoa(i+10)))); err != nil {
				t.Errorf("Reload = %v", err)
			}
		}()
	}
	wg.Wait()

	if got := s.Current().Rev.Num; got != writers+1 {
		t.Errorf("revision after %d concurrent reloads = %d, want %d", writers, got, writers+1)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
