package replay

import (
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// Calibrate measures each roster venue's server-clock offset from the tape
// itself: the maximum of (SourceTime − RecvTime) over that venue's emitted
// readings, floored at zero.
//
// # Why a measurement pass exists at all
//
// §39p-8 first chose zero offsets — "skew modelling is not this slice's
// claim" — and the fixture refused it on first contact: Bybit's ticker stamp
// runs ~20 ms ahead of the laptop's clock, so a zero-offset model computes a
// NEGATIVE data age and marketdata correctly refuses the reading as a clock
// inversion (§3.6 rule 5's defect, named). The sim cannot wave that off —
// the refusal is the freshness seam working — so the model must carry the
// skew that is really in the data.
//
// # Why this bound is honest
//
// A message cannot arrive before it was stamped, so venue_stamp − our_recv is
// a LOWER bound on (venue clock − our clock) for every message; the maximum
// over the window is the tightest such bound the archive supports. Using it
// as the offset makes the worst reading's data age exactly zero and every
// other one positive — no inversion, no pretence of knowing the skew better
// than the data does. It is a pure function of the tape, so both Generate
// runs and the Rerun see one model, which is all the parity claim needs.
// The live pipeline's calibration is a different instrument (venue
// server-time endpoints, milestone 3's clients); this one is the replay's,
// scoped to it.
func Calibrate(t *Tape, specs []StreamSpec) ([]clock.Calibration, error) {
	ing, err := NewIngester(specs)
	if err != nil {
		return nil, err
	}

	// Every roster venue gets an entry — a venue whose readings all lag our
	// clock still needs a calibration row, or ServerNow returns the unset
	// ServerTime and the data-age question becomes unanswerable.
	ahead := map[ids.Venue]time.Duration{}
	var venues []ids.Venue
	for _, s := range specs {
		v := s.Key.Source.Instrument.Venue
		if _, ok := ahead[v]; !ok {
			ahead[v] = 0
			venues = append(venues, v)
		}
	}

	for _, e := range t.entries {
		env, emitted, err := ing.Ingest(e)
		if err != nil {
			return nil, fmt.Errorf("replay: calibrate: %w", err)
		}
		if !emitted || env.SourceTime.IsZero() {
			continue
		}
		v := env.Source.Instrument.Venue
		if d := env.SourceTime.Sub(env.RecvTime); d > ahead[v] {
			ahead[v] = d
		}
	}

	cals := make([]clock.Calibration, 0, len(venues))
	for _, v := range venues {
		cals = append(cals, clock.Calibration{Venue: v, Offset: ahead[v], Uncertainty: time.Millisecond})
	}
	return cals, nil
}
