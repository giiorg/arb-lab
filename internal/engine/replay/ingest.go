package replay

import (
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/capture"
	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/platform/events"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// Ingester is §3.5's raw-ingestion seam for the sim: the one place the
// process stamps AcceptOrdinal. It holds the decoder state each stream needs
// (the Bybit accumulator, the per-venue fundingInfo overlay) and turns tape
// entries into pre-dispatch envelopes.
//
// One instance per RUN, never shared between pumps: the ordinal counter and
// the decoder accumulators are run state, and two pumps sharing one would
// interleave two worlds.
type Ingester struct {
	specs []StreamSpec

	// bybit holds one accumulator per bybit-ticker stream, keyed by stream
	// index — per STREAM, not per venue, because the accumulated state is the
	// wire connection's, and two topics are two connections.
	bybit map[int]*venue.BybitTickerState

	// overlays holds one fundingInfo overlay per venue. Keyed by venue rather
	// than by stream because the overlay is consumed by a DIFFERENT stream
	// than the one that feeds it (markPrice composes with what fundingInfo
	// applied), and the venue is the join key both sides share.
	overlays map[ids.Venue]*venue.FundingInfoOverlay

	// next is the last AcceptOrdinal assigned. It advances on every consumed
	// Data/Rest record — §3.5: "stamped in raw arrival order at ingestion on
	// EVERY raw input", including inputs that emit no envelope.
	next uint64

	// restFailures counts recorded polls that failed (transport error or
	// non-2xx status). They are ordinary — §37g: a failed poll is a Rest
	// record with the transport's words — and counted so a run can report
	// them rather than absorbing them invisibly.
	restFailures uint64
}

// NewIngester builds a cold ingester for one run over specs.
func NewIngester(specs []StreamSpec) (*Ingester, error) {
	if len(specs) == 0 {
		return nil, ErrNoStreams
	}
	ing := &Ingester{
		specs:    specs,
		bybit:    map[int]*venue.BybitTickerState{},
		overlays: map[ids.Venue]*venue.FundingInfoOverlay{},
	}
	for i, s := range specs {
		if err := s.Validate(); err != nil {
			return nil, fmt.Errorf("replay: stream %d: %w", i, err)
		}
		switch s.Decoder {
		case DecodeBybitTicker:
			ing.bybit[i] = venue.NewBybitTickerState()
		case DecodeBinanceFundingInfo, DecodeBinanceMarkPrice:
			v := s.Key.Source.Instrument.Venue
			if _, ok := ing.overlays[v]; !ok {
				ing.overlays[v] = venue.NewFundingInfoOverlay()
			}
		}
	}
	return ing, nil
}

// LastOrdinal reports the most recently assigned AcceptOrdinal — zero before
// the first record. For assertions; the next ordinal is this seam's to mint.
func (g *Ingester) LastOrdinal() uint64 { return g.next }

// RestFailures reports how many recorded polls had failed on the wire.
func (g *Ingester) RestFailures() uint64 { return g.restFailures }

// Ingest consumes one tape entry: the ordinal is burned unconditionally, and
// the returned envelope exists only when the record emitted a reading
// (emitted false covers non-funding deltas, transport chatter, overlay
// applications, cold starts and failed polls — every §39p-2 no-news case).
func (g *Ingester) Ingest(e tapeEntry) (events.Envelope, bool, error) {
	g.next++
	spec := g.specs[e.stream]

	switch spec.Decoder {
	case DecodeBinanceMarkPrice:
		return g.binanceMark(spec, e.rec)
	case DecodeBinanceFundingInfo:
		return events.Envelope{}, false, g.fundingInfo(spec, e.rec)
	case DecodeBybitTicker:
		return g.bybitTicker(e.stream, spec, e.rec)
	}
	// Unreachable: NewIngester validated every spec. Refused rather than
	// assumed, because the spec slice is caller-owned memory.
	return events.Envelope{}, false, fmt.Errorf("replay: stream %s decoder %q: %w",
		spec.Key, string(spec.Decoder), ErrDecoderUnknown)
}

// binanceMark decodes one markPrice frame and composes it with the venue's
// overlay. Emission requires the overlay to answer for the symbol (§39p-2's
// no-default rule): before the first applied poll the frame is consumed
// without news, exactly like a Bybit delta before its snapshot.
func (g *Ingester) binanceMark(spec StreamSpec, rec capture.Record) (events.Envelope, bool, error) {
	m, err := venue.DecodeBinanceMarkPrice(rec.Payload)
	if err != nil {
		return events.Envelope{}, false, fmt.Errorf("replay: %s seq %d: %w", spec.Key, rec.CaptureSeq, err)
	}
	f, ok, err := g.overlays[spec.Key.Source.Instrument.Venue].Compose(m)
	if err != nil {
		return events.Envelope{}, false, fmt.Errorf("replay: %s seq %d: %w", spec.Key, rec.CaptureSeq, err)
	}
	if !ok {
		return events.Envelope{}, false, nil
	}
	return g.envelope(spec, rec, f, m.EventTime), true, nil
}

// fundingInfo applies one recorded poll to the venue's overlay. A poll that
// failed on the wire (transport error, non-2xx) is counted and applies
// nothing — the recorded fact is "we asked and this came back", and what came
// back was not an answer. A 2xx answer the decoder refuses is fatal: the
// venue answered and the seam cannot read it, which is a decoder gap, not a
// recorded outage.
func (g *Ingester) fundingInfo(spec StreamSpec, rec capture.Record) error {
	p, err := capture.DecodeRestPayload(rec.Payload)
	if err != nil {
		return fmt.Errorf("replay: %s seq %d: %w", spec.Key, rec.CaptureSeq, err)
	}
	if p.Meta.Error != "" || p.Meta.Status < 200 || p.Meta.Status > 299 {
		g.restFailures++
		return nil
	}
	if err := g.overlays[spec.Key.Source.Instrument.Venue].Apply(p.Body); err != nil {
		return fmt.Errorf("replay: %s seq %d: %w: %w", spec.Key, rec.CaptureSeq, ErrRestAnswerUnreadable, err)
	}
	return nil
}

// bybitTicker feeds one frame to the stream's accumulator.
func (g *Ingester) bybitTicker(stream int, spec StreamSpec, rec capture.Record) (events.Envelope, bool, error) {
	u, emitted, err := g.bybit[stream].Apply(rec.Payload)
	if err != nil {
		return events.Envelope{}, false, fmt.Errorf("replay: %s seq %d: %w", spec.Key, rec.CaptureSeq, err)
	}
	if !emitted {
		return events.Envelope{}, false, nil
	}
	return g.envelope(spec, rec, u.Funding, u.SourceTime), true, nil
}

// envelope wraps one emitted reading. The provenance split is §4's: RecvTime
// is the archive's recorded arrival (the injected clock's stamp, §32e),
// SourceTime is the venue's own from the payload, and the ordinal is the one
// this seam just burned for the triggering record — inheritance, §3.5.
// VenueSeq is zero for both funding wires (§39p-2: Binance sends no sequence;
// Bybit's cs would report a gap on virtually every funding-bearing message),
// so advance/supersede runs on arrival order, the documented fallback.
func (g *Ingester) envelope(spec StreamSpec, rec capture.Record, f venue.Funding, sourceTime time.Time) events.Envelope {
	return events.Envelope{
		AcceptOrdinal: g.next,
		Source:        spec.Key.Source,
		SourceTime:    sourceTime,
		RecvTime:      rec.RecvTime,
		Prio:          events.PrioNormal,
		Payload:       f,
	}
}
