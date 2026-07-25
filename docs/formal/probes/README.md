# Reviewer-authored probes (Cx8, 2026-07-25)

**Provenance: everything in this directory except this README was written by the
independent round-2 reviewer** (Codex `artifact-review`, session
`019f990c-5d2e-7760-9d22-2022b0e220da`), not by the model's author. It is kept
under version control precisely for that reason: it is the only property set in
`docs/formal/` that the author did not write, and the round it belongs to exists
*because* the preceding round's fixes had been checked only against properties
their own author wrote.

`FenceTakeoverReviewProbe.tla` `EXTENDS FenceTakeover` and adds two temporal
properties. Both produced the Cx8 findings as TLC counterexamples against the
**v0.9.3** module; both are recorded here as run against the **fixed** (v0.9.4)
module.

## Running them

From this directory (the `-DTLA-Library` flag resolves `FenceTakeover` from the
parent, so these always check the *live* model, never a stale copy):

```sh
cd docs/formal/probes
java -DTLA-Library=.. -XX:+UseParallelGC -cp ../.tools/tla2tools.jar tlc2.TLC \
  -config FenceTakeoverReviewExhausted.cfg -workers auto -lncheck final \
  -metadir /tmp/states-exh FenceTakeoverReviewProbe.tla
```

**The only edit made to the reviewer's files** is one line —
`ARBITRATION_SNAPSHOT = TRUE` — added to both configs, because FM-4's resolution
introduced that constant after the probes were written. No property text was
changed.

## Current status against v0.9.4 — read this before concluding anything

| Property | Result on v0.9.4 | Meaning |
|---|---|---|
| `ExhaustedHeldEventuallyParks` | **PASS** (1,705,418 / 249,936) | The Cx8-2 fix is confirmed **by the reviewer's own property**, not only by `P3`, which the author wrote. This is the strongest single piece of evidence in the round. |
| `LostEventuallyLeaves` | **FAILS — expected, not a regression** | See below. |

### Why `LostEventuallyLeaves` still fails, and why that is correct

It is **over-strong**. It catches the real FM-4 strand *and* a benign trace: the
bot dies **before** arbitration, the watchdog path legitimately takes over,
exhausts its close budget and parks in `HeldIncident`. `fenceCtl` stays `"lost"`
there because §3 branch selection is a **bot** action and no bot was alive to
take it — but the actuator engaged, did its bounded best, and the operator is
escalated. That is an honest escalation terminal, not a stall.

Loop state of the surviving counterexample:

```
botUp = FALSE        latched = TRUE        heldCause = "bounds"
pos = "naked"        attemptsLeft = 0      actPhase = "held"
fenceCtl = "lost"    branchMax = 0         incidentActive = TRUE
```

The permanent suite member derived from it is **`P4_ArbitrationCompletes`**
(in `FenceTakeover.tla`, checked by `_Live2.cfg`), scoped to
`botUp ∧ branchMax = 0 ∧ actPhase = "standby"` — the strand itself — with a
consequent that admits actuator engagement only as a **hand-off** to
`Branch2AutoResolves` + `P3`, never as a resolution in its own right. `NC5`
proves `P4` still bites.

**Do not "fix" the model to satisfy `LostEventuallyLeaves` as written.** Doing so
would demand that a watchdog-driven takeover somehow retro-select a §3 branch, or
would force `HeldIncident` to stop being a terminal. If a future round disagrees
with this scoping, that is a legitimate finding — but it is an argument to have
explicitly, against the reasoning in the Cx8 addendum
(`docs/review/2026-07-20-10-…`) and model doc §2d, not a silent edit.
