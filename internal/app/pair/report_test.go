package pair

import (
	"errors"
	"testing"

	"github.com/giiorg/arb-lab/internal/app/reconcile"
	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/boot"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// cleanBoot is this deployment's real clean shape: the two producerless gates
// block, nothing latches. An EMPTY Blocking would test a deployment that does
// not exist and would collapse "boot clean" into "checklist silent".
func cleanBoot() reconcile.Report {
	return reconcile.Report{
		Posture:  boot.PostureReconcileOnly,
		Blocking: []boot.Gate{boot.GateDeactivationsResolved, boot.GateWriterAuthority},
	}
}

// latchedBoot is a gate this deployment DOES produce, latched.
func latchedBoot() reconcile.Report {
	return reconcile.Report{
		Posture:  boot.PostureHalted,
		Latched:  []boot.Gate{boot.GateNetDeltaAsserted},
		Blocking: []boot.Gate{boot.GateNetDeltaAsserted},
	}
}

// incompleteBoot is a boot that could not finish: a real gate blocks and
// nothing latched.
func incompleteBoot() reconcile.Report {
	return reconcile.Report{
		Posture:  boot.PostureHalted,
		Blocking: []boot.Gate{boot.GateJournalReplayed},
	}
}

// cleanLeg is a leg that was sent and whose record landed whole.
func cleanLeg(id ids.ClientOrderID) *LegReport {
	return &LegReport{
		ID: id,
		Result: application.LegResult{
			ID: id,
			Outcome: application.Outcome{
				State: domain.StateSubmitted, Journaled: true, Applied: true,
			},
		},
	}
}

// unjournaledLeg was SENT and its record needs eyes: the order left the process
// and the write that would have recorded it did not land.
func unjournaledLeg() *LegReport {
	leg := cleanLeg("ab-01J8ZQ4M7K3P9V2XW5R6T8Y0BN-00-00")
	leg.Result.Outcome.Journaled = false
	return leg
}

// refusedLeg was ATTEMPTED and nothing was sent.
//
// It must differ from unjournaledLeg in whether Place ran and in NOTHING else
// ExitClass reads, or the two rungs it separates collapse: its Result is the
// zero value precisely because there is no outcome to report.
func refusedLeg() *LegReport {
	return &LegReport{Refusal: errors.New("refused before the wire")}
}

// reportWith is a pair whose two sides each carry one clean, journaled open leg.
func reportWith(bootA, bootB reconcile.Report, verdict Verdict, drove bool) Report {
	r := Report{Verdict: verdict, Drove: drove}
	r.Sides[0] = SideReport{Venue: "binance", Boot: bootA, Open: cleanLeg("ab-01J8ZQ4M7K3P9V2XW5R6T8Y0BN-00-00")}
	r.Sides[1] = SideReport{Venue: "bybit", Boot: bootB, Open: cleanLeg("ab-01J8ZQ4M7K3P9V2XW5R6T8Y0BN-00-00")}
	return r
}

// reportWithLegs is reportWith with side 0's open leg replaced.
func reportWithLegs(bootA, bootB reconcile.Report, verdict Verdict, leg *LegReport) Report {
	r := reportWith(bootA, bootB, verdict, true)
	r.Sides[0].Open = leg
	return r
}

// TestExitClassPrecedence pins the ORDER, which is the whole content of this
// method. Each case is built so that exactly one rung applies and the rungs
// below it would answer differently — a fixture satisfying only one rung shows
// every rung and hides the ladder's order.
func TestExitClassPrecedence(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		rep  Report
		want ExitClass
	}{
		{
			name: "a latched gate on either side outranks everything",
			rep:  reportWith(latchedBoot(), cleanBoot(), VerdictIntact, true),
			want: ExitOperator,
		},
		{
			// The mirror. A precedence that read side 0 only would pass above
			// and fail here.
			name: "a latched gate on side B",
			rep:  reportWith(cleanBoot(), latchedBoot(), VerdictIntact, true),
			want: ExitOperator,
		},
		{
			name: "an unfinished boot with nothing sent",
			rep:  reportWith(incompleteBoot(), cleanBoot(), VerdictUnopened, false),
			want: ExitIncomplete,
		},
		{
			name: "an unfinished boot on side B",
			rep:  reportWith(cleanBoot(), incompleteBoot(), VerdictUnopened, false),
			want: ExitIncomplete,
		},
		{
			// The gate refused after two clean boots: nothing was sent, and a
			// retry may lift it.
			name: "clean boots, a drive the gate refused",
			rep:  reportWith(cleanBoot(), cleanBoot(), VerdictUnopened, false),
			want: ExitIncomplete,
		},
		{
			// Exposure outranks a lost record: a live unbalanced position is
			// more urgent than an unjournaled ack.
			name: "exposure outranks a leg needing eyes",
			rep:  reportWithLegs(cleanBoot(), cleanBoot(), VerdictSkewed, unjournaledLeg()),
			want: ExitExposed,
		},
		{
			// A ban earned during dispatch is in the post-drive RateLimit
			// snapshot, not Boot. It is operator work when no exposure outranks
			// it.
			//
			// The leg is UNJOURNALED on purpose. With clean legs the rung below
			// this one would not fire either, so "the ban outranks a leg needing
			// eyes" and "the ban outranks a clean run" would be the same
			// observation — a one-violation fixture shows every rung and hides
			// the ladder's order. Here ExitExecution is genuinely available and
			// the ban has to beat it.
			name: "a runtime venue ban outranks a leg needing eyes",
			rep: func() Report {
				r := reportWithLegs(cleanBoot(), cleanBoot(), VerdictIntact, unjournaledLeg())
				r.Sides[1].RateLimit.Bans = 1
				return r
			}(),
			want: ExitOperator,
		},
		{
			name: "a dispatched leg needing eyes on an unexposed pair",
			rep:  reportWithLegs(cleanBoot(), cleanBoot(), VerdictIntact, unjournaledLeg()),
			want: ExitExecution,
		},
		{
			// A refused leg sent nothing, so it is ExitIncomplete's and not
			// ExitExecution's. Told apart from the case above by exactly one
			// fact, which is the point: the two legs differ only in whether
			// Place ran.
			name: "a refused leg sent nothing",
			rep:  reportWithLegs(cleanBoot(), cleanBoot(), VerdictIntact, refusedLeg()),
			want: ExitIncomplete,
		},
		{
			// Both classes present at once, on OPPOSITE sides, so the answer
			// cannot come from whichever side the loop happens to reach first.
			// A sent-and-broken record outranks a leg that never left.
			name: "a refused leg on side A and a broken record on side B",
			rep: func() Report {
				r := reportWithLegs(cleanBoot(), cleanBoot(), VerdictIntact, refusedLeg())
				r.Sides[1].Open = unjournaledLeg()
				return r
			}(),
			want: ExitExecution,
		},
		{
			// And the mirror of that pair, because "scan both before answering"
			// is a claim about ORDER and one arrangement cannot show it.
			name: "a broken record on side A and a refused leg on side B",
			rep: func() Report {
				r := reportWithLegs(cleanBoot(), cleanBoot(), VerdictIntact, unjournaledLeg())
				r.Sides[1].Open = refusedLeg()
				return r
			}(),
			want: ExitExecution,
		},
		{
			// The UNWIND is scanned too, not only the opening leg. A report that
			// looked at Open alone would call this clean while the order that
			// closed the exposure had no record.
			name: "an unwind whose record needs eyes",
			rep: func() Report {
				r := reportWith(cleanBoot(), cleanBoot(), VerdictUnopened, true)
				r.Sides[0].Unwind = unjournaledLeg()
				return r
			}(),
			want: ExitExecution,
		},
		{
			name: "two clean boots, an intact pair, every leg journaled",
			rep:  reportWith(cleanBoot(), cleanBoot(), VerdictIntact, true),
			want: ExitClean,
		},
		{
			// D6: an IOC that did not fill is CLEAN. It is the likely demo
			// outcome and must not read as a failure.
			name: "nothing filled on either side",
			rep:  reportWith(cleanBoot(), cleanBoot(), VerdictUnopened, true),
			want: ExitClean,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.rep.ExitClass(); got != tc.want {
				t.Errorf("ExitClass() = %s, want %s", got, tc.want)
			}
		})
	}
}

// TestExitClassStringNamesEveryMember is the roster check. A member added
// without a String arm renders as ExitClass(5) in the operator's report and in
// the exit-code table, which is the one place a new class must not be silent.
func TestExitClassStringNamesEveryMember(t *testing.T) {
	t.Parallel()
	want := map[ExitClass]string{
		ExitClean:      "ExitClean",
		ExitOperator:   "ExitOperator",
		ExitIncomplete: "ExitIncomplete",
		ExitExecution:  "ExitExecution",
		ExitExposed:    "ExitExposed",
	}
	for class, name := range want {
		if got := class.String(); got != name {
			t.Errorf("ExitClass(%d).String() = %q, want %q", uint8(class), got, name)
		}
	}
	if len(want) != 5 {
		t.Fatalf("the ExitClass roster has %d members here; add the new one to this table", len(want))
	}
	if got := ExitClass(len(want)).String(); got != "ExitClass(5)" {
		t.Errorf("an unnamed class renders as %q; it must be visibly unnamed rather than borrowing a name", got)
	}
}
