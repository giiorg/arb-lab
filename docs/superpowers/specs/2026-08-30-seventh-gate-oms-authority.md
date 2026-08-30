# Record §101 — the seventh gate: OMS authority is granted, not assumed

**Date:** 2026-08-30 (session 147) · **Task:** #243 · **Milestone:** 4 (pre-gate), §12 roster item 8's first landing
**Predecessors:** §100 (the web dashboard design, and §15.7's probe run — this record is the action that record's handoff named) · core §3.6 (the six gates, now seven) · core §11 (the deployment failure domains, and §4c's owed edit, which lands here) · core §7.3 (operator control-plane security)
**ADR rows:** **F47** (the specified deny-list polices an empty set and fails in the direction §3.6 already rejected once — §2) · **F48** (`oms/application` mixes the use cases with the report types they return, so the gate is necessarily coarser than the capability — §5) · **F49** (Go builds a package's tests as *three* import paths and §3.6's normalisation sentence named one — §4) · **F50** (four analyzers carry a module-root guard that no input can make decide anything, and each one's comment credits it with a protection the arithmetic was providing — §6)
**Status:** **landed.** `tools/lint/omsauthority` is the seventh `analysistest`-backed gate; `make ci` green on both modules. Spec edits landed in `build-spec.md` §3.6 (rule 11, the normalisation sentence, the Gate-status paragraph, a third bite-check paragraph) and §11 (§4c's `[SPEC EDIT OWED]`, now closed) and §12 (roster item 8). Doc set **v0.9.72**.
**Nothing was sent to a venue. No engine code changed** — the only non-`tools/lint` edits are documentation.

---

## 1. Why this record exists, and what it is NOT

§100 §15.7 ran a probe and found that the remedy §6a specified does not reach
the route it identified. Its closing list made the seventh gate the first item,
on the standing rule that a gate lands the session its subject appears. This is
that session.

**It is not an implementation of the dashboard**, and `writing-plans` still has
not run. What landed is the gate, the spec edits it forces, and one debt closed.
The reason the gate could land before the subsystem it was built for is the
subject of §2, and it turned out to be the interesting part.

## 2. The specification was backwards, and the fix is the direction rather than the rule (F47)

§100 §6a states the construct precisely, in the form this repo requires — the
construct the analyzer *reads*, not the feature that motivated it:

> **The construct: declared import edges from the web-facing package tree
> (`internal/app/serve/…`, later `internal/app/operator/…`) into the OMS tree.**
> Default-deny bare `internal/oms` and `internal/oms/application`; permit exactly
> one separately-packaged read facade.

Implemented literally, that analyzer's policed set is **empty**. Neither tree
exists. A lint over an empty tree passes trivially and is indistinguishable from
a broken lint — the exact condition rules 1, 6 and 10 were each deferred
against — so the gate could not have landed this session at all, and the standing
rule that made it item 1 would have been unsatisfiable on the terms item 1 stated.

**And it fails in a direction this section has already rejected once.** Rule 6
refused an opt-in `//arblab:writerepository` marker in as many words: *"the
repository somebody adds next year without the marker is silently unpoliced."*
Naming the web trees is that same shape with the **importer** as its subject
instead of the import. `internal/app/dashboard/`, `internal/app/ui/`,
`internal/app/admin/` — each is unpoliced the day somebody creates it, and the
gate reports nothing while the route it exists to close stands open.

**The inversion fixes both, and it is one change.** Police the **OMS use-case
surface**, which exists — seven packages, four external importers, measured with
`go list` rather than assumed — and default-deny it to every package outside
`internal/oms/` except a **named grant list**. A web-facing root is then denied
*because it is not on the list*. Nobody has to remember anything, and adding a
name is a visible act in a diff that a reviewer can weigh — which a deny-list
never gets, because there the grant arrives as the **absence** of an edit.

This is the standing rule about defaults, applied to the gate itself: *when a
default has two directions, pick the one that is right when somebody forgets it.*
Forgetting to grant a legitimate new root produces a refusal naming the package.
Forgetting to deny a new web root produces a running process whose HTTP handlers
can append to §5.1's hash chain.

> *Rejected alternative:* the deny-list as specified, kept and deferred until
> `internal/app/serve/` exists. Rejected on both grounds above, and on a third:
> the deferral would have been the fourth in this project, and the only reason
> that has ever survived scrutiny for one is that the construct the analyzer
> reads does not exist yet. Here it does — it is `internal/oms`, and it has
> existed since milestone 3.

> *Rejected alternative:* grants that cover a package **tree** rather than a
> package. Rejected on the same default-direction argument one level down: a
> granted tree hands the authority to a subpackage nobody weighed. `testdata`
> carries `internal/app/pair/sub` specifically because an exact comparison and a
> prefix comparison agree on every path that is **not** a subpackage of a granted
> root, so it is the only fixture that can separate them.

> *Rejected alternative:* reporting an import that rule 6 already reports.
> Rejected because §3.6's own bite-check note says a gate must be shown to bite
> where **no other gate would** — an import two gates report cannot isolate
> either, and the isolating probe for this gate is exactly the import rule 6 is
> silent on. The two surfaces are kept disjoint and the disjointness is
> **measured**: see §3.

**What this does not settle.** Rule 11 polices **packages**; a failure domain is
a property of a **binary**. A `serve` root linked into `cmd/arblab` shares an
address space with roots that hold OMS authority, and no lint over declared
import edges sees that. §11's deployment line now says so in its own words, and
names the check that carries it at the transition point — a `go list -deps`
fence over the binary's whole graph, the shape `internal/app/capture/imports_test.go`
already uses for §32b.

## 3. The probe, re-run against the live gate

Round 3's four rows, rebuilt at `internal/app/serve/httpapi`, run against
`make lint`, and deleted in the same session. The **control** is the half that
carries the information, and it is in the same file three lines away.

| Probe | §100 §15.7 (six gates) | Now (seven gates) |
|---|---|---|
| bare `internal/oms`, `Module.Execute` selected | all six silent | **rule 11 alone** |
| `internal/oms/application` direct, `*application.Executor`, no `*oms.Module` | all six silent | **rule 11 alone** |
| `oms/application/ports` — **the control** | rule 6 reports | **rule 6 alone** — no second report |
| `oms.ReconcileMarginOp(ClearanceDeps{…}, act)` + `time.Now()`, no module, no facade | `go build` succeeds, all six silent | **rule 11 alone**; still builds |

Two things this measures that no reading could. **The route F44 found is closed**
— the fourth row is the one that put the facade behind the gate rather than in
front of it, because `ReconcileMarginOp` is a package-level function over two
strings and there is no value to wrap. And **the two gates are disjoint**: three
imports in one file, each reported by exactly one analyzer, in one run where both
analyzers were demonstrably live.

**What is still silent is `time.Now()`.** `notimenow`'s `policedLayers` is
`{domain, application, engine}` (`notimenow.go:123`), so a handler in the web tree
is outside it **by position** — §4a's hole seen from the other side. That is F42,
it is unchanged, and the audit timestamp in row 4 still comes from the ambient
clock with nothing objecting.

## 4. Go builds a package's tests as THREE import paths (F49)

§3.6 states the normalisation once, ahead of the rules: *a rule identified by
position reads a Go external test package as the package it tests.* It names
`p_test`. The toolchain also synthesizes **`p.test`** — the generated `main` that
runs the suite, which imports both `p` and `p_test`.

**No previous gate could meet it.** Rule 6's subjects are `oms/application/ports`
and `oms/infrastructure/driven`, which no generated main names. Rule 11 polices
`internal/oms` itself, which every one of `oms`'s test mains imports — so it met
the case on its suite's **first run**, as a diagnostic against
`"…/internal/oms.test"`: the owner's own test binary reported for holding the
owner's own authority. That is `singlewriter`'s original defect in a third form,
found the same way, and the sentence in §3.6 now covers all three paths.

The in-package case needs no rule and gets none, deliberately: a `*_test.go` file
in `package main` can call anything `package main` can, so it holds exactly the
authority its package holds. That is the honest reading rather than an oversight,
and it is why the grant list carries `cmd/arblab` — see below. ***[2026-08-30, record §102: it no longer does. This paragraph is what F51 turned out to be about — the grant existed for one test file's fixtures, the fixtures moved to `internal/app/reconcile` with the renderer they test, and the grant was deleted.]***

## 5. The gate is coarser than the capability, and the reason is in `oms/application` (F48)

`cmd/arblab`'s production graph imports **none** of the surface. Its grant exists
for `reconcile_test.go`, which names `application.Reconciliation`,
`application.Recovery`, `application.Join`, `application.UnprovedOrder` — the
**report value types a reconcile run returns**, built as fixtures to test how the
CLI renders them. Nothing there is a use case.

So `oms/application` is two things in one package: §6's journal-then-apply path,
and the result types that path produces. An importer who wants only to *render* a
reconciliation acquires the whole surface, and no import-edge gate can tell the
two apart. **The gate is therefore necessarily coarser than the capability it
polices**, which is a stated limit rather than a defect — and narrowing it is a
debt (§8), because splitting the report types into their own package is a change
to a shipped context for a reason no code currently needs.

## 6. Four gates carry a guard that no input can make decide anything (F50)

The mutation harness returned one `SURVIVED` out of thirteen, on the branch every
path-scoped analyzer in `tools/lint` opens with:

```go
root, ok := moduleRoot(path)
if !ok {
        continue        // "not under internal/ at all — the rule cannot apply"
}
```

**Diagnosed as an equivalent mutant, provably rather than by sampling.**
`strings.Cut` returns the **whole path** when the separator is absent, so every
policed directory built from that root is `root + "/internal/…"` — strictly longer
than the path that produced it — and cannot match it on the equality arm or the
prefix arm. The one input that could reach a subsequent check, a package at
`<x>/internal/oms` importing exactly `<x>`, takes the same `continue` one branch
later.

Measured in all four: `omsauthority`, `singlewriter`, `hexboundary`,
`kernelpurity` — **all four SURVIVED**, all four equivalent. It is removed from
all four, in the session it was found, under the standing rule that a fix applied
in one place is applied to its mirror immediately.

**The cost is not the dead branch; it is the comment above it.** Each one reads
*"not under internal/ at all — the rule cannot apply"*, which is **true**, and
attributes it to a mechanism that is not providing it. `hexboundary`'s own
`withoutTestSuffix`, one function down in the same file, carries a paragraph
about exactly this — *"the comment above it credited the split with a protection
`TrimSuffix` was already providing"* — written when `singlewriter`'s mutation run
found the twin case. The lesson was recorded in the file and not carried to the
function above it. **A finding recorded in a file is not a finding applied to the
file.**

## 7. What was checked, and how

**The fixture-collapse checklist ran BEFORE the mutations**, per the standing
rule, and it found two collapses in fixtures written the same hour:

1. **`cmd/arblab` cannot separate the two ways to resolve the module root.** It
   is granted, so it is silent whichever way the root is derived — and
   silent-for-the-right-reason and silent-for-the-wrong-one are the same output.
   The separating fixture has to be one that must **not** be silent, so
   `cmd/rogue` was added. This is the eleventh-plus instance of the house shape,
   and its specific form is *a negative assertion admits the degenerate value*.
2. **`ruleSix` listed a tree no input could make it decide anything about.**
   `oms/infrastructure/driven` is outside both surface entries already, so its
   presence in the exclusion list was inert — the list read as doing twice the
   work it does. Removed; the exclusion now has exactly the one entry that can
   fire, and the disjointness is carried by §3's measurement rather than by a
   longer list.

A third was caught in the test file rather than in `testdata`: a
`TestSurfaceEntriesReportDistinctly` that compared two **copies** of the cost
sentences declared in the test file itself — a check fed only by the thing it was
checking, which would have passed with the real strings identical. It now reads
the analyzer's own strings through an accessor, and checks non-**containment**
rather than non-equality, because a `want` regex keying on a substring is
satisfied by a sentence that contains the other one.

**Mutations: 13 run, 12 caught, 1 survived and diagnosed** (§6).

| Claim | Verdict |
|---|---|
| the owner tree is exempt | caught |
| rule 6's subject is excluded | caught |
| `internal/oms` is matched EXACTLY, not as a tree | caught |
| `oms/application` is matched as a TREE, not exactly | caught |
| a grant covers its package, not its tree | caught |
| the `.test` generated-main strip | caught |
| the `_test` external-package strip | caught |
| `within` compares FULL segments | caught |
| `within`'s equality arm | caught |
| the grant list names `internal/app/pair` | caught |
| an unused grant is reported | caught |
| the surface does not match everything (over-report control) | caught |
| the module-root not-found guard | **SURVIVED — equivalent, §6** |

**The check `make lint` cannot perform** has its own test.  The analyzer only
ever reports a package **missing** from the grant list, so a grant whose root
stopped importing the surface stays there silently and reads as evidence that the
authority is still held — a rejection's reason expiring with nothing failing.
`TestEveryGrantIsStillUsed` runs `go list` over the engine module and fails on an
unused grant, printing the reason that has expired. It carries a positive control
(the graph must be non-empty **and** contain `internal/oms`), because a `go list`
that silently produced nothing would satisfy the loop perfectly.

It is a test rather than a diagnostic, and the reason is worth keeping: an
analyzer runs per package **variant**, and `cmd/arblab`'s production variant
imports none of the surface — only its in-package test file does. A per-variant
*"this grant permits nothing"* report would fire on a real, correct package.

## 8. Debts this record opens

1. **Narrow `oms/application`** so rendering a reconciliation does not require
   the write surface (F48). Not urgent, and it is a change to a shipped context.
2. **§100 §6a's capability-limited facade is still owed** as the backstop. The
   gate closes the short way round — a declared import — and the facade closes
   the long way, a granted root handing an `*oms.Module` to a package that
   imports nothing. F43's inventory is what sizes it: **eleven** exported fields,
   and the obligation is the property *"it excludes every write capability on
   `Module`"*, not the list.
3. **F42's hole is unchanged.** `notimenow` does not police the web tree, so an
   HTTP handler may read the ambient clock — and the audit timestamp on a control
   action is exactly the value that must not come from it. This is a candidate
   eighth gate or a `policedLayers` edit, and it needs its own decision.
4. **The `go list -deps` fence for the operator binary**, at §11's transition
   point. Named in the spec edit, not built, because the binary does not exist.
