// Package cashcarry is the cash-and-carry evaluation core (§40b): one pure
// function per direction — Enter, Continue — over a single validated
// DecisionInput, returning a Decision.
//
// # Why a domain subpackage
//
// §3.2's context anatomy puts pure signal math in domain/, and hexboundary
// keys on the domain/ path position — a peer package beside domain/ would sit
// outside the gate that exists to police exactly this code (§40b records the
// rejection). The package imports strategy/domain for the vocabulary and
// nothing outward: no ports, no clock, no ambient anything. A decision is a
// function of its input, not of what a port answered mid-call.
//
// # What the core refuses vs what it errors
//
// A refusal is a Decision (§40f): a policy gate or §2.4 rule declining to
// score names itself in RefusedBy and journals like any verdict. An error is
// a fault — an input the service should never have passed (Validate failed,
// wrong Kind) or arithmetic that cannot complete (overflow). The service maps
// core errors to refusal Decisions so the journal never has a silent hole,
// but the two are different claims: a refusal is the gate working, an error
// is the machinery failing.
//
// # The conservative directions, in one place
//
// Every rounding and side choice in this package points the same way the
// slice B adapters point (§40m): income is understated, costs are overstated.
// Funding revenue is priced on the perp BID and rounded toward negative
// infinity; every fee and every threshold on the right-hand side (the scaled
// hurdle, the entry margin, the exit band) is priced on the ASK or rounded
// away from zero. A minimum-notional gate is checked against the BID — if the
// understated notional clears the venue's floor, the real order does too.
package cashcarry
