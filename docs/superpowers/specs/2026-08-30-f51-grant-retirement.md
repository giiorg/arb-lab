# Record §102 — F51's remedy: retiring `cmd/arblab`'s OMS grant

**Date:** 2026-08-30 (session 148) · **Task:** none (§100 §14 debt 6, promoted) · **Milestone:** 4 (pre-gate)
**Predecessors:** §101 (the seventh gate, and F48 — the debt this record cashes) · §100 §15.9 (F51, the finding) · core §3.6 rule 11 · core §11:821 clause (a)
**ADR rows:** **F51** (closed — the remedy is here) · **F58** (a fixture whose isolating property is an exemption dies with the exemption — §5) · **F59** (a silent positive control proves nothing unless something in the same file speaks — §4)
**Status:** **landed.** `make ci` exit 0 on both modules; tree clean; both probe files deleted. Spec edits in `build-spec.md` §3.6 rule 11, §3.6's normalisation sentence, §3.6's Gate-status paragraph and §11:821 clause (a). Doc set **v0.9.74**.
**Nothing was sent to a venue.** Engine code changed: one renderer and its tests moved between packages; no behaviour changed and no output byte changed.

---

## 1. What F51 was, in one paragraph

§100 §4 specifies `cmd/arblab/serve.go` as the phase-1 HTTP entrypoint.
`cmd/arblab` was entry 1 of §3.6 rule 11's grant list, so the package that
starts the server held unrestricted OMS write authority — `oms.ReconcileMarginOp`
takes a `ClearanceDeps{JournalPath, Account string}` and appends an operator act
to §5.1's hash chain, which is two strings a server already reads from its own
config. Round 4 probed it: the entrypoint built with all seven gates silent
while an ungranted control reported three lines away. §11:821 clause (a) —
*"the CAPABILITY boundary is enforced … 'read-only' is a build failure rather
than a promise"* — was the condition round 2 attached to accepting a phase-1
shape at all, and it was false of the one file that most needed it.

**This record is the remedy, not the finding.** The finding, its probe and its
falsified grant reason are §100 §15.9 and ADR row F51.

## 2. The grant was bought by test fixtures, and by nothing else

Measured before anything was changed:

```
go list -f '{{.ImportPath}} {{join .Imports " "}} … {{join .TestImports " "}} …' ./cmd/arblab
  Imports:      …/internal/oms/domain          (not policed — rule 11's entry 1 is internal/oms EXACTLY)
  TestImports:  …/internal/oms/application     (the surface)
```

One in-package test file, `cmd/arblab/reconcile_test.go`, named
`application.Reconciliation`, `Recovery`, `Join`, `Assertion`, `UnprovedOrder`,
`InstrumentDelta`, `AssetDelta`, `UnassertedInstrument`, `UnconcludedBalance` and
four reason constants — to build fixtures for `writeReconcileReport`, the CLI's
operator-report renderer.

**The renderer itself named none of them.** It walks
`appreconcile.Report.Reconciliation`, and Go needs no import for field access; a
composite literal is what needs one. So the whole of `cmd/arblab`'s OMS
authority — over every subcommand, including the `serve` one — was created by
*fixtures*.

That is F48's stated limit cashed in: `oms/application` holds the use cases and
the report value types they return, one import path grants both, and no gate
reading declared import edges can tell rendering a result from invoking the path
that produced it.

## 3. The remedy, and the three shapes that were weighed

**Taken: move the renderer to the package that declares the `Report`.**
`writeReconcileReport`, `writeReconcileVerdict`, `writeReconcileDiagnosis` and
`writeRateLimitStatus` became `WriteReport`, `WriteVerdict`, `WriteDiagnosis` and
`WriteRateLimitStatus` in `internal/app/reconcile/render.go`; the six renderer
tests moved with them into `render_test.go`. `cmd/arblab` keeps flag parsing,
config loading, `dispatch` routing and the `ExitClass`→exit-code spelling.
`exercise.go` and `pair.go` call the exported names. **No output byte changed** —
both goldens moved verbatim and pass.

It stands on cohesion independently of the gate: `Report` is declared there,
`ExitClass` — *what a run means* — is decided there, and a renderer must know the
shape of every nested field it prints. `internal/app/reconcile` holds rule 11's
authority for a reason of its own (§7.5's reconcile-only boot, and the one caller
of `oms.ReconcileMarginOp`).

**Rejected — split the report value types out of `oms/application`** into a
package rule 11 does not police, so any read-only consumer could name a
diagnosis without holding write authority. This is the fix to the *conflation*
rather than to this instance of it, and it is the one §100's dashboard would
eventually want. Rejected for this session on cost and on a cut that is not
clean: ~34 references outside `oms/` plus the producer's own; `Recovery` is not
only an output — `marginreconcile.go:87` takes it as an **input** to a use case;
and `join.go:501` declares `UnpinnedReason = ports.UnpinnedReason`, an alias into
rule 6's policed tree, so the new package would inherit that edge. **It stays a
debt** (§101 §8 item 1, unchanged) and it is the right move the day a read-only
consumer needs these types — which is a *when*, not an *if*, since §7 of §100
renders a reconciliation diagnosis.

**Rejected — re-export the value types as Go type aliases** from
`internal/app/reconcile` (`type Reconciliation = application.Reconciliation`, and
so on), so the fixtures name `appreconcile.X` and the import disappears. Fifteen
lines, nothing moves. **Rejected on principle rather than on cost:** rule 11
reads *declared import edges*, so this deletes the edge while leaving the same
types named in the same fixtures — routing around the instrument instead of
answering it. The gate would then report a clean tree it had stopped measuring,
which is strictly worse than the state F51 found, because F51 at least left a
grant a reader could see. *When an import edge exists only to name a value type,
move the naming or move the type; never re-export it across the boundary the
gate reads.*

## 4. The re-run probe, and the control that was nearly vacuous (F59)

Two files, four cells, one `make lint` run per arm. Line 4 of each file is the
`internal/oms` import (rule 11's subject); line 5 is `oms/application/ports`
(rule 6's, and rule 11 excludes it by name so exactly one gate can speak).

| | `cmd/arblab/serve_probe.go` L4 | L5 | `internal/app/exercise/probe_control.go` L4 | L5 |
|---|---|---|---|---|
| **A — grant restored** (the pre-remedy state) | silent | **rule 6** | silent | **rule 6** |
| **B — grant deleted** (landed) | **rule 11** | **rule 6** | silent | **rule 6** |

`cmd/arblab/serve_probe.go` is the file §100 §4 specifies, calling
`oms.ReconcileMarginOp`. `internal/app/exercise` is a *granted* root making the
identical new edge — the positive control, and it is the half that says rule 11
is objecting to the **importer** rather than to the **import**. Rule 6's
diagnostic is constant in all four cells: it is what proves each run selected
each file. Both files deleted; `make lint` clean; tree clean.

**The first version of this probe had a vacuous control, and the checklist
caught it before it was recorded (F59).** The control file originally carried
*only* the `internal/oms` import, so its result was **silence** — and a silent
file is indistinguishable from a file the invocation never selected. The subject
carried its own speaker (rule 6, three lines down, exactly as §100 §15.7's probe
did) and the control did not. *A positive control whose expected outcome is
silence needs its own speaker in the same file; otherwise it measures the
runner's file selection, not the gate's predicate.* Adding line 5 to the control
is the whole of the fix, and it turned a decorative row into the one that
distinguishes "granted" from "unanalysed".

## 5. Deleting a grant kills its fixture, and the fixture does not fail (F58)

`omsauthority`'s testdata carried
`.../cmd/arblab/arblab.go`, documented as *"a granted root OUTSIDE internal/,
which is the fixture that forces the module root to be resolved from the IMPORTED
path"*, and the suite's roster listed it among the clean packages for that
reason.

Deleting the grant does not make it fail in a way that names the problem. It
makes it **reported** — and `cmd/rogue`, two directories over, already asserts
exactly that: *a cmd/ package that must not be silent*. A `want` comment on
`arblab.go` would have made the suite green with two fixtures asserting one
predicate, and `rogue.go`'s own doc comment — which argues for its existence by
citing `cmd/arblab` as *"granted and clean, which cannot do this job"* — would
have been left stating something false about the file beside it.

So the fixture was **deleted**, not re-pointed, and `rogue.go`'s argument was
rewritten to stand without it — and the deletion was **bite-checked** rather than
argued, because "the remaining fixture still carries the property" is exactly the
kind of claim this repo has been wrong about:

```
MUTATE_MODULE=tools/lint tools/mutate.sh tools/lint/omsauthority/omsauthority.go 396 \
  's/moduleRoot(path)/moduleRoot(self)/' '…resolve the root from the ANALYZED package' ./omsauthority/
  caught  (1 failing test)

  analysistest.go:713: .../cmd/rogue/rogue.go:19: no diagnostic was reported matching
                       `not granted OMS write authority.*OMS control surface`
```

One failing test, and it is `cmd/rogue` by name — the property the deleted
fixture was credited with is carried, alone, by the fixture that remains.

This is F50's lesson moved from a guard to a fixture. F50: *a guard that cannot
fail reads in review as load-bearing, and the next person to simplify it has to
re-derive that it never was.* Here: **a fixture whose isolating property is an
EXEMPTION stops isolating anything the moment the exemption is deleted, and
nothing fails** — `analysistest` is satisfied either way, and the redundancy is
visible only by reading the two fixtures' doc comments against each other.
*When an exemption is removed, go and find what its existence was the only
justification for.*

## 6. The check that fired, one session after it was written

`TestEveryGrantIsStillUsed` (`tools/lint/omsauthority/omsauthority_test.go`) does
a `go list` over the engine module and fails a grant whose package no longer
imports the surface. It exists because **lint alone can only report a package
MISSING from the grant list**, so an expired grant sits there silently and reads
as evidence that the authority is still held.

The fixtures moved; the graph lost the edge; the test failed by name and quoted
the reason back:

```
the grant for cmd/arblab is unused — nothing in it imports the OMS use-case
surface. It was justified as: the CLI binary — §11:826's actuator-domain
process, as against arblab-operator, which is web-facing and has none of this …
That reason has expired; remove the grant rather than leaving it as evidence of
an authority nobody holds
```

Worth stating plainly because it is the rarer outcome: the check was written in
§101, the finding it would have caught (F51) was found by hand one session later
by a *different* instrument, and the check then turned the remedy into a
**refusal** rather than into a thing somebody had to remember. It also means the
grant list cannot silently regrow: adding the renderer's import back to
`cmd/arblab` reports under rule 11, and adding the grant back without a real
importer fails this test.

## 7. What is and is not claimed

**Claimed.** `cmd/arblab` no longer imports the OMS use-case surface in any of
Go's three import classes; the grant is deleted; the gate reports the specified
entrypoint, measured A/B with a selection control in every cell and a granted
positive control; `make ci` is exit 0 on both modules; the renderer's output is
byte-identical, held by two goldens that moved unchanged.

**Not claimed.** Nothing about the *failure-domain* half — §11:821 clause (b) is
untouched, and a `serve` root linked into `cmd/arblab` still shares an address
space with roots that hold OMS authority. Nothing about the long way round: rule
11 reads declared import edges, and a granted root can still hand an
`*oms.Module` to a package that imports nothing. And nothing about F48's general
limit, which stands — the remedy removed the one grant that rested on it, not the
conflation that produced it.

## 8. Debts

1. **Split the report value types out of `oms/application`** — §101 §8 item 1,
   unchanged and now better argued (§3 above records why it was not taken here
   and what the cut costs). Due when a read-only consumer needs to name a
   reconciliation diagnosis, which §100 §7 will.
2. **`ReconcileMarginOp`'s actor identity** — §100 §14's item, and F51's closure
   sharpens rather than settles it: the entrypoint that would supply an operator
   identity is now *outside* the authority, which is the right shape and means
   the identity has to arrive some other way.
