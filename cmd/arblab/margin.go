package main

import (
	"flag"
	"fmt"
	"io"
	"time"

	appreconcile "github.com/giiorg/arb-lab/internal/app/reconcile"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/clock"
)

// marginReconcile is `arblab margin reconcile -config <file> -op <id> -actor
// <name> -at <RFC3339> -note <text>`: the operator act that ends an Ambiguous
// margin op's incident (task #213, record §82).
//
// # Why every field is a required flag with no default
//
// core §7.3 requires an immutable per-actor audit of every control action —
// "who / what / when" — and each of the three has a default that would look
// helpful and record nothing true. The host user is not the actor (a shared
// operator account names nobody); `now` is not the instant (the accounting was
// done before the operator reached a terminal, and by however long); and there
// is no default note, because with the FINDING deliberately absent from the
// record the note is the only field saying what was actually reconciled.
//
// # There is no bulk form, and there will not be one
//
// One `-op`, always. A flag that cleared every open ambiguity would let one
// keystroke end an incident nobody has looked at, which satisfies the audit
// requirement's shape while making it worthless. If a second incident is open
// the command says so — see the `remaining` line below — so the operator learns
// it from the act rather than from a boot that still blocks.
func marginReconcile(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("margin reconcile", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cfgPath := fs.String("config", "", "reconcile configuration file (required)")
	op := fs.String("op", "", "the ambiguous margin op's id (required)")
	actor := fs.String("actor", "", "who performed the reconciliation (required)")
	at := fs.String("at", "", "when it was performed, RFC3339 (required)")
	note := fs.String("note", "", "what was reconciled, and against what (required)")
	if err := fs.Parse(args); err != nil {
		return 64
	}
	if *cfgPath == "" || *op == "" || *actor == "" || *at == "" || *note == "" {
		fmt.Fprintln(stderr, "margin reconcile: -config, -op, -actor, -at and -note are all required")
		return 64
	}

	// Parsed here rather than passed as a string, so a malformed instant is a
	// USAGE error against the flag the operator typed. Deeper in it would arrive
	// as a zero time, which domain.MarginOpReconciliation.Validate refuses by
	// name — a correct refusal naming the wrong thing.
	when, err := time.Parse(time.RFC3339, *at)
	if err != nil {
		fmt.Fprintf(stderr, "margin reconcile: -at %q is not RFC3339: %v\n", *at, err)
		return 64
	}

	wall, err := clock.NewWall(nil)
	if err != nil {
		fmt.Fprintln(stderr, "margin reconcile:", err)
		return 70
	}
	snap, err := loadReconcileConfig(*cfgPath, wall)
	if err != nil {
		fmt.Fprintln(stderr, "margin reconcile:", err)
		return 70
	}

	cleared, err := appreconcile.ReconcileMarginOp(snap.Value, domain.MarginOpReconciliation{
		Op:    domain.MarginOpID(*op),
		Actor: *actor,
		At:    when,
		Note:  *note,
	})
	if err != nil {
		// Every refusal lands here and each one names a different next step:
		// the id is not in the log, the op has no verdict yet, it resolved
		// applied/not_applied and blocks nothing, or somebody already cleared it.
		// A live engine holding the journal also lands here, from the store's own
		// single-writer refusal.
		fmt.Fprintln(stderr, "margin reconcile:", err)
		return 1
	}

	fmt.Fprintf(stdout, "reconciled: %s\n", cleared.Act.Op)
	fmt.Fprintf(stdout, "actor:      %s\n", cleared.Act.Actor)
	fmt.Fprintf(stdout, "at:         %s\n", cleared.Act.At.UTC().Format(time.RFC3339))
	fmt.Fprintf(stdout, "chain:      seq %d\n", cleared.ChainSeq)
	if len(cleared.Remaining) == 0 {
		fmt.Fprintln(stdout, "remaining:  none — no margin op blocks new openings")
		return 0
	}
	// Printed to STDOUT with a zero exit, because the act succeeded. The
	// remaining incidents are not this command's failure; they are the reason the
	// next boot will still block, and an operator who is not told here finds out
	// from a gate that names no op.
	fmt.Fprintf(stdout, "remaining:  %d still blocking new openings\n", len(cleared.Remaining))
	for _, id := range cleared.Remaining {
		fmt.Fprintf(stdout, "            %s\n", id)
	}
	return 0
}
