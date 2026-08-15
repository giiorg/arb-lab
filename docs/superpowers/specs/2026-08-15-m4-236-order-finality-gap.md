# The order lifecycle stops at `Submitted`, and nothing in this repository advances it

**Session:** 141, 2026-08-15. **Found by:** Codex artifact review round 2 of
`docs/superpowers/plans/2026-08-15-m4-236-crossvenue-pair-root.md`, verified
against the tree link by link before being accepted.

**Status:** the finding is settled. The remedy is split out of #236 into its own
task (§5), on the ground stated in §4.

---

## §1 The finding

A leg this system successfully places reaches `domain.StateSubmitted` and
**stays there for the life of the process**. No code path advances it. Every
link:

| Link | Evidence |
|---|---|
| A successful `Place` is classified `StateSubmitted` | `internal/oms/application/dispatch.go:503` applies it; `classifyPlace` returns it with the comment *"Not SubmissionUnknown — that state is ambiguous about whether we submitted, and here we are not."* `internal/oms/application/execute_test.go:260` expects exactly this. |
| `StateSubmitted` is **not** terminal | `internal/oms/domain/orderstate.go:146`. Terminal is exactly `Filled`, `Canceled`, `Rejected`, `AbsentAfterFinality`. |
| `Submitted` may advance only to `{Acknowledged, Rejected, SubmissionUnknown}` | `internal/oms/domain/transition.go:186`. There is no direct edge to a terminal fill state. |
| The join never advances state | `internal/oms/application/join.go`'s only repository writes are `Load` (777) and `ApplyFill` (881). Neither `ApplyFill` nor `ApplyAck` touches `rec.state` — `ApplyAck` sets `venueOrderID` and bumps the version, and `ApplyFill` writes the ledger and bumps the version (`internal/oms/infrastructure/driven/orders.go:313, 466`). |
| The resolver refuses `Submitted` | `internal/oms/application/resolve.go:397` derives its resolvable set from *"proved absence is one of the answers the domain would accept"*, which is `DispatchAuthorized` and `SubmissionUnknown` and nothing else. |
| Nothing else writes state | The only `ApplyState` callers are `resolve.go:741`, `dispatch.go:503` and `replay.go:521` (journal replay). |

So the fill ledger moves — `Cumulative` becomes real — while the order's
**state** does not.

## §2 Why it was invisible until now, which is the part worth keeping

`arblab exercise run` treats `StateSubmitted` **as its success state** and exits
on it (`internal/app/exercise/app.go:142`: a leg below `StateSubmitted` is the
failure). The single-venue root is a one-shot proof that the wire contract, the
transport and the journal work. **It never needed to know whether a leg filled.**

#236 is the first thing in this system that does, because a hedge sized to a
partner's fill is the first computation whose input is the fill itself. The gap
is therefore genuine and newly reachable rather than a defect anybody shipped:
the states exist in SM §1, the transitions are declared and policed, and no
consumer had yet required the system to walk them.

That is the reusable lesson, and it is a shape rather than an anecdote: **a
declared state nothing drives is invisible while every consumer's success
condition stops short of it.** The compiler is satisfied, `exhaustiveswitch` is
satisfied — every switch names the member — and coverage is satisfied, because
fixtures that *seed* a terminal record exercise the reading code perfectly well.
Only a consumer that must **arrive** at the state from a real `Place` can tell.

## §3 What it would have cost, stated as the run rather than as a risk

#236's D4 establishes a leg's exposure only when the order is terminal for state
and the join reports a clean reading. Against the current OMS that condition is
unsatisfiable for a placed leg, so:

1. Leg A is placed and fills 60. The join admits the 60 into the ledger.
2. `Establish` reads a non-terminal state on every settle attempt and returns
   **not established** with `Because = "state \"submitted\" is not terminal"`.
3. The drive treats A as indeterminate and **does not send leg B**.
4. D8 permits an unwind only on an *established* `VerdictHalfOpen`, so the
   compensation is refused.
5. The run exits 4 — "the pair is exposed" — with a real, naked 60-unit position
   and no automatic remedy.

The failure is fail-*safe* in direction (nothing worse is traded) and total in
effect: the happy path produces the outcome the slice was built to prevent. Note
also the second-order case, which is the one that would have survived a casual
fix: task 7 re-uses `establish` to confirm the unwind, so **even a successful
reduce-only close could never be recognised as closed.**

## §4 The decision, and the two alternatives rejected

**Taken: the remedy is its own task, landing before #236 is re-planned.**

The remedy touches SM §1 — the order lifecycle — which is one of the seven
normative state machines and is `docs/state-machines.md`'s subject. This
repository's own precedent for that situation is record §95 ruling 3, which kept
the §5.1/§6.7 normative re-scope *out* of a live-venue slice and gave it to #241.
The argument transfers unchanged and is not about tidiness: a slice that places
real orders against real venues should not also be the slice that edits the state
machine those orders move through, because the two failure classes need different
reviewers and different evidence. A lifecycle edit also has to be checked against
`docs/formal/` — `make formal-fresh` tracks sections by digest, and a transition
change is exactly the class that can void a model's coverage while its companion
still reports passing runs.

**Rejected — fold it into #236 as extra tasks.** One slice, no dependency wait.
Rejected on the ground above: it bundles a state-machine edit into a live-venue
composition root, which is the bundling ruling 3 already declined once, and it
would put an SM §1 change behind a review whose attention is on two-venue
composition.

**Rejected — re-scope D4 to drop the terminal-state requirement**, establishing
exposure from the fill ledger plus join completeness alone and arguing that an
IOC cannot rest. Cheapest, and it needs no OMS change. Rejected because it
replaces a guarantee the system can prove with a claim about venue behaviour that
it cannot: "the venue canceled the unfilled remainder" is an external fact, of
the class `docs/venue-profiles.md` governs and this project turns into a measured
row or a tested refusal rather than a paragraph. It is also the flattering
direction — it concludes "this is all that filled" from evidence that only shows
"this is what filled *so far*", which is precisely the confident-zero hazard D4
was written to close. Worth recording that the gap is *narrow*: the join's
`Unattributed` check already compares the venue's own cumulative against the
attributed fills, so what is unproven is not the amount but the **finality**.

## §5 What the new task must deliver

**The remedy is much smaller than "build a settlement subsystem", and this is the
fact that sizes it:** `ports.VenueOrder` already carries
`State domain.OrderState` — its own comment says *"the venue's lifecycle reading,
**already translated into SM §1's**"* vocabulary — and the joiner **already
fetches it**, at `internal/oms/application/join.go:801`
(`status, err := j.venue.Resolve(rec.Order)`), using it today for fill
attribution and the `Unattributed` comparison. The venue's terminal state is in
hand, in the domain's own words, and simply never journaled.

So the task is to journal it. Required:

1. **An OMS-owned operation that applies the venue's read state as journaled,
   legal transitions**, walking the intermediate hops `transition.go` requires
   (`Submitted → Acknowledged → Filled` / `Canceled`), rather than inventing a
   direct edge. Exposed to composition roots through `oms.Module` only —
   `singlewriter` keeps `ports` unnameable from every `app/` root, which is the
   same fence #236's `LegState` projection was minted for.
2. **A decision about where it runs**: inside the existing join (which already
   holds both the record and the venue reading) or as a sibling use case. That
   choice is the task's own, and the `Unattributed` check's placement is evidence
   for it.
3. **Integration tests that start from a real successful `Place`** and walk to
   each terminal: full-fill IOC, partial-fill-then-canceled IOC, zero-fill
   canceled IOC, venue rejection, and a status the venue will not resolve.
   **Tests that seed a terminal record prove nothing here** — the whole defect is
   that the path from `Place` to terminal does not exist, and a seeded fixture
   starts on the far side of it.
4. **A `make formal-fresh` check**, since SM §1's sections may be tracked as
   `subject` or `premise` by a model's correspondence map. A re-run plus
   `make formal-restamp`, or a `knownStale` marker naming this task.

## §6 What #236 inherits

D4, D5, D7 and D8 are unchanged and correct — they were written against SM §1 as
specified, and SM §1 is not wrong; it was unimplemented. #236 gains a stated
dependency rather than a redesign. When the new task lands, #236's `establish`
loop calls the new operation in place of `Join` + `LegState` polling, and the
plan's `venueDouble` gains the venue-status responses the walk consumes.
