package venue

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// This file decodes Bybit's funding wire: the v5 `tickers.<sym>` topic.
//
// The same topic also carries §3.4's MARK class, and it is decoded by
// wire_bybit_mark.go rather than here — the emission rule below is the opposite
// of the one the mark needs, and the mark has no state to accumulate. That file
// states the argument; the pointer is here so a reader adding a ticker field
// finds it before folding a second class into this accumulator.
//
// # The ticker is a DELTA stream, so the decoder is a state machine
//
// Full state arrives once, on the post-subscribe `type:"snapshot"` message;
// after that a field appears only when it changes (Profiles §1: "absent field
// = unchanged, never null-coerce"). fundingRate moves roughly once a minute;
// interval, cap and settlement instant may not change for hours. A stateless
// decoder therefore cannot exist — the accumulated last-known state IS the
// reading — and the state lives here rather than in marketdata because it is
// venue wire mechanics (which JSON keys mean what, when a field is present),
// not a market judgement.
//
// # Emission: only when a message carries funding news
//
// Apply reports a reading exactly when the message is a snapshot or a delta
// carrying at least one funding field. Most deltas are bid/ask churn, which is
// not funding news (§31b: the reading is the venue's policy computation), and
// emitting ~10/s identical readings would round-trip the full two-hop
// dispatch for nothing.

// BybitTickerUpdate is one emitted reading and the instant the venue stamped
// on the message that produced it (`ts`, epoch ms) — the envelope's
// SourceTime, surfaced because the payload is the only place Bybit states it.
type BybitTickerUpdate struct {
	Funding    Funding
	SourceTime time.Time
}

// BybitTickerState accumulates one tickers stream. One instance per stream —
// its zero value is deliberately unusable for emission (no snapshot seen), so
// a file replayed from a mid-connection rotation boundary emits nothing until
// the stream's next snapshot, which is §39p-2's documented cold start.
type BybitTickerState struct {
	hasRate, hasNext, hasInterval, hasCap bool

	rate       fixedpoint.Rate
	nextSettle time.Time
	interval   time.Duration
	cap        fixedpoint.Rate
}

// NewBybitTickerState returns a cold accumulator.
func NewBybitTickerState() *BybitTickerState { return &BybitTickerState{} }

// bybitTickerWire is the frame envelope; bybitTickerDataWire the funding
// subset of its data object. Every venue value is a JSON string — including
// fundingIntervalHour ("8") and nextFundingTime (epoch ms, "1785427200000"),
// both observed in the archive — so presence is string non-emptiness, which
// is exactly the "absent = unchanged" rule made mechanical: encoding/json
// leaves an absent key as "", and Bybit does not send empty strings for
// funding fields it means.
type bybitTickerWire struct {
	Topic string          `json:"topic"`
	Type  string          `json:"type"`
	Op    string          `json:"op"`
	TS    int64           `json:"ts"`
	Data  json.RawMessage `json:"data"`
}

type bybitTickerDataWire struct {
	Symbol       string `json:"symbol"`
	FundingRate  string `json:"fundingRate"`
	NextFunding  string `json:"nextFundingTime"`
	IntervalHour string `json:"fundingIntervalHour"`
	FundingCap   string `json:"fundingCap"`
}

// Apply consumes one raw frame. The bool reports whether a reading was
// emitted; false covers transport chatter (an `op` acknowledgement), a delta
// with no funding news, and a funding-bearing delta arriving before any
// snapshot (cold start — the fields seen are still absorbed, so the state
// completes as early as the wire allows).
func (s *BybitTickerState) Apply(raw []byte) (BybitTickerUpdate, bool, error) {
	var w bybitTickerWire
	if err := json.Unmarshal(raw, &w); err != nil {
		return BybitTickerUpdate{}, false, fmt.Errorf("venue: bybit ticker frame: %w: %v", ErrWireUndecodable, err)
	}

	// Op frames (subscribe acks, pongs) are transport chatter riding the same
	// socket: consumed without news. A frame that is NEITHER an op nor a
	// ticker topic is foreign — most plausibly a misfiled file — and refused.
	if w.Topic == "" {
		if w.Op != "" {
			return BybitTickerUpdate{}, false, nil
		}
		return BybitTickerUpdate{}, false, fmt.Errorf("venue: bybit frame with no topic and no op: %w", ErrWireForeign)
	}
	if w.Type != "snapshot" && w.Type != "delta" {
		return BybitTickerUpdate{}, false, fmt.Errorf("venue: bybit ticker type %q: %w", w.Type, ErrWireForeign)
	}
	if w.TS <= 0 {
		return BybitTickerUpdate{}, false, fmt.Errorf("venue: bybit ticker ts=%d: %w", w.TS, ErrWireIncomplete)
	}

	var d bybitTickerDataWire
	if err := json.Unmarshal(w.Data, &d); err != nil {
		return BybitTickerUpdate{}, false, fmt.Errorf("venue: bybit ticker data: %w: %v", ErrWireUndecodable, err)
	}

	// A snapshot restates full ticker state, so the accumulator resets: a
	// field the snapshot does not carry is a field the venue no longer
	// states, and holding the old value past a resubscribe would be the
	// null-coerce mistake with extra steps.
	if w.Type == "snapshot" {
		*s = BybitTickerState{}
	}

	news, err := s.absorb(d)
	if err != nil {
		return BybitTickerUpdate{}, false, err
	}

	// A snapshot that leaves the state incomplete is refused rather than
	// waited out: it claimed to be full state, and a perp ticker without its
	// funding fields is a stream this seam has not been taught (a pre-listing
	// symbol, a category change) — someone must look.
	if w.Type == "snapshot" && !s.complete() {
		return BybitTickerUpdate{}, false, fmt.Errorf(
			"venue: bybit ticker snapshot missing funding state (rate %t, next %t, interval %t, cap %t): %w",
			s.hasRate, s.hasNext, s.hasInterval, s.hasCap, ErrWireIncomplete)
	}

	if !news || !s.complete() {
		return BybitTickerUpdate{}, false, nil
	}

	f, err := s.funding()
	if err != nil {
		return BybitTickerUpdate{}, false, err
	}
	return BybitTickerUpdate{Funding: f, SourceTime: time.UnixMilli(w.TS).UTC()}, true, nil
}

// absorb merges one data object's funding fields and reports whether any were
// present. Parse errors refuse the frame and leave the state as it was — the
// same all-or-nothing rule the overlay applies, per field group.
func (s *BybitTickerState) absorb(d bybitTickerDataWire) (news bool, err error) {
	if d.FundingRate != "" {
		rate, err := fixedpoint.ParseRate(d.FundingRate)
		if err != nil {
			return false, fmt.Errorf("venue: bybit fundingRate %q: %w", d.FundingRate, err)
		}
		s.rate, s.hasRate, news = rate, true, true
	}
	if d.NextFunding != "" {
		ms, err := strconv.ParseInt(d.NextFunding, 10, 64)
		if err != nil || ms <= 0 {
			return false, fmt.Errorf("venue: bybit nextFundingTime %q: %w", d.NextFunding, ErrWireIncomplete)
		}
		s.nextSettle, s.hasNext, news = time.UnixMilli(ms).UTC(), true, true
	}
	if d.IntervalHour != "" {
		hours, err := strconv.Atoi(d.IntervalHour)
		if err != nil {
			return false, fmt.Errorf("venue: bybit fundingIntervalHour %q: %w", d.IntervalHour, ErrWireIncomplete)
		}
		interval, err := FundingIntervalFromHours(hours)
		if err != nil {
			return false, fmt.Errorf("venue: bybit fundingIntervalHour %q: %w", d.IntervalHour, err)
		}
		s.interval, s.hasInterval, news = interval, true, true
	}
	if d.FundingCap != "" {
		capRate, err := fixedpoint.ParseRate(d.FundingCap)
		if err != nil {
			return false, fmt.Errorf("venue: bybit fundingCap %q: %w", d.FundingCap, err)
		}
		s.cap, s.hasCap, news = capRate, true, true
	}
	return news, nil
}

func (s *BybitTickerState) complete() bool {
	return s.hasRate && s.hasNext && s.hasInterval && s.hasCap
}

// funding builds the reading from complete state.
//
// # The floor is the cap mirrored, and the mirror is a dated measurement
//
// The ticker carries fundingCap and no floor; the floor lives in REST
// `instruments-info.upper/lowerFundingRate`, which the capture set does not
// poll — so a replay has nothing else to read. The mirror is justified by a
// live measurement, not assumed: instruments-info for BTCUSDT on 2026-07-30
// returned upperFundingRate "0.005" / lowerFundingRate "-0.005", exactly
// symmetric and equal to the ticker's observed fundingCap (Profiles §5). The
// live pipeline replaces this mirror with the polled pair when the
// instruments-info poll exists (milestone 3's venue clients); a re-measurement
// that finds asymmetry invalidates the mirror, and Profiles §5 records that
// standing caveat.
func (s *BybitTickerState) funding() (Funding, error) {
	floor, err := s.cap.Neg()
	if err != nil {
		return Funding{}, fmt.Errorf("venue: bybit funding floor from cap: %w", err)
	}
	f := Funding{
		Rate:       s.rate,
		Interval:   s.interval,
		NextSettle: s.nextSettle,
		Cap:        s.cap,
		Floor:      floor,
	}
	if err := f.Validate(); err != nil {
		return Funding{}, fmt.Errorf("venue: bybit funding: %w", err)
	}
	return f, nil
}
