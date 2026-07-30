package replay

import (
	"errors"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/platform/events"
)

// TestIngesterBurnsOrdinalOnEveryRecord walks §3.5's stamping rule through
// every no-news case §39p-2 names — transport chatter, a markPrice before any
// overlay, a failed poll, a funding-silent delta — and asserts the one thing
// each has in common: the ordinal advanced anyway, and the eventual emission
// inherits the ordinal of ITS OWN record, not a compressed emission count.
func TestIngesterBurnsOrdinalOnEveryRecord(t *testing.T) {
	specs := canonicalSpecs() // 0 mark, 1 info, 2 bybit
	ing, err := NewIngester(specs)
	if err != nil {
		t.Fatalf("NewIngester: %v", err)
	}

	steps := []struct {
		name     string
		entry    tapeEntry
		wantEmit bool
	}{
		{"bybit chatter", tapeEntry{stream: 2, rec: dataRec(1, at(1*time.Second), bybitChatterJSON())}, false},
		{"mark before overlay", tapeEntry{stream: 0, rec: dataRec(1, at(2*time.Second), binanceMarkJSON(at(2*time.Second), "0.0001"))}, false},
		{"failed poll", tapeEntry{stream: 1, rec: restRec(1, at(3*time.Second), restFailedPayload(t, at(3*time.Second)))}, false},
		{"throttled poll", tapeEntry{stream: 1, rec: restRec(2, at(3500*time.Millisecond), restThrottledPayload(t, at(3500*time.Millisecond)))}, false},
		{"mark after failed poll", tapeEntry{stream: 0, rec: dataRec(2, at(4*time.Second), binanceMarkJSON(at(4*time.Second), "0.0001"))}, false},
		{"answer without our symbol", tapeEntry{stream: 1, rec: restRec(3, at(5*time.Second), restOKPayload(t, at(5*time.Second), fundingInfoBodyWithoutSymbol))}, false},
		{"mark against absent row", tapeEntry{stream: 0, rec: dataRec(3, at(6*time.Second), binanceMarkJSON(at(6*time.Second), "0.0001"))}, false},
		{"applied poll", tapeEntry{stream: 1, rec: restRec(4, at(7*time.Second), restOKPayload(t, at(7*time.Second), fundingInfoBodyJSON))}, false},
		{"mark composes", tapeEntry{stream: 0, rec: dataRec(4, at(8*time.Second), binanceMarkJSON(at(8*time.Second), "0.0001"))}, true},
		{"bybit snapshot", tapeEntry{stream: 2, rec: dataRec(2, at(9*time.Second), bybitSnapJSON(at(9*time.Second), "0.0002"))}, true},
		{"funding-silent delta", tapeEntry{stream: 2, rec: dataRec(3, at(10*time.Second), bybitQuoteDeltaJSON(at(10*time.Second)))}, false},
		{"rate delta", tapeEntry{stream: 2, rec: dataRec(4, at(11*time.Second), bybitRateDeltaJSON(at(11*time.Second), "0.0003"))}, true},
	}

	for i, s := range steps {
		env, emitted, err := ing.Ingest(s.entry)
		if err != nil {
			t.Fatalf("step %d (%s): %v", i, s.name, err)
		}
		wantOrdinal := uint64(i + 1)
		if got := ing.LastOrdinal(); got != wantOrdinal {
			t.Errorf("step %d (%s): LastOrdinal %d, want %d", i, s.name, got, wantOrdinal)
		}
		if emitted != s.wantEmit {
			t.Errorf("step %d (%s): emitted %t, want %t", i, s.name, emitted, s.wantEmit)
		}
		if emitted && env.AcceptOrdinal != wantOrdinal {
			t.Errorf("step %d (%s): envelope ordinal %d, want %d — inheritance, not renumbering", i, s.name, env.AcceptOrdinal, wantOrdinal)
		}
	}
	if got := ing.RestFailures(); got != 2 {
		t.Errorf("RestFailures = %d, want 2 (the transport failure and the 429)", got)
	}
}

// TestIngesterEnvelopeProvenance pins §4's split on the emitted envelope:
// RecvTime is the archive's recorded arrival, SourceTime the venue's own stamp
// from the payload, VenueSeq zero (both funding wires, §39p-2), Prio the
// per-source constant, Source the spec's.
func TestIngesterEnvelopeProvenance(t *testing.T) {
	specs := canonicalSpecs()
	ing, err := NewIngester(specs)
	if err != nil {
		t.Fatalf("NewIngester: %v", err)
	}
	recv := at(30 * time.Second)
	stamp := at(30*time.Second - 20*time.Millisecond)
	env, emitted, err := ing.Ingest(tapeEntry{stream: 2, rec: dataRec(1, recv, bybitSnapJSON(stamp, "0.0001"))})
	if err != nil || !emitted {
		t.Fatalf("snapshot ingest: emitted %t, err %v", emitted, err)
	}
	if env.Source != specs[2].Key.Source {
		t.Errorf("Source = %v, want %v", env.Source, specs[2].Key.Source)
	}
	if !env.RecvTime.Equal(recv) {
		t.Errorf("RecvTime = %v, want the record's %v", env.RecvTime, recv)
	}
	if !env.SourceTime.Equal(stamp) {
		t.Errorf("SourceTime = %v, want the payload's %v", env.SourceTime, stamp)
	}
	if env.VenueSeq != 0 {
		t.Errorf("VenueSeq = %d, want 0", env.VenueSeq)
	}
	if env.Prio != events.PrioNormal {
		t.Errorf("Prio = %v, want PrioNormal", env.Prio)
	}
	if env.IngressSeq != 0 {
		t.Errorf("IngressSeq = %d pre-dispatch, want 0 — stamping it is the dispatcher's one job", env.IngressSeq)
	}
	if _, ok := env.Payload.(venue.Funding); !ok {
		t.Errorf("Payload is %T, want venue.Funding", env.Payload)
	}
}

// TestIngesterRefusals: a 2xx answer the decoder cannot read is a decoder gap,
// not a recorded outage (§37g's split); an undecodable venue frame names its
// stream and seq.
func TestIngesterRefusals(t *testing.T) {
	t.Run("unreadable 2xx rest answer", func(t *testing.T) {
		ing, err := NewIngester(canonicalSpecs())
		if err != nil {
			t.Fatalf("NewIngester: %v", err)
		}
		entry := tapeEntry{stream: 1, rec: restRec(1, at(time.Second), restOKPayload(t, at(time.Second), `{"not":"a list"}`))}
		if _, _, err := ing.Ingest(entry); !errors.Is(err, ErrRestAnswerUnreadable) {
			t.Errorf("err = %v, want ErrRestAnswerUnreadable", err)
		}
	})
	t.Run("undecodable venue frame", func(t *testing.T) {
		ing, err := NewIngester(canonicalSpecs())
		if err != nil {
			t.Fatalf("NewIngester: %v", err)
		}
		entry := tapeEntry{stream: 0, rec: dataRec(7, at(time.Second), []byte("not json"))}
		if _, _, err := ing.Ingest(entry); !errors.Is(err, venue.ErrWireUndecodable) {
			t.Errorf("err = %v, want venue.ErrWireUndecodable", err)
		}
	})
	t.Run("no streams", func(t *testing.T) {
		if _, err := NewIngester(nil); !errors.Is(err, ErrNoStreams) {
			t.Errorf("err = %v, want ErrNoStreams", err)
		}
	})
}
