// Package exercise is the composition root for §9's synthetic spot↔perp cycle
// driver: the first binary in this repo that DISPATCHES — and it dispatches
// only after running §7.5's reconcile-only boot to conclusion, on the same
// checklist, through the same phases, in the same order as `arblab reconcile`.
//
// # A–F composed, then G
//
// The boot is reconcile.Runner's — this package deliberately owns none of it.
// What it adds is one decision and one act: read the boot's verdict, and if
// and only if that verdict is ExitClean, plan and execute the configured
// package through oms.Module's own use cases. §59d records the decision this
// package is built on: the gate is the VERDICT, not the posture, because this
// deployment's posture is PostureReconcileOnly on every boot while five of
// §7.5's gates have no producer — a driver gated on PostureTrading could never
// run, waiting on contexts sequenced after the evidence it exists to produce.
// The regime is stated plainly: this binary trades testnet-class targets under
// a posture naming five unverified preconditions, which is §12's sub-$1K carve-
// out, and the same gate tightens by itself as producers land.
//
// # Execution is NOT a boot phase
//
// boot.Sequence enforces KindReconcile last, and that rule is right: a phase
// after the verdict would run while the process reports whether it may trade.
// So the execute step happens AFTER the sequence concludes and BEFORE its
// teardown — runtime work gated on the verdict, exactly where a strategy
// loop will sit in the engine binary.
package exercise

import (
	"context"
	"fmt"
	"io"

	"github.com/giiorg/arb-lab/internal/app/reconcile"
	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/config"
	"github.com/giiorg/arb-lab/internal/platform/observability"
)

// Report is what one exercise run established: the boot's whole report, and
// what the execute step did about it.
type Report struct {
	// Boot is §7.5's verdict, unchanged from the reconcile binary's — the
	// exercise adds to it and never re-derives it.
	Boot reconcile.Report

	// Executed reports whether the package was fanned out at all. False with
	// a Refusal naming why: the boot's verdict, or a package-level refusal
	// from planning or the executor's preflight — in every such case NOTHING
	// was sent.
	Executed bool

	// Refusal is why Executed is false, nil when it is true.
	Refusal error

	// Execution is the per-leg outcome, meaningful only when Executed.
	Execution application.Execution
}

// ExitClass is what this run means for the operator driving §9's campaign.
type ExitClass uint8

const (
	// ExitClean : the boot was clean, every leg's Place ran, every leg reached
	// StateSubmitted with its records journaled and applied.
	ExitClean ExitClass = iota

	// ExitOperator is the boot's own operator alert — a latched gate. Nothing
	// was executed.
	ExitOperator

	// ExitIncomplete : the boot could not finish, or the execute step refused
	// before any leg was attempted (a planning refusal, a package preflight
	// refusal, the boot verdict). Nothing was sent; a retry may lift it.
	ExitIncomplete

	// ExitExecution : the package was dispatched and at least one leg needs a
	// human's eyes — a leg refused inside the fan-out, a Place that failed or
	// was rejected, a record that did not journal, or an apply whose failure
	// holds new trading (§6 step 7). The other legs may be live at the venue,
	// which is exactly why this is not ExitIncomplete: a retry is NOT safe,
	// §7.5's boot is the recovery path.
	ExitExecution
)

// String implements fmt.Stringer.
func (e ExitClass) String() string {
	switch e {
	case ExitClean:
		return "ExitClean"
	case ExitOperator:
		return "ExitOperator"
	case ExitIncomplete:
		return "ExitIncomplete"
	case ExitExecution:
		return "ExitExecution"
	}
	return fmt.Sprintf("ExitClass(%d)", uint8(e))
}

// ExitClass reduces the report, boot verdict first: a latched gate outranks
// everything on reconcile's own precedence, and an un-clean boot means nothing
// was executed, so the boot's class IS the run's.
func (r Report) ExitClass() ExitClass {
	switch r.Boot.ExitClass() {
	case reconcile.ExitOperator:
		return ExitOperator
	case reconcile.ExitIncomplete:
		return ExitIncomplete
	case reconcile.ExitClean:
		// Fall through to the execute step's own verdict.
	}
	if !r.Executed {
		return ExitIncomplete
	}
	for _, leg := range r.Execution.Legs {
		if leg.Err != nil || leg.Outcome.State != domain.StateSubmitted ||
			!leg.Outcome.Journaled || !leg.Outcome.Applied {
			return ExitExecution
		}
	}
	return ExitClean
}

// Run performs one boot-then-execute cycle.
//
// The error contract is reconcile.Run's: an error means the boot could not be
// PERFORMED. Every refusal — including the execute step's — is in the Report.
func Run(ctx context.Context, snap config.Snapshot[Config], wall *clock.Wall,
	reg *observability.Registry, stderr io.Writer) (Report, error) {

	// The boot is the reconcile root's, over the embedded §5.2 block. The
	// snapshot is re-wrapped rather than re-read: same bytes, same revision,
	// one bootstrap read.
	boot := config.Snapshot[reconcile.Config]{Value: snap.Value.OMS, Rev: snap.Rev}
	r, err := reconcile.NewRunner(boot, wall, reg, stderr)
	if err != nil {
		return Report{}, err
	}

	running, err := r.Sequence().Run(ctx)
	if err != nil {
		return Report{Boot: r.BootReport()}, err
	}

	rep := Report{Boot: r.BootReport()}
	rep.Executed, rep.Execution, rep.Refusal = execute(snap.Value, r)

	shutdown, _ := running.Shutdown(ctx, wall)
	for _, p := range shutdown.Phases {
		if p.Err != nil {
			fmt.Fprintf(stderr, "exercise: phase %q close: %v\n", p.Name, p.Err)
		}
	}
	return rep, nil
}

// execute is the one step this root adds: verdict, plan, fan out.
func execute(cfg Config, r *reconcile.Runner) (bool, application.Execution, error) {
	boot := r.BootReport()
	if class := boot.ExitClass(); class != reconcile.ExitClean {
		return false, application.Execution{}, fmt.Errorf("boot verdict is %s: %w", class, ErrBootNotClean)
	}
	if r.Halted() || r.Module() == nil {
		// Unreachable while ExitClean implies a completed boot, and checked
		// because dispatching through a halted boot's module is the one
		// mistake this function must be unable to make.
		return false, application.Execution{}, fmt.Errorf("boot halted: %w", ErrBootNotClean)
	}

	plan, err := cfg.plan()
	if err != nil {
		return false, application.Execution{}, err
	}
	records, err := r.Module().Plan.Plan(plan)
	if err != nil {
		// Plan's contract: nothing is dispatchable. A journal already holding
		// this trade's ids lands here — §5.3 forbids the reuse, and the §9
		// campaign advances the trade id per cycle.
		return false, application.Execution{}, err
	}

	refs := make([]application.DispatchRef, len(records))
	for i, rec := range records {
		refs[i] = application.DispatchRef{ID: rec.Order.ClientOrderID, Version: rec.Version}
	}
	// Entry legs create exposure: §7.6's Opening class, by definition — see
	// ports.Urgency. A close cycle is also an "opening" in the shed sense only
	// when it is not §6.7 safety traffic, which a configured exercise is not.
	ex, err := r.Module().Execute.Execute(refs, application.UrgencyOpening)
	if err != nil {
		// The package preflight refused whole: nothing was sent.
		return false, application.Execution{}, err
	}
	return true, ex, nil
}
