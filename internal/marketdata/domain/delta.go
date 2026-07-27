package domain

import (
	"fmt"
	"sort"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// LevelChange is one instruction from a venue's incremental depth feed: set the
// quantity resting at a price on a side.
//
// # Zero quantity means REMOVE, and that is why it is a separate type
//
// A BookLevel refuses a zero quantity (ErrLevelQuantityNotPositive) and a
// LevelChange requires one to be legal, because the two say different things: a
// level is a state and a change is an instruction, and "there is nothing here"
// is expressible only as the second. Collapsing them into one type — the obvious
// simplification, since the fields are identical — is what puts a removal into a
// published book as a resting order of size zero.
type LevelChange struct {
	// Side is which half of the book to change.
	Side BookSide

	// Price identifies the level. It is an identity here, not a measurement:
	// the venue's depth feed keys on it, so a price that does not match an
	// existing level exactly is an insert rather than an update.
	Price fixedpoint.Price

	// Quantity is the new resting size at Price, where ZERO means remove the
	// level. Negative is refused: no venue publishes it, and the reading that
	// would let it through — treating it as a delta to apply arithmetically —
	// is a different feed protocol than the one §3.4 describes.
	Quantity fixedpoint.Quantity
}

// Validate reports whether the change is one a venue could have sent.
func (c LevelChange) Validate() error {
	if err := c.Side.Validate(); err != nil {
		return fmt.Errorf("level change side: %w", err)
	}
	if err := c.Price.Validate(); err != nil {
		return fmt.Errorf("level change price: %w", err)
	}
	if err := c.Quantity.Validate(); err != nil {
		return fmt.Errorf("level change quantity: %w", err)
	}
	sign, err := c.Quantity.Sign()
	if err != nil {
		return fmt.Errorf("level change quantity: %w", err)
	}
	if sign < 0 {
		return fmt.Errorf("%s at %s: %w", c.Quantity, c.Price, ErrChangeQuantityNegative)
	}
	return nil
}

// Removes reports whether this change deletes the level rather than resting size
// at it.
//
// A named predicate rather than `Sign() == 0` at each call site: the encoding is
// the one thing about this type a reader has to be told, and a bare comparison
// against zero is where it goes untold.
func (c LevelChange) Removes() (bool, error) {
	sign, err := c.Quantity.Sign()
	if err != nil {
		return false, fmt.Errorf("level change quantity: %w", err)
	}
	return sign == 0, nil
}

// BookDelta is one incremental update from a venue's depth feed, normalized to
// a sequence SPAN.
//
// # The span is FirstSeq..LastSeq because venues disagree about how many they send
//
// Binance's depth events carry both a first and a final update ID and may cover
// several venue sequences in one message; Bybit's carry a single one. Normalizing
// to a span with First == Last for the single-sequence venues is the shape that
// holds both, and it is `engine/venue`'s job to produce it. The alternative —
// one sequence field, with multi-sequence venues reporting only their last — is
// what makes the overlap case below inexpressible, and overlap is the NORMAL
// state immediately after a resnapshot.
//
// # A delta never reaches a lane
//
// §3.4 forbids coalescing or drop-oldest on raw snapshot+delta traffic, because
// dropping a delta desyncs the book. What is published is the BookSnapshot that
// applying this produced.
type BookDelta struct {
	// Instrument is the book this updates. Checked against the book's own on
	// Apply — see ErrBookInstrumentMismatch.
	Instrument ids.VenueInstrument

	// Changes are the level instructions, and MAY be empty. An empty delta with
	// a sequence span is a heartbeat, which real venues send and which carries
	// real information: it advances the sequence, so refusing it would
	// manufacture a gap out of a healthy feed.
	Changes []LevelChange

	// FirstSeq and LastSeq are the venue sequence span this message covers,
	// inclusive. FirstSeq is non-zero and LastSeq is at least FirstSeq.
	FirstSeq uint64
	LastSeq  uint64

	// Watermark is the delivery's provenance. Its VenueSeq MUST equal LastSeq —
	// see ErrDeltaSeqDisagrees for why both spellings exist and why they are
	// required to agree.
	Watermark Watermark
}

// Validate reports whether the delta is one a venue could have sent.
func (d BookDelta) Validate() error {
	if err := d.Instrument.Validate(); err != nil {
		return fmt.Errorf("book delta instrument: %w", err)
	}
	if err := d.Watermark.Validate(); err != nil {
		return fmt.Errorf("book delta watermark: %w", err)
	}
	if d.FirstSeq == 0 {
		return fmt.Errorf("instrument %s: %w", d.Instrument, ErrDeltaSeqUnset)
	}
	if d.LastSeq < d.FirstSeq {
		return fmt.Errorf(
			"instrument %s: span %d..%d: %w",
			d.Instrument, d.FirstSeq, d.LastSeq, ErrDeltaSeqInverted)
	}
	if d.Watermark.VenueSeq != d.LastSeq {
		return fmt.Errorf(
			"instrument %s: watermark seq %d, span ends %d: %w",
			d.Instrument, d.Watermark.VenueSeq, d.LastSeq, ErrDeltaSeqDisagrees)
	}
	return d.validateChanges()
}

// validateChanges checks each instruction and refuses two touching one level.
//
// The duplicate scan is O(n²) on the change count, deliberately. A depth message
// carries tens of levels, so the constant is irrelevant; a map keyed on the
// price would need a comparable key, and fixedpoint's canonical scale is an
// implementation detail this package should not be reaching through to build
// one.
func (d BookDelta) validateChanges() error {
	for i, c := range d.Changes {
		if err := c.Validate(); err != nil {
			return fmt.Errorf("change %d: %w", i, err)
		}
		for j := 0; j < i; j++ {
			p := d.Changes[j]
			if p.Side != c.Side {
				continue
			}
			cmp, err := p.Price.Cmp(c.Price)
			if err != nil {
				return fmt.Errorf("change %d: %w", i, err)
			}
			if cmp == 0 {
				return fmt.Errorf(
					"changes %d and %d both set %s %s: %w",
					j, i, c.Side, c.Price, ErrDeltaDuplicateLevel)
			}
		}
	}
	return nil
}

// SeqRelation is where a delta's sequence span sits relative to the last
// sequence already applied to a book.
//
// # Four relations, four different actions, and only one of them is "apply"
//
// The relation exists as a named type rather than a pair of bools because the
// four cases do not collapse: contiguous and overlapping are both applied but
// mean different things about the feed, stale is dropped without complaint, and
// only a gap requires a resnapshot. A bool pair would make two of the sixteen
// combinations meaningless and leave a reader to work out which.
//
// # Integer enum, which is §3.6 rule 8's documented exception
//
// The opposite call from BookSide, one file over, and for the stated reason: a
// relation is computed at a decision point and never stored or transmitted, so
// what matters is that the zero value be an unusable sentinel. It is
// SeqRelationUnknown, and every switch over this type must name it — rule 10
// (exhaustiveswitch) is what makes that a build failure rather than a silent
// fallthrough into the apply path.
type SeqRelation uint8

const (
	// SeqRelationUnknown is the zero value: no relation is computable, which is
	// the honest answer when the book has no prior sequence at all. A book in
	// that state is awaiting its first snapshot, and a delta against it cannot
	// be placed — not because the delta is bad, but because there is nothing to
	// place it against.
	SeqRelationUnknown SeqRelation = iota

	// SeqRelationStale means the whole span was already applied — an
	// at-least-once transport re-delivering. Dropped, and not an error: it is
	// the ordinary outcome of a redelivery, and counting it as a fault would
	// make a healthy reconnect look like a feed problem.
	SeqRelationStale

	// SeqRelationContiguous means the span begins exactly where the book ends.
	// The normal steady-state case; apply it.
	SeqRelationContiguous

	// SeqRelationOverlap means the span begins at or before the book's position
	// and extends past it, so it contains the next sequence needed.
	//
	// This is the NORMAL state immediately after a resnapshot, not an anomaly:
	// a REST snapshot is taken at some sequence the live stream has already
	// passed, so the first delta that can continue it necessarily straddles that
	// point. A reconstructor that admitted only the contiguous case would
	// re-request a snapshot forever, at a rate that grows with how far behind
	// the REST read lands.
	SeqRelationOverlap

	// SeqRelationGap means the span begins beyond where the book ends: at least
	// one sequence was never delivered. §3.4 requires "automatic resnapshot"
	// here, and §3.5 permits the source to be HELD while that is in flight
	// without stalling dispatch of other sources.
	SeqRelationGap
)

const maxSeqRelation = SeqRelationGap

// Validate reports whether r is a declared value.
func (r SeqRelation) Validate() error {
	if r > maxSeqRelation {
		return fmt.Errorf("sequence relation %d: %w", uint8(r), ErrSeqRelationInvalid)
	}
	return nil
}

// String names the value, for diagnostics and metric labels.
func (r SeqRelation) String() string {
	switch r {
	case SeqRelationUnknown:
		return "SeqRelationUnknown"
	case SeqRelationStale:
		return "SeqRelationStale"
	case SeqRelationContiguous:
		return "SeqRelationContiguous"
	case SeqRelationOverlap:
		return "SeqRelationOverlap"
	case SeqRelationGap:
		return "SeqRelationGap"
	}
	return fmt.Sprintf("SeqRelation(%d)", uint8(r))
}

// Follows places this delta's span relative to prev, the last venue sequence
// applied to the book.
//
// prev is zero when nothing has been applied, and the answer is then
// SeqRelationUnknown rather than a guess: with no position, "contiguous" would
// admit any delta as the start of a book and "gap" would demand a resnapshot the
// caller is already waiting for.
//
// # The four cases are exhaustive over the span, and the boundaries are the test
//
// Given 1 <= FirstSeq <= LastSeq and prev >= 1, exactly one holds: LastSeq <=
// prev (stale), FirstSeq == prev+1 (contiguous), FirstSeq <= prev (overlap), or
// FirstSeq > prev+1 (gap). Note that overlap is UNREACHABLE for a delta whose
// span is a single sequence — First == Last cannot be both at-or-before prev and
// past it — so a fixture built only from single-sequence venues tests three of
// the four cases while appearing to test all of them.
func (d BookDelta) Follows(prev uint64) SeqRelation {
	switch {
	case prev == 0:
		return SeqRelationUnknown
	case d.LastSeq <= prev:
		return SeqRelationStale
	case d.FirstSeq == prev+1:
		return SeqRelationContiguous
	case d.FirstSeq <= prev:
		return SeqRelationOverlap
	default:
		return SeqRelationGap
	}
}

// Apply returns the book that results from applying d to b. The receiver is not
// modified.
//
// # Immutability is enforced by copying, not by convention
//
// §3.4 publishes "the resulting reconstructed immutable snapshot", and a
// published snapshot is held by consumers. Mutating a level in place — which is
// what the natural implementation does, since both slices are already the right
// length — would change a book a strategy is mid-evaluation on, with no data
// race the detector can see, because the reconstructor is single-writer and the
// consumer holds a value whose backing array it shares.
//
// # Every change is applied before anything is checked
//
// The result is validated once, at the end, and that ordering is load-bearing. A
// single delta legitimately adds a bid at a price whose ask it removes in the
// same message — a level crossing the spread as the market moves through it — so
// a book checked after each individual change would refuse a correct feed.
//
// # A failure here means resnapshot, not "drop this delta"
//
// The invariants that can fail are ErrBookCrossed and ErrBookUnordered, and both
// say the same thing: the book and the venue have diverged. Applying the next
// delta to the un-updated book would compound it. The application layer treats
// this exactly as it treats SeqRelationGap.
func (b BookSnapshot) Apply(d BookDelta) (BookSnapshot, error) {
	if err := b.Validate(); err != nil {
		return BookSnapshot{}, fmt.Errorf("apply to book: %w", err)
	}
	if err := d.Validate(); err != nil {
		return BookSnapshot{}, fmt.Errorf("apply delta: %w", err)
	}
	if b.Instrument != d.Instrument {
		return BookSnapshot{}, fmt.Errorf(
			"book is %s, delta is %s: %w", b.Instrument, d.Instrument, ErrBookInstrumentMismatch)
	}

	next := BookSnapshot{
		Instrument: b.Instrument,
		Bids:       append([]BookLevel(nil), b.Bids...),
		Asks:       append([]BookLevel(nil), b.Asks...),
		Watermark:  d.Watermark,
	}
	for i, c := range d.Changes {
		if err := next.applyChange(c); err != nil {
			return BookSnapshot{}, fmt.Errorf("change %d: %w", i, err)
		}
	}
	if err := next.Validate(); err != nil {
		return BookSnapshot{}, fmt.Errorf("book after delta %d..%d: %w", d.FirstSeq, d.LastSeq, err)
	}
	return next, nil
}

// applyChange sets or removes one level on the receiver's own slices.
//
// It mutates, and it is unexported for exactly that reason: the only caller is
// Apply, which owns freshly copied slices. Exporting it would hand a caller a
// method that silently edits a published book.
func (b *BookSnapshot) applyChange(c LevelChange) error {
	var target *[]BookLevel
	switch c.Side {
	case SideBid:
		target = &b.Bids
	case SideAsk:
		target = &b.Asks
	}
	if target == nil {
		// Unreachable: BookDelta.Validate has already refused an undeclared
		// side. Refused rather than assumed, because the alternative is a
		// dereference of nil in the reconstructor's hot path.
		return fmt.Errorf("%q: %w", string(c.Side), ErrSideUnknown)
	}

	idx, found, err := locate(*target, c.Side, c.Price)
	if err != nil {
		return err
	}
	removes, err := c.Removes()
	if err != nil {
		return err
	}

	switch {
	case removes && found:
		*target = append((*target)[:idx], (*target)[idx+1:]...)
	case removes:
		// A removal for a level we do not hold is a no-op, not an error. Venues
		// send them routinely after a resync — the snapshot we started from
		// simply never contained the level being deleted — and refusing would
		// turn the ordinary end of a resync into a desync report.
	case found:
		(*target)[idx] = BookLevel{Price: c.Price, Quantity: c.Quantity}
	default:
		*target = append(*target, BookLevel{})
		copy((*target)[idx+1:], (*target)[idx:])
		(*target)[idx] = BookLevel{Price: c.Price, Quantity: c.Quantity}
	}
	return nil
}

// locate binary-searches one side for a price, returning where it sits or where
// it would be inserted.
//
// # The predicate is strictlyOrders, which is what keeps the search honest
//
// It is the same function Validate uses to check the side's ordering, so the
// search and the invariant cannot drift apart. A search that ascends a
// descending side finds nothing, inserts every level at index zero, and produces
// a book that fails validation somewhere far from the mistake. Its monotonicity
// — the precondition sort.Search needs — is exactly what validateSide has
// already established, which is why Apply validates before it applies.
//
// # The loop belongs to sort.Search, and that is not a style choice
//
// The hand-written version this replaces was three lines shorter and correct.
// It was replaced because a mutation to its advance step (`lo = mid + 1` →
// `lo = mid`) made the SUITE HANG rather than fail: tools/mutate.sh reported
// TIMEOUT!! instead of a kill, and a hanging test names no defect. That left
// "this search terminates" as an unchecked claim sitting under every book
// update. sort.Search owns the loop, so termination is no longer a property this
// package asserts — and a bounded counter, the other candidate fix, would have
// added a branch reachable only from a defect and therefore untestable itself.
//
// The error is hoisted out of the predicate because sort.Search's takes none.
// Returning true on a comparison failure stops the search early rather than
// letting it walk a book whose tags disagree.
func locate(levels []BookLevel, side BookSide, p fixedpoint.Price) (idx int, found bool, err error) {
	var cmpErr error
	i := sort.Search(len(levels), func(i int) bool {
		cmp, err := levels[i].Price.Cmp(p)
		if err != nil {
			cmpErr = err
			return true
		}
		return !strictlyOrders(side, cmp)
	})
	if cmpErr != nil {
		return 0, false, fmt.Errorf("locate %s %s: %w", side, p, cmpErr)
	}
	if i == len(levels) {
		return i, false, nil
	}
	cmp, err := levels[i].Price.Cmp(p)
	if err != nil {
		return 0, false, fmt.Errorf("locate %s %s: %w", side, p, err)
	}
	return i, cmp == 0, nil
}
