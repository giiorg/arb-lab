package reconcile

import (
	"fmt"
	"io"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// The operator report's renderer, moved here from cmd/arblab by F51's remedy.
//
// # Why the renderer is in the package that produces the Report
//
// It was in cmd/arblab, which is the obvious home for a CLI's output — and the
// obvious home was wrong for a reason no reading of either file would have
// surfaced. WriteDiagnosis walks Report.Reconciliation, whose type is
// application.Reconciliation, so every FIXTURE for it must name that type. Field
// access is structural and needs no import; a composite literal does. So the
// renderer's tests, and only its tests, made cmd/arblab an importer of the OMS
// use-case surface — and §3.6 rule 11's grant list carried cmd/arblab for that
// one reason, which handed `arblab`'s every subcommand unrestricted OMS write
// authority. Record §100 §4 then specified `cmd/arblab/serve.go` as the phase-1
// HTTP entrypoint, and the gate built to keep a web-facing process out of the
// write path was green on it (F51).
//
// Moving the renderer to the package that DECLARES Report is what lets that
// grant be deleted. It is not a workaround for the gate: the alternative
// considered and rejected was a set of type aliases re-exporting the value
// types from here, which would have deleted the declared import edge while
// leaving the same types named in the same fixtures — routing around this
// project's own measurement rather than answering it (record §102).
//
// The cohesion argument stands on its own. Report is declared here, ExitClass —
// what a run MEANS — is decided here, and the renderer must know the shape of
// every nested field it prints. What stays in cmd/arblab is the shell's genuine
// job: flag parsing, config loading, and the ExitClass→exit-code spelling.
//
// # Three shells compose these four functions, which is why they are separate
//
// `arblab reconcile run` prints WriteReport. `arblab exercise run` prints the
// verdict, its own execute section, then ONE rate-limit line drawn from the
// post-execute snapshot. `arblab pair run` prints verdict and diagnosis per
// side. The split is theirs, not this package's, and it predates the move.

// WriteReport renders what the boot found.
//
// Posture is printed FIRST and labelled with why it is what it is, because it is
// the number a reader will reach for as the verdict and it is not one: this
// binary declares all eight of §7.5's gates and produces five of them, so
// reconcile-only is the honest answer on a completely clean account.
func WriteReport(w io.Writer, r Report) {
	WriteVerdict(w, r)
	WriteRateLimitStatus(w, r.RateLimit)
	WriteDiagnosis(w, r)
}

// WriteVerdict is the report's judgement half: posture, exit class,
// and what latched. Split from the diagnosis so the exercise renderer can
// place the rate-limit section once, after its execute half.
func WriteVerdict(w io.Writer, r Report) {
	fmt.Fprintf(w, "posture:  %s (this binary reports 5 of §7.5's 8 gates; the rest have no producer yet)\n", r.Posture)
	fmt.Fprintf(w, "verdict:  %s\n", r.ExitClass())
	if len(r.Latched) > 0 {
		fmt.Fprintf(w, "latched:  %v  <- operator clearance required; a retry will not lift these\n", r.Latched)
	} else {
		fmt.Fprintln(w, "latched:  none")
	}
}

func WriteDiagnosis(w io.Writer, r Report) {
	fmt.Fprintf(w, "blocking: %v\n", r.Blocking)
	if r.Storage != nil {
		fmt.Fprintf(w, "storage:  %v\n", r.Storage)
	}
	// §4.1's verdict, and the two halves are not interchangeable: a cause means
	// the posture could not be ESTABLISHED (fix the wiring or retry), and a
	// failure list means it was read and the ACCOUNT is wrong. Every failed
	// check is printed rather than the first, so an operator acts once — a
	// checker that named one violation per boot costs a round trip per check.
	if r.PostureCause != nil {
		fmt.Fprintf(w, "posture!: could not be established: %v\n", r.PostureCause)
	}
	for _, f := range r.AccountPosture.Failures {
		fmt.Fprintf(w, "  posture %s\n", f)
	}

	rec := r.Reconciliation
	fmt.Fprintf(w, "replayed: %d records, %d dispatched orders\n",
		rec.Recovery.Records, len(rec.Recovery.Dispatched))
	fmt.Fprintf(w, "joined:   %d orders, %d fills admitted, %d duplicates\n",
		rec.Join.Orders, rec.Join.Fills, rec.Join.Duplicates)
	// application.Join's per-order finding lists, directly under the `joined:`
	// line they qualify and in joinOutcome's own ladder order — the two that
	// latch, then the one a retry may lift. The absence block below is a
	// different PHASE's evidence (§5.3's resolution decides what an absence
	// licenses), which is why it reads after these rather than among them.
	//
	// # An obligation, because the last version of this comment stated a FACT
	// # and the fact was false
	//
	// Every per-order finding list `application.Join` carries is EITHER
	// rendered here OR exempt by name, with its reason, in
	// TestReconcileReportRendersEveryJoinFindingList — and a list added there
	// owes one or the other. Absent is today's only exemption. The disjunction
	// is not hedging: the previous wording said every list is rendered here,
	// which is false of Absent and was rescued only by an exemption clause
	// several lines further down. A sentence true by virtue of a distant
	// clause is the same prose shape as a totality claim true only until
	// somebody edits elsewhere (ruling R13).
	//
	// That is a standing obligation rather than a description of today's
	// membership, and it is ENFORCED rather than asserted — because three
	// versions of this claim have now decayed silently. Task #242 added
	// Contradicted and found this renderer printing neither it nor
	// Unattributed; the comment then written to repair that claimed the
	// renderer enumerated "every other finding", while Incomplete was
	// unrendered the whole time; the test then written to enforce THAT
	// selected one exact element type, so a finding list of any other shape —
	// an id carried beside its reason, which Reconciliation.Unproved already
	// is — fell out of the guard without changing a thing it asserted (ruling
	// R12). Each version read as true and was checked by nothing. The failing
	// state now lives in the test, which is the only thing that has ever
	// caught this.
	//
	// The omission was not cosmetic. All three of these hold GateVenueJoined
	// below PostureTrading, so the boot stops and Blocking() names the gate —
	// and without a line here the operator gets a halted boot, a `joined:`
	// summary, no order id and no reason. Held, and not told what for.
	for _, id := range rec.Join.Unattributed {
		fmt.Fprintf(w, "  unattributed %s: the venue's cumulative disagrees with the executions this join could attribute\n", id)
	}
	for _, id := range rec.Join.Contradicted {
		fmt.Fprintf(w, "  contradicted %s: the venue and this system hold lifecycle states SM §1 has no edge between\n", id)
	}
	for _, id := range rec.Join.Incomplete {
		fmt.Fprintf(w, "  incomplete %s: the executions sweep did not prove coverage through the instant it was asked about (a retry may lift this)\n", id)
	}
	if n := len(rec.Join.Absent); n > 0 {
		fmt.Fprintf(w, "absent:   %d order(s) the venue does not hold; %d resolved, %d unproved\n",
			n, rec.Resolved, len(rec.Unproved))
	}
	for _, u := range rec.Unproved {
		fmt.Fprintf(w, "  unproved %s: %s\n", u.Order, u.Reason)
	}
	for _, id := range rec.Stale {
		fmt.Fprintf(w, "  stale projection for %s\n", id)
	}
	for _, d := range rec.Assertion.Unexplained {
		fmt.Fprintf(w, "  unexplained %s: venue %s, ours %s, delta %s\n",
			d.Instrument, d.Observed.Decimal(), d.Expected.Decimal(), d.Unexplained.Decimal())
	}
	for _, d := range rec.Assertion.SpotUnexplained {
		// Beside the derivative deltas rather than after the sections below:
		// the two are the SAME finding — §7.5's unexplained delta — on the
		// leg with no position surface, and an operator triaging a latch
		// reads them together (task #179, record §70c).
		fmt.Fprintf(w, "  spot unexplained %s %s: balance-minus-baseline %s, ours %s, delta %s\n",
			d.Venue, d.Asset, d.Observed.Decimal(), d.Expected.Decimal(), d.Unexplained.Decimal())
	}
	for _, s := range rec.Assertion.Foreign {
		fmt.Fprintf(w, "  foreign symbol %s\n", s)
	}
	for _, u := range rec.Assertion.Unasserted {
		fmt.Fprintf(w, "  unasserted %s: %s\n", u.Instrument, u.Reason)
	}
	for _, u := range rec.Assertion.SpotUnconcluded {
		fmt.Fprintf(w, "  spot unconcluded %s %s: %s (re-reportable; a further attempt may pin it)\n",
			u.Venue, u.Asset, u.Reason)
	}
	if rec.Cause != nil {
		fmt.Fprintf(w, "cause:    %v\n", rec.Cause)
	}
}

// WriteRateLimitStatus renders §7.6's rate-limit facts, and it draws that
// section's one line in the output: a ban is an OPERATOR ALERT, a throttle
// pause is information. A zero snapshot — no venue client was ever built, so no
// venue traffic happened — prints nothing at all, because a standing line that
// is almost always "nothing" is how an operator learns to skip the one that
// is not.
func WriteRateLimitStatus(w io.Writer, s ratelimit.Snapshot) {
	for _, l := range s.Limits {
		if l.FoldMisses > 0 {
			fmt.Fprintf(w, "ratelimit: %s %s: %d response(s) carried no usable budget reading this run (§7.6 fold-health — venue-side corrections were missed)\n",
				s.Venue, l.Name, l.FoldMisses)
		}
	}
	for _, l := range s.Limits {
		if l.Reserved > 0 {
			fmt.Fprintf(w, "ratelimit: %s %s: %d unit(s) reserved for an in-flight package\n",
				s.Venue, l.Name, l.Reserved)
		}
	}
	if s.Bans > 0 {
		fmt.Fprintf(w, "ratelimit: %s banned this IP %d time(s) (418/IP ban)  <- §7.6 OPERATOR ALERT: do not retry — repeat violations escalate the ban",
			s.Venue, s.Bans)
		if s.Paused {
			fmt.Fprintf(w, "; paused until %s", s.PausedUntil.UTC().Format(time.RFC3339))
		}
		fmt.Fprintln(w)
		return
	}
	if s.Paused {
		fmt.Fprintf(w, "ratelimit: %s paused until %s (%s — a hard pause, not an operator matter)\n",
			s.Venue, s.PausedUntil.UTC().Format(time.RFC3339), s.PauseKind)
	}
}
