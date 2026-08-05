package driven

import (
	"context"
	"errors"
	"fmt"

	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// SpotBalances is §7.5's spot balance surface as an ACL adapter: engine/venue's
// wallet enumeration translated into this context's port (task #179, record
// §70).
//
// A fourth adapter beside OrderGateway, OrderResolver and VenuePositions, on
// the argument that split those: the use case that compares a wallet against a
// recorded baseline must hold a type whose one method reads balances, so it
// cannot re-resolve an order or re-enumerate positions to explain a delta
// away.
type SpotBalances struct {
	client  SpotBalanceReader
	venue   ids.Venue
	urgency ratelimit.Priority
}

// SpotBalanceReader is the venue-side capability this adapter needs, declared
// where it is CONSUMED (VenuePositionReader's shape, one surface over).
type SpotBalanceReader interface {
	SpotBalances(ctx context.Context, urgency ratelimit.Priority) (venue.SpotBalanceReport, error)
}

// NewSpotBalances builds the adapter.
//
// # The priority is a CONSTRUCTOR argument, and this is the first read adapter
// # where it has to be
//
// §7.6 puts read priority at the call site, and this port is the first read
// surface with two call sites on different rungs: the reconcile boot's
// assertion is Safety-class (§7.6 names §7.5's reconcile in the Safety row),
// while `arblab baseline record` is an operator maintenance read that must not
// spend the emergency reserve — Routine. VenuePositions can pin a const
// because reconcile is its only consumer; here a const would silently put one
// caller on the wrong rung, and the WRONG wrong direction is cheap-looking (a
// record command riding Safety works fine until the incident where the reserve
// it drained was the flatten's). The zero value is refused so a wiring that
// forgot to choose does not inherit either rung.
func NewSpotBalances(client SpotBalanceReader, v ids.Venue, urgency ratelimit.Priority) (*SpotBalances, error) {
	if client == nil {
		return nil, ErrSpotBalanceReaderMissing
	}
	if err := v.Validate(); err != nil {
		return nil, fmt.Errorf("spot balances venue: %w", err)
	}
	if err := urgency.Validate(); err != nil {
		return nil, fmt.Errorf("spot balances urgency: %w", err)
	}
	return &SpotBalances{client: client, venue: v, urgency: urgency}, nil
}

// Balances implements ports.SpotBalances.
func (b *SpotBalances) Balances(scope ports.SpotBalanceScope) (ports.BalanceReport, error) {
	if err := scope.Validate(); err != nil {
		return ports.BalanceReport{}, err
	}
	if scope.Venue != b.venue {
		// One adapter speaks for one venue's client, and a scope naming
		// another would be answered by the wrong exchange entirely —
		// VenuePositions' venue check, without the assets table it derives its
		// own answer from.
		return ports.BalanceReport{}, fmt.Errorf("scope %s: %w", scope, ErrSpotBalanceVenueMismatch)
	}

	// Not cancellable, on VenuePositions' argument: cancelling converts "the
	// venue was about to say what the wallet holds" into "the enumeration was
	// not answered", and the second refuses to conclude the assertion.
	report, err := b.client.SpotBalances(context.Background(), b.urgency)
	if err != nil {
		// Unwrapped: engine/venue already classified the effect, and an
		// enumeration that failed must not be re-derived into an empty one.
		return ports.BalanceReport{}, err
	}

	absence, err := absenceSemanticsOf(report.Absence)
	if err != nil {
		return ports.BalanceReport{}, fmt.Errorf("spot balances on %s: %w", scope.Venue, err)
	}
	out := ports.BalanceReport{
		ObservedAt: report.ObservedAt,
		Rows:       make([]ports.AssetBalance, 0, len(report.Rows)),
		Unpinned:   make([]ports.UnpinnedBalance, 0, len(report.Unpinned)),
		Absence:    absence,
	}
	for _, row := range report.Rows {
		words := make([]ports.VerbatimField, 0, len(row.Words))
		for _, w := range row.Words {
			words = append(words, ports.VerbatimField{Name: w.Name, Value: w.Value})
		}
		out.Rows = append(out.Rows, ports.AssetBalance{
			Asset: row.Asset,
			Total: row.Total,
			Words: words,
		})
	}
	for _, row := range report.Unpinned {
		reason, err := unpinnedReasonOf(row.Reason)
		if err != nil {
			return ports.BalanceReport{}, fmt.Errorf("spot balance %s on %s: %w", row.Asset, scope.Venue, err)
		}
		out.Unpinned = append(out.Unpinned, ports.UnpinnedBalance{Asset: row.Asset, Reason: reason})
	}
	return out, nil
}

// unpinnedReasonOf translates the engine's reason into the port's.
//
// Two enums exist because the two layers may not import each other's
// (hexboundary both ways at once: ports cannot see engine, engine cannot see
// ports), and the translation is a switch so exhaustiveswitch makes a member
// added on either side loud here rather than silently dropped.
func unpinnedReasonOf(r venue.SpotUnpinnedReason) (ports.UnpinnedReason, error) {
	switch r {
	case venue.SpotUnpinnedLockedSplit:
		return ports.UnpinnedLockedSplitUnverified, nil
	case venue.SpotUnpinnedBorrow:
		return ports.UnpinnedBorrowOutstanding, nil
	case venue.SpotUnpinnedReasonUnspecified:
		// Named rather than left to the trailing return (§3.6 rule 10): the
		// zero reason is a row the codec never classified, which is a distinct
		// fact from a value out of roster.
		return ports.UnpinnedReasonUnspecified, r.Validate()
	}
	return ports.UnpinnedReasonUnspecified, r.Validate()
}

// absenceSemanticsOf translates the engine's absence statement into the
// port's, on unpinnedReasonOf's argument: two enums because the layers may not
// import each other, a switch so exhaustiveswitch makes a member added on
// either side loud here.
func absenceSemanticsOf(s venue.SpotAbsenceSemantics) (ports.AbsenceSemantics, error) {
	switch s {
	case venue.SpotAbsenceZero:
		return ports.AbsenceIsZero, nil
	case venue.SpotAbsenceInconclusive:
		return ports.AbsenceInconclusive, nil
	case venue.SpotAbsenceUnspecified:
		// Named rather than left to the trailing return (§3.6 rule 10): the
		// zero value is a report that never stated its silences' meaning,
		// distinct from a value out of roster.
		return ports.AbsenceSemanticsUnspecified, s.Validate()
	}
	return ports.AbsenceSemanticsUnspecified, s.Validate()
}

// The adapter's own refusals.
var (
	// ErrSpotBalanceReaderMissing reports an adapter constructed with nothing
	// behind it.
	ErrSpotBalanceReaderMissing = errors.New("oms: spot balance reader missing")

	// ErrSpotBalanceVenueMismatch reports a scope naming a venue this adapter
	// does not speak for.
	ErrSpotBalanceVenueMismatch = errors.New("oms: spot balance scope names a different venue than this adapter")
)

// Compile-time proof that the adapter satisfies the port (§3.6 rule 2).
var _ ports.SpotBalances = (*SpotBalances)(nil)
