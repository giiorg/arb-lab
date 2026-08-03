package domain

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// ReductionProducer names which mechanism reserved a ledger entry.
//
// §6.7's reservation-aware sizing names the producers: "a PackageDelever's
// in-progress reserved second leg, a protective order's leg reservation", plus
// the determined branch's own fenced reduction generation and the
// deadline-exceeded IntervalFloorReduction. Three of those four are declared
// here — see the note below the constants for the fourth, which is deliberately
// absent.
//
// The kind is carried rather than inferred because §6.7's rule for WHEN an entry
// is backed differs per producer, and reading one producer's rule onto another
// is itself a documented way to get this wrong (SM §2 Invariants). It is also
// the seam FM-9's netting needs: the premise quantity is the residual with ONE
// producer's own entry removed from the sum, which is only expressible if the
// sum knows which entry belongs to whom.
type ReductionProducer string

const (
	// ProducerExposureReduction is the determined branch's fenced reduction
	// generation — §6.7's "tracked as a fenced reduction generation — coalesce /
	// amend / cancel-replace it, never a fresh command per event".
	ProducerExposureReduction ReductionProducer = "exposure_reduction"

	// ProducerIntervalFloor is the deadline-exceeded branch's
	// IntervalFloorReduction. §6.7: "before submission it reserves its own
	// generation, which subsequent net-delta recomputation counts in
	// outstanding_reserved_reductions — so a still-in-flight interval-floor order
	// is not re-read as fresh naked exposure".
	ProducerIntervalFloor ReductionProducer = "interval_floor"

	// ProducerPackageDeleverSpot is a PackageDelever generation's reserved spot
	// leg — SM §7's "atomically reserved part of the fenced generation", and the
	// producer FM-5, FM-8 and FM-9 are all about.
	ProducerPackageDeleverSpot ReductionProducer = "package_delever_spot"
)

// (There is deliberately NO member for a protective order's leg reservation,
// which §6.7 names as the fourth producer. SM §5 owns protective orders and no
// code in this milestone arms one, so the member would be a value nothing ever
// emits — which the compiler accepts, exhaustiveswitch accepts, and every
// switch-coverage count accepts, while the state it names is never reported.
// This repo has that shape written down; the member lands with its emitter.)

// Validate reports whether p is one of the declared producers.
func (p ReductionProducer) Validate() error {
	switch p {
	case ProducerExposureReduction, ProducerIntervalFloor, ProducerPackageDeleverSpot:
		return nil
	}
	return fmt.Errorf("%q: %w", string(p), ErrReductionProducerInvalid)
}

func (p ReductionProducer) String() string { return string(p) }

// ReservationID identifies one entry in the reserved-reduction ledger. It is the
// reserving generation's own handle: §6.7 requires a reduction to be amended and
// cancel-replaced rather than re-issued, so the entry has to outlive any single
// order.
type ReservationID string

// ReservedReductions is §6.7's `outstanding_reserved_reductions` — the
// SUBTRAHEND of the classifier's residual, and the shared surface on which
// formal model (a) found three of its five defects.
//
// # An entry is CREATED AT ZERO, and that is FM-5 made inexpressible
//
// §6.7's determined branch is sized from `realized_delta −
// outstanding_reserved_reductions`, unconditionally. FM-5 is what that
// unconditional subtraction costs when an entry is booked before its backing
// exists: a PackageDelever generation booking its TARGET at Planned — before its
// perp leg has bought anything — makes a neutral package read as SHORT by that
// amount, so the determined branch buys back exactly the exposure the
// generation's own perp leg is about to buy. Every invariant that would object
// is computed FROM this ledger, so a corrupt ledger corrupts the objection too:
// an invariant built on the ledger cannot audit the ledger.
//
// The normative fix (v0.9.24) is that an entry carries its producer's
// OUTSTANDING NET INTENT — "the exposure change its reserving action has already
// COMMITTED and not yet REALIZED, never a target it merely intends".
//
// Reserve therefore takes NO amount. In AmbiguousOrder.tla the nominal reading
// is literally `Pend(0, "pkgspot", FALSE)` and the control that IS FM-5
// (`RESERVE_AT_CREATION = TRUE`) is the branch booking the target; here that
// branch has no spelling. An entry becomes nonzero only through Amend, which is
// the producer stating what it currently has outstanding.
//
// The one producer nonzero from its first instant — the IntervalFloorReduction,
// which carries "its full dispatched size from dispatch" — reserves and then
// amends, and the two-step is what §6.7 asks for rather than a tax: the
// reservation must exist BEFORE submission, so reserve → amend → submit is the
// mandated order made visible.
//
// # What this type does NOT check, stated so it is not mistaken for checked
//
// Whether the amount a producer amends to is TRULY backed by what it has
// committed. The producer is outside this ledger and this ledger has no way to
// see its legs. What the constructor removes is the one misuse the spec's own
// wording invited and four review rounds read past.
//
// # Not safe for concurrent use
//
// The Ledger's argument, unchanged: serialisation is decided one layer out by
// §5.1's OCC version, and a mutex here would make two writers safe from the race
// detector while leaving them exactly as wrong about the version.
type ReservedReductions struct {
	asset   fixedpoint.Asset
	entries map[ReservationID]*reservationEntry
}

type reservationEntry struct {
	producer    ReductionProducer
	outstanding fixedpoint.Quantity
}

// NewReservedReductions returns an empty ledger for one hedge group's base
// asset.
//
// The asset is fixed at construction so that Outstanding on an empty ledger
// answers zero OF THAT ASSET rather than fixedpoint's untagged zero — which
// would fail its first Sub and make the first classification of a group with no
// reservations the one that cannot compute.
func NewReservedReductions(asset fixedpoint.Asset) *ReservedReductions {
	return &ReservedReductions{
		asset:   asset,
		entries: make(map[ReservationID]*reservationEntry),
	}
}

// Reserve creates an entry, outstanding ZERO. See the type doc: the absence of
// an amount parameter is the whole of FM-5's fix.
//
// A repeat reservation is refused rather than resolved either way, on
// Ledger.Register's argument: replacing the entry discards an outstanding
// commitment whose order is live at the venue, and keeping it silently attaches
// a second generation to the first one's amount. Both are recoveries that look
// like they worked.
func (r *ReservedReductions) Reserve(id ReservationID, producer ReductionProducer) error {
	if id == "" {
		return ErrReservationUnset
	}
	if err := producer.Validate(); err != nil {
		return fmt.Errorf("reservation %q: %w", id, err)
	}
	if _, ok := r.entries[id]; ok {
		return fmt.Errorf("reservation %q: %w", id, ErrDuplicateReservation)
	}
	r.entries[id] = &reservationEntry{
		producer:    producer,
		outstanding: fixedpoint.ZeroQuantity(r.asset),
	}
	return nil
}

// Amend sets an entry's outstanding net intent — the exposure change its
// producer has COMMITTED and not yet REALIZED.
//
// It is a set rather than an add, deliberately. §6.7 says "coalesce / amend /
// cancel-replace it, never a fresh command per event", and every producer knows
// its own outstanding amount directly: a PackageDelever generation carries the
// confirmed perp reduction not yet matched by its sale, and once it authorizes a
// spot sale of s it carries s — which is why an additive API would be wrong for
// the second half of that ladder, not merely awkward.
//
// The quantity is SIGNED. A reserved reduction of a long position is negative
// exposure change and one of a short is positive, and the classifier subtracts
// whatever is here — the sign is the producer's statement about direction, not a
// convention this type imposes.
//
// Zero is legal and is not the same as released: a generation that has committed
// nothing yet still holds its slot, which is what stops a second generation
// opening against the same package.
func (r *ReservedReductions) Amend(id ReservationID, outstanding fixedpoint.Quantity) error {
	entry, ok := r.entries[id]
	if !ok {
		return fmt.Errorf("reservation %q: %w", id, ErrUnknownReservation)
	}
	if err := outstanding.Validate(); err != nil {
		return fmt.Errorf("reservation %q outstanding: %w", id, err)
	}
	if outstanding.Asset() != r.asset {
		return fmt.Errorf("reservation %q outstanding is %s, ledger holds %s: %w",
			id, outstanding.Asset(), r.asset, ErrReservationAssetMismatch)
	}
	entry.outstanding = outstanding
	return nil
}

// Release removes an entry, reporting the amount that stopped being subtracted.
//
// The returned quantity is why this is not a void method. §6.7 row 4 releases a
// reduction generation whose OWN result came back ambiguous — "it never blind
// retries, and its own ambiguous result routes through this same §6.7 finality
// contract" — and the exposure it was covering does not disappear at that
// moment: it re-enters as an unresolved ORDER contributing a possible-delta
// interval. A caller that could not see the amount could not re-enter it, and
// the group would silently lose exposure it still holds. That release is also
// half of FM-8: what a released entry leaves behind is a PackageDelever
// generation still running on a premise that has stopped holding, and nothing
// here can tell it — SM §7's ladder owns that half.
func (r *ReservedReductions) Release(id ReservationID) (fixedpoint.Quantity, error) {
	entry, ok := r.entries[id]
	if !ok {
		return fixedpoint.Quantity{}, fmt.Errorf("reservation %q: %w", id, ErrUnknownReservation)
	}
	delete(r.entries, id)
	return entry.outstanding, nil
}

// Holds reports whether the ledger has an entry under this id.
func (r *ReservedReductions) Holds(id ReservationID) bool {
	_, ok := r.entries[id]
	return ok
}

// Count reports how many entries are outstanding, of any amount.
//
// It is not derivable from Outstanding and the difference is the case it exists
// for: entries of +1 and −1 sum to zero while two generations are live. §6.7's
// "never issued twice" and SM §7's "only after the prior generation reaches
// finality" are claims about ENTRIES, and a caller answering them from the sum
// would read two live generations as none.
func (r *ReservedReductions) Count() int { return len(r.entries) }

// Outstanding is the sum every classification subtracts: §6.7's
// `outstanding_reserved_reductions`.
func (r *ReservedReductions) Outstanding() (fixedpoint.Quantity, error) {
	return r.sum(func(*reservationEntry) bool { return true })
}

// OutstandingFor is the same sum restricted to one producer.
//
// It exists for FM-9's shape rather than for reporting. The premise a
// PackageDelever generation opens under must be re-evaluated NET OF THAT
// GENERATION'S OWN LEGS — "a rule evaluated on a quantity that includes its own
// subject" fires on its own subject's behaviour — and netting requires the sum
// to be decomposable by producer. SM §7's ladder, which is the consumer, is not
// built here; this is the seam it needs, and it is exercised by Outstanding's
// own tests rather than left as an untested accessor.
func (r *ReservedReductions) OutstandingFor(producer ReductionProducer) (fixedpoint.Quantity, error) {
	if err := producer.Validate(); err != nil {
		return fixedpoint.Quantity{}, err
	}
	return r.sum(func(e *reservationEntry) bool { return e.producer == producer })
}

func (r *ReservedReductions) sum(include func(*reservationEntry) bool) (fixedpoint.Quantity, error) {
	total := fixedpoint.ZeroQuantity(r.asset)
	for id, entry := range r.entries {
		if !include(entry) {
			continue
		}
		next, err := total.Add(entry.outstanding)
		if err != nil {
			return fixedpoint.Quantity{}, fmt.Errorf("summing reservation %q: %w", id, err)
		}
		total = next
	}
	return total, nil
}
