package application

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// bookSilenceTimerPrio is the priority a book's silence timer takes.
//
// One rung down from most urgent, matching silenceTimerPrio, and chosen for the
// reason §3.5 makes the direction normative: the zero value is MOST urgent, so
// taking it by default would put market-data staleness ahead of the safety
// kernel's own timers because nobody chose.
const bookSilenceTimerPrio uint8 = 1

// BookReconstruct is §3.4's order-book reconstruction: apply deltas losslessly
// in venue-sequence order, detect gaps, resnapshot automatically, and publish
// the resulting immutable snapshot.
//
// # What it is FOR is the thing §3.4 forbids doing anywhere else
//
// "A raw snapshot+delta order book is never coalesced or drop-oldest'd on the
// wire — dropping a delta desyncs the book." Every lossiness the market-data
// lane is allowed applies to what comes OUT of here, not to what goes in. This
// type is the boundary between a stream that must not lose a message and a lane
// that may drop all but the newest.
//
// # No goroutine, no ticker, not safe for concurrent use — all three deliberate
//
// The same three calls FundingIngest makes, for the same three reasons: every
// scheduling decision goes through Clock.SetTimer so a TestClock can drive
// replay, delivery is pulled by the dispatcher, and §3.4 makes the marketdata
// reducer single-writer so a mutex here would make concurrent use compile while
// leaving the published order nondeterministic.
type BookReconstruct struct {
	clock   clock.Clock
	publish BookPublisher
	budgets BookBudgets
	resnap  Resnapshotter

	// holdDepth bounds the per-instrument buffer of deltas awaiting a
	// resnapshot. See ErrHoldDepthInvalid: the buffer is what makes resync
	// terminate, and an unbounded one is the queue §3.4 refuses everywhere else.
	holdDepth int

	books map[ids.VenueInstrument]*bookState
	armed map[clock.TimerID]ids.VenueInstrument
}

// bookState is one instrument's reconstruction position.
type bookState struct {
	// book is the last valid reconstruction. Meaningful only while synced.
	book domain.BookSnapshot

	// synced reports whether book is a position deltas may be applied to. False
	// from construction until the first snapshot, and again from the moment a
	// gap or a divergence is detected until the next one.
	synced bool

	// held buffers deltas arriving while unsynced, oldest first, bounded by
	// holdDepth.
	held []domain.BookDelta

	// overflows counts how many times held was discarded for being full. It is
	// state rather than a log line because §3.5 requires a drop on the capture
	// path to be a RECORDED gap rather than a silent one, and a counter is what
	// a metric can read without the reconstructor knowing about metrics.
	overflows uint64
}

// NewBookReconstruct constructs the use case, refusing a missing dependency and
// a non-positive hold depth.
//
// Refused at construction rather than at first use, on this codebase's standing
// rule that the zero value must be unusable: a BookReconstruct{} would otherwise
// accept deltas, hold none of them, request resnapshots through a nil interface
// and panic on the first gap — in the hot path, where the diagnosis is a stack
// trace rather than a named refusal at boot.
func NewBookReconstruct(
	c clock.Clock, p BookPublisher, b BookBudgets, r Resnapshotter, holdDepth int,
) (*BookReconstruct, error) {
	if c == nil {
		return nil, ErrClockNil
	}
	if p == nil {
		return nil, ErrBookPublisherNil
	}
	if b == nil {
		return nil, ErrBookBudgetsNil
	}
	if r == nil {
		return nil, ErrResnapshotterNil
	}
	if holdDepth <= 0 {
		return nil, fmt.Errorf("hold depth %d: %w", holdDepth, ErrHoldDepthInvalid)
	}
	return &BookReconstruct{
		clock: c, publish: p, budgets: b, resnap: r, holdDepth: holdDepth,
		books: make(map[ids.VenueInstrument]*bookState),
		armed: make(map[clock.TimerID]ids.VenueInstrument),
	}, nil
}

// OnSnapshot accepts a full book — the initial subscription's, or the one a
// resnapshot request produced — and drains whatever was held behind it.
//
// # The drain is the whole reason the hold buffer exists
//
// A REST snapshot is taken at a sequence the live stream has already passed, so
// the deltas that arrived during the fetch are exactly the ones needed to carry
// the book from the snapshot's position to the present. Discarding them means
// the next live delta gaps against a book that is now behind, which requests
// another snapshot, which lands behind again. Under any delta rate that outruns
// the fetch, that loop never terminates and every metric reports a connected
// feed.
//
// Held deltas that the snapshot already covers are dropped silently — that is
// SeqRelationStale, the ordinary case, not a fault.
func (r *BookReconstruct) OnSnapshot(snap domain.BookSnapshot, trigger AcceptOrdinal) (BookView, error) {
	if trigger == 0 {
		return BookView{}, fmt.Errorf("instrument %s: %w", snap.Instrument, ErrNoTrigger)
	}
	if err := snap.Validate(); err != nil {
		return BookView{}, fmt.Errorf("book snapshot: %w", err)
	}

	inst := snap.Instrument
	budget, err := r.budgetFor(inst)
	if err != nil {
		return BookView{}, err
	}

	st := r.stateFor(inst)
	st.book = snap
	st.synced = true

	// Drained BEFORE publishing, so what is published is the book at the
	// furthest position reconstruction can reach — not the REST snapshot, which
	// is already stale by the length of the fetch. Publishing the snapshot first
	// and the drained book second would put a KNOWN-OLD book on the lane, where
	// latest-value coalescing might well keep it if the drain produced nothing.
	held := st.held
	st.held = nil
	for i, d := range held {
		switch d.Follows(st.book.Watermark.VenueSeq) {
		case domain.SeqRelationStale:
			// Already covered by the snapshot. The ordinary case.
		case domain.SeqRelationContiguous, domain.SeqRelationOverlap:
			next, aerr := st.book.Apply(d)
			if aerr != nil {
				// A held delta that will not apply means the snapshot and the
				// stream disagree. Re-request rather than publish: the
				// alternative publishes a book whose provenance says it reached
				// a sequence it did not.
				return BookView{}, r.desync(st, inst, held[i:], fmt.Errorf("draining held delta: %w", aerr), ErrBookDesynced)
			}
			st.book = next
		case domain.SeqRelationGap:
			// The offending delta AND ITS SUCCESSORS go back into the buffer.
			// Re-holding only the one that failed is the mistake that looks
			// harmless: the successors are the deltas the next snapshot has to
			// chain to, so dropping them guarantees the next drain gaps too, and
			// the recovery loop never converges even though each individual step
			// looks like it did the right thing.
			return BookView{}, r.desync(st, inst, held[i:], nil, ErrSequenceGap)
		case domain.SeqRelationUnknown:
			// Unreachable: synced was just set, and snap.Validate above refuses a
			// zero VenueSeq (domain.ErrBookSeqUnset). That refusal was ADDED for
			// this line — the claim was written here first and the domain did not
			// yet make it, so a book snapshot with no sequence was marked synced
			// at position zero and every later delta landed exactly here, forever.
			// Named rather than defaulted, which is rule 10 doing its job: a
			// relation added later lands here as a build failure instead of
			// falling into the apply path.
			return BookView{}, fmt.Errorf(
				"instrument %s: held delta %d..%d against seq %d: %w",
				inst, d.FirstSeq, d.LastSeq, st.book.Watermark.VenueSeq, ErrAwaitingSnapshot)
		}
	}

	return r.emit(st, inst, budget, domain.SeqRelationUnknown, trigger)
}

// OnDelta accepts one incremental update.
//
// The relation decides everything, and the switch over it names every member
// because §3.6 rule 10 makes that a build requirement — the alternative is a
// new relation landing in whichever branch a `default:` happened to be.
func (r *BookReconstruct) OnDelta(d domain.BookDelta, trigger AcceptOrdinal) (BookView, error) {
	if trigger == 0 {
		return BookView{}, fmt.Errorf("instrument %s: %w", d.Instrument, ErrNoTrigger)
	}
	if err := d.Validate(); err != nil {
		return BookView{}, fmt.Errorf("book delta: %w", err)
	}

	inst := d.Instrument
	budget, err := r.budgetFor(inst)
	if err != nil {
		return BookView{}, err
	}

	st := r.stateFor(inst)
	if !st.synced {
		// Held, not dropped. §3.5: "reconstruction may hold a book delta behind
		// a sequence gap (pending resnapshot) while a later-arriving input from
		// an independent source is dispatchable" — the holding is what keeps
		// this SourceID's unreadiness from stalling anyone else's.
		return BookView{}, r.hold(st, inst, []domain.BookDelta{d}, ErrAwaitingSnapshot)
	}

	rel := d.Follows(st.book.Watermark.VenueSeq)
	switch rel {
	case domain.SeqRelationStale:
		// A redelivery. Reported so a caller can count it, and deliberately not
		// treated as a fault: an at-least-once transport produces these on every
		// reconnect, and escalating would make a healthy recovery look like a
		// feed problem.
		return BookView{}, fmt.Errorf(
			"instrument %s: span %d..%d against seq %d: %w",
			inst, d.FirstSeq, d.LastSeq, st.book.Watermark.VenueSeq, ErrSuperseded)

	case domain.SeqRelationContiguous, domain.SeqRelationOverlap:
		next, aerr := st.book.Apply(d)
		if aerr != nil {
			// The sequence was right and the RESULT is not a book — a crossed or
			// unordered outcome, which domain.Apply refuses. Same recovery as a
			// gap, different diagnosis: every message arrived, so the disagreement
			// is with what the venue says rather than with what reached us.
			return BookView{}, r.desync(st, inst, []domain.BookDelta{d}, aerr, ErrBookDesynced)
		}
		st.book = next

	case domain.SeqRelationGap:
		return BookView{}, r.desync(st, inst, []domain.BookDelta{d}, nil, ErrSequenceGap)

	case domain.SeqRelationUnknown:
		// Unreachable while synced implies a positioned book, and refused rather
		// than assumed: it is the shape a future path that sets synced without
		// setting a book would produce, and the failure would otherwise be a
		// delta applied to a zero-valued snapshot.
		return BookView{}, fmt.Errorf("instrument %s: %w", inst, ErrAwaitingSnapshot)
	}

	return r.emit(st, inst, budget, rel, trigger)
}

// OnSilence re-assesses a book whose silence timer has fired.
//
// §3.4 requires staleness be detected "by a timer, not only by the next update",
// because a feed that simply stops emits no event. It matters more here than on
// the funding path, not less: §3.4 gives books "sub-second budgets" precisely
// because they gate execution, so the window in which a dead book feed looks
// healthy is the window in which an order is priced against it.
//
// What it publishes is the SAME book with a WORSE verdict, which is an honest
// statement rather than a fabricated reading: nothing new has been observed, and
// that is the news.
func (r *BookReconstruct) OnSilence(id clock.TimerID) (BookView, error) {
	inst, ok := r.armed[id]
	if !ok {
		return BookView{}, fmt.Errorf("timer %q: %w", id, ErrUnknownTimer)
	}
	// Dropped before anything that can fail: a re-assessment that errored would
	// otherwise leave a fired ID mapped, and the next legitimate arm would look
	// like a duplicate.
	delete(r.armed, id)

	st, held := r.books[inst]
	if !held || !st.synced {
		return BookView{}, fmt.Errorf("timer %q: %w", id, ErrUnknownTimer)
	}
	budget, err := r.budgetFor(inst)
	if err != nil {
		return BookView{}, err
	}

	// Trigger zero and relation Unknown, both meaning "derived from no input".
	// §3.5's inheritance rule covers an envelope derived from a raw input; this
	// one is derived from the ABSENCE of one. Reusing the last input's ordinal —
	// which is what an implementation with nothing to put here reaches for —
	// makes the capture log claim one raw input produced two dispatched
	// envelopes, so a replay reconciling them reports a phantom duplicate and
	// concludes the feed is fine, inverting the meaning of the one event that
	// fired to say it had gone quiet.
	return r.emit(st, inst, budget, domain.SeqRelationUnknown, 0)
}

// Overflows reports how many times an instrument's hold buffer filled and was
// discarded, and whether the instrument is known at all.
//
// Exposed because ErrHoldOverflow is returned only on the call that overflowed,
// and a resync loop is visible in the RATE rather than in any one occurrence.
func (r *BookReconstruct) Overflows(inst ids.VenueInstrument) (uint64, bool) {
	st, ok := r.books[inst]
	if !ok {
		return 0, false
	}
	return st.overflows, true
}

// emit computes freshness, arms the silence timer and publishes.
func (r *BookReconstruct) emit(
	st *bookState, inst ids.VenueInstrument, budget domain.Budget,
	applied domain.SeqRelation, trigger AcceptOrdinal,
) (BookView, error) {
	transport, data, err := assessFreshness(r.clock, budget, st.book.Watermark, inst)
	if err != nil {
		return BookView{}, err
	}
	view := BookView{
		Book:      st.book,
		Transport: transport,
		Data:      data,
		Applied:   applied,
		Trigger:   trigger,
	}

	r.arm(inst, st.book.Watermark, budget)

	if err := r.publish.Publish(view); err != nil {
		return view, fmt.Errorf("publish %s: %w", inst, err)
	}
	return view, nil
}

// desync marks the book unsynced, re-holds the deltas that could not be
// applied, requests a fresh snapshot and returns the named cause.
//
// # The offending deltas are HELD rather than discarded
//
// They are what the incoming snapshot will need to chain to — dropping them
// guarantees a gap on the next drain, which requests another snapshot, and the
// recovery never converges while every individual step looks correct.
//
// cause carries the underlying failure when there is one (a refused Apply) and
// is nil for a plain sequence gap, where the span in the message is the whole
// diagnosis.
func (r *BookReconstruct) desync(
	st *bookState, inst ids.VenueInstrument, ds []domain.BookDelta, cause, sentinel error,
) error {
	st.synced = false
	err := r.hold(st, inst, ds, sentinel)
	if cause != nil {
		return fmt.Errorf("%w: %w", err, cause)
	}
	return err
}

// hold buffers deltas and requests a snapshot, returning the sentinel wrapped
// with the state a reader needs.
//
// It takes a RUN rather than one delta because the drain path re-holds a delta
// together with every successor behind it, and a single-delta signature made
// that call site quietly drop the successors — a defect whose symptom is a
// resync loop, not a wrong book.
func (r *BookReconstruct) hold(
	st *bookState, inst ids.VenueInstrument, ds []domain.BookDelta, sentinel error,
) error {
	overflowed := st.retain(ds, r.holdDepth)

	// Requested on every held delta, not only the first. SetTimer-style
	// deduplication does not apply to a REST call, so this leans on the adapter
	// to be idempotent — which is the honest place for it, since only the
	// adapter knows the venue's rate limit (§7.6). Requesting once and never
	// again is the failure that cannot recover: a dropped request leaves the
	// book held forever with nothing retrying.
	if rerr := r.resnap.RequestSnapshot(inst); rerr != nil {
		return fmt.Errorf("instrument %s: requesting snapshot: %w: %w", inst, sentinel, rerr)
	}
	if overflowed {
		return fmt.Errorf("instrument %s: depth %d: %w: %w", inst, r.holdDepth, sentinel, ErrHoldOverflow)
	}
	return fmt.Errorf("instrument %s: span %d..%d: %w", inst, ds[0].FirstSeq, ds[len(ds)-1].LastSeq, sentinel)
}

// retain appends ds to the hold buffer within depth, reporting whether anything
// had to be dropped to make room.
//
// # Overflow keeps the NEWEST depth deltas, and the direction is the decision
//
// Three candidates were live. Growing unbounded is the queue §3.4 refuses
// everywhere else. Keeping the OLDEST — which is what a plain
// append-and-truncate-the-tail does — keeps a prefix, and a prefix is worth
// nothing here: the snapshot that eventually arrives is NEWER than it, so those
// deltas were destined to be stale on the drain anyway, while the ones that
// would actually have chained to the snapshot are the ones evicted. Keeping the
// newest keeps the bridging set by construction.
//
// A fourth was live for a while and was wrong: DISCARDING the whole buffer and
// starting from ds. It keeps the newest too, so it reads as equivalent — but
// when ds is a single delta it retains exactly one, throwing away depth-1
// deltas that were still perfectly usable, and the next drain gaps on the hole
// it just made. The distinction is invisible unless a fixture overflows with a
// run SHORTER than the buffer, which is the only way it ever overflows.
//
// The drop is COUNTED, never silent, because §3.5 makes a loss on the capture
// path a recorded gap rather than a reorder: a reconstructor that quietly shed
// held deltas presents a resync loop as a slow venue.
//
// # [SPEC EDIT LANDED — v0.9.10] the core defined no overflow policy for THIS
// # buffer
//
// §3.4 defined one for the post-reconstruction snapshot lane — "bounded depth +
// defined overflow (drop-oldest + freshness gap counter)" — and §3.5 permits
// reconstruction to hold a delta behind a gap, but neither said what a FULL hold
// buffer does. The two are different buffers with opposite policies, which is
// why the silence was easy to read past: drop-oldest is right for a latest-value
// snapshot lane and is the wrong half to keep here. §3.4 now states this policy
// — bounded, keep-newest, counted — and states WHY the direction differs from
// the lane's: these deltas exist to bridge to a snapshot that has not arrived
// and will be newer than the buffer's head, so a retained prefix was destined to
// be stale on the drain.
func (st *bookState) retain(ds []domain.BookDelta, depth int) bool {
	if len(st.held)+len(ds) <= depth {
		st.held = append(st.held, ds...)
		return false
	}
	st.overflows++

	// Built into a fresh slice rather than appended in place: the drain path
	// passes a sub-slice of the buffer this is about to overwrite, so appending
	// onto st.held would write through the array ds is still reading from.
	combined := make([]domain.BookDelta, 0, len(st.held)+len(ds))
	combined = append(combined, st.held...)
	combined = append(combined, ds...)
	st.held = append([]domain.BookDelta(nil), combined[len(combined)-depth:]...)
	return true
}

// stateFor returns the instrument's reconstruction state, creating it unsynced.
//
// Created rather than refused, because an unknown instrument is the ordinary
// first message of a subscription: the delta stream connects before the REST
// snapshot returns as often as not.
func (r *BookReconstruct) stateFor(inst ids.VenueInstrument) *bookState {
	st, ok := r.books[inst]
	if !ok {
		st = &bookState{}
		r.books[inst] = st
	}
	return st
}

// budgetFor reads and validates the venue's book budget.
func (r *BookReconstruct) budgetFor(inst ids.VenueInstrument) (domain.Budget, error) {
	budget, ok := r.budgets.BookBudget(inst.Venue)
	if !ok {
		return domain.Budget{}, fmt.Errorf("venue %s: %w", inst.Venue, ErrNoBookBudget)
	}
	if err := budget.Validate(); err != nil {
		return domain.Budget{}, fmt.Errorf("venue %s book budget: %w", inst.Venue, err)
	}
	return budget, nil
}

// arm schedules the instant this book's freshness would next degrade.
//
// Identical in shape to the funding path's, and the shared part is
// domain.Budget.NextTransition rather than this function: what differs is the
// timer ID namespace, and merging the two would mean one ID per instrument
// across two data classes with different budgets — so the class whose budget
// fires later would silently cancel the class whose budget fires sooner, which
// on these two classes means funding cancelling the book.
func (r *BookReconstruct) arm(inst ids.VenueInstrument, w domain.Watermark, b domain.Budget) {
	id := bookSilenceTimerID(inst)
	at, ok := b.NextTransition(w, r.clock.Now())
	if !ok {
		r.clock.CancelTimer(id)
		delete(r.armed, id)
		return
	}
	r.clock.SetTimer(id, at, bookSilenceTimerPrio)
	r.armed[id] = inst
}

// bookSilenceTimerID names an instrument's book silence timer.
//
// The "book" segment is what keeps it distinct from the funding path's ID for
// the same instrument. SetTimer replaces by ID (§3.5), so a shared name would
// make each class's arm cancel the other's.
func bookSilenceTimerID(inst ids.VenueInstrument) clock.TimerID {
	return clock.TimerID("marketdata.book.silence." + inst.String())
}
