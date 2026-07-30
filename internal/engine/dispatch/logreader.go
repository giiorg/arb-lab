package dispatch

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/giiorg/arb-lab/internal/engine/capture"
)

// LogReport is what a replayed log turned out to be.
type LogReport struct {
	// Entries is how many sequence records were delivered to the consumer;
	// LastSeq is the last one's position. Density from 1 makes them equal, and
	// both exist anyway because the seal cross-checks each independently.
	Entries uint64
	LastSeq uint64

	// TornBytes reports a tail that ended inside a record — recovered, never
	// silent: the good prefix was delivered and the tear is this field's to
	// disclose. Closed reports the seal. The two compose: Closed false is a
	// run that did not stop cleanly (whether or not anything tore), and Closed
	// true WITH TornBytes nonzero is a file something appended to after the
	// seal and tore — both facts, both reported, neither absorbed.
	TornBytes int64
	Closed    bool
}

// ReplayLog walks the dispatch log at path and re-drives c with every entry,
// in order. It is the read half of §3.5's "replay re-applies the persisted
// DispatchSeq log exactly".
//
// The refusal discipline is the container's, inherited whole (§39n-6): a torn
// tail is recovery (reported, no error), bytes present and wrong are a refusal
// naming the offset. On top of the container's checks, the log's own — density
// from 1, the closed kind roster {Data, Close}, nothing after the seal, the
// seal's two counts — because "every file the reader accepts is exactly one
// the writer would have produced" (§32l's canonicality, worn by this format).
// A consumer error stops the replay and is returned as the consumer's own
// value, distinguishable from every refusal above by errors.Is.
func ReplayLog(path string, c LogConsumer) (LogReport, error) {
	var rep LogReport
	if c == nil {
		return rep, ErrLogConsumerNil
	}
	f, err := os.Open(path)
	if err != nil {
		return rep, fmt.Errorf("dispatch: open log: %w", err)
	}
	defer f.Close()

	sum, err := capture.WalkRecords(f, func(rec capture.Record, _ []byte) error {
		if rep.Closed {
			return fmt.Errorf("%s at seq %d: %w", rec.Kind, rec.CaptureSeq, ErrLogRecordAfterSeal)
		}
		switch rec.Kind {
		case capture.RecordKindData:
			if rec.CaptureSeq != rep.LastSeq+1 {
				return fmt.Errorf("entry %d after %d: %w",
					rec.CaptureSeq, rep.LastSeq, ErrLogSeqNotDense)
			}
			if err := decodeEntry(logFrame{
				Seq:      rec.CaptureSeq,
				RecvTime: rec.RecvTime,
				Payload:  rec.Payload,
			}, c); err != nil {
				return err
			}
			rep.Entries++
			rep.LastSeq = rec.CaptureSeq
			return nil

		case capture.RecordKindClose:
			var p closePayload
			if err := json.Unmarshal(rec.Payload, &p); err != nil {
				return fmt.Errorf("seal payload: %w: %w", ErrLogEntryInvalid, err)
			}
			// A seal over zero entries is a file the writer never produces —
			// refused on canonicality, not repaired into an empty log.
			if rep.Entries == 0 || rec.CaptureSeq != rep.LastSeq || p.Entries != rep.Entries {
				return fmt.Errorf("seal claims seq %d over %d entries, file holds %d ending at %d: %w",
					rec.CaptureSeq, p.Entries, rep.Entries, rep.LastSeq, ErrLogSealMismatch)
			}
			rep.Closed = true
			return nil

		case capture.RecordKindUnspecified, capture.RecordKindOpen,
			capture.RecordKindAlive, capture.RecordKindGap,
			capture.RecordKindRest, capture.RecordKindClockStep:
			// Named, not defaulted (§3.6 rule 10): these are the ARCHIVE's
			// kinds, valid in the container and foreign to this file — a
			// capture kind here most plausibly means an archive file handed to
			// the wrong reader. A kind added to the container later lands as a
			// build failure on this switch, where somebody must decide whether
			// the log format adopts it.
			return fmt.Errorf("kind %s: %w", rec.Kind, ErrLogKind)
		}
		// Unreachable for a decoded record: DecodeRecord validates the kind
		// against the declared set, so an undeclared byte never gets here.
		return fmt.Errorf("kind %s: %w", rec.Kind, ErrLogKind)
	})
	if err != nil {
		return rep, fmt.Errorf("dispatch: replay %s: %w", path, err)
	}
	rep.TornBytes = sum.TornBytes
	return rep, nil
}
