package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	appexercise "github.com/giiorg/arb-lab/internal/app/exercise"
	appreconcile "github.com/giiorg/arb-lab/internal/app/reconcile"
	"github.com/giiorg/arb-lab/internal/platform/boot"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// TestExerciseReportPrintsTheBanAlertExactlyOnce polices the composition:
// writeExerciseReport renders the boot through writeReconcileReport — which
// prints the PRE-execute snapshot — and then adds its own post-execute call,
// gated on Executed. The two cases below are the two ways a ban can arrive,
// and in each exactly ONE of the two calls must speak, or the operator reads
// two counts for one venue and has to work out which is real.
func TestExerciseReportPrintsTheBanAlertExactlyOnce(t *testing.T) {
	until := time.Date(2026, 8, 3, 15, 0, 0, 0, time.UTC)
	banned := ratelimit.Snapshot{
		Venue: ids.Venue("binance"), Bans: 1,
		Paused: true, PausedUntil: until, PauseKind: ratelimit.PenaltyBan,
	}

	// A ban earned during the BOOT: nothing executed, the post-execute
	// snapshot holds the same facts, and only the boot's line may print.
	var buf bytes.Buffer
	writeExerciseReport(&buf, appexercise.Report{
		Boot: appreconcile.Report{
			Posture:   boot.PostureReconcileOnly,
			Blocking:  []boot.Gate{boot.GateNetDeltaAsserted},
			RateLimit: banned,
		},
		Executed:  false,
		Refusal:   errors.New("boot verdict is ExitOperator: boot did not conclude clean"),
		RateLimit: banned,
	})
	if n := strings.Count(buf.String(), "OPERATOR ALERT"); n != 1 {
		t.Errorf("boot-banned report contains %d alert lines, want exactly 1:\n%s", n, buf.String())
	}

	// A ban earned during the DISPATCH: the boot's snapshot is clean, so the
	// post-execute line is the only one that can say so — and must.
	buf.Reset()
	writeExerciseReport(&buf, appexercise.Report{
		Boot: appreconcile.Report{
			Posture:   boot.PostureReconcileOnly,
			Blocking:  []boot.Gate{boot.GateAccountPosture},
			RateLimit: ratelimit.Snapshot{Venue: ids.Venue("binance")},
		},
		Executed:  true,
		RateLimit: banned,
	})
	if n := strings.Count(buf.String(), "OPERATOR ALERT"); n != 1 {
		t.Errorf("dispatch-banned report contains %d alert lines, want exactly 1:\n%s", n, buf.String())
	}
	if !strings.Contains(buf.String(), "banned this IP 1 time(s)") {
		t.Errorf("dispatch-banned report does not carry the ban:\n%s", buf.String())
	}
}

func TestExerciseReportPrintsFoldHealthExactlyOnceFromTheFinalSnapshot(t *testing.T) {
	bootSnapshot := ratelimit.Snapshot{
		Venue:  ids.Venue("bybit"),
		Limits: []ratelimit.LimitState{{Name: "order_realtime", FoldMisses: 1}},
	}
	final := ratelimit.Snapshot{
		Venue:  ids.Venue("bybit"),
		Limits: []ratelimit.LimitState{{Name: "order_realtime", FoldMisses: 2}},
	}

	var buf bytes.Buffer
	writeExerciseReport(&buf, appexercise.Report{
		Boot: appreconcile.Report{
			Posture:   boot.PostureReconcileOnly,
			Blocking:  []boot.Gate{boot.GateAccountPosture},
			RateLimit: bootSnapshot,
		},
		Executed:  true,
		RateLimit: final,
	})
	got := buf.String()
	if n := strings.Count(got, "ratelimit:"); n != 1 {
		t.Errorf("executed report contains %d rate-limit lines, want exactly 1:\n%s", n, got)
	}
	if !strings.Contains(got, "2 response(s) carried no usable budget reading this run") {
		t.Errorf("executed report does not carry the final miss count:\n%s", got)
	}

	buf.Reset()
	writeExerciseReport(&buf, appexercise.Report{
		Boot: appreconcile.Report{
			Posture:   boot.PostureReconcileOnly,
			Blocking:  []boot.Gate{boot.GateAccountPosture},
			RateLimit: bootSnapshot,
		},
		Executed:  false,
		Refusal:   errors.New("boot did not conclude clean"),
		RateLimit: final,
	})
	got = buf.String()
	if n := strings.Count(got, "ratelimit:"); n != 1 {
		t.Errorf("not-executed report contains %d rate-limit lines, want exactly 1:\n%s", n, got)
	}
	if !strings.Contains(got, "1 response(s) carried no usable budget reading this run") {
		t.Errorf("not-executed report does not carry the boot miss count:\n%s", got)
	}
}
