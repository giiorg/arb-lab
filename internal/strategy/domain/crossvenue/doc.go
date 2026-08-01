// Package crossvenue holds strategy #2's pure gate math (§45 of the
// 2026-08-01 crossvenue-gate design record): the committed rules that turn
// matched cross-venue funding evidence and measured book costs into
// episode-level economics and the portfolio verdict statistic. It mirrors
// domain/calibrate — computation over values it is handed, no I/O — so the
// battery can drive every rule without a filesystem.
//
// # What lives here, and what does not
//
// Here: settlement pairing by venue-published instants (the screening's
// committed rules restated as domain code), the persistence-conditioned
// episode machine over decision-time signals, the funding-phase ledger that
// accrues each leg at its own venue's instants and carries the unhedged
// term explicitly, the §8 conservative cost selections, the pre-declared
// portfolio combination (ruling §45f-3), and the stationary-bootstrap lower
// confidence bound the gate scores at.
//
// Not here: I/O and assembly. Reading the capture archive, sampling the
// decision-time streams, building the shared period grid the portfolio
// series lives on (idle periods contribute zero — that construction IS the
// utilization honesty of §42e), and the post-cutoff seal belong to the
// crossvenue walker (strategy/infrastructure/crossvenue) and the procedure
// commit.
//
// # Parameters are pre-registered, not chosen here
//
// Functions take their entry levels, hysteresis divisors, pairing rounds,
// block lengths, confidence levels and allocation policies as arguments.
// The committed VALUES live in slice D's procedure commit (§45i), where
// each constant's rationale sits beside it — this package states the rules,
// the procedure commit states the numbers the rules ran with.
package crossvenue
