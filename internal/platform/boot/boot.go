// Package boot is build-spec §3.2's bootstrap phases and §7.5's reconcile-only
// boot: the ordered start-up, the preconditions that must be satisfied before
// trading resumes, and the bounded teardown on the way out.
//
// # Reconcile-only is a posture, not a stage
//
// §7.5 reads as a sequence — rebuild state, assert the invariants, resume — and
// the obvious implementation follows the prose: a list of steps, each a
// func() error, and trading starts when the last one returns nil. That shape is
// wrong, and it is wrong in the type signature rather than anywhere a test
// would find it.
//
// A func() error has two outcomes. §7.5 has three. Trading may resume; the boot
// may fail; or the process may end up ALIVE AND REFUSING — foreign orders and
// unexplained position deltas "alert for operator clearance" and are "not
// auto-corrected", an Ambiguous MarginOp blocks new openings "until the
// staged-float ledger is explicitly reconciled", and §7.4 puts a bot that
// recovers mid-takeover in reconcile-only "until an operator clears it". None
// of those is a boot failure and none of them is a boot success. The middle
// outcome is the one the package is named after, and it is the one the obvious
// signature cannot express.
//
// Collapsing it costs money in both directions. Folded into failure, the
// process exits — so a condition the spec answers with "alert an operator and
// keep monitoring" is answered by making §7.4's actuator cover a host that died
// deliberately, with the exposure it was watching now unmanaged by anything on
// this machine. Folded into success, trading resumes over an unexplained
// position delta, which is §7.5's own example of the thing that must not be
// auto-corrected.
//
// So the verdict is a Posture — a four-valued ladder from PostureHalted to
// PostureTrading — and Checklist computes it from §7.5's named preconditions by
// taking the weakest. Default-deny falls out of the reduction: it runs over the
// gates that were REQUIRED, not the ones that were reported, so forgetting to
// report cannot produce a more permissive answer than reporting a failure.
//
// # What is here, and what is deliberately not
//
// The journal is milestone 3, and that draws the same line platform/outbox drew
// for its store. Absent, as a decision:
//
//   - the join itself — OrderByClientID, Fills, Positions, FundingLedger
//   - the write-ahead journal replay
//   - the net-delta computation and the §4.1 posture REST calls
//
// A Reconciler port here would compile and would read as §7.5's contract, and
// it would be a shape guessed at with nothing to check it against: the join's
// arguments are the journal's cursors, which do not exist. Worse, it would make
// the checklist look satisfiable — an implementation returning nil would report
// every gate Passed with nothing joined, and the boot would be green.
//
// What IS buildable is everything that decides what the join's result MEANS,
// and that is the half where the defects live. The gates, their outcomes, the
// clearance rule, the posture ladder, the ordering constraints and the teardown
// bound are all checkable today, and none of them changes when the journal
// lands. The gate roster is a closed vocabulary rather than caller data, which
// is a departure from the "machinery, not a table" split ratelimit, config and
// apperror took — see Gate for why §7.5 is the opposite case.
//
// # Two ordering constraints §3.2 implies and never states
//
// §3.2's roster is "signal, config, obs, reconcile-only-boot, graceful
// shutdown". Read as a list it is unremarkable. Read as constraints, two of the
// positions are load-bearing, and both failures are invisible when they happen:
//
// Signal is FIRST because a Go process's default disposition for SIGINT and
// SIGTERM is to die. Installed after a forty-second journal join, an operator's
// interrupt during that join terminates the process outright — and the window
// where a boot is slowest is the window where an operator is most likely to
// interrupt it.
//
// Observability precedes everything that can fail because every later phase
// reports its failure through the machinery this one installs. Started late, a
// boot that dies in the journal join does not merely lose telemetry; it dies
// SILENTLY, leaving an exit code as the diagnosis.
//
// Sequence.Validate enforces both. It does not close the roster: KindService is
// an open slot, because §11 gives the three binaries different phase sets and
// forcing a venue client into one of §3.2's five buckets would misname it.
//
// # The shutdown budget is a safety number
//
// §7.4 gives the watchdog a host-silence trigger and §7.5
// gives the process a graceful shutdown, and nothing joins them — but they run
// against the same wall clock, and the join has a wrong side. A total stop
// budget at or above the silence threshold means an ORDINARY RESTART is
// indistinguishable, to the watchdog, from a dead host: it escalates, the
// actuator fences the bot's signer mid-flush and latches a takeover epoch, and
// the process that comes back up finds itself fenced and stays in reconcile-only
// until an operator clears it. A deployment becomes an operator-cleared
// incident, and the cause is a sum of durations in a wiring file that nothing
// was adding up. Sequence.Validate adds them up.
//
// The bound is declared, allotted and accounted for — not enforced. This
// package has no goroutines (platform/clock's call, for platform/clock's
// reason), so it cannot interrupt a closer that ignores its context. It says so
// rather than pretending otherwise, and the enforcement it lacks lives where an
// operator can reach it: SignalImmediateHalt. See Shutdown for why every closer
// runs even after the budget is gone, and SignalImmediateHalt for why handing
// an operator a way to kill a trading process is the safe direction here.
package boot
