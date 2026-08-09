# Record §90 — task #233: the maintenance-bracket criterion is a fixed point, and its uniqueness is a property of the fetched table rather than of the rule

**Status:** landed as a design record, v0.9.69 (2026-08-09). Milestone 4, task
**#233**, correcting record **§87 R1b** on record **§89 F3**'s measurement.
**No code.** Documents touched: `build-spec.md` §7.1 (normative),
`adr-history.md` §3, `venue-profiles.md` §9, `docs/formal/freshness.json`
(marker re-pin), and a forward-pointer block on §87 R1b itself.

---

## 1. What was wrong, and what was NOT wrong

§87 R1b specifies the `PackageDelever` sizing as a piecewise solve — enumerate
on-grid reductions ascending, classify each candidate into a maintenance bracket,
reject on `D_i ≤ 0`, reject on R1a's cash-feasibility inequality, reconstruct `L`,
accept the first candidate reaching `L ≥ L*`. **Everything about that survives
except step 2's acceptance criterion.** Step 2 says "classify `q_final` into its
**actual** bracket at the venue's risk basis (§5g)", and §5g explicitly left the
*basis* open — mark notional, `L*`, or something venue-specific — naming it as an
ambiguity to be closed by measurement rather than guessed.

#230 measured it. The answer is none of the three readings a person would try
first.

**Binance selects the bracket for `liquidationPrice` at the LIQUIDATION notional,
and the bracket it selects is the self-consistent one** — bracket `i` whose own
reconstructed `L` puts `q·L` back inside bracket `i`'s range. §89 F3 measured it
on both legs of the same symbol:

| leg | current notional | predicted by | error | the other bracket |
|---|---|---|---|---|
| short, `q` 370 000 | 4 847 → b1 | **b2** | +5.6E-10 | b1, off by 1.0E-5 |
| long, `q` 400 000 | 5 240 → b2 | **b1** | +3.2E-9 | b2, off by 9.8E-6 |

A short reaches **up** a bracket and a long reaches **down**, so it is not a
directional rule and not "the current tier". §89 filed the correction as its own
follow-up rather than patching §87 in place, on the ground that a normative change
to the sizing algorithm on the incident path wants its own record. This is it.

## 2. The derivation — and why it is a derivation rather than a restatement

The measurement establishes *which* bracket the venue picked in two cases. A spec
clause has to say what to compute in every case, which needs three things the
measurement does not supply: a closed form, an existence guarantee, and a
uniqueness guarantee.

**The liquidation NOTIONAL is the natural variable.** For an isolated short the
liquidation condition is `m + (e − L)·q = MM(q·L)` with `MM(N) = N·MMR_i − cum_i`,
which rearranges to

```
N_i := q·L_i = (m + q·e + cum_i) / (1 + MMR_i)
```

— a closed form per bracket, depending on `q` only through `m + q·e`. The long
mirror is `N_i = (q·e − m − cum_i)/(1 − MMR_i)`. So step 2 becomes: compute `N_i`
for **every** bracket the tier read supplies, and accept the one with
`lo_i ≤ N_i < hi_i`.

**Existence and uniqueness follow from one identity.** Using the continuity
relation `cum_{i+1} = cum_i + b_i·(MMR_{i+1} − MMR_i)` — where `b_i` is the
boundary between brackets `i` and `i+1`, and which is *why* `cum` exists — the
difference between adjacent brackets' predictions collapses to

```
N_{i+1} − N_i = (MMR_{i+1} − MMR_i) · (1 + MMR_i) · (b_i − N_i)      [short]
N_{i+1} − N_i = (MMR_{i+1} − MMR_i) · (1 − MMR_i) · (N_i − b_i)      [long]
```

Read the short case: if bracket `i` predicts a notional **above** its own ceiling,
the next bracket up predicts a **lower** one, and the algebra shows it still lands
at or above that bracket's floor — the step toward a bracket never overshoots it.
So the search converges. And once one bracket is self-consistent, `N` falls below
the floor of every higher bracket and rises above the ceiling of every lower one,
so no second bracket qualifies. **The long's sign is flipped, which is exactly the
non-directionality the measurement saw** — the same identity explains both legs,
which is the check that it is the right identity and not a fit.

## 3. The finding that did not come from the venue — and it is the normative half

The identity above **consumes** the continuity relation. That relation is
documented behaviour and it is nonetheless a claim about a **fetched response**.

Encoding the rejected reading as a control rather than arguing it (this repo's
habit, and how `FM-6`…`FM-9` were each settled): **zero every `cum` and re-run the
uniqueness sweep.** The criterion then admits **two** self-consistent brackets in
some configurations and **none** in others — 494 violations across 2 904
configurations, in both directions.

So **"exactly one bracket qualifies" is a property of the TABLE, not of the rule.**
That converts a piece of arithmetic into a normative requirement:

- The tier read (**#220**) MUST assert **contiguity from zero** (each
  `notionalCap` equals the next `notionalFloor`; first floor 0) and the
  **continuity relation** on the table it fetched.
- A table failing either is **refuse-to-size**, never a nearest-bracket fallback.
  The fallback's error is silent, and it lands on the incident path — the
  `platform/config` direction-of-default rule, applied to venue reference data.
- A liquidation notional above the top bracket's **finite** cap has no fixed point
  either, and refuses on the same ground. The cap is finite: 3 000 000 on the
  measured symbol.

On the live table continuity holds with residual **exactly 0** at all eight
boundaries and the table is contiguous from zero — so the check is being
*asserted*, not hoped for, and it is asserted against a shape we have seen.

## 4. A second defect in R1b, found while making the first one precise

R1b uses the bracket in **two** places, and they take **two different bases**:

- **step 4, feasibility** — the margin a candidate needs *to reach `L*`* is
  `m = q·D_i − cum_i` with `D_i = (1 + MMR_i)·L* − e`, which is `MM` evaluated at
  the **target** liquidation notional `q·L*`. `L*` is given, so this is a direct
  classification and no fixed point arises.
- **step 5, reconstruction** — `L` is the unknown, so this is the fixed point
  proper, at the **achieved** `q·L`.

Step 2 as written is a single "classify the candidate" instruction, which reads as
one selection serving both. It is not. The enumeration accepts the **first**
on-grid candidate with `L ≥ L*` — an overshoot — so `q·L*` and `q·L` differ, and
they can straddle a boundary: **7 of 960** candidates in a sweep around the
measured position select different brackets under the two bases.

The unifying statement, which is what §7.1 now carries: **the bracket is always
the one containing the LIQUIDATION notional, and the fixed point is what that rule
becomes when the liquidation price is the unknown.** Neither use is `q_final·e`.

## 5. Why four sessions of measurement did not see this — and where it bites

**Inside a single bracket the criterion is invisible.** Mark notional, entry
notional and `q·L` all select the same bracket while the position stays inside
one, so all three candidate bases predict an identical observation. §7.2's
small-size posture keeps this system inside bracket 1, and every measurement
before #230 sat there. This is the collapse checklist at the venue layer — the
fixture parameter pinned to the value that collapses the distinction — and it is
the same shape as `α = 0.5` in §89 F2, found in the same session.

**And the boundary is NOT the published cap in the units sizing thinks in.** A
short's liquidation notional *exceeds* its current notional by
`(1 + 1/λ)/(1 + MMR)`, so bracket 1 expressed in **current**-notional terms ends at
`hi₁·(1 + MMR₁)/(1 + 1/λ)`:

| effective leverage | bracket-1 ceiling as % of the published cap |
|---|---|
| 1.5× | 61% |
| 2× | 67% |
| 3× | 76% |
| 5× | 84% |

§7.1's stated range is 2–3× on BTC and 1.5–2× on alts, so the whole operating
range sits at **61–76%**. A size check written against the published cap crosses
the boundary while reading as compliant.

## 6. The evidence, and how it was checked

An oracle (`research/venue-probes/233-bracket-fixed-point/fixedpoint_oracle.py`)
computes everything above in `Decimal` from the **raw capture's own bracket
table**, not from a summary of it. Four checks:

| check | result |
|---|---|
| **A. reproduction** — regenerate §89 F3's four rows | all four match; residuals 5.59E-10 and 3.23E-9 against the record's 5.6E-10 / 3.2E-9 |
| **B. existence + uniqueness** over the real 9-bracket table | 2 904 configurations, **0** violations |
| **C. the step identity**, both legs | 9 552 adjacent-bracket steps, **0** violations |
| **D. the bite point** in current-notional terms | the table in §5 |

Three things about *how* it was checked are the reusable part:

- **The first run of check C failed, and the harness was wrong, not the claim.**
  `Decimal.copy_sign(D(1))` copies the sign of the *argument onto self* — it
  returns `|x|` — so the test compared magnitudes while claiming to compare signs.
  A and B passing is what identified it as a harness fault rather than a refutation:
  a universally-objecting predicate would have failed those too.
- **Check B's first version passed VACUOUSLY, and the collapse checklist caught
  it.** The sweep stepped by `top_cap/400` = 7 500 — wider than bracket 1 itself
  — so the lowest notional it ever tested was 7 500 and it never entered brackets
  1–2, which is exactly where §89 measured the defect. Replaced with
  boundary-relative sampling and a per-bracket coverage count, which the run now
  prints: every one of the nine brackets is selected at least 87 times. Counting
  executions says nothing until you count how many reach the interesting branch.
- **The negative control is what produced §3**, not a confirmation of §2. Breaking
  `cum` was run to prove the checks could fail; what it showed was that the
  property under test belongs to the venue's data rather than to the algorithm.

## 7. Limits — stated, not absorbed

- **Binance only.** Both measured legs are Binance demo. Bybit is untested on this
  question and deliberately so — see §8.
- **One symbol, one table.** The continuity residual of exactly 0 is a property of
  the `DRIFTUSDT` table as fetched on 2026-08-09. That is precisely why §3 makes it
  a runtime assertion instead of an assumption; it is not evidence that every
  symbol's table is well-formed, and it is not meant to be.
- **The 7-of-960 straddle rate is illustrative, not a distribution.** It comes from
  a sweep around one measured position with one `L*` and three margin bumps. It
  establishes that the two bases *can* disagree — which is all the normative claim
  needs — and nothing about how often that happens in production.
- **Uniqueness is proved for tables satisfying continuity + contiguity.** A table
  violating either has no guarantee, which is the refusal, not a gap.
- **Nothing here is executed by engine code**, because there is no engine code on
  this path yet: #220 (the tier read) and #222 (slice B4) are both open. The
  assertion in §3 is a requirement placed on #220, and the first thing that can
  falsify this record is #220's implementation meeting a real table.

## 8. Rejected alternatives

- **"Patch it into §87 R1b in place."** Rejected. §87 is a landed record; editing
  its ruling silently would make the correction invisible to anyone who read §87
  before today. R1b instead carries a **forward pointer** naming this record, and
  the normative text lives in §7.1 where an implementer reads it.
- **"The current tier"** — the natural reading, and the one the enumeration's
  wording invites. Refuted by measurement: off by 1.0E-5 on the short leg and
  9.8E-6 on the long, in a quantity published to 8 decimals.
- **"State `q·L*` as the single basis for the whole solve."** Tempting because it
  is right for step 4 and needs no fixed point. Rejected: step 5 reconstructs the
  *achieved* `L`, which the enumeration allows to overshoot `L*`, and the two
  straddle a boundary in the sweep. One basis for two questions is the shape §89
  F3 already found in the venue's own two published fields.
- **"Assume continuity because the venue documents `cum` as the continuity
  term."** Rejected on this project's standing direction-of-default rule: the
  claim is about a response, so the burden sits with the reader. The control shows
  the failure is not hypothetical — it breaks uniqueness in **both** directions,
  and a solve that picked the first matching bracket would have silently taken
  whichever the loop reached first.
- **"Fall back to the nearest bracket when no fixed point exists."** Rejected. It
  converts a malformed table into a wrong position size with nothing failing, on
  the incident path, during the rally that fired the trigger.
- **"Derive `MM(q·L)` from the row's own `maintMargin`."** Rejected, and it is a
  trap rather than a preference: `maintMargin` sits on the **current**-notional
  bracket (measured: 48.47 and 53.60 on the two legs) while `liquidationPrice`
  sits on the liquidation-notional one. They arrive on one row and neither derives
  the other.
- **"Lift Bybit's tier-1 cap now that the basis is understood."** Rejected — the
  understanding is Binance's. §87 §5g's cap is what makes the basis question moot
  on Bybit, and the cap is now recorded as **load-bearing for this clause too**:
  lifting it re-opens this on that venue. Note also §89 F4: Bybit's liquidation
  coefficient is already known *not* to match the common relation, so its bracket
  arithmetic cannot be assumed to mirror Binance's either.

## 9. What this opens

- **#220 gains a requirement, not just a read.** The tier read must assert
  contiguity-from-zero and the continuity relation on the fetched table, and
  refuse on failure. Its task description is updated.
- **#222 (slice B4)** consumes the corrected criterion. #233 is discharged as a
  blocker on it.
- **#227** is unaffected by this record and still open: Bybit's coefficient
  question is separate from the bracket-basis question, and F4 leaves two
  candidate coefficients alive.
- **Nothing is owed to the formal models.** Both exclude money by construction, and
  this record is money arithmetic. The `build-spec.md §7.1` marker in
  `freshness.json` was **re-pinned** to the new digest with that judgement written
  into its note — the gate's own rule is that a second edit fires again, and it
  did.
