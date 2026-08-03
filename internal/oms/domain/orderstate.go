// Package domain is the OMS bounded context's domain layer: §5.1's aggregate
// and §5.3's order lifecycle, expressed as types that refuse the states the
// spec forbids.
//
// # What lives here and what does not
//
// §3.6 rule 6 makes this context the single writer of order, fill, trade and
// position state. That authority is about the WRITE PATH — the journal-then-
// apply sequence of §6 — and none of it is here: this layer holds the rules a
// write must satisfy, with no store, no clock and no venue.
//
// The division that matters for reading the code: SM §1 splits an order into
// two orthogonal things, and so does this package. OrderState tracks
// submission and finality; the Ledger tracks realized quantity. They do not
// gate each other, and the whole §5.3 paragraph explaining why is the reason
// the fill path never consults a state.
package domain

import "fmt"

// OrderState is §5.3's fixed order-lifecycle state set, the states of SM §1.
//
// # It is `type X string` because it crosses a persistence boundary
//
// §3.6 rule 8's shape exists for enums crossing persistence or wire, and this
// one crosses the most consequential persistence boundary in the system: §6
// step 5's write-ahead journal, which §7.5's reconcile-only boot replays to
// decide what to do about every order that was in flight when the process died.
// Under an integer encoding, inserting a constant renumbers every stored row —
// and the specific corruption is not a crash but a misclassification, since the
// states are ordered by lifecycle progress and a shift by one turns
// DispatchAuthorized into Submitted. Boot would then believe the send returned.
//
// # The state set is fixed, and rule 10 is what makes that a build failure
//
// §5.3 calls this "the fixed state set". Every switch over it in this repo
// names all twelve, because the exhaustiveswitch gate refuses a `default:` as
// an excuse — and a safety machine gaining a state is exactly where a silent
// fallthrough would be most expensive.
type OrderState string

const (
	// StatePlanned is intent and client ID journaled, NOT yet committed to
	// dispatch. §5.3: such an order "was never dispatched (no venue order can
	// exist) → safe to replan or discard".
	//
	// It is the one non-terminal state that contributes NO possible delta —
	// see ContributesPossibleDelta, where that asymmetry is the whole point.
	StatePlanned OrderState = "planned"

	// StateDispatchAuthorized is §5.3's crash boundary: a single Place for this
	// client ID is fsync-committed BEFORE the non-transactional network write.
	//
	// Recovery resolves it by query only — never resubmit, never infer "not
	// sent", because the single Place may have executed with its response lost.
	StateDispatchAuthorized OrderState = "dispatch_authorized"

	// StateSubmitted is "the single Place returned a transport result; awaiting
	// or parsing the venue ack".
	StateSubmitted OrderState = "submitted"

	// StateSubmissionUnknown is an ambiguous submission — a timeout or 5xx on
	// the Place. Resolved ONLY by OrderByClientID/Fills, never resubmitted.
	StateSubmissionUnknown OrderState = "submission_unknown"

	// StateAcknowledged is live on the venue.
	StateAcknowledged OrderState = "acknowledged"

	// StatePartiallyFilled is some fills applied and MORE POSSIBLE — §6.7 states
	// that meaning explicitly, and it is why this state contributes a
	// possible-delta interval rather than the point its realized fills describe.
	StatePartiallyFilled OrderState = "partially_filled"

	// StateCancelPending is a cancel requested.
	StateCancelPending OrderState = "cancel_pending"

	// StateCancelUnknown is a cancel with an ambiguous transport result —
	// timeout, 5xx or unknown ack. Symmetric with Submitted → SubmissionUnknown
	// (FLAG-1's round-4 ruling), and resolved only by query under SM §2's
	// finality contract.
	StateCancelUnknown OrderState = "cancel_unknown"

	// StateFilled is terminal for STATE. The fill ledger stays open.
	StateFilled OrderState = "filled"

	// StateCanceled is terminal for STATE, reached by an explicit venue-terminal
	// cancel plus the final executions sweep — never by a cancel ack alone.
	StateCanceled OrderState = "canceled"

	// StateRejected is terminal for STATE, and it REQUIRES an explicit venue
	// rejection. Proved absence is a different evidence class and has its own
	// terminal below; conflating them is what FLAG-2 resolved against.
	StateRejected OrderState = "rejected"

	// StateAbsentAfterFinality is terminal for STATE: proved stable absence, the
	// venue-specific stable-not-found rule completed across order AND execution
	// surfaces past the watermark.
	//
	// It is terminal and yet has outgoing edges, which is not a contradiction
	// but the §3.0 rule that venue reality wins: a discovered late execution
	// supersedes it. See CanTransition, where those two edges are the only ones
	// leaving a terminal.
	StateAbsentAfterFinality OrderState = "absent_after_finality"
)

// Validate reports whether s is one of the declared values.
func (s OrderState) Validate() error {
	switch s {
	case StatePlanned, StateDispatchAuthorized, StateSubmitted, StateSubmissionUnknown,
		StateAcknowledged, StatePartiallyFilled, StateCancelPending, StateCancelUnknown,
		StateFilled, StateCanceled, StateRejected, StateAbsentAfterFinality:
		return nil
	case "":
		return ErrOrderStateUnset
	}
	return fmt.Errorf("%q: %w", string(s), ErrOrderStateUnknown)
}

func (s OrderState) String() string { return string(s) }

// IsTerminal reports whether s is terminal for STATE — which is a narrower
// claim than it sounds, and the narrowing is §5.3's.
//
// "Terminal states are terminal for *state* only: the fill ledger never closes
// for a *known* order." So this predicate answers a question about submission
// and finality tracking, and answers nothing at all about whether more fills
// can arrive. Nothing in the Ledger consults it, deliberately.
//
// # Why it returns an error, and why the error is NOT s.Validate()
//
// A `switch` with a trailing `return false` puts the unset and misspelt states
// into the false group silently, and — worse — makes that group's membership
// untestable: every member returns what the fallthrough returns, so deleting a
// name from the case survives every test that could be written.
//
// Returning s.Validate() at the bottom looks like the fix and is not, which this
// package established the hard way: Validate answers nil for every DECLARED
// state, so a declared state dropped from the false case still falls through to
// (false, nil) — the same answer, and the group is still untestable. The
// mutation harness caught it here after the identical shape had already been
// diagnosed and fixed one function away in successors.
//
// So validation happens FIRST, which keeps the unset/unknown sentinels a caller
// branches on, and the fallthrough is an unconditional refusal reachable only
// from a declared state with no case. Both groups are then observable.
func (s OrderState) IsTerminal() (bool, error) {
	if err := s.Validate(); err != nil {
		return false, err
	}
	switch s {
	case StateFilled, StateCanceled, StateRejected, StateAbsentAfterFinality:
		return true, nil
	case StatePlanned, StateDispatchAuthorized, StateSubmitted, StateSubmissionUnknown,
		StateAcknowledged, StatePartiallyFilled, StateCancelPending, StateCancelUnknown:
		return false, nil
	}
	return false, fmt.Errorf("%q: %w", string(s), ErrOrderStateUnhandled)
}

// ContributesPossibleDelta reports whether an order in this state contributes a
// possible-delta INTERVAL to its hedge group — §6.7's ambiguity test.
//
// # This is NOT the negation of IsTerminal, and that is the entire reason it exists
//
// §6.7 lists the states making a residual ambiguous, and the list has SEVEN
// members: DispatchAuthorized, Submitted, SubmissionUnknown, Acknowledged,
// PartiallyFilled, CancelPending, CancelUnknown. There are EIGHT non-terminal
// states. The missing one is Planned, and §5.3 says why in a sentence written
// for a different purpose: "a Planned-only order was never dispatched (no venue
// order can exist)". An order that cannot have reached the venue cannot move the
// venue's delta, so it contributes a point — zero — and not an interval.
//
// Writing this as `!IsTerminal()` is the plausible simplification, it passes
// every test whose fixtures use a terminal and an Acknowledged order, and it
// deadlocks the system. §6.7 requires an ambiguous residual to be QUIESCED TO
// FINALITY before any compensating trade, and the finality contract resolves an
// order by querying the venue for it. There is nothing at the venue to query for
// a Planned order, so the contract runs to its deadline, the group enters
// DeadlineExceededAmbiguous, new trading is held and an operator is alerted —
// over an order the OMS itself has merely not sent yet, and which it will
// happily leave Planned indefinitely because §6.7 also told it to stop opening.
//
// The general shape, which is the same one marketdata's book-at-position-zero
// defect had: the wrong predicate here produces no wrong number and no crash. It
// produces a system that halts itself and reports every component healthy.
//
// The (bool, error) shape and the leading Validate are IsTerminal's, for
// IsTerminal's reason: without them the false group here — which is where
// Planned lives, and Planned is the entire finding — cannot be checked by any
// test.
// MayExistAtVenue reports whether an order in this state may have a record at
// the venue — §7.5's join asks exactly this, and it is a THIRD partition of the
// same twelve states.
//
// # Its true-set is every state but Planned, and that is not the same claim as
// # "not Planned"
//
// The sentence it encodes is §5.3's: "a Planned-only order was never dispatched
// (no venue order can exist)". Every other state was reached by a journaled
// authorization, which §6 step 6 fsyncs BEFORE the network write precisely so
// that a process dying mid-call leaves evidence — so from DispatchAuthorized
// onward the venue may hold an order whatever this system last heard.
//
// The terminals are in the true set, and they are the members a reader deletes
// first. StateFilled and StateCanceled are where this system's ordinary
// executions live — a marketable IOC that took part of the book ends terminal
// with real fills — and §7.5's replay rebuilds STATE without quantity, so a
// recovered Filled order carries a zero cumulative and a venue position nothing
// accounts for. StateRejected and StateAbsentAfterFinality are in it too, on the
// weaker ground that costs a query and nothing else: an execution surfacing
// against either of them supersedes the state (§3.0's venue-reality rule, which
// CanTransition implements as the only edges leaving a terminal), and the
// opposite reading skips the query that would find it.
//
// # Why it is not ContributesPossibleDelta, whose false-set looks similar
//
// That predicate answers whether an order's exposure is still UNCERTAIN, so it
// excludes every terminal. This one answers whether the venue can be ASKED, and
// a terminal order is precisely the one whose executions are settled fact. Using
// the §6.7 predicate here would skip the join for every finished order and
// rebuild a cumulative of zero for the fills that actually moved the position.
//
// The (bool, error) shape and the leading Validate are IsTerminal's, for
// IsTerminal's reason.
func (s OrderState) MayExistAtVenue() (bool, error) {
	if err := s.Validate(); err != nil {
		return false, err
	}
	switch s {
	case StateDispatchAuthorized, StateSubmitted, StateSubmissionUnknown,
		StateAcknowledged, StatePartiallyFilled, StateCancelPending, StateCancelUnknown,
		StateFilled, StateCanceled, StateRejected, StateAbsentAfterFinality:
		return true, nil
	case StatePlanned:
		return false, nil
	}
	return false, fmt.Errorf("%q: %w", string(s), ErrOrderStateUnhandled)
}

func (s OrderState) ContributesPossibleDelta() (bool, error) {
	if err := s.Validate(); err != nil {
		return false, err
	}
	switch s {
	case StateDispatchAuthorized, StateSubmitted, StateSubmissionUnknown,
		StateAcknowledged, StatePartiallyFilled, StateCancelPending, StateCancelUnknown:
		return true, nil

	// Planned sits with the terminals here and with the live states in
	// IsTerminal, which is exactly the case that makes the two predicates
	// different functions rather than one function and its negation.
	case StatePlanned, StateFilled, StateCanceled, StateRejected, StateAbsentAfterFinality:
		return false, nil
	}
	return false, fmt.Errorf("%q: %w", string(s), ErrOrderStateUnhandled)
}
