package replay

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/capture"
)

// TestTapeMergeOrder pins §39p-4's rule end to end over the canonical archive:
// RecvTime first, and the 40.0s cross-stream tie (fundingInfo vs the bybit
// snapshot) ordered by the stream key, binance before bybit. The expected list
// is written out entry by entry because the merge IS the claim — a looser
// assertion ("sorted by time") would pass with the tie broken either way.
func TestTapeMergeOrder(t *testing.T) {
	root := writeCanonicalArchive(t)
	tape, err := Load(root, canonicalSpecs())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	want := []struct {
		stream int // index into canonicalSpecs: 0 mark, 1 info, 2 bybit
		seq    uint64
		recv   time.Time
	}{
		{1, 1, at(tPoll)}, // binance fundingInfo — binance < bybit at the tie
		{2, 1, at(tSnap)}, // bybit snapshot
		{0, 1, at(tMark1)},
		{0, 2, at(tTie)}, // binance < bybit at the second tie too
		{2, 2, at(tTie)},
		{2, 3, at(tChat)},
		{2, 4, at(tLast)},
	}
	if tape.Len() != len(want) {
		t.Fatalf("tape holds %d entries, want %d", tape.Len(), len(want))
	}
	if !tape.Start().Equal(at(tPoll)) {
		t.Errorf("Start() = %v, want %v", tape.Start(), at(tPoll))
	}
	for i, w := range want {
		e := tape.entries[i]
		if e.stream != w.stream || e.rec.CaptureSeq != w.seq || !e.rec.RecvTime.Equal(w.recv) {
			t.Errorf("entry %d: stream %d seq %d at %v, want stream %d seq %d at %v",
				i, e.stream, e.rec.CaptureSeq, e.rec.RecvTime, w.stream, w.seq, w.recv)
		}
	}
}

// TestCmpStreamOrdersEveryField gives each comparator arm a pair differing in
// that field alone — the fixture-collapse rule applied in advance: a pair
// differing in two fields cannot say which arm ordered it.
func TestCmpStreamOrdersEveryField(t *testing.T) {
	base := bybitTickerKey()
	vary := func(mut func(*capture.StreamKey)) capture.StreamKey {
		k := bybitTickerKey()
		mut(&k)
		return k
	}
	cases := []struct {
		name string
		a, b capture.StreamKey
	}{
		{"venue", vary(func(k *capture.StreamKey) { k.Source.Instrument.Venue = "binance" }), base},
		{"product", vary(func(k *capture.StreamKey) { k.Source.Instrument.Product = "aaa_perp" }), base},
		{"symbol", vary(func(k *capture.StreamKey) { k.Source.Instrument.Symbol = "AAAUSDT" }), base},
		{"contract", vary(func(k *capture.StreamKey) { k.Source.Instrument.Contract = "inverse" }), base},
		{"stream", vary(func(k *capture.StreamKey) { k.Source.Stream = "book" }), base},
		{"topic", vary(func(k *capture.StreamKey) { k.Topic = "aaa.topic" }), base},
	}
	for _, c := range cases {
		if got := cmpStream(c.a, c.b); got >= 0 {
			t.Errorf("%s: cmpStream = %d, want < 0", c.name, got)
		}
		if got := cmpStream(c.b, c.a); got <= 0 {
			t.Errorf("%s reversed: cmpStream = %d, want > 0", c.name, got)
		}
	}
	if got := cmpStream(base, base); got != 0 {
		t.Errorf("equal keys: cmpStream = %d, want 0", got)
	}
}

// TestTapeSortBreaksSeqTieWithinStream aims at the comparator's third clause
// directly. It cannot be reached through Load with an honest archive — within
// one stream, file order IS CaptureSeq order, so the stable sort never needs
// the clause — which is exactly why the unit constructs the entries scrambled:
// without this test, dropping the CaptureSeq tie-break is an equivalent
// mutant.
func TestTapeSortBreaksSeqTieWithinStream(t *testing.T) {
	instant := at(10 * time.Second)
	tape := &Tape{
		specs: canonicalSpecs(),
		entries: []tapeEntry{
			{stream: 2, rec: capture.Record{Kind: capture.RecordKindData, CaptureSeq: 3, RecvTime: instant}},
			{stream: 2, rec: capture.Record{Kind: capture.RecordKindData, CaptureSeq: 1, RecvTime: instant}},
			{stream: 2, rec: capture.Record{Kind: capture.RecordKindData, CaptureSeq: 2, RecvTime: instant}},
		},
	}
	tape.sort()
	for i, wantSeq := range []uint64{1, 2, 3} {
		if got := tape.entries[i].rec.CaptureSeq; got != wantSeq {
			t.Errorf("entry %d: seq %d, want %d", i, got, wantSeq)
		}
	}
}

// TestLoadRefusals walks the refusal doors: each is the empty-tree argument at
// a different radius (§39n-3 one layer up), so each must carry its own name.
func TestLoadRefusals(t *testing.T) {
	t.Run("no streams", func(t *testing.T) {
		if _, err := Load(t.TempDir(), nil); !errors.Is(err, ErrNoStreams) {
			t.Errorf("err = %v, want ErrNoStreams", err)
		}
	})
	t.Run("unknown decoder", func(t *testing.T) {
		specs := []StreamSpec{{Key: bybitTickerKey(), Decoder: "nope"}}
		if _, err := Load(t.TempDir(), specs); !errors.Is(err, ErrDecoderUnknown) {
			t.Errorf("err = %v, want ErrDecoderUnknown", err)
		}
	})
	t.Run("unset decoder", func(t *testing.T) {
		specs := []StreamSpec{{Key: bybitTickerKey()}}
		if _, err := Load(t.TempDir(), specs); !errors.Is(err, ErrDecoderUnset) {
			t.Errorf("err = %v, want ErrDecoderUnset", err)
		}
	})
	t.Run("stream dir with no files", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(bybitTickerKey().Dir(root), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		specs := []StreamSpec{{Key: bybitTickerKey(), Decoder: DecodeBybitTicker}}
		if _, err := Load(root, specs); !errors.Is(err, ErrNoFiles) {
			t.Errorf("err = %v, want ErrNoFiles", err)
		}
	})
	t.Run("stream dir missing", func(t *testing.T) {
		specs := []StreamSpec{{Key: bybitTickerKey(), Decoder: DecodeBybitTicker}}
		if _, err := Load(t.TempDir(), specs); err == nil {
			t.Error("Load over a missing stream dir succeeded")
		}
	})
	t.Run("only metadata records", func(t *testing.T) {
		root := t.TempDir()
		writeStreamFile(t, root, bybitTickerKey(), []capture.Record{
			{Kind: capture.RecordKindOpen, CaptureSeq: 1, RecvTime: at(0), Payload: []byte("{}")},
			{Kind: capture.RecordKindAlive, CaptureSeq: 2, RecvTime: at(time.Minute)},
			{Kind: capture.RecordKindClose, CaptureSeq: 3, RecvTime: at(2 * time.Minute)},
		})
		specs := []StreamSpec{{Key: bybitTickerKey(), Decoder: DecodeBybitTicker}}
		if _, err := Load(root, specs); !errors.Is(err, ErrTapeEmpty) {
			t.Errorf("err = %v, want ErrTapeEmpty", err)
		}
	})
}

// TestLoadSkipsMetadataKinds pins the consumable set: Data and Rest enter the
// tape, the five capture-metadata kinds are stepped over — evidence about the
// archive, never inputs (§39p-3) — and, crucially for the ordinal discipline,
// skipping happens at LOAD, so a metadata record never burns an ordinal.
func TestLoadSkipsMetadataKinds(t *testing.T) {
	root := t.TempDir()
	writeStreamFile(t, root, bybitTickerKey(), []capture.Record{
		{Kind: capture.RecordKindOpen, CaptureSeq: 1, RecvTime: at(0), Payload: []byte("{}")},
		{Kind: capture.RecordKindAlive, CaptureSeq: 2, RecvTime: at(time.Second)},
		dataRec(3, at(2*time.Second), bybitSnapJSON(at(2*time.Second), "0.0001")),
		{Kind: capture.RecordKindGap, CaptureSeq: 4, RecvTime: at(3 * time.Second), Payload: []byte("{}")},
		dataRec(5, at(4*time.Second), bybitQuoteDeltaJSON(at(4*time.Second))),
		{Kind: capture.RecordKindClose, CaptureSeq: 6, RecvTime: at(5 * time.Second)},
	})
	tape, err := Load(root, []StreamSpec{{Key: bybitTickerKey(), Decoder: DecodeBybitTicker}})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if tape.Len() != 2 {
		t.Fatalf("tape holds %d entries, want 2 (the Data records alone)", tape.Len())
	}
	for i, wantSeq := range []uint64{3, 5} {
		if got := tape.entries[i].rec.CaptureSeq; got != wantSeq {
			t.Errorf("entry %d: seq %d, want %d", i, got, wantSeq)
		}
	}
}

// TestLoadMergesFilesChronologically covers a stream spanning a rotation
// boundary: two hour files, order carried by the path layout's lexical =
// chronological property.
func TestLoadMergesFilesChronologically(t *testing.T) {
	root := t.TempDir()
	early := at(59*time.Minute + 59*time.Second)
	late := at(time.Hour + 500*time.Millisecond)
	writeStreamFile(t, root, bybitTickerKey(), []capture.Record{
		dataRec(1, early, bybitSnapJSON(early, "0.0001")),
	})
	writeStreamFile(t, root, bybitTickerKey(), []capture.Record{
		dataRec(2, late, bybitQuoteDeltaJSON(late)),
	})
	tape, err := Load(root, []StreamSpec{{Key: bybitTickerKey(), Decoder: DecodeBybitTicker}})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if tape.Len() != 2 {
		t.Fatalf("tape holds %d entries, want 2", tape.Len())
	}
	if !tape.Start().Equal(early) {
		t.Errorf("Start() = %v, want %v", tape.Start(), early)
	}
	if got := tape.entries[1].rec.CaptureSeq; got != 2 {
		t.Errorf("second entry seq %d, want 2 (the next hour's file)", got)
	}
}
