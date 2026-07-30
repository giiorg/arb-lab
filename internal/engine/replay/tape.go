package replay

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/capture"
)

// DecoderKind names which engine/venue decoder owns a stream's payloads.
// Stated by the caller rather than inferred from the topic, because an
// inference table is a second spelling of the capture config that drifts; the
// spec is validated, so a wrong pairing fails at Load, not mid-tape.
type DecoderKind string

const (
	// DecodeBinanceMarkPrice is the `<sym>@markPrice` WS frame.
	DecodeBinanceMarkPrice DecoderKind = "binance-markprice"

	// DecodeBinanceFundingInfo is the fundingInfo REST overlay: it feeds the
	// venue's overlay state and emits no envelope of its own (§39p-2).
	DecodeBinanceFundingInfo DecoderKind = "binance-fundinginfo"

	// DecodeBybitTicker is the v5 `tickers.<sym>` topic.
	DecodeBybitTicker DecoderKind = "bybit-ticker"
)

// Validate reports whether k is a declared decoder.
func (k DecoderKind) Validate() error {
	switch k {
	case DecodeBinanceMarkPrice, DecodeBinanceFundingInfo, DecodeBybitTicker:
		return nil
	case "":
		return ErrDecoderUnset
	}
	return fmt.Errorf("%q: %w", string(k), ErrDecoderUnknown)
}

// StreamSpec names one archive stream and its decoder.
type StreamSpec struct {
	Key     capture.StreamKey
	Decoder DecoderKind
}

// Validate reports whether the spec could name a replayable stream.
func (s StreamSpec) Validate() error {
	if err := s.Key.Validate(); err != nil {
		return err
	}
	return s.Decoder.Validate()
}

// tapeEntry is one consumable record and the stream it came from.
type tapeEntry struct {
	stream int // index into specs
	rec    capture.Record
}

// Tape is a loaded archive slice in raw arrival order: every Data and Rest
// record of every stream, merged by §39p-4's rule. It is immutable after Load
// and shared by both pumps — the same tape driven two ways is what makes the
// parity comparison about the PUMPS rather than about two reads of disk.
type Tape struct {
	specs   []StreamSpec
	entries []tapeEntry
}

// Load reads every archive file of every stream under root and merges them.
//
// Only Data and Rest records enter the tape: they are the venue's bytes, the
// inputs the live pipeline dispatched. Open/Alive/Gap/Close/ClockStep are
// capture's own metadata — evidence about the archive, never inputs — so they
// are skipped WITHOUT burning ordinals (§39p-3). Torn tails are recovered
// exactly as every other reader recovers them (WalkRecords); integrity beyond
// that is capture verify's verdict, deliberately not re-judged here.
func Load(root string, specs []StreamSpec) (*Tape, error) {
	if len(specs) == 0 {
		return nil, ErrNoStreams
	}
	for i, s := range specs {
		if err := s.Validate(); err != nil {
			return nil, fmt.Errorf("replay: stream %d: %w", i, err)
		}
	}

	var entries []tapeEntry
	for i, s := range specs {
		files, err := streamFiles(s.Key.Dir(root))
		if err != nil {
			return nil, fmt.Errorf("replay: stream %s: %w", s.Key, err)
		}
		if len(files) == 0 {
			return nil, fmt.Errorf("replay: stream %s under %s: %w", s.Key, root, ErrNoFiles)
		}
		for _, path := range files {
			if err := loadFile(path, i, &entries); err != nil {
				return nil, fmt.Errorf("replay: %s: %w", path, err)
			}
		}
	}
	if len(entries) == 0 {
		return nil, ErrTapeEmpty
	}

	t := &Tape{specs: specs, entries: entries}
	t.sort()
	return t, nil
}

// streamFiles lists a stream's archive files in lexical order, which the
// path layout (YYYY/MM/DD/<window>.arbcap, zero-padded UTC) makes
// chronological — the same property the verify walker leans on.
func streamFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".arbcap") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.Sort(files)
	return files, nil
}

// loadFile appends one file's consumable records.
func loadFile(path string, stream int, entries *[]tapeEntry) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = capture.WalkRecords(f, func(rec capture.Record, _ []byte) error {
		switch rec.Kind {
		case capture.RecordKindData, capture.RecordKindRest:
			*entries = append(*entries, tapeEntry{stream: stream, rec: rec})
		case capture.RecordKindUnspecified, capture.RecordKindOpen,
			capture.RecordKindAlive, capture.RecordKindGap,
			capture.RecordKindClose, capture.RecordKindClockStep:
			// Capture metadata: named, not defaulted (§3.6 rule 10), so a kind
			// added to the container lands here as a build failure where
			// somebody must decide whether the tape consumes it.
		}
		return nil
	})
	return err
}

// sort orders the tape by §39p-4's rule: RecvTime, then the stream's total
// sort key, then CaptureSeq. A cross-stream tie is two frames the recorded
// clock could not order — live arrival would have resolved it by scheduler
// accident, and any stable resolution is as honest as the accident.
func (t *Tape) sort() {
	slices.SortStableFunc(t.entries, func(a, b tapeEntry) int {
		if c := a.rec.RecvTime.Compare(b.rec.RecvTime); c != 0 {
			return c
		}
		if c := cmpStream(t.specs[a.stream].Key, t.specs[b.stream].Key); c != 0 {
			return c
		}
		switch {
		case a.rec.CaptureSeq < b.rec.CaptureSeq:
			return -1
		case a.rec.CaptureSeq > b.rec.CaptureSeq:
			return 1
		}
		return 0
	})
}

// cmpStream is the total order over stream keys — the dispatcher's cmpSource
// shape extended by Topic, because two streams may share a SourceID (Binance's
// markPrice and fundingInfo both ride the funding stream of one instrument;
// §32c is the finding). Field-wise so it is injective by construction; the
// order itself carries no meaning.
func cmpStream(a, b capture.StreamKey) int {
	if c := strings.Compare(string(a.Source.Instrument.Venue), string(b.Source.Instrument.Venue)); c != 0 {
		return c
	}
	if c := strings.Compare(string(a.Source.Instrument.Product), string(b.Source.Instrument.Product)); c != 0 {
		return c
	}
	if c := strings.Compare(a.Source.Instrument.Symbol, b.Source.Instrument.Symbol); c != 0 {
		return c
	}
	if c := strings.Compare(string(a.Source.Instrument.Contract), string(b.Source.Instrument.Contract)); c != 0 {
		return c
	}
	if c := strings.Compare(string(a.Source.Stream), string(b.Source.Stream)); c != 0 {
		return c
	}
	return strings.Compare(a.Topic, b.Topic)
}

// Len reports how many consumable records the tape holds.
func (t *Tape) Len() int { return len(t.entries) }

// Start reports the first record's arrival instant — what the harness seeds
// its TestClock with, since NewTestClock refuses the zero time and SetNow
// refuses to move backwards.
func (t *Tape) Start() time.Time { return t.entries[0].rec.RecvTime }
