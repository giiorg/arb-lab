// Package application holds the risk context's use cases.
//
// The §4.1 posture guard is the first: it reads the account's posture through a
// port and judges it with `risk/domain`'s rules. It holds no venue knowledge
// and no wire shapes — §3.6 rule 1 forbids the import, and the reason the rule
// exists is that a use case able to reach the transport can reach for a second
// read to explain a refusal away.
package application

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/risk/application/ports"
	"github.com/giiorg/arb-lab/internal/risk/domain"
)

// PostureGuard asserts §4.1's account posture.
//
// # ONE use case, TWO callers — and only one of them exists today
//
// §1.1's non-negotiable is *"account & margin-mode verification … at boot AND
// before each trade (§4.1, §6.3)"*. §7.5's reconcile-only boot is the caller
// that lands with this slice; §6 step 3's pre-trade re-verify lands with the
// trading loop, which does not exist (§12 item 4 — nothing in this system sizes
// a trade today). The second path is deliberately NOT built speculatively: it
// is the same Assert, called again (record §73b F6).
//
// # It holds NO snapshot, and that is structural rather than a convention
//
// Record §73c-2 ruled that every trade performs the full read regardless of any
// snapshot's age — the TTL bounds a cached answer, it never licenses skipping
// the read. So there is no report field, no verdict field and no observed-at
// field on this struct: a caller cannot ask it for a remembered answer because
// it does not have one, and a future edit that adds one has to argue with this
// comment first.
//
// # An error is not a verdict, and the two are not confusable
//
// Assert returns a zero Verdict beside an error, and the zero Verdict evaluated
// no checks — so a caller that ignores the error and asks the verdict whether
// it is satisfied is REFUSED by name (ErrVerdictVacuous) rather than told yes.
// The interlock is deliberate: this is the one place where "the read failed"
// could otherwise decay into "nothing was wrong".
type PostureGuard struct {
	source      ports.AccountPostureSource
	requirement domain.Requirement
}

// NewPostureGuard builds the guard.
//
// The requirement is validated HERE rather than at each Assert, because a
// misconfigured requirement is a wiring defect that should stop a process from
// booting rather than surface as a refusal on the first trade — and because
// §7.5's checklist wants the gate's dependencies constructible before any gate
// runs. Assert validates it again through Check, which is not redundant: the
// requirement is a value, a caller could hold one this constructor never saw.
func NewPostureGuard(source ports.AccountPostureSource, requirement domain.Requirement) (*PostureGuard, error) {
	if source == nil {
		return nil, ErrPostureSourceMissing
	}
	if err := requirement.Validate(); err != nil {
		return nil, err
	}
	return &PostureGuard{source: source, requirement: requirement}, nil
}

// Venue is whose account this guard speaks for.
func (g *PostureGuard) Venue() string { return g.requirement.Venue.String() }

// Assert performs a fresh posture read and judges it.
//
// The returned Verdict lists EVERY failed check by name. An error means the
// posture could not be established at all — the read failed, or the answer
// could not be judged — and is never a posture that happens to be fine.
func (g *PostureGuard) Assert() (domain.Verdict, error) {
	// The want-set is derived from the SAME requirement the verdict is judged
	// against, so the read and the judgement cannot disagree about which
	// settings this deployment is governed by. Deriving it here rather than in
	// the adapter is what keeps that true: an adapter with its own opinion would
	// be a second statement of the rule table (task #199, record §73p).
	settings, err := g.requirement.Dialect.ProbedSettings(g.requirement.Coverage)
	if err != nil {
		return domain.Verdict{}, err
	}
	if len(settings) == 0 {
		// # A deployment with nothing to READ is judged without a read
		//
		// It is reachable on exactly one shape today and the shape is real: a
		// Binance deployment configuring no derivative instrument. Every probed
		// rule in that dialect governs the derivatives leg, so all five lapse,
		// and §4.1's only remaining Binance check is the Portfolio-Margin
		// ATTESTATION — which no venue read could answer in any case (record
		// §73b F1). Issuing a read here would ask three futures endpoints a
		// question no rule is waiting for.
		//
		// This is the one path that reaches Check with an empty report on
		// purpose, and what keeps it honest is that it does not get to declare
		// the outcome: Check still evaluates the attestation, still records the
		// five lapses, and Satisfied still refuses a verdict that evaluated
		// nothing. So if the attested rule were ever removed from that table,
		// this branch would produce Evaluated==0 and REFUSE, rather than a clean
		// posture for an account nobody asked about (task #199, record §73p).
		return domain.CheckWithoutVenueRead(g.requirement)
	}
	// The roster is the REQUIREMENT's, so the rules and the read range over one
	// list. Derived here rather than carried alongside, because the second
	// derivation is what would let a symbol be required by Check and never asked
	// of the venue — which reads as ReasonUnobserved, i.e. as the venue's
	// silence rather than as our omission (record §97 §4).
	symbols := make([]string, 0, len(g.requirement.Symbols))
	for _, p := range g.requirement.Symbols {
		symbols = append(symbols, p.Symbol)
	}
	scope := ports.PostureScope{Venue: g.requirement.Venue, Settings: settings, Symbols: symbols}
	if err := scope.Validate(); err != nil {
		// Unreachable through NewPostureGuard, which validated the venue — and
		// reachable through `var g PostureGuard`, which is the case it exists
		// for. The zero value's source is nil, so without this the next line
		// PANICS inside §7.5's boot: a crash where a named refusal belongs, and
		// the gate the crash prevents from reporting is the one whose job is
		// refusing to trade. A mutation campaign found this branch untested
		// (record §73j M20).
		return domain.Verdict{}, err
	}

	report, err := g.source.AccountPosture(scope)
	if err != nil {
		// Unwrapped, on the read path's standing rule: a read that was not
		// answered must not be re-derived into one that answered with nothing
		// to report. Wrapping it in this package's own sentinel would also lose
		// the venue-layer classification a caller uses to decide whether to
		// retry.
		return domain.Verdict{}, err
	}

	verdict, err := domain.Check(g.requirement, report)
	if err != nil {
		return domain.Verdict{}, fmt.Errorf("asserting %s posture: %w", g.requirement.Venue, err)
	}
	return verdict, nil
}
