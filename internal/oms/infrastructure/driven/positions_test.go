package driven_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/infrastructure/driven"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// This file checks the position ACL (slice F2, record §56).
//
// The fixtures vary two things the checklist demanded: the position is a SHORT,
// so a translation that dropped the sign is visible, and the reader records the
// URGENCY it was called with, because a §7.6 shed class pinned inside an adapter
// is invisible from its return value — and the class this one pins is the reason
// the enumeration is not shed during the incident it is reserved for.

type recordingPositionReader struct {
	report venue.PositionReport
	err    error

	scopes    []venue.PositionScope
	urgencies []ratelimit.Priority
	assets    []map[ids.VenueInstrument]venue.InstrumentAssets
}

func (r *recordingPositionReader) Positions(_ context.Context, scope venue.PositionScope, assets map[ids.VenueInstrument]venue.InstrumentAssets, urgency ratelimit.Priority) (venue.PositionReport, error) {
	r.scopes = append(r.scopes, scope)
	r.urgencies = append(r.urgencies, urgency)
	r.assets = append(r.assets, assets)
	if r.err != nil {
		return venue.PositionReport{}, r.err
	}
	return r.report, nil
}

var _ driven.VenuePositionReader = (*recordingPositionReader)(nil)

func positionAssets() map[ids.VenueInstrument]venue.InstrumentAssets {
	return map[ids.VenueInstrument]venue.InstrumentAssets{
		perpInstrument(): {Base: "BTC", Quote: "USDT"},
	}
}

func perpInstrument() ids.VenueInstrument {
	return ids.VenueInstrument{
		Venue: "binance", Product: ids.ProductLinearPerp, Symbol: "BTCUSDT", Contract: ids.ContractLinear,
	}
}

func perpPositionScope() ports.PositionScope {
	return ports.PositionScope{Venue: "binance", Product: ids.ProductLinearPerp, Settle: "USDT"}
}

func shortPosition(t *testing.T) fixedpoint.Quantity {
	t.Helper()
	q, err := fixedpoint.ParseQuantity("BTC", "-0.75")
	if err != nil {
		t.Fatalf("ParseQuantity: %v", err)
	}
	return q
}

func TestVenuePositionsCarriesTheSignAndTheSafetyClass(t *testing.T) {
	t.Parallel()

	reader := &recordingPositionReader{report: venue.PositionReport{
		Positions:    []venue.Position{{Instrument: perpInstrument(), Net: shortPosition(t)}},
		Unrecognized: []string{"SOLUSDT"},
	}}
	adapter, err := driven.NewVenuePositions(reader, positionAssets())
	if err != nil {
		t.Fatalf("NewVenuePositions: %v", err)
	}

	report, err := adapter.Positions(perpPositionScope())
	if err != nil {
		t.Fatalf("Positions: %v", err)
	}
	if len(report.Positions) != 1 || report.Positions[0].Net.Decimal() != "-0.75" {
		t.Errorf("positions = %+v, want one SHORT of 0.75", report.Positions)
	}
	if len(report.Unrecognized) != 1 || report.Unrecognized[0] != "SOLUSDT" {
		t.Errorf("unrecognized = %v, want [SOLUSDT] carried through", report.Unrecognized)
	}
	if len(reader.urgencies) != 1 || reader.urgencies[0] != ratelimit.PrioritySafety {
		t.Errorf("urgency = %v, want Safety — §7.6 puts §7.5's reconcile in the Safety row, and an assertion shed under load is an assertion that did not happen", reader.urgencies)
	}
	if len(reader.scopes) != 1 || reader.scopes[0].Settle != "USDT" || reader.scopes[0].Product != ids.ProductLinearPerp {
		t.Errorf("scope = %+v, want the caller's product and settlement asset", reader.scopes)
	}
}

func TestVenuePositionsRefusesAnotherVenuesScope(t *testing.T) {
	t.Parallel()

	reader := &recordingPositionReader{}
	adapter, err := driven.NewVenuePositions(reader, positionAssets())
	if err != nil {
		t.Fatalf("NewVenuePositions: %v", err)
	}

	scope := perpPositionScope()
	scope.Venue = "bybit"
	if _, err := adapter.Positions(scope); !errors.Is(err, driven.ErrPositionScopeVenueMismatch) {
		t.Errorf("Positions(other venue) = %v, want ErrPositionScopeVenueMismatch", err)
	}
	if len(reader.scopes) != 0 {
		t.Error("the call reached the venue client — a scope answered by the wrong exchange is the accident this refusal exists for")
	}
}

func TestVenuePositionsRefusesARowOutsideTheScope(t *testing.T) {
	t.Parallel()

	spot := ids.VenueInstrument{Venue: "binance", Product: ids.ProductSpot, Symbol: "BTCUSDT"}
	reader := &recordingPositionReader{report: venue.PositionReport{
		Positions: []venue.Position{{Instrument: spot, Net: shortPosition(t)}},
	}}
	adapter, err := driven.NewVenuePositions(reader, positionAssets())
	if err != nil {
		t.Fatalf("NewVenuePositions: %v", err)
	}
	if _, err := adapter.Positions(perpPositionScope()); !errors.Is(err, driven.ErrPositionOutsideScope) {
		t.Errorf("Positions(row from another family) = %v, want ErrPositionOutsideScope", err)
	}
}

func TestVenuePositionsPassesAFailedEnumerationThrough(t *testing.T) {
	t.Parallel()

	refused := errors.New("rate limited")
	reader := &recordingPositionReader{err: refused}
	adapter, err := driven.NewVenuePositions(reader, positionAssets())
	if err != nil {
		t.Fatalf("NewVenuePositions: %v", err)
	}

	report, err := adapter.Positions(perpPositionScope())
	if !errors.Is(err, refused) {
		t.Errorf("Positions(failing venue) = %v, want the venue's own error", err)
	}
	// The report must be empty rather than partial: an enumeration that failed
	// read as a flat account is the failure this whole surface exists to avoid.
	if len(report.Positions) != 0 || len(report.Unrecognized) != 0 {
		t.Errorf("report = %+v, want empty", report)
	}
}

func TestNewVenuePositionsRefusals(t *testing.T) {
	t.Parallel()

	if _, err := driven.NewVenuePositions(nil, positionAssets()); !errors.Is(err, driven.ErrVenuePositionReaderMissing) {
		t.Errorf("NewVenuePositions(nil client) = %v, want ErrVenuePositionReaderMissing", err)
	}
	// An empty table would report every position the account holds as foreign,
	// which reads as an account full of foreign orders rather than as a
	// deployment that declared nothing.
	if _, err := driven.NewVenuePositions(&recordingPositionReader{}, nil); !errors.Is(err, driven.ErrInstrumentAssetsMissing) {
		t.Errorf("NewVenuePositions(no assets) = %v, want ErrInstrumentAssetsMissing", err)
	}
}

// TestNewVenuePositionsRefusesATableSpanningTwoVenues is the coin flip removed.
//
// venueOf returns the first key of a map, which is only a function at all while
// every key agrees. A table naming two venues makes this adapter's identity
// depend on Go's randomized map iteration, so the scope check below it admits a
// scope on one boot and refuses it on the next — with no input having changed.
//
// It is Send-and-Receive's lesson applied to a map instead of a select: a claim
// about code whose behaviour a coin flip decides needs repetitions, and one
// construction would itself have been a coin flip about whether the refusal fired.
func TestNewVenuePositionsRefusesATableSpanningTwoVenues(t *testing.T) {
	t.Parallel()

	// The message the first trial produced, compared against by every later one:
	// the refusal must not name a different venue first on two identical calls.
	var firstMixedMessage string

	bybitPerp := ids.VenueInstrument{
		Venue: "bybit", Product: ids.ProductLinearPerp, Symbol: "BTCUSDT", Contract: ids.ContractLinear,
	}
	mixed := map[ids.VenueInstrument]venue.InstrumentAssets{
		perpInstrument(): {Base: "BTC", Quote: "USDT"},
		bybitPerp:        {Base: "BTC", Quote: "USDT"},
	}
	// 30 trials: the defect it replaces was reachable only when the map yielded
	// one particular key first, so a single construction would have been a coin
	// flip about whether the refusal fired.
	for i := 0; i < 30; i++ {
		_, err := driven.NewVenuePositions(&recordingPositionReader{}, mixed)
		if !errors.Is(err, driven.ErrInstrumentVenuesMixed) {
			t.Fatalf("trial %d: NewVenuePositions(mixed table) = %v, want ErrInstrumentVenuesMixed", i, err)
		}
		// The message names BOTH venues and in a stable order, so an operator is
		// not told only that something is mixed.
		if got := err.Error(); !strings.Contains(got, "binance") || !strings.Contains(got, "bybit") {
			t.Fatalf("trial %d: error %q names one venue, want both", i, got)
		}
		if i > 0 && err.Error() != firstMixedMessage {
			t.Fatalf("trial %d: message %q differs from %q — the order is still map-dependent",
				i, err.Error(), firstMixedMessage)
		}
		firstMixedMessage = err.Error()
	}
	// The positive control: the single-venue table this adapter is for still
	// builds, so a constructor that refused everything would not read as a pass.
	if _, err := driven.NewVenuePositions(&recordingPositionReader{}, positionAssets()); err != nil {
		t.Errorf("NewVenuePositions(single-venue table) = %v, want success", err)
	}
}
