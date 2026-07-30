// Package dispatch is build-spec §3.5's single total-order event dispatcher —
// the component that decides the order the state machine consumes, stamps that
// order as IngressSeq (a.k.a. DispatchSeq, the replay key), and is the ONE
// place that arbitration happens.
//
// # One caller, and why this is engine/ rather than platform/
//
// Exactly one component pumps this per process. §3.5 gives the dispatcher the
// total order over "events *and* timers", and an order decided in two places is
// two orders. That one-caller contract is why this is not a kernel package: a
// platform/ package whose contract is "only one caller may exist" is a category
// error in a shared kernel (decision record 2026-07-30 §39c). engine/ placement
// also gets the right gates for free — notimenow already denies ambient clock
// reads here, and hexboundary keeps domain/ and application/ from reaching in.
//
// # Three inputs, one arbitration
//
// The dispatcher composes three pull-based primitives, and the pull direction
// is the design: each of them refuses to push precisely so that the arbitration
// that decides the replay key happens here and nowhere else.
//
//	clock.TimerSource        due timers, already totally ordered by (At, Prio, ID)
//	events.ControlLane       lossless FIFO — private truth and commands
//	events.MarketDataLane    per-SourceID mailboxes — bounded, coalescing
//
// Step dispatches at most one item: a due timer first, else the control lane's
// head, else one ready market-data mailbox's head. The rank and the market
// arbitration rule are documented on Step and recorded with their rejected
// alternatives in the §39 design record.
//
// # The core is clockless, and that is invariant 6 made structural
//
// The dispatcher holds a TimerSource and no Clock. §39f-6 says the dispatcher
// never advances time — the driver owns the clock: live mode's select loop
// waits on real time, the sim's driver advances TestClock only when nothing is
// dispatchable. Holding only the consumption interface makes that a
// compile-shaped guarantee rather than a discipline: TimerSource has no
// Advance, no SetNow, and no Now.
//
// # The persisted log
//
// Slice B's dispatch log (§39n) is here: LogWriter is a Sink that freezes each
// dispatch as a sequence record — capture's frame, sequences not payloads —
// ahead of the sink it wraps, and ReplayLog re-drives a consumer with the
// persisted order, which is §3.5's replay authority. Payloads are re-derived
// from the raw archive at replay; the log stores none.
//
// # What is deliberately NOT here
//
// The live pump's wake signal and the sim driver are slice C's wiring. The entry door for
// ABSENCE-derived envelopes — §3.5's silence rule, the one envelope that
// carries AcceptOrdinal == 0 — is deferred to the slice that builds its
// producer, §3.4's silence re-judger in marketdata: a door only tests would
// walk through is a check over an empty tree. Until then every envelope
// reaching this dispatcher is input-derived, and a zero ordinal is refused
// (Envelope.Validate, run post-stamp, exactly as §3.5 places it).
//
// The command sequencer (§3.4's fourth bullet) is not here either; its trigger
// is the first Risk→OMS command path, milestone 4.
package dispatch
