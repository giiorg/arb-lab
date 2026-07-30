package capture

// WalkRecords' own surface. The sliding-window machinery underneath is proved
// by the scanArchive battery (writer_internal_test.go) — rebuilt on this walker,
// so those tests bite here too. What needs its own coverage is the part that is
// NEW at the export: the callback contract (exact frame bytes, in order, error
// stops the walk) and the summary fields a non-archive caller reads directly.

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// walkFile writes body to a fresh file and opens it for reading.
func walkFile(t *testing.T, body []byte) *os.File {
	t.Helper()
	path := filepath.Join(t.TempDir(), "walk.arbcap")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

// walkRecord builds a valid n-th data record with a payload that identifies it.
func walkRecord(n uint64) Record {
	return Record{
		Kind:       RecordKindData,
		CaptureSeq: n,
		RecvTime:   time.Date(2026, 7, 30, 12, 0, int(n), 0, time.UTC),
		Mono:       time.Duration(n) * time.Second,
		Payload:    []byte(fmt.Sprintf("record-%d", n)),
	}
}

func TestWalkRecordsHandsExactFrameBytesInOrder(t *testing.T) {
	body := AppendFileHeader(nil)
	var frames [][]byte
	for n := uint64(1); n <= 3; n++ {
		start := len(body)
		var err error
		body, err = AppendRecord(body, walkRecord(n))
		if err != nil {
			t.Fatal(err)
		}
		frames = append(frames, append([]byte(nil), body[start:]...))
	}

	var got [][]byte
	var seqs []uint64
	sum, err := WalkRecords(walkFile(t, body), func(rec Record, raw []byte) error {
		// Copy: raw is only valid during the call, and asserting on kept
		// references would test the buffer's reuse pattern, not the contract.
		got = append(got, append([]byte(nil), raw...))
		seqs = append(seqs, rec.CaptureSeq)
		return nil
	})
	if err != nil {
		t.Fatalf("WalkRecords: %v", err)
	}
	if sum.Fresh || sum.TornBytes != 0 || sum.GoodEnd != int64(len(body)) {
		t.Errorf("summary = %+v, want GoodEnd %d, nothing torn, not fresh", sum, len(body))
	}
	if len(got) != 3 {
		t.Fatalf("callback ran %d times, want 3", len(got))
	}
	for i := range frames {
		if !bytes.Equal(got[i], frames[i]) {
			t.Errorf("record %d: raw bytes differ from the frame AppendRecord produced", i+1)
		}
		if seqs[i] != uint64(i+1) {
			t.Errorf("record %d decoded with CaptureSeq %d — order not preserved", i+1, seqs[i])
		}
	}
}

func TestWalkRecordsCallbackErrorStopsTheWalk(t *testing.T) {
	body := AppendFileHeader(nil)
	var err error
	var secondAt int64
	for n := uint64(1); n <= 3; n++ {
		if n == 2 {
			secondAt = int64(len(body))
		}
		body, err = AppendRecord(body, walkRecord(n))
		if err != nil {
			t.Fatal(err)
		}
	}

	sentinel := errors.New("walk: consumer refused")
	calls := 0
	sum, err := WalkRecords(walkFile(t, body), func(rec Record, raw []byte) error {
		calls++
		if rec.CaptureSeq == 2 {
			return sentinel
		}
		return nil
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want the callback's own error", err)
	}
	if calls != 2 {
		t.Errorf("callback ran %d times after refusing on the second, want 2", calls)
	}
	if !strings.Contains(err.Error(), fmt.Sprintf("at offset %d", secondAt)) {
		t.Errorf("error %q does not name the refusing record's offset %d", err, secondAt)
	}
	if sum.GoodEnd != secondAt {
		t.Errorf("GoodEnd = %d, want %d — the boundary before the record the consumer refused",
			sum.GoodEnd, secondAt)
	}
}

func TestWalkRecordsEmptyAndHeaderOnlyAreDistinct(t *testing.T) {
	sum, err := WalkRecords(walkFile(t, nil), func(Record, []byte) error {
		t.Fatal("callback ran on an empty file")
		return nil
	})
	if err != nil {
		t.Fatalf("empty file: %v", err)
	}
	if !sum.Fresh {
		t.Error("a zero-byte file did not report Fresh")
	}

	sum, err = WalkRecords(walkFile(t, AppendFileHeader(nil)), func(Record, []byte) error {
		t.Fatal("callback ran on a header-only file")
		return nil
	})
	if err != nil {
		t.Fatalf("header-only file: %v", err)
	}
	if sum.Fresh {
		t.Error("a header-only file reported Fresh — that is a file a previous run created")
	}
	if sum.GoodEnd != int64(FileHeaderLen) || sum.TornBytes != 0 {
		t.Errorf("summary = %+v, want GoodEnd %d and nothing torn", sum, FileHeaderLen)
	}
}

func TestWalkRecordsTornTailIsReportedNotRefused(t *testing.T) {
	body := AppendFileHeader(nil)
	var err error
	body, err = AppendRecord(body, walkRecord(1))
	if err != nil {
		t.Fatal(err)
	}
	boundary := int64(len(body))
	body, err = AppendRecord(body, walkRecord(2))
	if err != nil {
		t.Fatal(err)
	}
	// Tear INSIDE the second record — a boundary tear is TestWalkRecords
	// EmptyAndHeaderOnlyAreDistinct's case, and the two must not collapse.
	torn := body[:boundary+7]

	calls := 0
	sum, err := WalkRecords(walkFile(t, torn), func(Record, []byte) error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("a torn tail must recover, got: %v", err)
	}
	if calls != 1 {
		t.Errorf("callback ran %d times, want 1 — only the whole record", calls)
	}
	if sum.GoodEnd != boundary || sum.TornBytes != 7 {
		t.Errorf("summary = %+v, want GoodEnd %d and TornBytes 7", sum, boundary)
	}
}

func TestWalkRecordsCorruptionIsRefusedWithOffset(t *testing.T) {
	body := AppendFileHeader(nil)
	var err error
	body, err = AppendRecord(body, walkRecord(1))
	if err != nil {
		t.Fatal(err)
	}
	secondAt := int64(len(body))
	body, err = AppendRecord(body, walkRecord(2))
	if err != nil {
		t.Fatal(err)
	}
	// Flip a payload byte of the second record: present and WRONG, which must
	// never be reported as a tail to recover from.
	body[len(body)-1] ^= 0xFF

	sum, err := WalkRecords(walkFile(t, body), func(Record, []byte) error { return nil })
	if !errors.Is(err, ErrArchiveCorrupt) || !errors.Is(err, ErrChecksum) {
		t.Fatalf("err = %v, want ErrArchiveCorrupt wrapping ErrChecksum", err)
	}
	if !strings.Contains(err.Error(), fmt.Sprintf("at offset %d", secondAt)) {
		t.Errorf("error %q does not name the corrupt record's offset %d", err, secondAt)
	}
	if sum.GoodEnd != secondAt {
		t.Errorf("GoodEnd = %d, want %d — the last good boundary", sum.GoodEnd, secondAt)
	}
}
