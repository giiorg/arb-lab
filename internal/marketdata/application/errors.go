package application

import "errors"

// The use-case layer's refusals. Sentinels for branching, always wrapped at the
// construction site with the values that failed — domain/errors.go's rule, for
// the same reason: one sentinel cannot say WHICH check fired.
var (
	// ErrClockNil, ErrPublisherNil and ErrBudgetsNil report a dependency
	// nobody supplied. Separate from each other so the message names the wire
	// that is missing, and refused at construction rather than at first use:
	// the zero value of this service would otherwise accept observations,
	// publish them nowhere, and report freshness against a nil clock.
	ErrClockNil     = errors.New("marketdata: clock not supplied")
	ErrPublisherNil = errors.New("marketdata: funding publisher not supplied")
	ErrBudgetsNil   = errors.New("marketdata: freshness budgets not supplied")

	// ErrNoBudget reports a venue with no configured funding budget. It is a
	// refusal rather than a permissive default, on §3.6 rule 4's argument: the
	// default that a forgotten budget would need is either "always fresh",
	// which trades on a dead feed, or "always blind", which reads as a venue
	// outage and sends an operator to the exchange. Naming the missing config
	// is the only answer that is not one of those two.
	ErrNoBudget = errors.New("marketdata: no funding freshness budget configured for venue")

	// ErrSuperseded reports an observation that does not advance the source —
	// a replayed or reordered delivery. Not an error in the sense of a defect:
	// it is the ordinary outcome of an at-least-once transport, and it is
	// named so that a caller can count it without treating it as a feed fault.
	ErrSuperseded = errors.New("marketdata: observation does not advance the source")

	// ErrUnknownTimer reports a silence timer this service did not arm. Refused
	// rather than ignored: the dispatcher pumps one timer queue for the whole
	// process, and a handler that silently accepted foreign IDs would make a
	// mis-routed timer look like a healthy no-op.
	ErrUnknownTimer = errors.New("marketdata: silence timer was not armed by this service")
)
