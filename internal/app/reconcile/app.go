// Package reconcile is the composition root for §7.5's reconcile-only boot: the
// first thing in this repo that constructs a boot.Checklist, and the first that
// builds a venue client from CONFIGURATION rather than from a test fixture.
//
// # What three slices of deferral were waiting on, named as constructs
//
// F1, F2 and G1 each closed recording that platform/boot had no importer outside
// itself. G1's deferral finally named what it waited on rather than what it
// wanted: "a venue.OrderClient built from configuration", because §5.2's config
// schema for the OMS did not exist and credential resolution had no home outside
// engine/venue's own package. Both are here. config.SecretRef's own doc had
// already assigned the second — "it belongs in the composition root" — so this
// package is a scheduled arrival rather than a new idea.
//
// # The checklist is declared HERE, over all EIGHT gates, and five have no
// # producer
//
// That is deliberate and it is the safe shape rather than the tidy one. §7.5's
// roster spans contexts: the account posture is risk's and engine/venue's, the
// takeover epoch is §7.4's, the deactivation and margin-op re-drives are the
// emergency path's. A checklist declaring only the three this context can report
// would produce a boot that checked three of eight preconditions and reported
// PostureTrading — §7.5 argues that exact failure from the other side, about the
// fold's seed, and a short roster reaches it through the roster instead of
// through the arithmetic.
//
// The consequence is that this binary's posture is PostureReconcileOnly on every
// run, however healthy the account, and that is the honest answer rather than a
// limitation to work around. What it costs is that the POSTURE cannot be the
// tool's verdict — see Report.ExitClass, which is the finding this wiring
// produced.
package reconcile

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/giiorg/arb-lab/internal/oms"
	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/platform/boot"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/config"
	"github.com/giiorg/arb-lab/internal/platform/observability"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// Report is what one reconcile-only boot established, as an operator reads it.
type Report struct {
	// Posture is the weakest posture consistent with every required gate, and on
	// this binary it is PostureReconcileOnly always — see the package doc.
	Posture boot.Posture

	// Blocking are the gates holding the posture below trading, and Latched are
	// the ones no retry can lift. Latched is a strict subset.
	Blocking []boot.Gate
	Latched  []boot.Gate

	// Reconciliation is §7.5's own diagnosis: what the replay rebuilt, what the
	// join found, which absences §5.3's rule could not conclude about.
	Reconciliation application.Reconciliation

	// Storage is #168's verdict on the journal's filesystem, nil when it holds.
	Storage error

	// RateLimit is the venue governor's state as this report was taken — §7.6's
	// alert facts, read by ExitClass (Bans) and rendered for the operator. Zero
	// when the boot halted before a venue client existed, and that zero is the
	// true answer rather than a blind spot: a penalty is filed only by a
	// transport, and every transport spends against this governor, so no
	// governor means no venue traffic and therefore no ban to report.
	RateLimit ratelimit.Snapshot
}

// ExitClass is what this run means, and it is deliberately NOT derived from the
// posture.
//
// # The posture cannot be the verdict, and finding that out is what wiring a
// # consumer is for
//
// Checklist.Posture reduces over the REQUIRED gates, which is the default-deny
// mechanism §7.5 insists on — and this root correctly declares five gates whose
// producers live in contexts that do not exist yet. So the posture is
// PostureReconcileOnly on a boot that found nothing at all and on a boot that
// found a foreign order, and an exit code derived from it would answer the same
// on both. Blocking() has the same problem one layer down: it names those five
// on every run, so a real finding arrives as one row among six.
//
// What distinguishes the two is whether anything LATCHED — which is precisely
// §7.5's own distinction between preconditions that clear themselves and ones
// that "alert for operator clearance", and which had no read-only accessor until
// Checklist.Latched. A tool whose whole job is to show a human what is waiting
// for them could not ask the question its own spec section is about.
type ExitClass uint8

const (
	// ExitClean is a reconciliation that ran to its end and found nothing a human
	// must act on. It does NOT mean the process may trade.
	ExitClean ExitClass = iota

	// ExitOperator is an operator alert, from either of the two sections that
	// mandate one. §7.5's: a latched gate — a foreign order, an unexplained
	// position delta, a journal whose chain does not verify, an instrument
	// outside the invariant for a reason waiting cannot discharge. §7.6's: the
	// venue banned this IP (Binance 418, Bybit's 10018/403 gateway ban), which
	// no retry may answer — on Binance a repeat violation escalates the ban
	// from two minutes toward three days, so the retry ExitIncomplete invites
	// is exactly the response the ban forbids.
	ExitOperator

	// ExitIncomplete is a reconciliation that could not finish: a venue that
	// would not answer, a journal that would not open, a filesystem this build
	// cannot vouch for. Nothing is known to be wrong; nothing is known to be
	// right either, which is why it is not ExitClean.
	ExitIncomplete
)

// ExitClass reduces the report to what an operator does next.
//
// Latching outranks incompleteness: a boot that both failed to reach the venue
// AND holds an operator alert is an operator matter, because the alert survives
// every retry the incompleteness invites.
func (r Report) ExitClass() ExitClass {
	if len(r.Latched) > 0 {
		return ExitOperator
	}
	// §7.6's MUST: "surface 418 / IP ban as immediate operator alerts". This is
	// NOT the second-reading trap the comment below records for Storage and
	// Cause, because no gate carries this fact: a 418 during the join reports
	// GateVenueJoined as OutcomeFailed, which alone reads ExitIncomplete —
	// "a retry may lift it" — and retrying into a Binance ban is what escalates
	// it toward three days. The counter is the ONLY reading of the ban, and it
	// also covers the quieter shape where a fallback absorbed the banned call
	// and every gate concluded clean.
	if r.RateLimit.Bans > 0 {
		return ExitOperator
	}
	// Storage and Reconciliation.Cause are deliberately NOT consulted, and a
	// mutation deleting a version of this method that did consult them SURVIVED —
	// correctly, because every path that sets either of them also reports a
	// produced gate below Passed, so the loop below already answers. A second
	// reading of the same fact is not defence in depth: it is a branch nothing can
	// execute, which reads as handled and stops the next reader asking which of
	// the two is the authority. §7.5's answer is the checklist; those two fields
	// are the diagnosis an operator reads afterwards.
	//
	// An unreported required gate that nothing in this binary produces is NOT
	// incompleteness — it is this deployment's known shape, and reporting it as a
	// failure every single run is how an operator learns to ignore the exit code.
	// What is incomplete is a gate this binary DOES produce and did not conclude.
	for _, g := range r.Blocking {
		switch g {
		case boot.GateJournalDurable, boot.GateJournalReplayed,
			boot.GateVenueJoined, boot.GateNetDeltaAsserted:
			return ExitIncomplete
		case boot.GateUnspecified, boot.GateAccountPosture, boot.GateDeactivationsResolved,
			boot.GateMarginOpsResolved, boot.GateWriterAuthority:
			// The five with no producer anywhere, named rather than defaulted
			// (§3.6 rule 10): a gate added to §7.5's roster has to choose a side
			// here, and choosing "not ours" by falling through is how a real
			// refusal becomes invisible.
		}
	}
	return ExitClean
}

// String implements fmt.Stringer.
func (e ExitClass) String() string {
	switch e {
	case ExitClean:
		return "ExitClean"
	case ExitOperator:
		return "ExitOperator"
	case ExitIncomplete:
		return "ExitIncomplete"
	}
	return fmt.Sprintf("ExitClass(%d)", uint8(e))
}

// Run performs one reconcile-only boot and returns what it found.
//
// # The sequence's own bounds cannot come from inside the sequence
//
// boot.Sequence.Validate refuses a total stop budget at or above §7.4's watchdog
// silence, and Run checks it BEFORE starting any phase — so the two numbers must
// exist before the first phase runs, and they are configuration. A boot that
// loaded its own config inside its KindConfig phase could therefore not bound its
// own teardown until after that phase had already run unbounded. So the caller
// performs the bootstrap read and hands the snapshot in, and what the config
// phase does instead is the work that genuinely cannot happen earlier: resolving
// §5.2's key refs into the one credential this process holds.
//
// # An error result means the BOOT could not be performed, never that it refused
//
// Every refusal §7.5 can legitimately reach is in the Report. The error result is
// for a sequence that would not validate or a phase that could not start — a
// distinction §7.5 spends a paragraph on, because boot.Run turns a phase error
// into an unwind and a dead process, and a journal that will not verify must
// leave a process that is alive, halted and audible to §7.4's watchdog.
func Run(ctx context.Context, snap config.Snapshot[Config], wall *clock.Wall,
	reg *observability.Registry, stderr io.Writer) (Report, error) {

	r, err := NewRunner(snap, wall, reg, stderr)
	if err != nil {
		return Report{}, err
	}

	running, err := r.Sequence().Run(ctx)
	if err != nil {
		return r.BootReport(), err
	}
	// The teardown runs on the same budget the sequence validated against the
	// watchdog threshold, and its failures are reported rather than returned: a
	// journal that would not close is a real problem and it is not a reason to
	// discard the verdict this boot just established.
	shutdown, _ := running.Shutdown(ctx, wall)
	for _, p := range shutdown.Phases {
		if p.Err != nil {
			fmt.Fprintf(stderr, "reconcile: phase %q close: %v\n", p.Name, p.Err)
		}
	}
	return r.BootReport(), nil
}

// NewRunner assembles §7.5's boot for a caller that needs to hold it open —
// the exercise root runs this same sequence and then acts on its verdict, so
// the phases, the checklist and the module have to be reachable as one value
// rather than sealed inside Run. Run remains the whole story for a caller that
// only reconciles.
func NewRunner(snap config.Snapshot[Config], wall *clock.Wall,
	reg *observability.Registry, stderr io.Writer) (*Runner, error) {

	r := &Runner{
		cfg: snap.Value, clk: wall, reg: reg, stderr: stderr,
		rev: snap.Rev.HashString(), storageCheck: checkJournalStorage,
	}
	checklist, err := boot.NewChecklist(declaredGates()...)
	if err != nil {
		return nil, err
	}
	r.checklist = checklist
	return r, nil
}

// declaredGates is §7.5's full roster, which this root declares in full and
// produces four of.
//
// A function rather than a literal at the one call site, because the SET is the
// decision — declaring a short roster is the shape §7.5 argues against, and
// Report.ExitClass has to name the same eight to sort them into produced and
// structural. Two literals would be two rosters able to disagree about which
// gates this binary is responsible for.
func declaredGates() []boot.Gate {
	return []boot.Gate{
		boot.GateJournalDurable,
		boot.GateJournalReplayed,
		boot.GateVenueJoined,
		boot.GateNetDeltaAsserted,
		boot.GateAccountPosture,
		boot.GateDeactivationsResolved,
		boot.GateMarginOpsResolved,
		boot.GateWriterAuthority,
	}
}

// Runner holds what the phases hand each other. A composition root's phases are
// closures over one struct rather than a pipeline of return values, because
// boot.Phase's Start returns a closer and nothing else — the values are the
// root's business.
//
// It is exported for exactly one composing caller — the exercise root, which
// runs this same boot and then executes on its verdict — and the exported
// surface is deliberately read-mostly: Sequence to run; Module, Halted and
// BootReport to read; Checklist for the execute step's use cases to report
// into. The phases stay unexported; a caller cannot reorder them.
type Runner struct {
	cfg    Config
	clk    *clock.Wall
	reg    *observability.Registry
	stderr io.Writer
	rev    string

	checklist *boot.Checklist
	module    *oms.Module

	// governor is the venue's §7.6 budget, held here because the root is the
	// alert deliverer ratelimit's Snapshot doc names: the transports spend
	// against it, and BootReport reads it. Nil until the oms phase builds the
	// venue client, nil forever on a boot that halted first.
	governor *ratelimit.Governor

	// storageCheck is #168's check, injected so the sequence's own tests do not
	// depend on what filesystem the machine running them uses for temporary
	// files. Production always passes checkJournalStorage; the real check has its
	// own tests, one of which runs it against the host it is on.
	storageCheck func(string) error

	// storageErr is #168's verdict, and halted records that a phase concluded the
	// ones after it must not run. Both are read by later phases rather than by
	// boot.Run, because neither is a reason to unwind: the process stays alive,
	// halted, and audible.
	storageErr error
	halted     bool

	reconciliation application.Reconciliation
	signals        chan os.Signal
}

// Sequence is §3.2's phase list in the one order it is correct in; the
// KindReconcile phase is last, which boot.Sequence enforces.
func (r *Runner) Sequence() boot.Sequence {
	budget := r.cfg.StopBudget.Duration()
	return boot.Sequence{
		WatchdogSilence: r.cfg.WatchdogSilence.Duration(),
		Phases: []boot.Phase{
			{Name: "signal", Kind: boot.KindSignal, Start: r.startSignal, StopBudget: budget},
			{Name: "config", Kind: boot.KindConfig, Start: r.startConfig, StopBudget: budget},
			{Name: "observability", Kind: boot.KindObservability, Start: r.startObservability, StopBudget: budget},
			{Name: "storage", Kind: boot.KindService, Start: r.startStorage, StopBudget: budget},
			{Name: "oms", Kind: boot.KindService, Start: r.startOMS, StopBudget: budget},
			{Name: "reconcile", Kind: boot.KindReconcile, Start: r.startReconcile, StopBudget: budget},
		},
	}
}

// startSignal installs the process's signal disposition, first.
//
// # This binary is the falsification test boot.requiredKinds asked for, and it
// # passes
//
// That roster carries a claim in as many words: every binary in this system is a
// long-running process that loads configuration and reports on itself, "and if a
// binary lands that legitimately has none of one of them — and a genuinely
// one-shot tool would have no signal disposition worth installing — relax the
// rule rather than declaring a phase that does nothing".
//
// `arblab reconcile` is the first one-shot tool, and it does not falsify it. The
// disposition here is load-bearing for the same reason it is in the engine: this
// process opens a SQLite journal and makes signed venue calls, and an interrupt
// during a slow join under the default disposition kills it outright — leaving a
// journal handle released by the kernel rather than by Close, with the WAL
// unlanded. The window where a boot is slowest is the window where an operator
// interrupts it, and this tool's whole runtime IS that window.
func (r *Runner) startSignal(context.Context) (boot.StopFunc, error) {
	r.signals = make(chan os.Signal, boot.SignalBufferMin)
	stop, err := boot.Install(r.signals)
	if err != nil {
		return nil, err
	}
	return func(context.Context) error {
		stop()
		return nil
	}, nil
}

// startConfig resolves §5.2's key refs into the one credential this process
// holds.
//
// It is the config phase's real work rather than a formality, and it is genuinely
// unable to happen earlier: a reference is a document, a credential is a fact
// about this host, and turning the first into the second reads an environment or
// a file that a supervisor may not have populated yet. Its failure is an error
// and not a report, because a process with no credential cannot establish ANY of
// §7.5's venue preconditions — there is no verdict to carry, only an inability to
// start.
func (r *Runner) startConfig(context.Context) (boot.StopFunc, error) {
	if _, err := r.cfg.Venue.credential(); err != nil {
		return nil, err
	}
	return nil, nil
}

// startObservability installs §3.4's reporting, before anything that can fail
// interestingly.
//
// A registry that will not accept this boot's own series is a registry every
// later phase would report its failure into and lose it, which is §3.2's whole
// argument for the position: a boot that dies in the journal join must die with a
// diagnosis rather than with an exit code.
func (r *Runner) startObservability(context.Context) (boot.StopFunc, error) {
	if r.reg == nil {
		return nil, ErrConfigFieldUnset
	}
	fmt.Fprintf(r.stderr, "reconcile: config %s, venue %s, account %s, journal %s\n",
		r.rev[:12], r.cfg.Venue.Venue, r.cfg.Account, r.cfg.JournalPath)
	return nil, nil
}

// startStorage is #168: §13's precondition on the journal's filesystem.
//
// It runs BEFORE the journal is opened, which is the only order that helps — a
// database created on a network filesystem is already the thing this refuses.
//
// Its refusal is OutcomeEvidenceUnusable: halted, because a process whose
// evidence is untrustworthy must not act on it, and LATCHING, because the whole
// content of §7.5's reconciliation writes is that they re-drive what the journal
// says happened, and the DeactivationPending re-drive is terminal with no
// reactivation. Exposure-neutrality is a property of the act given trustworthy
// inputs, and this is the gate whose failure means they are not.
func (r *Runner) startStorage(context.Context) (boot.StopFunc, error) {
	if err := r.storageCheck(r.cfg.JournalPath); err != nil {
		r.storageErr = err
		r.halted = true
		return nil, r.checklist.Report(boot.GateJournalDurable, boot.OutcomeEvidenceUnusable)
	}
	return nil, r.checklist.Report(boot.GateJournalDurable, boot.OutcomePassed)
}

// startOMS opens the journal and assembles the context.
//
// A journal that will not OPEN is reported rather than returned, on the same
// argument §7.5 makes about one that will not verify: exiting presents to §7.4's
// watchdog exactly as a dead host and escalates to an actuator takeover, and a
// permissions fault on a file is not a reason to fence a bot. OutcomeFailed
// rather than OutcomeEvidenceUnusable, because nothing about the RECORDS is in
// question — the process could not reach them, which a retry may fix.
func (r *Runner) startOMS(context.Context) (boot.StopFunc, error) {
	if r.halted {
		return nil, nil
	}
	client, governor, err := newOrderClient(r.cfg, r.clk)
	if err != nil {
		r.halted = true
		r.reconciliation.Cause = err
		return nil, r.checklist.Report(boot.GateVenueJoined, boot.OutcomeFailed)
	}
	r.governor = governor
	module, err := oms.New(oms.Deps{
		JournalPath:      r.cfg.JournalPath,
		VenueOrders:      client,
		VenueOrderReads:  client,
		VenuePositions:   client,
		PositionScopes:   r.cfg.scopes(),
		InstrumentAssets: r.cfg.assets(),
		Clock:            r.clk,
		Account:          r.cfg.Account,
		Resolution:       r.cfg.policy(),
	})
	if err != nil {
		r.halted = true
		r.reconciliation.Cause = err
		return nil, r.checklist.Report(boot.GateJournalReplayed, boot.OutcomeFailed)
	}
	r.module = module
	return func(context.Context) error { return module.Close() }, nil
}

// startReconcile is §7.5's phase: replay, join, resolve every absence, assert.
//
// It is last, which boot.Sequence enforces — a phase after the one whose result
// decides the posture would be running while the process is already reporting a
// verdict about whether it may trade.
func (r *Runner) startReconcile(context.Context) (boot.StopFunc, error) {
	if r.halted {
		return nil, nil
	}
	rec, err := r.module.Reconcile.Reconcile(r.checklist)
	r.reconciliation = rec
	if err != nil {
		// Reconcile's error means the verdict could not be FILED — a gate this root
		// did not declare, a latched gate reported against. That is a wiring defect
		// in this package and not a boot outcome, so it is the one thing here that
		// legitimately ends the process.
		return nil, err
	}
	return nil, nil
}

// Module is the OMS this boot assembled, nil until the oms phase has run and
// nil forever on a halted boot. The exercise root plans and executes through
// it — via the module's own use cases, which is the only shape singlewriter
// admits.
func (r *Runner) Module() *oms.Module { return r.module }

// Halted reports whether some phase concluded the ones after it must not run.
func (r *Runner) Halted() bool { return r.halted }

// Checklist is the boot's checklist, which outlives the sequence on purpose: a
// composing root that executes after a clean verdict hands it to the use cases
// that report into it — Module.Execute files §6 step 7's hold against it and
// reads it back per package — so a latch a runtime finding earns lands on the
// same record the boot's verdict was derived from, and both the executor's
// admission read and this root's next BootReport see it. It is the live value,
// never a copy; Clear travels with it, and Clear remains the operator's alone
// (§7.5) the same way the phases remain unexported — a root is trusted with
// the sharper tool because the blunt one would make the executor's filing
// unreachable.
func (r *Runner) Checklist() *boot.Checklist { return r.checklist }

// RateLimit is the venue governor's state now — §7.6's alert facts, live. The
// zero snapshot when no venue client was ever built; see Report.RateLimit for
// why that zero is an answer rather than a gap. A composing root reads this
// AFTER its runtime work, because Bans is monotone and a report taken before a
// dispatch cannot carry a ban the dispatch earned.
func (r *Runner) RateLimit() ratelimit.Snapshot {
	if r.governor == nil {
		return ratelimit.Snapshot{}
	}
	return r.governor.Snapshot()
}

// BootReport is what the boot established so far, readable at any point — a
// halted or failed sequence still carries its verdict, which is the
// alive-halted-audible contract §7.5 puts on this whole package.
func (r *Runner) BootReport() Report {
	rep := Report{
		Reconciliation: r.reconciliation,
		Storage:        r.storageErr,
		RateLimit:      r.RateLimit(),
	}
	if r.checklist != nil {
		rep.Posture = r.checklist.Posture()
		rep.Blocking = r.checklist.Blocking()
		rep.Latched = r.checklist.Latched()
	}
	return rep
}
