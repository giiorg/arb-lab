package crossvenue

// Decision-point sampling (§45e): the gate's entry/exit signal is what the
// capture's funding streams SHOWED at the instant — Binance's markPrice
// estimate and Bybit's ticker estimate — never the settled rate, which is
// the ledger's truth. Sample instants are the UTC epoch grid at Every; the
// estimate "at t" is the last frame received at or before t, required
// fresh within Slack, with RecvTime the anchor on both venues so the two
// legs' instants are the same question (family2's stated pairing rule).
//
// A grid instant landing exactly on a frame's RecvTime is sampled on the
// NEXT frame with the prior state as "at t" — the no-lookahead convention
// family2 committed; the boundary fixture pins it here too.

import (
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/capture"
	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	dcv "github.com/giiorg/arb-lab/internal/strategy/domain/crossvenue"
)

// ErrSampleParamsUnset reports a sampling procedure that is not completely
// declared — an unset cadence or slack would sample nothing and read as a
// quiet market.
var ErrSampleParamsUnset = fmt.Errorf("strategy/crossvenue: sampling cadence/slack unset")

// SampleParams is the committed sampling procedure. Values are slice-D
// parameters; this package only refuses an undeclared one.
type SampleParams struct {
	// Every is the sampling cadence on the UTC epoch grid.
	Every time.Duration

	// Slack bounds the sampled estimate's staleness at the grid instant.
	Slack time.Duration
}

// Validate reports whether the procedure is completely declared.
func (p SampleParams) Validate() error {
	if p.Every <= 0 || p.Slack <= 0 {
		return ErrSampleParamsUnset
	}
	return nil
}

// EstimatePoint is one sampled decision-time estimate.
type EstimatePoint struct {
	At   time.Time
	Rate fixedpoint.Rate
}

// EstimateSeries is one stream's sampled estimates plus the coverage
// counts that keep a thin walk from reading as a quiet market.
type EstimateSeries struct {
	Points []EstimatePoint

	// Frames counts decoded data frames; Skipped counts grid instants
	// with no sufficiently fresh estimate (cold starts, gaps, stale
	// holds) — dropped and counted, never defaulted.
	Frames  int
	Skipped int
}

// SampleBinanceEstimates samples the markPrice stream's live funding
// estimate on the epoch grid.
func SampleBinanceEstimates(root string, days []ManifestDay, key capture.StreamKey, p SampleParams) (EstimateSeries, error) {
	if err := p.Validate(); err != nil {
		return EstimateSeries{}, err
	}
	out := EstimateSeries{}

	type held struct {
		rate fixedpoint.Rate
		recv time.Time
	}
	var cur *held
	var lastGrid time.Time

	err := scanStream(root, key, days, func() { cur = nil }, func(rec capture.Record) error {
		switch rec.Kind {
		case capture.RecordKindGap:
			cur = nil
			return nil
		case capture.RecordKindData:
		case capture.RecordKindUnspecified, capture.RecordKindOpen, capture.RecordKindAlive,
			capture.RecordKindClose, capture.RecordKindRest, capture.RecordKindClockStep:
			// Framing and liveness records carry no frame to decode.
			return nil
		}
		m, err := venue.DecodeBinanceMarkPrice(rec.Payload)
		if err != nil {
			return fmt.Errorf("%s at %s: %w", key, rec.RecvTime.Format(time.RFC3339Nano), err)
		}
		out.Frames++

		lastGrid = sampleGrid(rec.RecvTime, lastGrid, p, func(t time.Time) {
			if cur == nil || t.Sub(cur.recv) > p.Slack {
				out.Skipped++
				return
			}
			out.Points = append(out.Points, EstimatePoint{At: t, Rate: cur.rate})
		})

		cur = &held{rate: m.Rate, recv: rec.RecvTime}
		return nil
	})
	if err != nil {
		return EstimateSeries{}, err
	}
	return out, nil
}

// SampleBybitEstimates samples the tickers stream's live funding estimate
// on the epoch grid. The ticker wire is absent-means-unchanged, so the
// stream is consumed through the venue's own accumulator, restarted cold
// at every file boundary and recorded gap — carrying ticker state across a
// hole would silently hold a stale rate as fresh.
func SampleBybitEstimates(root string, days []ManifestDay, key capture.StreamKey, p SampleParams) (EstimateSeries, error) {
	if err := p.Validate(); err != nil {
		return EstimateSeries{}, err
	}
	out := EstimateSeries{}

	type held struct {
		rate fixedpoint.Rate
		recv time.Time
	}
	var cur *held
	var lastGrid time.Time
	state := venue.NewBybitTickerState()

	err := scanStream(root, key, days, func() {
		state = venue.NewBybitTickerState()
		cur = nil
	}, func(rec capture.Record) error {
		switch rec.Kind {
		case capture.RecordKindGap:
			state = venue.NewBybitTickerState()
			cur = nil
			return nil
		case capture.RecordKindData:
		case capture.RecordKindUnspecified, capture.RecordKindOpen, capture.RecordKindAlive,
			capture.RecordKindClose, capture.RecordKindRest, capture.RecordKindClockStep:
			// Framing and liveness records carry no frame to decode.
			return nil
		}
		upd, emitted, err := state.Apply(rec.Payload)
		if err != nil {
			return fmt.Errorf("%s at %s: %w", key, rec.RecvTime.Format(time.RFC3339Nano), err)
		}
		out.Frames++

		lastGrid = sampleGrid(rec.RecvTime, lastGrid, p, func(t time.Time) {
			if cur == nil || t.Sub(cur.recv) > p.Slack {
				out.Skipped++
				return
			}
			out.Points = append(out.Points, EstimatePoint{At: t, Rate: cur.rate})
		})

		if emitted {
			cur = &held{rate: upd.Funding.Rate, recv: rec.RecvTime}
		} else if cur != nil {
			// A no-news frame (bid/ask churn) still testifies the stream
			// is alive and the held rate unchanged — the venue's
			// absent-means-unchanged semantics — so it refreshes the
			// anchor rather than letting a stable rate go stale.
			cur = &held{rate: cur.rate, recv: rec.RecvTime}
		}
		return nil
	})
	if err != nil {
		return EstimateSeries{}, err
	}
	return out, nil
}

// sampleGrid advances the epoch grid to just before recv, invoking sample
// for each instant strictly between the previous grid mark and recv. An
// instant equal to recv is left for the NEXT frame (the current state is
// still "at t" there) — family2's committed convention.
func sampleGrid(recv time.Time, lastGrid time.Time, p SampleParams, sample func(t time.Time)) time.Time {
	grid := recv.Truncate(p.Every)
	if lastGrid.IsZero() {
		return grid
	}
	for t := lastGrid.Add(p.Every); !t.After(recv); t = t.Add(p.Every) {
		if t.Equal(recv) {
			break
		}
		lastGrid = t
		sample(t)
	}
	return lastGrid
}

// MergeStats reports the decision-point join's coverage.
type MergeStats struct {
	Joined      int
	BinanceOnly int
	BybitOnly   int
}

// MergeDecisionPoints joins the two venues' sampled estimates on equal
// grid instants into the decision series: signal = Binance − Bybit,
// Match.Spread's orientation. An instant only one venue sampled is
// dropped and counted — a one-legged spread is not a spread.
func MergeDecisionPoints(binance, bybit []EstimatePoint) ([]dcv.DecisionPoint, MergeStats, error) {
	var out []dcv.DecisionPoint
	var stats MergeStats
	i, j := 0, 0
	for i < len(binance) && j < len(bybit) {
		b, y := binance[i], bybit[j]
		switch {
		case b.At.Before(y.At):
			stats.BinanceOnly++
			i++
		case y.At.Before(b.At):
			stats.BybitOnly++
			j++
		default:
			signal, err := b.Rate.Sub(y.Rate)
			if err != nil {
				return nil, MergeStats{}, fmt.Errorf("decision signal at %s: %w", b.At.Format(time.RFC3339Nano), err)
			}
			out = append(out, dcv.DecisionPoint{At: b.At, Signal: signal})
			stats.Joined++
			i++
			j++
		}
	}
	stats.BinanceOnly += len(binance) - i
	stats.BybitOnly += len(bybit) - j
	return out, stats, nil
}
