package domain

import "fmt"

// CanTransition reports whether from → to is an edge of SM §1's order
// lifecycle.
//
// # A reachability predicate rather than an (state, event) → state function
//
// SM §1's table is keyed on a state and an EVENT, and two of its rows resolve to
// several targets: a query on SubmissionUnknown yields any of five states, and a
// fill on Acknowledged yields PartiallyFilled or Filled depending on cumulative
// quantity. So the event does not determine the target, and a function taking
// one would have to take the resolving evidence too — which is the venue's
// answer, an infrastructure type this layer must not name.
//
// The split taken instead: the EVIDENCE CLASS is encoded in the target state,
// which is what FLAG-2 already required of this design (Rejected needs an
// explicit venue rejection, AbsentAfterFinality needs the stable-not-found rule
// to have completed — different states because different evidence), and the
// domain owns which targets are reachable at all. An application use case maps
// evidence to a target; this refuses the targets no evidence could justify.
//
// What that buys is a claim a test can check exhaustively: there are 144 ordered
// pairs of states and this function answers all of them, so the edge set is
// asserted as a matrix rather than sampled.
func CanTransition(from, to OrderState) error {
	if err := from.Validate(); err != nil {
		return fmt.Errorf("transition source: %w", err)
	}
	if err := to.Validate(); err != nil {
		return fmt.Errorf("transition target: %w", err)
	}
	if from == to {
		return fmt.Errorf("%s: %w", from, ErrTransitionNoOp)
	}
	legal, err := successors(from)
	if err != nil {
		return fmt.Errorf("transition source: %w", err)
	}
	for _, s := range legal {
		if s == to {
			return nil
		}
	}
	return fmt.Errorf("%s -> %s: %w", from, to, ErrTransitionForbidden)
}

// successors is SM §1's transition table, one case per source state.
//
// A switch rather than a package-level map, and the reason is the gate: a map
// literal missing a key answers "no legal successors" for that state — the
// silent-fallthrough failure in its purest form, since a state with no
// successors is a legitimate answer here (three of them have none). The
// exhaustiveswitch gate refuses a switch that omits a member, so the omission
// becomes a build failure instead of an order that can never leave the state a
// later contributor added.
//
// # Why the error return exists, when CanTransition has already validated
//
// It is not for an undeclared state — CanTransition rejects those first, with
// the sentinel that names which kind. It exists so the THREE TERMINALS' case is
// distinguishable from the fallthrough. Both answer "no successors", so with a
// bare `return nil` at the bottom, deleting StateFilled from the terminal case
// changes nothing any test could observe: the state falls through to the same
// answer it was given explicitly. That is this repo's recorded
// trailing-return-hides-its-own-group trap, and the group it would hide here is
// the terminal one — the claim most worth checking in the file.
//
// ErrOrderStateUnhandled is therefore reachable only from a DECLARED state with
// no case, which is precisely the mutation, and precisely what a new member
// added without a case would be if the gate were ever removed.
//
// The returned slice is never retained by a caller and is small enough that Go
// keeps it on the stack.
func successors(from OrderState) ([]OrderState, error) {
	// resolved is the outcome set of an OrderByClientID/Fills query on an order
	// whose disposition is unknown. Named once because two states share it, and
	// they must: the second is the finding recorded below.
	//
	// [SPEC EDIT LANDED — v0.9.34] StateCanceled was NOT in this set, and its
	// absence is the v0.9.11 venue-initiated-termination finding repeating on the
	// OTHER axis. That edit added Acknowledged → Canceled and PartiallyFilled →
	// Canceled, because "an IOC is immediately terminal at the venue" (SM §2 step
	// 1) and the venue cancels the unfilled remainder with no request from us —
	// so without those edges "the only order type this system sends can never
	// reach a terminal state on its ordinary outcome". Every word of that applies
	// here, and the fix was carried across the LIVE states only.
	//
	// The recovery path is where it costs more. §6 step 6 sends nothing but
	// marketable IOC limits, so the single most common thing a resolving query
	// can find is an order the venue already terminated unfilled — Binance
	// EXPIRED, Bybit Cancelled. Without this edge that answer has nowhere to go:
	// the order stays at DispatchAuthorized or SubmissionUnknown, both of which
	// ContributesPossibleDelta answers true for, so its hedge group is Ambiguous
	// for ever, §6.7 holds new trading, and nothing reports an error.
	//
	// Worse than stuck, it MISDIRECTS. The nearest available terminal for "the
	// venue has it and it ended with no fills" is StateAbsentAfterFinality, and
	// reaching for it is the evidence-class conflation FLAG-2 exists to prevent:
	// a venue-terminal record is not proved absence, and recording it as one
	// tells §7.5 we never found the order we in fact found. A missing edge that
	// leaves a wrong edge as the only legal one is worse than a missing edge.
	//
	// SM §1's transitions table now carries Canceled among the query-resolution
	// targets for both source states. The evidence rule is unchanged and is the
	// one this state has always carried: an explicit venue-terminal record PLUS
	// the final executions sweep, never a cancel ack alone (SM §2 step 3).
	resolved := []OrderState{
		StateAcknowledged, StatePartiallyFilled, StateFilled,
		StateCanceled, StateRejected, StateAbsentAfterFinality,
	}

	switch from {
	case StatePlanned:
		// The only edge. §5.3's other Planned row — crash recovery — is "replan
		// or discard", which is the order leaving the system rather than
		// changing state, so it is not an edge here.
		return []OrderState{StateDispatchAuthorized}, nil

	case StateDispatchAuthorized:
		// [SPEC EDIT LANDED — v0.9.11] SM §1's transitions table gives this state exactly two
		// rows — "send returns → Submitted" and "no-return / crash →
		// DispatchAuthorized (stays)" — and the second names the resolution
		// ("resolved by query only") in its ACTION column while leaving the
		// target column pointing back here. Read as the table alone, an order
		// crash-recovered at the dispatch boundary has no way out.
		//
		// The requirement is stated twice elsewhere and neither statement made it
		// into the table. Core §5.3: "an order at DispatchAuthorized or beyond is
		// resolved only by OrderByClientID/Fills under the venue-specific
		// stable-not-found rule". SM §1's own invariants repeat it as "crash
		// recovery is defined for every pre-ack state", and FLAG-2's ruling ends
		// "applies equally to DispatchAuthorized-recovery resolution (core
		// §5.3)". So the edges belong to the design; the table is silent about
		// them.
		//
		// Implemented literally, the cost is not a wrong order — it is a stall
		// with every light green. This state contributes a possible-delta
		// interval (§6.7), so the hedge group holding it is permanently
		// Ambiguous; §6.7 then forbids compensating the residual and holds new
		// trading, while the order itself sits at a state whose only listed exit
		// is a send that already happened. The exposure stays open, the operator
		// is alerted about an order the venue may well have filled hours ago, and
		// nothing anywhere reports an error.
		//
		// SM §1's transitions table now carries the five query-resolution edges,
		// same set and same evidence rules as SubmissionUnknown's — the exit the
		// "stays" row's action column named but never targeted.
		//
		// [SPEC EDIT LANDED — v0.9.11] The StateSubmissionUnknown edge is a SECOND silence in
		// the same row, and it is the one the core states in words while SM §1
		// tabulates no path to it. Core §6 step 6: "A timeout/5xx moves the order
		// to SubmissionUnknown, resolved by OrderByClientID/Fills, never
		// resubmitted." The order it moves is this one — the fsync of
		// DispatchAuthorized happens "immediately before the single Place" — and
		// SM §1's only entry edge into SubmissionUnknown leaves Submitted.
		//
		// Neither path the table does offer can carry it, and each fails by
		// recording something untrue:
		//
		//   - Routing through Submitted journals "the single Place returned a
		//     transport RESULT", which is precisely what a timeout is not. §7.5
		//     replays this journal to decide what to do about every order that was
		//     in flight, so the false fact is not cosmetic — it is read back as
		//     evidence the request reached the venue's edge.
		//   - Staying here relies on the "no-return / crash → stays" row, whose
		//     subject is a process that DIED (its action column is the recovery
		//     path, "resolved by query only"). A call that returned an error is
		//     not a send still in flight, and the live path that just watched it
		//     fail is not crash recovery.
		//
		// The symmetry argument is the cheap one, and it is FLAG-1's own: the
		// round-4 ruling created CancelUnknown as a first-class entry edge on an
		// "ambiguous cancel transport result … by symmetry with Submitted →
		// SubmissionUnknown", rather than routing it through a state claiming an
		// ack had been parsed. This is that same edge one step earlier, and the
		// asymmetry is its absence rather than its presence.
		//
		// SM §1's transitions table now carries DispatchAuthorized →
		// SubmissionUnknown on an ambiguous transport result (timeout / 5xx /
		// unknown ack), as a row DISTINCT from "no-return / crash" — whose
		// subject is a process that died, not a call that returned.
		return append([]OrderState{StateSubmitted, StateSubmissionUnknown}, resolved...), nil

	case StateSubmitted:
		// [SPEC EDIT LANDED — v0.9.61] This row carried the three LIVE-path
		// answers — an ack, a rejection, a transport timeout — and no
		// query-resolution row at all, while the two states either side of it
		// carry one. It is v0.9.11's finding on a third axis and v0.9.34's
		// sentence applies unchanged: the fix was carried across the states it
		// was noticed on.
		//
		// The requirement is stated twice elsewhere and neither statement made it
		// into the table. SM §1's own invariants: "an order at DispatchAuthorized
		// OR BEYOND is resolved only by OrderByClientID/Fills under the
		// venue-specific stable-not-found rule" — "or beyond" is this state.
		// Core §6.7 lists Submitted among the states whose residual is ambiguous
		// and which must be QUIESCED TO FINALITY through that same contract.
		//
		// What its absence cost is not a wrong number, it is a lifecycle with no
		// exit. §6 step 6 sends nothing but marketable-IOC limits and SM §2 step 1
		// makes an IOC "immediately terminal at the venue", so a query about a
		// successfully-placed order ordinarily finds it FILLED or CANCELED — and
		// three of the five readings §4's ACL can produce had no legal target
		// here. The order stayed Submitted for the life of the process,
		// ContributesPossibleDelta answered true for it for ever, and every
		// consumer whose success condition required finality was unsatisfiable
		// while nothing reported an error. The one consumer that existed —
		// `arblab exercise run` — treats Submitted AS its success state, which is
		// why four slices passed over this.
		//
		// The evidence rules are the other two rows' and this row does not weaken
		// them: an explicit venue rejection → Rejected; an explicit venue-terminal
		// record PLUS the final executions sweep → Canceled (SM §2 step 3, never a
		// status read alone); the stable-not-found rule COMPLETING →
		// AbsentAfterFinality, never mapped onto Rejected.
		return append([]OrderState{StateSubmissionUnknown}, resolved...), nil

	case StateSubmissionUnknown:
		return resolved, nil

	case StateAcknowledged:
		return []OrderState{
			StatePartiallyFilled, StateFilled, StateCancelPending,
			StateCanceled, // venue-initiated — see the note below
		}, nil

	case StatePartiallyFilled:
		// [SPEC EDIT LANDED — v0.9.11] The StateCanceled edge on this row and the one above is
		// VENUE-initiated, and SM §1's table has no such edge: its only paths into
		// Canceled leave CancelPending or CancelUnknown, both of which we enter by
		// requesting a cancel ourselves.
		//
		// That gap is not an edge case here, it is the normal path. §6 step 6
		// makes the execution pattern "parallel marketable-IOC limit orders (never
		// raw MARKET)", and SM §2's finality contract states the consequence in
		// its own step 1: "an IOC is immediately terminal at the venue". An IOC
		// that fills part of its quantity has the remainder canceled BY the venue,
		// with no cancel request from us; one that crosses nothing is canceled
		// outright. Neither passes through CancelPending.
		//
		// So without these edges the only order type this system sends can never
		// reach a terminal state on its ordinary outcome. It sits Acknowledged or
		// PartiallyFilled, both of which contribute a possible-delta interval, so
		// §6.7 classifies every hedge group as Ambiguous forever and the one-leg
		// failure handler never issues the ExposureReduction it exists for. A
		// naked leg from a partially-filled hedge would stay naked while the
		// classifier waited for more fills on an order the venue closed
		// milliseconds after we sent it.
		//
		// SM §1's transitions table now carries a venue-initiated termination row
		// covering Acknowledged and PartiallyFilled → Canceled — IOC expiry, GTD
		// expiry, venue cancel-all, self-trade prevention — distinct from the
		// FLAG-1 rows, which are about a cancel WE requested.
		return []OrderState{StateFilled, StateCancelPending, StateCanceled}, nil

	case StateCancelPending:
		// Filled is FLAG-1's "cancel lost the race to a full fill". Canceled needs
		// the explicit venue-terminal cancel AND the final executions sweep —
		// never a cancel ack alone, which is a claim CanTransition cannot enforce
		// and SM §2's contract does.
		return []OrderState{StateCancelUnknown, StateCanceled, StateFilled}, nil

	case StateCancelUnknown:
		return []OrderState{StateCanceled, StateFilled}, nil

	case StateAbsentAfterFinality:
		// The only outgoing edges from a terminal, and they are §3.0's rule that
		// venue reality wins: "a discovered late execution supersedes
		// AbsentAfterFinality". The ledger admitted that fill regardless — it
		// never closed — so what these edges repair is the STATE's disagreement
		// with a quantity already counted.
		return []OrderState{StatePartiallyFilled, StateFilled}, nil

	case StateFilled, StateCanceled, StateRejected:
		// Genuinely terminal — an ANSWER, not an omission, which is the whole
		// reason this pair is (nil, nil) and the fallthrough below is not.
		return nil, nil
	}

	return nil, fmt.Errorf("%q: %w", string(from), ErrOrderStateUnhandled)
}
