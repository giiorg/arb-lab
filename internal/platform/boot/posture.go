package boot

import "fmt"

// Posture is what the process is currently permitted to do at a venue.
//
// Integer enum with a zero sentinel — §3.6 rule 8's documented exception, same
// as ratelimit.Priority and observability.Kind. It never crosses a wire; what
// crosses is the refusal it produced.
//
// The four values are ORDERED by permissiveness, and unlike an ordinary enum
// the order is the whole semantics: every posture permits everything the
// posture below it permits, so a comparison is a complete answer and Permits is
// a single >=. Two consequences follow, and both are deliberate.
//
// The zero value is the most restrictive. A Posture nobody populated permits
// nothing, so the forgotten-field failure is a refusal rather than a trade.
//
// Combining postures is a minimum (see Weakest), which means combining is
// associative and order-independent — the caller cannot get a different answer
// by asking the gates in a different sequence.
//
// # Why four, and not a boolean
//
// §7.5 says "new execution disabled while state is rebuilt" and, separately,
// that an Ambiguous MarginOp "blocks new openings (not safety actions, which
// read live state)". Those are two different refusals: one withholds every
// venue write, the other withholds only the ones that grow exposure. A boolean
// may-I-trade collapses them, and collapsing costs money in both directions —
// treating an ambiguous margin op as a full stop withholds a flatten the spec
// says is permitted, and treating a mid-reconcile boot as stop-opening admits a
// reduction sized from a position the process has not finished rebuilding.
type Posture uint8

const (
	// PostureHalted is the zero value: no venue write of any kind.
	//
	// Where a failed boot lands, and where §7.4's (c) hard-halt-and-alert mode
	// puts a running process. Note that it withholds reconciliation too, which
	// is not an oversight: the phase that would reconcile is the one that
	// failed, so a halted process reconciling would be acting on the collaborator
	// it just failed to establish.
	PostureHalted Posture = iota

	// PostureReconcileOnly is §7.5's boot phase, and the package's namesake.
	//
	// Reconciliation writes are permitted — the DeactivationPending re-drive,
	// ResolveMarginOp — and no exposure-changing execution is. It is reached
	// two ways that a linear boot model cannot tell apart, which is why it is a
	// value rather than a stage: a boot still rebuilding state is in it
	// transiently, and a bot that recovers mid-takeover "stays in reconcile-only
	// until an operator clears it" (§7.4) is in it indefinitely, with the
	// process alive, connected and monitoring throughout.
	PostureReconcileOnly

	// PostureStopOpening permits safety execution and withholds new exposure —
	// §5.3's stop-opening, §7.4's first ladder step, and §7.5's Ambiguous
	// MarginOp, which are the same posture reached from three directions.
	PostureStopOpening

	// PostureTrading permits everything. Reached only when every required gate
	// has been affirmatively reported Passed; see Checklist.
	PostureTrading
)

const maxPosture = PostureTrading

// Validate reports whether p names a posture. Unlike the other enums in this
// kernel the zero value is legal, because "permits nothing" is a real answer
// and the one a zeroed struct should give.
func (p Posture) Validate() error {
	if p > maxPosture {
		return fmt.Errorf("posture %d: %w", uint8(p), ErrPostureInvalid)
	}
	return nil
}

// String implements fmt.Stringer.
func (p Posture) String() string {
	switch p {
	case PostureHalted:
		return "PostureHalted"
	case PostureReconcileOnly:
		return "PostureReconcileOnly"
	case PostureStopOpening:
		return "PostureStopOpening"
	case PostureTrading:
		return "PostureTrading"
	}
	return fmt.Sprintf("Posture(%d)", uint8(p))
}

// Act is a class of venue write, named by what it does to exposure.
//
// Three, because that is how many distinct answers the postures give. Reads are
// deliberately not an Act: §7.5's own wording — safety actions "read live
// state" — makes reading the thing that stays available when writing does not,
// and §7.4's watcher observes with no trading key at all. A posture that could
// withhold a read would be able to blind the monitoring that decides when to
// escalate.
type Act uint8

const (
	// ActUnspecified is the zero value and names no act. Unusable in either
	// direction: a caller that forgot to say what it was doing gets a refusal
	// that says so, rather than the most or least permissive reading.
	ActUnspecified Act = iota

	// ActReconcile is the §7.5 join and the writes it re-drives: the
	// EmergencyPlanStore.Deactivate acknowledgement a closed trade left behind,
	// and ResolveMarginOp's evidence read-back. Exposure-neutral by
	// construction — it resolves what already happened and never opens or
	// closes a leg.
	ActReconcile

	// ActReduce is safety execution: ExposureReduction, EmergencyClosePackage,
	// §7.1's margin top-up, quiescing an ambiguous residual to finality. It
	// reduces or bounds exposure and never grows it.
	ActReduce

	// ActOpen creates new exposure — a strategy entry, the second leg of a
	// package that is not yet on. The first thing withheld everywhere in this
	// spec, and the act every posture below PostureTrading refuses.
	ActOpen
)

const maxAct = ActOpen

// Validate reports whether a names an act, excluding the sentinel.
func (a Act) Validate() error {
	if a == ActUnspecified || a > maxAct {
		return fmt.Errorf("act %d: %w", uint8(a), ErrActInvalid)
	}
	return nil
}

// String implements fmt.Stringer.
func (a Act) String() string {
	switch a {
	case ActUnspecified:
		return "ActUnspecified"
	case ActReconcile:
		return "ActReconcile"
	case ActReduce:
		return "ActReduce"
	case ActOpen:
		return "ActOpen"
	}
	return fmt.Sprintf("Act(%d)", uint8(a))
}

// minPostureFor is the weakest posture that permits a.
//
// Written as a switch over Act rather than as arithmetic on the two enums, even
// though the ladders currently line up value for value. The alignment is a
// coincidence of how many rungs each ladder happens to have, and arithmetic
// would make inserting a posture silently re-map every act.
func minPostureFor(a Act) Posture {
	switch a {
	case ActReconcile:
		return PostureReconcileOnly
	case ActReduce:
		return PostureStopOpening
	case ActOpen:
		return PostureTrading
	}
	return PostureHalted
}

// Permits reports whether posture p allows act a.
//
// An invalid or unspecified act is refused, not permitted: minPostureFor maps
// it to PostureHalted, which nothing is above. That is the safe direction — a
// caller passing a zero Act is a caller that has not said what it wants to do.
func (p Posture) Permits(a Act) bool {
	if a.Validate() != nil {
		return false
	}
	return p >= minPostureFor(a)
}

// Weakest returns the least permissive of the postures given.
//
// The identity is PostureHalted, so Weakest of nothing refuses everything.
// That is the load-bearing half: a caller reducing over an empty set of gate
// outcomes has established no preconditions, and the alternative identity —
// PostureTrading, which is what a fold seeded with the maximum produces —
// would make "nothing was checked" and "everything passed" the same answer.
func Weakest(ps ...Posture) Posture {
	weakest := PostureHalted
	for i, p := range ps {
		if i == 0 || p < weakest {
			weakest = p
		}
	}
	return weakest
}
