package driven

import (
	"context"
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// VenueMarginActs is core §7.1's ACT-side evidence as an ACL adapter: the
// venue's margin-change history translated into this context's port (task #214).
//
// A fifth adapter beside OrderGateway, OrderResolver, VenuePositions and
// VenueMarginStatus, on the argument that split those: the use case that resolves
// a margin op must not be able to place an order or enumerate the account, so the
// type it holds has one method and that method reads one instrument's history.
//
// # The translation this adapter performs is the ONE that must not be got wrong
//
// It converts a `(rows, published, error)` triple into a domain.MarginActHistory,
// and the domain type's three states exist because two of the inputs mean
// opposite things:
//
//	published=false            -> NoMarginActHistory        the venue keeps none
//	published=true, 0 rows     -> PublishedMarginActHistory the venue kept none
//	err != nil                 -> an error, never a history
//
// The middle row is EVIDENCE — it is what turns a moved posted margin into an
// Ambiguous verdict — and the first is the absence of evidence, under which the
// same arithmetic concludes Applied. Every line below exists to keep those apart,
// and the bottom row is the one a hurried version gets wrong: reporting a failed
// read as an empty published history manufactures the middle row out of a
// timeout.
type VenueMarginActs struct {
	client VenueMarginActReader
	assets map[ids.VenueInstrument]venue.InstrumentAssets
}

// VenueMarginActReader is the venue-side capability this adapter needs, declared
// where it is CONSUMED. A composition root satisfies it with *venue.OrderClient
// and never names this interface.
type VenueMarginActReader interface {
	MarginActs(ctx context.Context, instrument ids.VenueInstrument, assets venue.InstrumentAssets,
		w venue.MarginActWindow, adds bool, urgency ratelimit.Priority) ([]venue.MarginAct, bool, error)
}

// NewVenueMarginActs builds the adapter.
//
// The assets table and its two refusals are NewVenueMarginStatus's, unchanged and
// for its reasons: an instrument missing from it cannot be read at all, because
// the venue's decimal strings cannot be typed without its assets, and a
// mixed-venue table lets a caller name venue B's instrument against venue A's
// client and be refused two layers down by route().
func NewVenueMarginActs(client VenueMarginActReader, assets map[ids.VenueInstrument]venue.InstrumentAssets) (*VenueMarginActs, error) {
	if client == nil {
		return nil, ErrVenueMarginActReaderMissing
	}
	if len(assets) == 0 {
		return nil, ErrInstrumentAssetsMissing
	}
	var first ids.Venue
	for instrument, a := range assets {
		if err := a.Validate(); err != nil {
			return nil, fmt.Errorf("instrument %s: %w", instrument.Symbol, err)
		}
		if first == "" {
			first = instrument.Venue
			continue
		}
		if instrument.Venue != first {
			a, b := first, instrument.Venue
			if b < a {
				a, b = b, a
			}
			return nil, fmt.Errorf("instrument table names %s and %s: %w", a, b, ErrInstrumentVenuesMixed)
		}
	}
	return &VenueMarginActs{client: client, assets: assets}, nil
}

// marginActsUrgency is the §7.6 shed class this adapter's calls carry.
//
// Safety, and by a different argument from marginStatusUrgency's. That read is on
// the incident path itself; this one runs inside §7.5's reconcile boot. What makes
// it Safety anyway is what shedding it would produce: a shed read is an ERROR,
// which fails the gate and halts the boot, so the "cheaper" class does not
// degrade the answer — it withholds the whole boot. And the op it resolves is by
// construction one a crash left in flight, so a deployment meeting this call is
// already in the state §7.1 wrote the reserved allocation for.
const marginActsUrgency = ratelimit.PrioritySafety

// MarginActs implements ports.VenueMarginActs.
func (m *VenueMarginActs) MarginActs(instrument ids.VenueInstrument, from, to time.Time) (domain.MarginActHistory, error) {
	if err := instrument.Validate(); err != nil {
		return domain.MarginActHistory{}, err
	}
	assets, ok := m.assets[instrument]
	if !ok {
		// An instrument this deployment did not declare. Refused rather than
		// reported as a venue with no history, which is VenueMarginStatus's rule
		// with a sharper consequence: "no history" is a positive claim about the
		// VENUE, and deriving it from a missing config line would resolve a
		// Binance op by Bybit's contract because somebody forgot a row.
		return domain.MarginActHistory{}, fmt.Errorf("instrument %s: %w",
			instrument.Symbol, ErrMarginInstrumentUndeclared)
	}

	window := venue.MarginActWindow{From: from, To: to}

	// Both directions are fetched and merged rather than the op's own.
	//
	// The port takes no direction, and that is deliberate: §7.1's duplicate case
	// counts rows matching an op, and the domain's match applies the direction
	// itself. Asking the venue for only one direction would be a second place the
	// direction rule lives — and the two places would disagree silently, because
	// a wrong narrowing here returns FEWER rows, which reads as a venue that
	// recorded nothing rather than as a bug.
	adds, err := m.read(instrument, assets, window, true)
	if err != nil {
		return domain.MarginActHistory{}, err
	}
	if !adds.published {
		// The venue keeps no act history. Asked once and answered from a declared
		// capability rather than from a call, so the second read is skipped: it
		// would answer the same way, and issuing it would spend budget to
		// re-derive a documented fact.
		return domain.NoMarginActHistory(), nil
	}
	reduces, err := m.read(instrument, assets, window, false)
	if err != nil {
		return domain.MarginActHistory{}, err
	}

	rows := make([]domain.MarginAct, 0, len(adds.rows)+len(reduces.rows))
	rows = append(rows, adds.rows...)
	rows = append(rows, reduces.rows...)

	hist := domain.PublishedMarginActHistory(rows)
	if err := hist.Validate(); err != nil {
		// The PORT's checks, run here rather than inherited from the engine's.
		// This adapter's reader is an interface, so what reaches this line is
		// whatever satisfies it — the real client validates first, and nothing
		// about the seam requires that.
		return domain.MarginActHistory{}, fmt.Errorf("margin acts for %s: %w", instrument.Symbol, err)
	}
	return hist, nil
}

// marginActPage is one direction's answer, kept as a named pair so the
// published bit cannot be dropped on the way back up.
type marginActPage struct {
	rows      []domain.MarginAct
	published bool
}

// read fetches and translates one direction.
func (m *VenueMarginActs) read(instrument ids.VenueInstrument, assets venue.InstrumentAssets,
	w venue.MarginActWindow, adds bool) (marginActPage, error) {

	// Not cancellable, on VenueMarginStatus's argument: cancelling converts "the
	// venue was about to state which acts it recorded" into "the act history was
	// not answered", and here the difference between those is a verdict.
	acts, published, err := m.client.MarginActs(context.Background(), instrument, assets, w, adds, marginActsUrgency)
	if err != nil {
		// Unwrapped: engine/venue already classified the effect, and a read that
		// failed must NEVER be re-derived into an empty history.
		return marginActPage{}, err
	}
	if !published {
		return marginActPage{published: false}, nil
	}

	rows := make([]domain.MarginAct, 0, len(acts))
	for i, act := range acts {
		cause, err := marginActCause(act.Cause)
		if err != nil {
			return marginActPage{}, fmt.Errorf("margin act %d for %s: %w", i, instrument.Symbol, err)
		}
		rows = append(rows, domain.MarginAct{
			Amount: act.Amount,
			Adds:   act.Adds,
			At:     act.At,
			Cause:  cause,
		})
	}
	return marginActPage{rows: rows, published: true}, nil
}

// marginActCause translates the engine's cause into the domain's.
//
// Exhaustive over the engine enum-like (§3.6 rule 10) and deliberately NOT a
// string cast: the two vocabularies happen to spell their members the same way
// today, and a cast would make that coincidence load-bearing — a renamed engine
// member would then arrive in the domain as an undeclared value rather than as a
// compile error.
func marginActCause(c venue.MarginActCause) (domain.MarginActCause, error) {
	switch c {
	case venue.MarginActCauseUserAdjust:
		return domain.MarginActUserAdjust, nil
	case venue.MarginActCauseOther:
		return domain.MarginActUnattributable, nil
	case venue.MarginActCauseUnset:
		return "", domain.ErrMarginActCauseUnset
	}
	return "", fmt.Errorf("%q: %w", string(c), domain.ErrMarginActCauseInvalid)
}

var _ ports.VenueMarginActs = (*VenueMarginActs)(nil)
