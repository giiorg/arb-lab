# ADR & Review History — Funding-Rate Arbitrage Platform

**Status:** v0.9.55 (2026-08-07) — §3 gains a **milestone-4 task #177 block (record §74): SM §7's `PackageDelever` generation ladder is built, `FM-9` has a consumer, `FM-8`'s standing obligation is DISCHARGED rather than merely permitted, and the repo's one `SPEC EDIT OWED` marker is closed.** Nine rows, and the lead item is that **four rulings came out of BUILDING the machine** — the quiesce obligation needs no cancel port (every order is a marketable IOC, so quiescing is awaiting finality, and the wait is bounded because the phases it binds on exit only by ending the generation); the entry gate and the standing trigger want **different thresholds on one counter**, so the reservation is taken before the premise is asked for, else a stale sibling's entry is silently netted out; ending a generation **releases** its entry rather than amending it to zero, which is the same final ledger and a different act; and whether §7.1's ladder **resumes** after a supersession is a property of which row fired. §6.7 also gains the ledger's **sign** — exposure removed, never the order's own change, the direction its `FM-5` amount rule left open — and §5.1 the journal's second **scope**. `ambiguous-order-model.md` is **corrected**: a parenthetical in its clause map claimed a reachability route (`or via Supersede`) that reaches the guard while permanently disabling the action, so the model is *silent* on ladder resumption rather than supporting either answer. Two mutation verdicts on one signal are recorded pointing opposite ways: a collapsed report **survived and the mutant was wrong** (the guard is one layer down), while a deleted hedge-group check survived on a **real gap** (every fixture planted its legs by construction). Thirteen mutants, all caught. **Left open and filed:** the journal adapter (#203) and the delever legs' §2.4 filter re-check, whose trigger has now fired (#184). Previous: v0.9.54 (2026-08-07) — §3's milestone-4-slice-A block gains **six rows for task #199 (record §73p), closing slice A's last open ruling by finding that the question rested on a conflated property rather than on a trade-off.** `Scope` is the granularity a §4.1 rule is answered at; the family requirement read it as the rule's **subject**, and demanded a futures endpoint from a deployment holding only the spot leg. A `subject` column settles it, and the two venues get **opposite** answers from that one rule — which is the evidence it is not a Binance exemption. Three things came out beside it, none absorbed: a comment asserting an empty roster was already refused was **false from the session it was written**; the gate's only end-to-end refusing-path fixture rested on the same conflation, in a test; and a 60s channel bound tied `mutate.sh`'s own 60s timeout, so the guard could never be seen to fire. One thing is **left open and filed** (#201): Profiles §2's Binance cell checks nothing about a spot leg where Bybit's checks three. Nine mutants, all caught. Previous: v0.9.53 (2026-08-06) — §3's milestone-4-slice-A block gains **three rows for task #199 item 1 (record §73o), and the first of them retires a finding by disproving its premise rather than by ruling on it.** **F18** was filed as one of three ways §4.1's gate could be unpassable, awaiting an operator ruling. It was a defect: `GET /fapi/v3/positionRisk` does not publish `isAutoAddMargin` at all, so the Binance gate refused on **every** account rather than only on flat ones — which also dissolves **§73j F9**'s Binance half. `GET /fapi/v1/symbolConfig` publishes all three per-symbol settings for every configured symbol, so the probe moved there and the fourth probe was deleted (80 weight → 60, six checks on three calls). **The evidence is a new source class for this project**: Binance's own spec-generated connector, raw and version-controlled, which confirms Profiles §9's dated transcription exactly and answers a question the WAF-blocked doc site has refused for four sessions — while answering nothing about weights, so **#180 is untouched**. Nine mutants, all caught. The third row records a coverage consequence that was noticed rather than absorbed: `ReasonAmbiguous` lost its end-to-end witness with the deleted probe, and gained a new one crossing codec → ACL → rules. Previous: v0.9.51 (2026-08-06) — §3 gains the dated block for **milestone 4 slice A (task #198)** — §4.1's account & margin-mode posture verified at boot, `internal/risk` created, and `GateAccountPosture` moved across `ExitClass` into the produced arm it had been declared beside since slice G2. **Seventeen rows**, and they split three ways: **one defect in the specification** (Profiles §2 requires Portfolio Margin OFF and supplies no `GET`, in a cell where every other item names one — resolved as an operator attestation with default refuse, and the venue doc gains a **marked unknown** rather than a value); **four design traps** where the natural implementation is vacuous or permissive in the direction that computes the safety gate in our favour (a leverage pin derived from the venue it checks; auto-add-margin driven by returned positions, which examines zero symbols on the flat account every boot starts from; an external enum read as a blacklist; a borrow field whose *other* consumer reads silence the opposite way and is right to); and the **vacuity family at two new levels**, both promoted to general checklist items — *a collapse checklist runs at ONE LAYER unless re-run* (every end-to-end boot fixture carried an empty roster, so §4.1's per-symbol half was reachable at boot and asserted nowhere, because a **legal** degenerate shape is the one nobody notices is degenerate) and *one-violation-per-fixture hides a ladder's ORDER* (eighteen cases, each breaking one rung, and swapping two rungs left all eighteen green). **The closing reading pass produced the row no fixture could:** the posture probe reads `isAutoAddMargin` from `/fapi/v3/positionRisk` while Profiles §9's own verbatim V3 transcription does not list the field and Profiles §2 cites the **V2** page — §7.5 picked V3 on an argument about what a missing row means for the **net-delta invariant**, §4.1 inherited the choice for the opposite question, and *a narrowing carries an argument whose scope the next consumer does not re-derive*. **Two operator questions are owed (task #199) and nothing downstream is blocked by either**: a flat Binance account refuses on every configured symbol, and a spot-only deployment cannot issue any of the four Binance probes at all — a gate that cannot pass is not conservative but broken, and the strict reading stays implemented rather than silently loosened. Previous: v0.9.50 (2026-08-05) — §3 gains the dated block for **milestone 3 task #197** — the §12 closure pass: milestone 3 is recorded complete, its item-3 annotation trail (which had **lapsed rather than been demoted** after slice E) is restored through G3 with per-slice task/record/version citations, and the pass's one finding is that **`sizing-by-live-tier` was listed as milestone-3 content with no implementation anywhere** — reassigned to milestone 4 on four grounds, the first being that **`C4`'s own traceability row in this document already read *"Profiles §1, core §7.1"***. Nothing in the system sizes a trade; eight comments across seven packages defer liq-distance to a computation no package owns, and that chain now terminates in milestone 4 by name. **The failure direction is a halt, not a wrong number**, which is how it survived five slices and a formal-model campaign — and §47e finding 1 is the same defect one milestone up, making the roster the artifact both were invisible on. Beside it, profiles §9 records a **third** unread `positionRisk` weight candidate and the shape-vs-value split two disagreeing summarizers establish (charged 20 unchanged = max over candidates; #180 open). Record §72. Previous: v0.9.49 (2026-08-05) — §3 gains the dated block for **milestone 3 task #179** — the spot leg's exposure baseline: `baseline := B − ours` recorded by operator command into a chain-marked inputs-only document, loaded by a recomputing verifying loader, pinned by the two-act `{path, sha256}` config edit, and asserted per baselined asset inside the join with a settle-window second look whose stable delta alone latches; `UnassertedNoPositionSurface` deleted, `UnassertedBaselineUnrecorded` holds the posture; three adversarial design rounds (fourteen majors, all folded), the §70h item 1–24 audit that falsified the safe-point summaries' own coverage claim, a final code_diff review round with four more majors folded (§70i — absence semantics per venue, the newly-absent downgrade, the loader's side recomputation, rule 3's documented exception), 28 mutants zero SURVIVED in all (record §70). Previous: v0.9.48 (2026-08-05) — §3 gains the dated block for **milestone 3 task #183** — trading filters gain a fetched-provenance writer (`arblab reference fetch`, unsigned public endpoints, full cell binding before the stamp) and the executor re-judges every leg's filter row age at ONE package admission instant against the declared bound, with a future-observed row refused separately; one design-review round (five majors, all accepted), second-model implementation, twenty-mutant campaign zero SURVIVED (record §69). Previous: v0.9.47 (2026-08-05) — §3 gains the dated block for **milestone 3 task #188** — the package reserves its whole rate-limit cost before any authorization fsync (untimed occupancy, all-or-nothing, blanket-cancelled) and the credentialed transport is constructed under the SpendDomain-keyed one-spender flock; eleven design rounds, a second-model implementation from Appendix A, five post-write/checklist findings fixed, fourteen-mutant campaign zero SURVIVED (record §68). Previous: v0.9.46 (2026-08-04) — §3 gains the dated block for **milestone 3 task #187** — the silent fold miss becomes a counted, rendered operator fact: penalty-scoped detection after classification, render-once in exercise, the one-spender premise recorded with its enforcement gap, and a code_diff-demonstrated format-drift survivor killed by a golden report test (record §67). Previous: v0.9.45 (2026-08-04) — §3 gains the dated block for **milestone 3 task #185** — Bybit's per-endpoint meter gets a profile-side home: `ScopeEndpoint` is venue-level with path-derived names from a closed registry, refused pre-send when undeclared and required complete at boot, with the header fold moved ahead of the body read and a pre-existing `freeAt` underflow fixed (record §66). Previous: v0.9.44 (2026-08-04) — §3 gains the dated block for **milestone 3 task #186** — a profile limit of a headered dimension must bear exactly the fold-derived name, checked at signed-client construction where the reader/governor pair is formed, with the runtime reader canonicalizing header windows through the same renderer so the certified name and the minted name cannot disagree (record §65). **THE FINDING** is the anchor rule caught at design time: the v1 check and the runtime derived names from different inputs, so a `60S`-spelled header would evade a check that certified `weight_1m` — boot passing while the fold still drops. **THE SECOND** is an inventory claim falsified twice: "all existing profiles conform" missed the exercise chaos JSON and then the typed `LimitConfig` fixtures, making three fixture families the third-through-fifth live instances of the very defect the check exists to catch — every exercise and spot-only reconcile run was silently dropping every venue reading. **THE THIRD** is a rebuttal that survived adjudication: the reviewer's spelling-tuple identity would reintroduce the observed silent loss to guard an unobserved venue shape whose failure direction is conservative anyway. Six review rounds; 13 targeted mutations, 12 kills + 1 diagnosed equivalent. Task **#186** closes. Previous: v0.9.43 (2026-08-04) — §3 gains the dated block for **milestone 3 task #173** — §7.6's cannot-count refusal as two enforced layers: the governor errors on a cost charging a dimension its applicable budgets do not count, and a profile is rejected at construction unless every family budgets every chargeable dimension or writes down why not, with the venue request envelope mandatory and never waivable (record §64). **THE FINDING** is a debt whose premise expired unread: the task was filed under "no production profile exists anywhere" with the first one as its trigger, and slice G2 then built that profile without the debt being re-checked. **THE SECOND** is a live cross-slice contradiction at a seam no suite composed: Bybit's read surfaces spelled their truthful request-only cost as the zero `Cost`, which the signed client's own validation refuses before Acquire — every Bybit read through `Do` failed at its own precondition while the builder and governor suites each passed; `RequestOnly()` is the claim with a carrier, and a composed test now exists. **THE THIRD** is the second-model review reshaping the design twice: a written waiver is a claim about traffic and never a suppression (the runtime refusal stands as the claim's falsification), and family-scoped request counters never satisfy the SHARED envelope. Tasks #185/#186 filed. Task **#173** closes. Previous: v0.9.42 (2026-08-04) — §3 gains the dated block for **milestone 3 task #174** — the Place ack's venue order id persisted at dispatch (fourth journal record kind, ack-before-state) and §5.3's gate 3 ruling swept rows out against the instrument's persisted-ack set (record §63). **THE FINDING** is an unbanked premise consumed by a safety rule: the rule-out rests on per-symbol `orderId` NON-REUSE — strictly stronger than the banked join-key fact, and identical to it in code — and the second-model review refused the slice until the venue's own pages were fetched and the statements banked verbatim (Profiles §8, 2026-08-04). **THE SECOND** is the conflict design: an ack contradicting the store latches the live process through `HoldsNewTrading` and leaves a journal that verifies and does not compose, so a boot never builds an index a conflict poisoned — the composed test drives both halves against the real store. **THE THIRD** is a review catch about authority: the Joiner must not read the index, because a persisted ack and a live answer are two authorities over one identity — it refuses a mismatch instead. 19 mutations, 19 kills. Task **#174** closes. Previous: v0.9.41 (2026-08-03) — §3 gains the dated block for the **milestone 3 quantization slice** — §2.4's trading filters enter the order path as a VERIFIED property, enforced by one shared check at plan and at execute preflight (record §62). **THE FINDING** is that the order path's quantization was a doc comment: the deriving quantizer (§40g's `TargetQty`) existed only on the scoring path, `LegIntent.Quantity` asserted "§6 step 4 has already quantized it" with no code making it true, and the spec mandated only the LOT intersection — so the tick and min-notional duties were two **unmarked** spec holes, the class the OWED grep cannot see because the clause is absent rather than departed from. The reading pass found them; §2.4 and §6 steps 5/6 now carry the clauses. **THE SECOND** is the harm class that makes verification load-bearing rather than pedantic: each leg is judged by its own venue against its own grid (BTCUSDT lot steps differ 100×, ticks 10×, between Binance's own spot and perp), so an off-grid package is one leg filled and one refused — **naked exposure, not an error message** — and the refusal must land before anything is journaled, package-atomic at both enforcement points. **THE THIRD** is operational: live probes re-confirmed the USDⓈ-M min-notional floor at **50 USDT**, so §9's $20 live test cannot run on the Binance perp as imagined — the package grows to ~$55 or the perp leg moves to Bybit (#171). Task **#182** closes; **#183**/**#184** filed. Previous: v0.9.40 (2026-08-03) — §3 gains the dated block for **milestone 3 slice G3** — §47f's last slice: the parallel two-leg marketable-IOC executor, §9's cycle driver `arblab exercise run`, and the process-kill-mid-IOC chaos test that proves A–F composed (record §59). **THE FINDING** is that the in-process double-dispatch backstop — the map that holds precisely when the projection broke — had silently scoped itself "per path" the moment a second dispatch path existed: a private set per dispatcher WAS the process-wide sent-set only by accident of there being one path, and nothing would have reported the narrowing. Fixed twice, deliberately: first as `DispatchAuthority`, one synchronised monotone set per process; then — after the battery proved the WIRING of the sharing untestable (a two-authorities mutant survived, observable only through a broken projection no real repository produces on command) — structurally, by making the executor's constructor take the DISPATCHER, so a second authority stopped being expressible. A check nothing could make became a signature. **THE SECOND** is the collapse checklist's tenth instance, caught by the battery rather than before it: the identical-base-quantity fixture varied only upward, where `cmp != 0` and `cmp > 0` coincide. Task **#181** closes; the §9 acceptance campaign (≥100 cycles, 14 days) is now runnable rather than run. Previous: set-version alignment, no change in this doc (v0.9.39 is **milestone 3 slice G2**: the composition root, §5.2's OMS schema, `arblab reconcile run`, and §7.5's exemption traced back to the one cause it was argued from; record §58). Previous: **Status:** v0.9.38 (2026-08-03) — §3 gains the dated block for **milestone 3 slice G1** — §7.5's reconcile-only boot composed into one use case, in the one order its phases are correct in (record §57). **THE FINDING** is a contradiction between two clauses of the SAME use case, visible only from the composition: the join records an absent order precisely because §5.3 forbids inferring "no executions" from an absence, and the assertion one function later read that very zero as a fact — so any fill such an order took reported as an **unexplained position delta**, which alerts for operator clearance and **latches**, on the boot §7.5 exists for and for a condition that discharges itself in seconds. **THE SECOND HALF** is the outcome vocabulary's THIRD empty cell: the fixed reading needs an outcome meaning "ran, refuses, a retry may lift it" at reconcile-only, and F2's tabulation showed that cell as occupied by two PRE-VERDICT members — so the missing property was whether an outcome reports a result at all, and a product is only as complete as the properties it is tabulated on. Task **#175** lands as a method on §5.3's `UnprovedReason`. Previous: v0.9.37 (2026-08-03) — §3 gains the dated block for **milestone 3 slice F2** — §7.5's venue join made executable, the fill ledger rebuilt from venue truth, and the net-delta invariant's real domain established (record §56). **FINDING 1** is that the invariant is stated over a hedge group while venue truth exists for only one of its legs — no venue has a position surface for spot, and the balance standing in its place contains holdings this system never bought — so the uncoverable leg is now REPORTED on every boot and deliberately does not hold the posture. **FINDING 2** is the outcome vocabulary's second empty cell, one slice after the first was filled without tabulating the rest: §6 step 7's hold is **stop-opening AND latching**, because the check that would lift it recomputes from the projection that is known stale, and the stale reading is the permissive one. Previous: **Status:** v0.9.36 (2026-08-03) — §3 gains the dated block for **milestone 3 slice F1** — §7.5's replay made executable and §5.3's anchor given a journal record kind (record §55). **THE FINDING** is that §7.5's preconditions vary on two independent axes — restrictiveness and whether a re-run may lift them — while the outcome vocabulary was a single ladder, so the one condition wanting *halted AND latching* had to be reported as whichever half mattered more, and the plausible choice (an operator-clearance matter) leaves the process in reconcile-only, permitting the very re-drives the unverifiable log is the input to. Previous: **Status:** v0.9.35 (2026-08-03) — §3 gains the dated block for **milestone 3 slice E** — the §6.7 classifier made executable, with `FM-5`, `FM-6` and `FM-7` becoming claims the types make rather than paragraphs (record §54). **FINDING 1** is that §5.1's hedge-group key has three components and an order record carries two, and the missing one fails by **concluding**; **FINDING 2** is a state-count citation that no run reproduces, caught by the reading pass one step before it was copied into the code. Previous: **Status:** v0.9.34 (2026-08-03) — §3 gains the dated block for **milestone 3 slice D** — the venue-specific stable-not-found rule made executable as **four independently falsifiable gates**, its two findings and the spec edits they forced (record §53). **FINDING 1: the rule's anchor had no home, and slice A's rejection of it had EXPIRED** — the journal codec refused a timestamp because it would add *"a value the chain covers but nothing reads"* and because *"when an event time is genuinely needed it is the venue's, and it arrives on a fill"*; both clauses were true then and both are false now, and the case that needs the anchor is an order with **no fill and no venue record at all**. A rejection's REASON is a claim about the future, and nobody re-derives a rejection when the future arrives — no grep can see it, because the clause is *absent* rather than departed from. **FINDING 2: State-Machines §1's query-resolution set had no `Canceled`**, so a missing edge left `AbsentAfterFinality` — FLAG-2's own conflation — as the only legal target for the commonest answer a resolving query can get. Core §4, §5.3 and §12 and State-Machines §1 are edited. Previous: **Status:** v0.9.33 (2026-08-03) — §3 gains the dated block for **milestone 3 slice C2b** — the order READ path, its four findings and the two spec edits they forced (record §52). **§4's `Cursor` cannot be either venue's cursor**: the three execution surfaces offer an ascending trade id and an opaque **descending** page token, none comparable to a time, while §6.7 step 3 sweeps *through a watermark strictly later than* a horizon — so the idiomatic reading implements §4 **completely and correctly on all three** and makes the one clause that consumes it **inexpressible**. The cursor is a server instant; the venue's handle is within-sweep paging, and on Bybit it walks into **older** data, so storing it as the next `since` re-reads one window forever **while every request succeeds**. **Not-found is a RESULT, not an error** — Binance states it in an error envelope, Bybit in a success body, C2a's finding with the venues swapped — and as an error it is indistinguishable from the rate-limited query §5.3 forbids inferring from. **The stable-not-found rule has an EXPIRY DATE**: an unfilled `CANCELED`/`EXPIRED` order is dropped after **3 days** on USDⓈ-M and after **24 hours** in Bybit's history, and §6 step 6's marketable IOCs make that this system's commonest terminal outcome — so `AbsentAfterFinality`, whose meaning is **proved** absence, is bounded by retention (§5.3 edited; #163 builds it). **On Binance a swept execution carries no client order id at all**, so §4's `Ack.VenueOrderID` reassurance — generalized from the one venue where it holds — is withdrawn. Profiles gains **§8**. The mutation round produced the **mirror rule's second axis**: C2a's own survivor recurred on the read path, because the mirror checked was the other *venue* and the one missed was the other *direction of travel*. Previous: v0.9.32 (2026-08-03) — §3 gains the dated block for **milestone 3 slice C2a** — the order write path, its three findings and the two spec edits they forced (record §51). **Bybit states a REFUSAL inside an HTTP 200**, so C1's status-only classifier read a rejection as an acceptance — and one step further on, a refusal body carries no `orderId`, the ack decode fails, and a failed decode after a 2xx is `EffectApplied`: an explicit venue rejection recorded as an order that **definitely exists**. Profiles §6 had named that seam for a *different input* (the zero-length 401) and the second instance landed anyway. **§7.6's shed classes had no carrier anywhere**, and the two defaults are unsafe unequally, so §4, §5.3 and §7.6 are edited and the class travels as an argument checked above the fsync. **A documented venue ZERO** (Binance's futures new-order weight) met this repo's zero-is-unusable rule, which was corrected without weakening its argument. Profiles gains **§7**. Previous: v0.9.31 (2026-08-02) — §3 gains the dated block for **milestone 3 slice C1** — the signed REST transport, its three findings and the §7.6 correction (v0.9.31 is **milestone 3 slice C1**: `engine/venue` has a **signed REST transport** at last — credentials that cannot leave their package, both venues' signature compositions, the clock-injected timestamp, the §7.6 governor attachment and §5.3's effect classification. §47f's slice C is **split by LAYER, not by venue**, because §47f's own mirror hazard forbids putting the two venues' clients in different sessions; C2 carries `Place`/`OrderByClientID`/`Fills`. **THREE FINDINGS.** (1) The slice's scope note claimed the venue facts were *"already recorded (profiles §1/§4/§7)"* — **there is no §7**, and how either venue composes the string it signs was recorded NOWHERE; a citation to a section that does not exist is the tell, and it is §47e's shape one layer out, invisible to the marker grep because the clause is absent rather than departed from. (2) **The evidence is not symmetric even though the code is:** Binance publishes three reproducible vectors (all re-computed and matching), Bybit's masks its secret (the literal-placeholder hypothesis was TESTED and refuted), and the credential-free differential probe is **refuted on both venues** — each checks the credential before the signature — a refutation trustworthy only because Binance ran first as its **positive control**. So writing Bybit's signer "the same way" inherits none of Binance's assurance (task #171). (3) **§7.6 was WRONG about Bybit's header and is corrected**: `X-Bapi-Limit-Status` is *"your remaining requests"*, the COMPLEMENT of Binance's used-weight, so the upward-only fold inflates the count on good news and **discards the venue's warning when it is nearly banned** — the silent direction, and the one that ends in the ban. Also: the acceptance window is **asymmetric** — `recvWindow` widens only the behind side while the ahead side is a fixed 1000 ms no parameter can raise, so the documented knob repairs one direction and cannot touch the other. 18 mutations, 18 kills; three survivors, one of which is a NEW shape — the fixture was correctly varied and the **assertion** could not see the variation. Record §50). Previous: v0.9.30 (2026-08-02) — §3 gains the dated block for **milestone 3 slice B** — **the planning use case exists and `module.go`'s stated hole is CLOSED.** Record **§49**. Deterministic `ClientOrderID` minting, every leg's intent fsynced before any submission, `ApplyPlanned`'s first caller, the OCC version threaded into `Dispatch`, and `Deps.Journal` converted to a path as `singlewriter` forced. **What closed the hole was a SIGNATURE, not a check:** `Dispatch` takes an id and a version and READS the order from the write repository, so an unplanned order cannot be dispatched and one mutated after planning cannot be submitted in place of the one that was journaled — and the same change made `admit`'s `Order.Validate` an unreachable branch, so it was removed and the refusal became a port contract. **`TradePlanned` is deliberately unwritten** (§5.1's `Trade` aggregate has no type) and what makes that safe is a *different clause of the same sentence* — all legs journaled before any submission, so every partial plan is `Planned`-only and §5.3-discardable; the premise is a run rather than a paragraph. **THE FINDING is `FM-5`'s shape at the projection layer:** a journal-then-apply failure after the send leaves a sent order at `Planned`, the one non-terminal state contributing no possible-delta interval, so §6.7 reads **ambiguous** as **determined** and unblocks the very compensation the ambiguity held — refuse-to-trade, §6 step 7, task #169, and no invariant computed from the projection can detect it. It carried **no marker** and the grep could never have found it; the reading pass did. 17 mutations, 17 kills; all four `NOBUILD!!` were bad seds and the discriminator is now stated generally. Previous: v0.9.29 (2026-08-02) — §3 gains the dated block for **milestone 3 slice A** — **milestone 3 slice A is landed** — `ports.Journal` has an adapter at last: a durable, append-only, hash-chained write-ahead journal on SQLite (WAL + `synchronous=FULL`, **verified at open rather than assumed**), closing §47e finding 2, so §5.3's fsync-before-network crash boundary is no longer proven only against a test fake. Record **§48**. **The store is SQLite on §5.3's ground, not §13's** — neither journal method needs a transaction today, so from inside the slice the engine reads as a free choice and it is not: the fill apply, the REST cursor advance and the journal/outbox append must share **one transaction**, and `driven/doc.go` had already recorded that a transaction spanning two stores cannot be added to one of them alone, so a bespoke log file would be rewritten at slice D/F or the rule would go unmet. **Three findings.** (1) Session 73's predicted design fork was **mis-premised**: `engine/capture` has a *sequence* chain plus per-record CRC32C and whole-file sidecars — **no hash chain** — so reuse would have supplied framing and the chain would have been new code regardless. (2) The pragma check hid a **self-consistent mutation**: `journalDSN` and `verifyPragmas` deliberately share `wantSynchronous` so they cannot drift, which means mutating the constant makes the request and the check agree and every *Open succeeds* test passes against a journal that no longer fsyncs — capture's wrong-CRC-polynomial note arriving as a **configuration constant**, and the fix is the same: the required values are stated as **literals** in the test. (3) §47f's collapse warning stopped **one short** — it named a chain of length 1, and at length **2** *link to the previous record* and *link to the first record* are still the same predicate; three is the shortest chain that separates them. Twelve mutations, twelve kills, fsync and the chain link checked separately; the collapse checklist ran before them. `Deps.Journal` stays a port field (slice B's, per §47f) and chain **verification** is not implemented — only construction. Previous: v0.9.28 (2026-08-02) — §3 gains the dated block for **milestone 3's pre-gate admission** (ruling record §47): §42d's rejected alternative (i) is reopened, the milestone-2b hard gate narrows to milestones 5–7 plus milestone 4's leg-shape-dependent half, and two scope findings land (§47e — `engine/venue`'s account/order half was never delivered; `ports.Journal` has no adapter). Previous: v0.9.27 (2026-08-02) — §3 gains the dated block for milestone 2b step 2's measured disk figure (v0.9.27 is milestone 2b step 2's **§32m-D per-tier disk figure, measured 2026-08-02 inside its 48-hour deadline**: the day is **11.165 GiB over 24 whole hours with all 40 sources `alive 60/60` and verify 0 integrity — BTC tier 4.314, alt tier 6.852, per shortlisted symbol 0.387 to 1.743**. §42c's derived ~0.6–1.1 G/day/symbol is refuted **at both ends and in its premise**: the venues' documented **update speed is a ceiling, not a cadence** — measured `pu`-chain breaks zero, minimum inter-push 102 ms, maximum silence 22.5 s in the hour checked and 94.8 s over the day — so every book leg is activity-driven and no symbol-independent floor exists. Books are **93.6%** of the burn, so the un-backfillable half is the whole cost, and **§38f's off-host destination is a live user decision again on a measured clock**: 85 days to a full local disk, the ~2026-09-30 gate at day 59 with ~678 GiB single-copy). Previous: v0.9.26 (2026-08-02) — **FORMAL MODEL (a)'s SLICE F: `FM-8`'s ruling is ENCODED AND CHECKED, and encoding it found `FM-9`** — §3 gains the `FM-9` traceability row and `FM-8`'s row gains its slice-F evidence. `NC21` (the pre-ruling world) **violates `INV9`**, `PC29` shows the new supersession row fires, `W5` shows the pre-ruling harm reaches a **nonzero spot sale**. **`FM-9`:** the v0.9.25 row named the §2 classifier's verdict as its trigger, which the generation's **own perp leg** moves by design — control `NC22` (the spec's own sentence, encoded) aborts a generation on a **physically flat** package **4 states from `Init` with no external event**. Fixed by evaluating the premise **net of the generation's own legs**; the ruling is untouched. `P6` converts that fix's own justification into a passing run over the complete graph, and `NC23` does the same for an argument this slice LOST — the standing obligation was first left merely admissible, with a reason, and the nominal liveness config refuted it. **Suite is 75 runs, ALL 75 MATCHING PREDICTIONS from a single unfiltered invocation, with all 40 probes still REACHED** — the standing guard cost no coverage. **The finding is tagged** (`[→ FM-9]`), so it lands in a Finding → resolution table and the **tagless total stays fifty-four**. Previous: v0.9.25 (2026-08-02) — **FORMAL MODEL (a)'s SLICE E IS COMPLETE, and it produced two further findings: `FM-7` and `FM-8`, both LANDED here (`FM-8` by user ruling; its model encoding is owed).** **`FM-7`:** SM §2's interval-floor rule *"never issued twice"* is **sound and stays** — but its stated justification (*"else, for `[0.4, 1.0]`, two uncertain `0.4` reductions could reach `−0.4` and defeat the proof"*) **assumes the reservation is NOT counted, which the adjacent sentence requires it to be.** Control `NC20` passes over the **complete** state graph (8,770,801 generated / 1,086,199 distinct / **0 left on queue**): double issuance cannot overshoot the sign while reservation-aware sizing is working. The rule is therefore recorded as **deliberate defence in depth over two independent mechanisms** — it is the one that survives a defect in the other — and the historical counterexample is retained as *what the rule protects against* (the `FM-5` world, an entry not backed at the instant it is read) rather than as a hazard reachable under the sizing this set mandates. It was fixed rather than left because **nobody re-derives a recorded justification**: a later reader checking the reason would find it does not hold, and the natural next move is to retire a rule that is doing real work. **`FM-8` — RULED by the user and LANDED here; only its MODEL ENCODING is owed (slice F):** §7's *"the package has no naked residual"* is enforced **only as an ENTRY GUARD**, and on a **reservation-netted** quantity — so probes `PC27`/`PC28` reach, at **3 and 4 steps from `Init`**, a **live `PackageDelever` generation over a package that is physically short and no longer even `Determined`, with nothing reserved against it.** The offsetting reduction's own ambiguous result releases its ledger entry exactly as SM §2 row 4 mandates; what is missing is anything that tells the **generation**. It is **`FM-5`'s mirror** (*an entry counted before its backing exists* / *an entry's premise outliving the entry*), and it is invisible to every invariant in the module because `INV5` polices the generation's **own legs** and never the premise it opened under. **The ruling is the STANDING PRECONDITION:** "no naked residual" is a standing condition, not only an entry test, so the classifier leaving `Determined` **supersedes** an active generation, quiescing its reserved leg to §6.7 finality first — a **third** row beside `EmergencyClosePackage` and `FenceReadyLost`, policed by the **same** quiesce invariant rather than a new one. The rejected alternative (tighten `PdPlan` to physical `realized = 0`) **moves the witness rather than removing the class** — a floor issued a step after the generation opens dissolves the premise identically — and is kept on record as an available hardening, not as a refuted idea. **What is NOT yet done, and is marked as such: the model encoding and its negative control (slice F, sketched in §46 §11f). The clause is normative from this release; the run that shows the hole closes has not been performed, and a fix argued is not a fix checked.** **Also this release: model (b)'s five negative controls were asked the witness-degeneracy question for the first time** — six probes (`FenceTakeover_W1`–`W6`), **all REACHED** — and the degeneracy was **systematic rather than four accidents**: every harm sits exactly one `VEntryFill` past the witness TLC reported, because breadth-first search always finds the cheap surrogate for exposure before exposure itself. It surfaced two suite defects invisible from a green run: **`I2_HostIndependentCoverage` had never been violated by any run in the suite** (listed in two configs, both halting on a shallower `I3` before the `I2` state exists to be found), and **`fence-takeover-model.md`'s own prose carried harm its runs never showed** — claims that were *true about the system*, which is exactly why four review rounds passed over them. Previous: v0.9.24 (2026-08-02) — **`FM-5` AND `FM-6`, formal model (a)'s fifth and sixth spec defects, are triaged and both normative edits have landed.** **`FM-6`** is the sharper of the two because the spec's own wording named it: §2 Scope said "classification is a function of **two inputs**", while the residual it gates on — `realized_delta − outstanding_reserved_reductions` — has **three**, and the missing one moves on events that are not order events. Its signature is a **dead end**, not a violated invariant. It is `FM-5`'s sibling (*the ledger must be backed when read* / *a change to the ledger must be observed*), and **neither was findable until the perp-side `Held/Alert` park was modeled**, since every other exit from a generation passes `PerpReduceFinal`, which reclassifies. And now `FM-5`, whose normative edit also landed (§3 gains the dated model-(a) block with FM-5's traceability row; §2 gains the companion artifact as model (a)'s review record). The finding: SM §7's FLAG-3 ruling composed with §2's unconditional reservation-aware sizing lets a `PackageDelever` generation book its **target** into the shared ledger before its perp leg has bought anything, so a **neutral** package reads as a **short** and §2's determined branch buys back exactly what that generation is about to buy. Resolution: FLAG-3 keeps its words — the entry is created at `Planned` as ruled, but carries the generation's **outstanding net intent** (zero until the perp leg confirms, amended as it fills). The finding is **tagged** (`[→ FM-5]`), so it lands in a Finding → resolution table and the **tagless total stays fifty-four**. **Also corrected here: this file's own Status line had drifted two versions behind** — v0.9.22 and v0.9.23 each added a dated §3 block and neither bumped it, which is the same class of drift as session 64's HANDOFF entry-point drift. The convention going forward is the one that already applies to every other doc in the set: **a change that adds a §3 block bumps this line in the same edit.** Previous (Status, unbumped through v0.9.22–v0.9.23): v0.9.21 (2026-08-01) — **milestone 2b step 1 is EXECUTED** (screening design record §43; ADR §3 gains the dated screening block): the screening ran over both venues' settled-funding history — 465/465 symbols, zero failures, 1.57M matched instants, zero pairing collisions — and the eight-symbol shortlist (LABUSDT through BCHUSDT, best-entry annualized net 71.6%/yr down to 1.5%/yr) plus the 2,972-event dislocation census are committed in `research/screening/` (raw bytes git-ignored, every SHA-256 pinned in the committed ledger). **No core normative change:** §12's milestone-2b row gains its dated step-1 outcome sentence (an outcome record, like the kill and ruling sentences beside it), so the tagless total **stays fifty-four** — the screening block below is a dated prose block like the scoping block it follows. Previous: v0.9.20 (2026-08-01) — **strategy #2 is SCOPED and §12 is RE-SEQUENCED** (scoping design record §42; ADR §3 gains the dated scoping block): milestone 2b — screening over backfillable settled-funding history + the dislocation census + the ~8-symbol four-stream capture expansion + the `strategy/crossvenue` read-path + strategy #2's own pre-registered gate — now precedes the OMS, with milestones 3–6 hard-gated on the gate and roster item 7 narrowed to the execution half. **One core normative change** (§12's roster); the ADR §3 tagless table is untouched, so the tagless total **stays fifty-four** — the scoping block below is a dated prose block like the kill and ruling blocks it follows. The four §42f forks were ruled the same day: shortlist ~8, broad tier deferred to the measured census, §38f re-deferred dated at the compressed clock, roster edit signed off. Previous: v0.9.19 (2026-08-01), the §12 post-kill fork RULED — (c), the alt/dislocation niche, strategy #2 early (calibration design record §41r; ADR §3's dated ruling block; no core normative change, total stayed fifty-four). Previous: v0.9.18 (2026-08-01), milestone 2's economic-gate kill (§41p/§41q; ADR §3's verdict block; §8's latency clause the one core edit, total stayed fifty-four).

**Previous (v0.9.16, 2026-07-30):** the parity wiring slice (§39j slice C closed): one core edit — §3.5's `Envelope.Validate` clause narrowed to `IngressSeq`, the absence door §39m-5 deferred now open at its construct — plus the Profiles instruments-info measurement, both reasoned in the parity dispatcher design record (§39p-5, §39q).
**Doc set:** normative core `build-spec.md` · venue facts `venue-profiles.md` · kernel machines `state-machines.md` · history/traceability `adr-history.md` · teaching companion `tutorial.md`.
**Role:** the convergence record — how the spec got here (review rounds, Codex convergence), plus the finding→resolution traceability that used to be the build-spec Appendix. Nothing here is normative on its own; the requirements live in the core.

## 1. Convergence narrative (v0.1 → v0.7.1)

The v0.7.1 status paragraph, frozen verbatim:

Draft v0.7.1 (2026-07-19) — Codex adversarial rounds 1–4 (11 → 6 → 4 → 4), then **two external human reviews (R1, R2) + a Codex convergence pass → v0.6**, then a **second external round (R1, R2) + 4 Codex convergence rounds → v0.7** (both reviews applied in full — near-zero false positives — plus 17 Codex convergence findings, all valid and applied; the **safety kernel** took most of them; new venue/tooling facts re-verified against official docs 2026-07-19). v0.7 hardens the **safety kernel**: the one-leg-failure handler now classifies residuals **determined vs ambiguous by *remaining executable quantity*** and quiesces-to-finality before compensating (a compensating trade against a `PartiallyFilled`/pending counter-leg could otherwise *create* the exposure it removes); a durable **`DispatchAuthorized`** crash boundary makes recovery query-only; rescue margin under Isolated is an **active `add-margin` top-up** with a **`PackageDelever`** fallback; a per-venue **`Fence`** capability (with signer isolation pulled to `$1K` where exchange-side fencing is unavailable); **one-way position-mode + leverage** posture verification; an **operator control-plane security** contract; a hardened, **calibration-artifact-gated** economic bound; **raw-arrival** (not `SourceTime`) event ordering; and SQLite **WAL + `synchronous=FULL`** durability. **Deliberately not "converged":** implementation-ready *by capital tier* (§1.1) — **foundation + read-path + edge-proof (milestones 1–2) are fully build-ready**; the **off-host actuator *and* the ambiguous-order-recovery state machine are the safety kernel** (§6.7, §7.4): the ambiguous-order machine is **model-tested *within* milestone 3 before live capital**, and the takeover is **prototyped + chaos-tested in its later actuator workstream** and proven in a **sub-`$1K` canary** — an explicit acceptance of bounded loss (§12) — before it gates real size. Before real capital: an empirical testnet liquidation test of the §4.1 posture. **v0.7.1** applies a **third external round (R1c, R2c) + a 5-round Codex convergence pass (→ converged)** — refinements/corrections on the v0.7 architecture, not a new redefinition: liq-distance is **mark-price-driven** (recomputed on every fresh mark, not only account events) with a **stale-mark block/escalate** `[→ R2c-1]`; a continuously-monitored, **differentiated `FenceReady`** health gate whose loss drives a **bounded `FenceReadyLost` wind-down (or actuator takeover if the signer itself failed)** within an RTO `[→ R2c-2, Cx4-5]`; first-class safety-capability **ports** (`MarginGateway`/`FenceGateway`/`ProtectionGateway`/`EmergencyPlanStore`; `TransferCollateral` gains submit-unknown semantics) `[→ R2c-3]`; an explicit **three-failure-domain topology** (trading engine ∥ operator/control ∥ a minimal actuator daemon with no dashboard/HTTP surface) `[→ R2c-4]`; the **dispatch sequence decoupled** from raw-arrival order (no cross-source head-of-line blocking; held/coalesced inputs need no 1:1 mapping) `[→ R2c-6]`; **signer isolation elevated to the primary `Fence`** (exchange-side disable the backup) with the Bybit fence credential's unavoidable **transfer-class authority** stated honestly `[→ R1c-2]`; **interval-floor** compensation in the deadline-exceeded ambiguous branch `[→ R1c-4]`; a **quantified canary loss bound** `[→ R2c-8]`; real **backup/anchor/retention/restore-drill** durability requirements `[→ R2c-9]`; a **near-risk-free, dated/haircut/revision-controlled hurdle** `[→ R2c-10]`; and tutorial + provenance corrections `[→ R2c-5, R2c-11]`. R2's two canary blockers (mark-driven liq, quantified loss) and two `$1K` blockers (incident-available fence, explicit safety-plane topology) are **closed at the design layer here**; their residual gating is now implementation + chaos-test, as the spec brackets it. Open: a normative/ADR/tutorial **doc split** (both R2 rounds — now escalated from optional to recommended) and a "what changed" tutorial appendix.

## 2. Review records

- `docs/formal/ambiguous-order-model.md` — **formal model (a)** (TLA+ §2 classifier + finality contract composed with §7's reservation ledger): the companion artifact, mapping every clause of both machines' Invariants sections to a construct or to a stated omission. **`FM-5` → v0.9.24.** Build record: design record §46 (`docs/superpowers/specs/2026-08-01-formal-model-a-ambiguous-order-design.md`) slices A–E. **Read the companion before the `.tla`.**
- `docs/review/2026-07-20-10-v0.9-formal-model-b-FM-1-record.md` — **formal model (b)** round (TLA+ fence/takeover/plan composition): FM-1 finding + resolution → v0.9.1; **its Cx6 addendum (same file): the Codex adversarial round on the model + FM-2 → v0.9.2**; **its Cx7 addendum: the round-2 *attempt* + FM-3 + Cx7-1/-2/-3 → v0.9.3 (read that addendum's Provenance section first — the Codex dispatch died on an upstream outage and the round was completed by self-probing)**; **its Cx8 addendum: the genuine independent round-2 — NO-SHIP, FM-4 + Cx8-2/-3, both findings counterexample-backed, the re-review obligation now discharged → v0.9.4**; model + results: `docs/formal/`.
- `docs/review/2026-07-20-09-v0.8-round4-R1d-review-and-resolution-record.md` — **round-4 (R1d)** findings + triage + venue-verification annexes → v0.9 (R2d pending).
- `docs/review/2026-07-19-08-v0.7.1-review-and-verification-record.md` — round-3 (R1c/R2c) findings + resolutions + Codex convergence pass.
- `docs/review/2026-07-19-07-v0.7-review-and-verification-record.md` — round-2 (R1/R2 second round) findings + resolutions + venue/tooling verification citations (§2 there).
- `docs/review/2026-07-18-06-v0.6-review-and-verification-record.md` — v0.6 round.
- `docs/review/2026-07-18-05-tutorial-critical-review.md` — the earlier critical review the spec resolves.
- `docs/legacy/` — frozen research record (the "why"). Do not edit.

## 3. Finding → resolution traceability

| Finding / Decision | Resolved in |
|---|---|
| P1 spread-vs-level | core §1, core §2.1–2.3 |
| P2 hurdle rate | core §2.1 |
| P3 ADL (realized hedge-failure) | core §2.1, core §7.1 (naked-leg-gap sizing) |
| P4 build-vs-buy | core §11 |
| C1 cost single-source | core §2.4 |
| C2 discrete settlement | core §2.4, core §6 |
| C3 drift-based min-edge | core §2.4 |
| C4 tiered MMR | Profiles §1, core §7.1 |
| C5 buffer policy | core §5.2 |
| C6 inverse perps | core §1, core §4 |
| C7 basis in funding audit | core §6 |
| C8 Bybit dynamic settlement | core §2.4, Profiles §1, core §6, core §4 (v0.9.9 — the interval is refused rather than defaulted at the venue seam, so a flip cannot be absorbed by a fallback) |
| A1 venue adapter | core §3.2, core §4 (segregated capabilities) |
| A2 multi-region / split-ready | core §3.7, core §11 |
| A3 durable state / reconcile-only boot | core §7.5, core §6.10 |
| A4 exactly-once funding | core §5.1 (UNIQUE + ledger txn ID) |
| A5 kill-switch modes | core §7.4 |
| S1 custody/OES | core §7.2 |
| S2 key security | core §7.3 |
| S3 audit durability | core §3.2, core §7.5 (append-only projection) |
| S4 escalation ladder | core §7.4 |
| B1 backtest fidelity | core §3.5, core §8 |
| B2 test taxonomy | core §9 |
| D1 topology (in-process engine) | core §3.0, core §3.1, core §3.7 |
| D2 durability (journal + outbox → embedded store) | core §3.0, core §5, core §6, core §7.5 |
| D3 money (`platform/fixedpoint`) | core §3.0, core §3.6, core §4, core §5.1, core §11 |
| D4 clock (parity dispatcher + server-time) | core §3.0, core §3.5, core §3.6 |
| **X1 durable order/account truth (single-writer OMS)** | core §3.0, core §3.2, core §4 (`OrderByClientID`/`Fills`), core §5.1, core §6 |
| **X2 ADL/margin controls + account contract** | core §2.1, core §4 (`AccountSource`), core §6.8, core §7.1 |
| **X3 Trade ownership + exclusive lease** | core §1, core §3.2, core §5.1 |
| **X4 bus back-pressure / delivery lanes / freshness** | core §3.4 |
| **X5 DDD boundary (strategy input, venue ACL)** | core §3.2, core §3.3, core §3.6 |
| **X6 pre-authorized (not human-gated) emergency** | core §7.4 |
| **X7 fixedpoint intermediates + dimensional types** | core §3.6, core §4, core §5.1 |
| **X8 clock total-order parity + server-time** | core §3.5, core §3.6, core §6.3 |
| **X9 DedupeKey → UNIQUE + ledger txn ID** | core §5.1 |
| **X10 key blast-radius = whole subaccount** | core §7.3 |
| **X11 paper acceptance-gate pass/fail** | core §9 |
| **Y1 fill dedup identity (`ExecutionID` + `UNIQUE`)** | core §4, core §5.1, core §5.3 |
| **Y2 `Place` non-idempotent / `SubmissionUnknown`** | core §4, core §5.3, core §6 |
| **Y3 `ExposureReduction` command (executable ADL path) + actuator failure domain** | core §5.3, core §6, core §7.4 |
| **Y4 replay/total order = `DispatchSeq`/`IngressSeq` (the consumed order); `AcceptOrdinal` = raw-arrival capture/audit (SourceTime → analysis only); deterministic coalescing** | core §3.5 |
| **Y5 funding fallback key uses symbol + ledger-entry type** | core §5.1 |
| **Y6 acceptance-gate numeric thresholds** | core §9 |
| **Z1 canonical `VenueInstrument` identity (spot ≠ perp; independent ID namespaces)** | core §3.3, core §3.4, core §3.5, core §4, core §5.1, core §5.3 |
| **Z2 orthogonal fill ledger + race-complete state machine + immediate-flatten** | SM §1, SM §2 |
| **Z3 mandatory emergency-actuator failover (venue-native baseline / replicated `EmergencyPlan`)** | core §7.4 |
| **Z4 single reconciliation truth (journal, not the audit projection)** | core §3.2, core §7.5 |
| **— External human review (R1, R2) → v0.6 —** | |
| **R1-F1 / R2-#2 account & margin architecture (boot-verified posture)** | core §4.1, core §2.1, core §7.1, core §7.5, core §9 |
| **R1-F2 / R2-#4 protection lifecycle: disarm + actuator MVP-mandatory on Binance** | Profiles §1 (attach row), SM §5, core §7.4, core §11, core §12 |
| **R1-F3 / R2-#3 forecasting model + total-carry + exit hysteresis** | core §2.2, core §2.4, core §3.3, core §5.2 |
| **R1-F4 cash-and-carry kill-math (break-even, utilization, maker lever)** | core §2.4, tutorial §3.9 |
| **R1-F5 spot-leg surface + per-(venue,API-family) rate limits** | Profiles §3, Profiles §1, core §7.6 |
| **R1-meta capital-gate tiering** | core §1.1, core §7.4, core §9, core §12 |
| **R1-meta solo-honest controls (time-delay, out-of-band, dead-man)** | core §7.4, core §5.2, core §9, core §3.2 |
| **R2-#1 Bybit order-history recovery fallback (realtime cache cleared on restart)** | Profiles §1, SM §1, core §12 |
| **R2-#5 two authorities named (venue = reality; journal = intent/allocation)** | core §3.0 |
| **R2-#6 `DispatchSeq`/`IngressSeq` = replay authority (dense, no "ties"); the earlier "arrival order = replay authority" formulation is superseded by R2c-6/Cx4-1** | core §3.5 |
| **R2-#7 coalescing only on reconstructed books; reconstruction ownership** | core §3.4, core §11 |
| **R2-#8 Intent idempotency (IntentID/Revision/ValidUntil)** | core §3.3 |
| **R2-#9 funding = venue-ledger amount (accounting truth); rate×mark = audit** | tutorial §3.8, core §5.1 |
| **R2-#10 money: compile-time category vs runtime unit check** | core §1, core §3.6 |
| **R2-#11 durability + local-journal-loss coverage** (fullfsync → WAL+`synchronous=FULL`, see R2b-9) | core §3.0, core §11 |
| **R2-#12 acceptance gate = operational; economic gate is separate** | core §9 |
| **R2-#13 milestone reorder — prove the edge before the OMS** | core §12 |
| **R1/R2 tax classification is downstream policy (emit raw events)** | core §10 |
| **R1 journal tamper-evidence (hash-chain the journal, not only audit)** | core §5.1 |
| **R1 `VenueInstrument` in normative signatures; `Account` = subaccount** | core §4, core §5.1 |
| **R1 rescue-margin float counts toward per-venue cap** | core §7.2 |
| **R1/R2 worked numerical trade ledger + crash/partial-fill timeline** | tutorial §3.9 |
| **— Codex convergence pass (v0.6) —** | |
| **Codex-C1 $1K host-death: off-host executor + two-leg plan, not deferred** | core §1.1, core §7.4, core §12 |
| **Codex-C2 ADL rehedge: `reduceOnly` by instrument position, not package delta** | SM §2, core §7.7, core §9 |
| **Codex-M3 disarm fenced state machine (`DisarmPending` + late-fill sweep)** | SM §5, core §7.7, core §9 |
| **Codex-M4 forecast/exit testable contract (quantile, horizon, dwell, basis default)** | core §2.4, core §5.2 |
| **Codex-M5 `C_allocated` = full-bankroll denominator (rescue + reserves)** | core §2.1, core §2.2, core §4.1 |
| **Codex-M6 posture re-verified pre-trade (`AccountPostureSource`); UTA-status fix** | core §4, core §4.1, core §6.3, core §7.5 |
| **Codex-M7 economic gate tiered (conservative $1K bound)** | core §8, core §9, core §12 |
| **Codex-m8 `AcceptOrdinal` = raw-arrival capture/audit stamp (the historical "final tie-breaker" framing is superseded by R2c-6/Cx4-1 — `DispatchSeq` is the dense replay key)** | core §3.5 |
| **— Codex round 2 (convergence) —** | |
| **Cx2-Blocker actuator takeover = fence-first, reduce the *reconciled residual* not allocated qty** | SM §4, core §7.7, core §9 |
| **Cx2 §6.7 reduction delegates to the §5.3 state matrix (reduce-only-by-position)** | SM §2 |
| **Cx2 forecast `HoldPlan` + joint-aggregation rule + one-unit normalization** | core §2.4, core §5.2 |
| **Cx2 $1K economic bound defined (dataset/slippage/threshold) + explicit m2 deliverable** | core §8, core §9, core §12 |
| **Cx2 `AcceptOrdinal` single stamping point; $1K/$10K actuator tier language split** | core §3.5, core §7.4 |
| **— Codex round 3 (verification) —** | |
| **Cx3-Blocker takeover must quiesce ALL resting orders (algo/TP-SL/OCO) to finality first** | SM §4, core §9 |
| **Cx3 `EmergencyPlan` replicates full recovery handles (client/algo IDs, cursors, `TradeID`)** | SM §6 |
| **Cx3 exit = forward continue-vs-close score (entry costs are sunk, not re-charged)** | core §2.4 |
| **Cx3-meta off-host actuator = bounded subproject; sub-$1K canary de-risking path** | core §7.4, core §12 |
| **— External human review round 2 (R1, R2) + 2 Codex convergence rounds → v0.7 —** | |
| **R2b-B1 ambiguous-vs-determined residual by *remaining executable quantity*; quiesce-to-finality before compensating (fixes partial-fill/pending-leg exposure creation)** | core §5.3, SM §2, core §6.4, core §7.4, core §9, tutorial §3.9 |
| **R2b-B2 canary = acceptance of bounded loss, not validation of an unbuilt path; Bybit create = async ack; spot-IOC bracket = integration-test item** | core §12, core §7.4, Profiles §1 |
| **R1b-1 rescue margin under Isolated = active `add-margin` top-up (not passive buffer); `PackageDelever` fallback; isolated-vs-cross trade-off** | core §7.1, core §4.1, SM §7, core §9 |
| **R1b-2 / R2b-3 per-venue `Fence` capability (verified endpoints/authority) + fence-unavailable branch + credential blast radius; signer isolation binds at `$1K` where exchange-side fencing unavailable** | SM §3, SM §4, core §7.3, core §1.1, core §12, core §9 |
| **R1b-3 §1.1 ↔ §7.4 attach-coverage alignment (actuator mandatory for any leg lacking *verified* attach, incl. Bybit spot IOC); §7.7 diagram fix** | core §1.1, core §7.4, core §7.7 |
| **R1b-4 exit score charges closing cost only where branches differ (no premature-exit bias)** | core §2.4 |
| **R1b-5 posture freshness = max-age TTL + pre-trade REST re-verify (position-mode changes emit no venue event)** | core §6.3, core §7.5 |
| **R1b-6a `holdFraction` defined; R1b-6b tutorial pre-ship checklist (posture assertion, tiered signer isolation); R1b-6c market-data capture-lane + losslessness discipline** | core §2.4, tutorial §9, core §3.5 |
| **R2b-4 crash boundary `DispatchAuthorized` (query-only recovery for every pre-ack state)** | SM §1, core §6 |
| **R2b-5 one-way position mode + configured leverage in posture verification (boot/pre-trade/on-change)** | core §4, Profiles §2, core §6.3, core §7.5 |
| **R2b-6 operator control-plane security contract (mTLS/authz/replay/CSRF/network/audit)** | core §7.3, core §12 |
| **R2b-7 economic gate hardened (depth-walk, latency, per-regime + rolling, drawdown, LCB) + versioned calibration artifact (not author-selectable)** | core §8, core §12 |
| **R2b-8 tutorial worked-trade denominator = `C_allocated` (not `C_legs`)** | tutorial §3.9 |
| **R2b-9 SQLite durability = WAL + `synchronous=FULL` on Linux (fullfsync is macOS-only); local fs only** | core §3.0, core §11, Profiles §4 |
| **R2b-10 shared IP-ban safety envelope default (separate product weight/order counters)** | core §7.6, Profiles §3 |
| **R2b-11 raw arrival (`AcceptOrdinal`) = capture/causal order (NOT the replay authority — see R2c-6); `SourceTime` → freshness/analysis only; capture-lane persistence** | core §3.5 |
| **R2b-12 fill-ledger admission unbounded for known orders (finality window bounds only ambiguous-acceptance resolution)** | SM §1 |
| **R2b-13 tutorial ADL re-hedge = ordinary non-reduce-only short** | tutorial §6.2 |
| **— External human review round 3 (R1c, R2c) + Codex convergence → v0.7.1 —** | |
| **R2c-1 liq distance is mark-price-driven (recompute on every mark + margin/position/tier change); stale mark blocks openings + escalates** | core §2.2, core §6, core §7.1, core §9, tutorial §6.2 |
| **R2c-2 `FenceReady` continuous health gate + proactive wind-down (bounded `FenceReadyLost` RTO — differentiated in Cx4-5); incident branch = last-resort residual only** | core §1.1, SM §3, core §4, core §9 |
| **R2c-3 first-class safety ports (`MarginGateway`/`FenceGateway`/`ProtectionGateway`/`EmergencyPlanStore`); `TransferCollateral` leaves `AccountSource`, gains submit-unknown semantics** | core §4 |
| **R2c-4 three-failure-domain topology (engine ∥ operator/control ∥ minimal actuator daemon, no dashboard/HTTP); "single process" = the engine only** | core §3.2, core §11 |
| **R2c-5 tutorial corrected to the safety kernel (top-up→`PackageDelever`; determined/ambiguous ladder; split <5 s / <30 s; `DispatchAuthorized` not IDs-alone)** | tutorial §3.6, §6.2, §6.5, §7.7 |
| **R2c-6 dispatch sequence decoupled: `IngressSeq`/`DispatchSeq` (replay key, consumed order) ≠ `AcceptOrdinal`/`RawIngressSeq` (raw-arrival causal); no cross-source head-of-line blocking; explicit mapping** | core §3.5 |
| **R2c-7 `EmergencyClosePackage` rationale fixed (CloseTrade is not `stop-opening`-subject; justification = foundation authority + delta≈0 no-op)** | core §5.3 |
| **R2c-8 canary loss = written risk-acceptance spec (max notional/collateral; worst-case = uncovered-leg notional; BTC-only; manual-recovery RTO; independent monitoring; signed acceptance)** | core §12, core §7.4 |
| **R2c-9 durability backup/anchor made real (SQLite online-backup / `VACUUM INTO`, not raw copy; RPO/RTO; encryption; external anchor; restore-drill gate)** | core §11 |
| **R2c-10 hurdle = near-risk-free, dated/sourced/haircut-adjusted, revision-controlled (not a hardcoded 4.6%, not "risk-free")** | core §2.1, core §8 |
| **R2c-11 provenance corrected (4 Codex rounds; verification dates)** | HANDOFF, review records, tutorial |
| **R1c-1 tutorial top-up fallback = `PackageDelever` (was `ExposureReduction`) — folded into R2c-5** | tutorial §6.2 |
| **R1c-2 fence credential holds unavoidable transfer-class authority on Bybit (grant narrowest — `Sub Member Transfer`, never `Withdrawal`); signer isolation elevated to primary `Fence`** | core §7.3, SM §4 |
| **R1c-3 sub-account availability is a verify-item (tier-gated on Binance); single-account fallback + signer-isolation fence** | core §7.2, SM §4 |
| **R1c-4 interval-floor compensation in the deadline-exceeded branch (compensate the residual invariant across the ambiguity; only when the interval doesn't straddle zero)** | core §6.7, SM §2, SM §3 |
| **R1c-5 strategic fork if the milestone-2 gate fails (maker entry / regime-selective / alt niche / tuition); mechanically-derived thresholds = solo default** | core §12, core §8 |
| **— Codex convergence (v0.7.1) —** | |
| **Cx4-1 replay/total-order language unified: `DispatchSeq`/`IngressSeq` = the sole replay authority; raw `AcceptOrdinal` = capture/audit only; parity = replay the persisted dispatch log (not regenerate from raw capture)** | core §3.5, ADR §3 |
| **Cx4-2 interval-floor = a reserved `IntervalFloorReduction` generation (counted in `outstanding_reserved_reductions`, no blind retry, own ambiguous result routes through finality) — never issued twice; "no compensation before finality" reworded to "uncertain part"** | SM §2, core §9 |
| **Cx4-3 mark-risk = OMS-owned read port over `engine/state`; adverse-preserving mark coalescing (worst-adverse never dropped); per-venue mark max-age + `Clock`-timer staleness-on-silence** | core §3.4, core §6, core §7.1 |
| **Cx4-4 signer isolation pinned to its own actuator-fenceable failure domain (authenticated protocol + health + revoke; not bot-co-located); "no venue credential" corrected to "no additional key-management/transfer-class credential"** | core §7.3, SM §4, core §11 |
| **Cx4-5 `FenceReadyLost` arbitration mapped to commands (block OpenTrade; supersede delever; `EmergencyClosePackage`/`ExposureReduction` via §6.7; overrides hard-halt); "never outlives its fence" → bounded wind-down + RTO** | SM §3 |
| **Cx4-6 parity seam `ExecutionVenue` gains `MarginGateway`+`ProtectionGateway` (same safety code in backtest/chaos); `EmergencyPlanStore` gains `ListActive` / generation-CAS / `Deactivate` + actuator reconcile-only boot** | core §3.5, core §4, SM §6, core §9 |
| **Cx4-7 deployment non-goals + tutorial qualified: "single process" = the trading engine only; operator + actuator are separate required domains** | core §1, core §11, tutorial §7.3 |
| **— External human review round 4 (R1d) → v0.9 —** | |
| **R1d-1 takeover terminal state = flat: state-dependent end action (fixed residual → `ExposureReduction`; neutral package → two-leg close within plan bounds — post-quiesce a package would be unprotected/unmanaged)** | core §1.1, core §7.4, core §7.7, core §9, SM §4 |
| **R1d-2 interval-floor pre-action capital bound = `max(\|a\|,\|b\|)` (the `\|b\|` phrasing was net-long-only; a net-short interval's worst exposure is `\|a\|`); §9 overshoot assertion generalized to "opposite sign"** | core §6.7, core §9, SM §2 |
| **R1d-3 classification re-runs on every input-changing event: fill/position (realized delta) + order-lifecycle/finality (possible-delta intervals — cancel ack, terminal status, sweep/watermark completion, finality timer, query result); never waits for the next fill/position event** | core §5.3, core §6.7, SM §1, SM §2 |
| **R1d-4 actuator execution authority made coherent: "no venue credential" shorthand fixed (fence *action* needs none; the signer holds the trade key); `ActuatorReady` (actuator's own independent trade credential + reachability + plan-store integrity) added as a monitored dimension + branch-2 precondition + blast-radius entry; durably-fenced reclassified as incident/takeover state** | core §1.1, core §7.3, core §7.4, core §9, SM §3, SM §4 |
| **R1d-5 plan generations: replication = freshness, never coverage — prefix-complete discovery from the acked entry generation (namespace-derived IDs + venue enumeration + cursor sweep); per-generation ack rejected for hot-path handles (would couple reductions to plan-store availability); policy-changing generations require ack before effect** | core §7.4, core §9, SM §6 |
| **R1d-6 margin ops are their own durable target-state machine: journaled `MarginOp` (local op ID, intended delta, before-state + watermark) fsynced before dispatch; resolution by per-venue evidence (Binance `positionMargin/history` symbol+type+amount+window match; Bybit `positionIM` delta vs before-state — its transaction-log documents NO margin type; Bybit transfers by client `transferId` UUID, Binance transfers read-back before retry), never OrderByClientID-like point lookup, never blind delta retry; boot resolves pending margin ops** | core §4, core §7.1, core §7.5, core §9, SM §7, Profiles §4 |
| **R1d-7 Bybit forced-close evidence corrected (no `closeType` on order/execution streams): normative source fields, dedup, and recovery query per verified schemas** | Profiles §1, Profiles §4, core §6.8 |
| **R1d-8 canary sequencing made implementable: actuator/`Fence`/takeover = canary-concurrent workstream; `$1K` promotion (not canary start) gates on the §9 pass; pre-`$1K` de-minimis actuator action conditioned on the actuator existing; "isn't built yet" corrected to "not yet built or not yet §9-proven"** | core §7.4, core §12, SM §3 |
| **R1d-9 stale-mark escalation defined: REST-mark/`MarginStatus` failover on the reserved rate-limit budget; `T_blind` + `mark_blind_action` (`delever`\|`close`) pre-authorized de-risk within a bounded response time — never an alert alone** | core §2.2, core §5.2, core §6.8, core §7.1, core §9, SM §7, tutorial §6.2 |
| **R1d-10 tutorial re-synced: t₅′ teaches `AbsentAfterFinality` (not "stable Rejected/not-found"); §3.7 monitoring leads mark-driven; Part 9 signer-isolation gate matches the core's `$1K` fence floor** | tutorial §3.7, §3.9, Part 9 |
| **R1d-11 governance: SM sections = normative mechanism incorporated by reference (core wins on conflict); tag conventions cover all lettered rounds + informal provenance tags; Profiles citation coverage stated honestly + ellipsis URLs expanded** | core header, SM header, Profiles header/§4 |
| **R1d-F1…F7 rulings on the seven v0.8 FLAGs — all adopted (details: ADR §4 list below)** | SM §1, SM §3, SM §6, SM §7, core §5.3, core §7.4 |
| **— Codex convergence (v0.9) —** | |
| **Cx5-1 takeover completion is evidence-gated, never issuance-gated: `CloseInFlight` awaits §6.7 finality + zero reconciled per-leg allocations + final surfaces (= the §6 tombstone evidence); re-attempts consume a replicated **close-attempt budget** (`max_close_attempts` + `T_close`, plan fields — round-2 refinement: the aggregate bound is a consumable budget, not a per-attempt adjective), exhaustion → `HeldIncident` unconditionally (fence latched, plan active, operator escalated) — never a false "flat"** | core §1.1, core §7.4, core §9, SM §4, SM §6 |
| **Cx5-2 core gains the 1→3 escalation (signing dies mid-wind-down with no latch or `ActuatorReady`) — branch 2 iff latch + `ActuatorReady`, else branch 3; core/SM/§9 now agree** | core §7.4, core §9, SM §3 |
| **Cx5-3 prefix-complete discovery made real: `HistoricalOrders` port (normal + algo history, server↔client ID mapping, end watermark); completeness is retention-bounded (Binance ≈90d orders / ≈6mo trades, no server-side prefix filter) → periodic acknowledged checkpoint generations (ID map + cursors) keep discovery inside retention** | core §4, core §7.4, SM §6, Profiles §4 |
| **Cx5-4 `MarginOpStatus` gains first-class `Ambiguous` (evidence ≠ causal identity); same-position margin ops serialized; on `Ambiguous`: no delta retry, openings blocked, live-state safety continues, explicit float reconciliation** | core §4, core §7.1, core §7.5, core §9, SM §7 |
| **Cx5-5 residual "no venue credential" shorthand purged (HANDOFF + SM tiering quote)** | HANDOFF, SM §4 |
| **— Formal model (b) counterexamples (v0.9.1; `docs/formal/`) —** | |
| **FM-1 takeover completion evidence terminalizes the trade: once `ActuatedFlat` exists (or any flatten reaches §6.7 finality with flat evidence) the trade re-admits no exposure-opening order — plan may only proceed to the tombstone; fresh exposure = successor trade via the lease. Closes the model-checked composition hazard (R1d-F7 re-drive × R1d-F6 clearance × §6 ack-before-entry): clearance in the `ActuatedFlat`→`Deactivate` crash window + same-plan re-entry → the stale re-driven tombstone revokes host-independent coverage under live exposure. Tombstone-as-clearance-precondition rejected (plan-store coupling, the R1d-5 argument)** | core §7.4, SM §3, SM §4, SM §6 |
| **— Codex convergence round on the formal model (Cx6) + its liveness-debug finding (v0.9.2; `docs/formal/`) —** | |
| **Cx6-1 (BLOCKER) the model's branch-3/branch-2 encodings were not write-gated: a signer restart re-enabled automated bot closes + protection cancels inside `ManualOnly`/`TakeoverTransfer` (contradicting "bounded pre-armed protection + operator manual kill only" and single-writer-by-transfer) — bot venue writes now gate on the branch; the atomic `OpManualKill` also erased the real cancel-vs-fill UI race — split into non-atomic flatten + cancel-all with **over-close fill semantics** (a close filling on an already-flat package creates reverse exposure)** | model only (`docs/formal/`; SM §3/§4 already normative on both points) |
| **Cx6-2 `ActuatorReady` constrains the whole takeover, not just the branch-2 hand-off: the watchdog-path latch requires it confirmed (else the attempt times out to `FenceUnavailable` — fencing with no execution capability transfers the writer to nobody); every actuator venue write (quiesce cancels included) requires execution readiness; **mid-takeover `ActuatorReady` loss → `HeldIncident`** (fence latched, plan `Active`, operator escalated) — new SM §4 edge + core §7.4 clause** | core §7.4, SM §4, `docs/formal/` |
| **Cx6-3 operator clearance is health-gated: a fresh propagation test cannot pass while signing / primary-revoke / `ActuatorReady` is failed (those dimensions are what it exercises), and clearing with only the backup still failed returns to `FenceDegraded`, never straight to `FenceReady`** | SM §3, `docs/formal/` |
| **Cx6-4 the model's `I1` single-writer instrumentation now observes every actuator venue write — the four quiesce-cancel actions record unfenced landings exactly like closes (an unfenced cancel is a two-writer event too)** | model only (`docs/formal/`) |
| **Cx6-5 liveness de-masked: a strongly-fair operator manual kill could rescue any stalled branch, so the single liveness property proved only "operator clearance eventually happens" — split into the operator-assisted claim (`IncidentResolves`) and an automated-convergence config (`OPERATOR_INTERVENES = FALSE` + SF fills) checking the §9 branch-1/2 composite-guarantee shape (`Branch12AutoResolves`: automated flat or an honest escalation terminal, no silent stall); both claims conditioned on an eventually-stable environment (`EnvStabilizes` — no fairness scheme can force progress through guards an anti-phase flapping environment keeps permanently unaligned)** | model only (`docs/formal/`) |
| **Cx6-6 (minor) recorded TLC state counts were stale against the reviewed revision — counts + revision identifier refreshed in the correspondence doc and the FM-1 record** | `docs/formal/`, review record |
| **FM-2 false-alarm dismissal: a watchdog fire that latched nothing and started no branch (bot verified alive/healthy, actuator stood down) is operator-DISMISSED as an alert — the R1d-F6 branch-exit preconditions (terminal outcome + clearance + fresh propagation test) are scoped to a *started* branch and do not apply (else acknowledging a false alarm demands flattening a healthy trade). Root cause of the post-Cx6 liveness lasso: with branch clearance as the only exit, a spurious-fire incident had no reachable resolution while the un-fenced healthy bot's obligated naked-reduce resubmissions raced the operator's non-atomic manual kill (every flatten over-closed by a live close's late fill). The model also gains the operator's kill-switch trip (`OpTripHalt` — core §7.4 modes (a)/(c), the §7.3 control plane's first power; halt-first is what ends the operator-vs-bot two-writer race), previously omitted entirely** | core §7.4, SM §4, `docs/formal/` |
| **— Round-2 re-review of the formal model (Cx7) + its spec finding (v0.9.3; `docs/formal/`) —** | |
| **FM-3 engagement is a durable obligation, not a delivered message: an actuator that boots (or stands down) into an incident whose §3 branch is `TakeoverTransfer` with an active plan and NO latched epoch MUST re-attempt the fence latch, keying off the durable branch record rather than a received hand-off signal (lost hand-off ≡ crashed actuator); the re-attempt is `T_fence`-bounded to the `FenceUnavailable` floor as usual. Closes the model-checked stall: branch 2 engages durably (one-way, R1d-F6), the actuator crashes pre-latch, and the Cx6-1 write gate — scoped to the *branch*, not to the failed dimension — permanently bars the healthy bot → unprotected exposure with both writers alive and idle. Stated corollary: a branch that transfers the writer MUST guarantee the transferee engages. Alternatives rejected: watchdog re-arm (moves stalled-takeover detection into the read-only watcher); bot-side re-drive (host dependency inside the takeover's own recovery path)** | core §7.4, SM §3, SM §4, `docs/formal/` |
| **Cx7-1 (MAJOR, model fidelity — SM §4 already correct) the takeover recorded completion without execution capability: `ActQuiesceDone`/`ActReconcileDone`/`ActMarkFlat`/`ActDeactivate` were ungated on `ActuatorReady` and `ActMarkFlat` accepted `held`, so `held → finalize → tombstone` escaped the Cx6-2 park and asserted a plan-store write while plan-store integrity was the failed dimension. Gated + `held` removed, **plus** the park's other spec-stated exit implemented (`ActHeldReentry` — reconcile-only re-entry once ready again), without which sealing the escape would create a liveness dead end. One clarifying (non-normative) sentence added to SM §4: the park is not exited by *recording completion*** | model only + SM §4 clarification (`docs/formal/`) |
| **Cx7-2 (MAJOR, model/property) `Branch12AutoResolves` certified FM-3's stall as "resolved": its pre-latch stand-down disjunct rested on "the healthy bot remains the writer" — a premise Cx6-1 falsified in the same round — and was additionally vacuous for branch 1 (it holds on entry to every wind-down). Disjunct now conditioned on `fenceCtl ∉ {manual, takeover}` **and** the property split into `Branch1AutoResolves` + `Branch2AutoResolves` (conditioning alone leaves branch 1 vacuous)** | model only (`docs/formal/`) |
| **Cx7-3 (MINOR, verification gap) nothing in `I1`…`I7` tied a completion transition to execution capability — why 7/7 green missed Cx7-1. `P1_NoCompletionWithoutCapability` (action property, scoped to the *edge* into `finalize`) added to the nominal safety config; `P2_NoBranch2Stall` added to the automated-convergence config as FM-3's regression check** | model only (`docs/formal/`) |
| *(Cx7-4 considered and **REJECTED**: `OpDismissFalseAlarm` is not health-gated, unlike Cx6-3 clearance — but a dismissal into an unhealthy state is re-raised by the WF-fair `CtlLoss`, and the asymmetry is justified: clearance exits a **started branch** and asserts `FenceReady`; dismissal exits an incident that started nothing and asserts nothing. Recorded so a later round does not re-raise it.)* | — |

**Cx8 — the genuine round-2 review of the model (v0.9.4, 2026-07-25).** The independent re-review Cx6 asked for and Cx7 could not deliver: a fresh Codex `artifact-review` that ran the suite, wrote its own TLA+ probe module, and returned **NO-SHIP with two MAJOR findings, both counterexample-backed**. Both valid, both applied. Record: the **Cx8 addendum** in `docs/review/2026-07-20-10-…`.

| Finding → resolution | Where |
|---|---|
| **FM-4 the arbitration branches on the RECORDED failure, not on live health: which dimension failed is snapshotted when the loss is detected, so a dimension that repairs inside the detect→select window does not abort a *pending* branch — the one-way rule binds from detection, not from selection. The 2-vs-3 choice still reads live capability (latchable + `ActuatorReady` confirmed is a claim about acting *now*). Closes the model-checked strand: with live-health guards a self-healed loss disables ALL THREE branch guards at once (branch 1 needs a live failed dimension, branches 2/3 need live signing loss), leaving the incident in un-arbitrated `FenceReadyLost` — openings blocked, exposure open, no branch, no automated exit. It also disproved the model's own SF→WF soundness argument, which rested on branch guards being *persistent* under `EnvStabilizes`; that persistence is now true by construction, since branches 1/2/3 partition the health space on the recorded bit. Alternatives rejected: a health-driven return from `FenceReadyLost` with no branch started (turns the detect→select window into a silent escape from an incident that already blocked openings); atomic detect-and-select (erases the very interleaving window in which FM-1/-2/-3 all hid)** | core §7.4, core §9, SM §3, `docs/formal/` |
| **Cx8-2 (MAJOR, model/verification) `HeldIncident` is two states wearing one name — SM §4 gives the readiness park and the budget-exhaustion park *different* exits, and `ActHeldReentry`'s cause-blind guard exited both, so WF forced an unbounded no-progress cycle (`quiesce → reconcile → reduce → exhausted → held`) with no attempt left to issue. Model records `heldCause`; re-entry requires the readiness cause **or** a position proved flat (the exhaustion row's own "any later fill re-classifies" — load-bearing: without it a manually-flattened exhausted park is unclearable, since clearance requires actPhase ∈ {standby, done, unavail}). SM §4 gains a clarifying sentence; no behavior change — both exits were already stated** | SM §4 (clarification), `docs/formal/` |
| **Cx8-3 (MAJOR, verification gap) an "honest escalation terminal" that is never actually terminal still discharged `Branch2AutoResolves` — one transient visit to `held` satisfies a `<>` consequent, which is why the Cx8-2 cycle was invisible to a green suite. `P3_ExhaustedParkIsStable` (`<>[]`) added to the automated-convergence config; `P4_ArbitrationCompletes` added as FM-4's regression check; and **NC5**, the first negative control over a *temporal property*, added so P4 is known to bite rather than assumed to — the failure mode behind both Cx7-2 and Cx7-3** | model only (`docs/formal/`) |

**Milestone-1 implementation decisions (v0.9.5, 2026-07-26).** The first decisions taken with code in hand rather than on paper. They are recorded here because each one either *deviates from* or *adds to* the normative core, and an unrecorded deviation is drift. Full reasoning with rejected alternatives: `docs/superpowers/specs/2026-07-26-milestone-1-foundation-decisions.md`.

| Decision → resolution | Where |
|---|---|
| **Go CONFIRMED over Rust — evaluated properly, not inherited.** Asked by the user before significant investment and assessed rather than deflected. **The sunk cost is not the code** (~700 lines, ~1 day): it is a 415 KB document set that survived four external review rounds and speaks Go natively (`interface`, `type X string` + `Validate()`, "No DI container", `Envelope{Payload any}`) — re-specifying §3.3/§3.4/§3.5/§4/§5.1/§5.3/§11 in Rust signatures is 1–2 sessions plus its own review round. `docs/formal/` is language-agnostic and would carry over untouched. **Where Rust genuinely wins, recorded without strawmanning:** (a) exhaustive `match` over sum types — the strongest argument, since the safety kernel is seven state machines and formal review has already found four defects; (b) the money kernel specifically — native `i128` would replace ~30 lines of hand-written `math/bits` overflow handling and make a 128-bit mantissa essentially free, and `Result`+`?` would eliminate the `Calc` type entirely, which exists solely to work around Go's lack of `?`; (c) mutation tooling — `cargo-mutants` is maintained, Go's `gremlins`/`go-mutesting` are not. **Explicitly debunked: "no GC pauses."** This is a funding-carry strategy on 8-hour settlement cycles, not latency arbitrage; sub-millisecond pauses cannot affect it, and any future latency argument for strategy #1 is pattern-matching rather than reasoning. **The decisive argument is structural: this design is ports-and-adapters maximalist, and async trait objects are Rust's worst-supported combination** — rule 2 makes every driven dependency an interface, §4 declares segregated capability interfaces per context, §3.7 requires ports stay swappable, and §3.5 makes live-vs-backtest adapter interchangeability *the entire parity mechanism*, which is ~20 I/O-performing ports needing dynamic dispatch; the `async-trait`/`Pin<Box<dyn Future>>` tax lands at every seam for the project's life. **The trade is not safety-vs-velocity:** Rust buys compile-time case coverage and better mutation tooling, Go buys frictionless dynamic dispatch across those ports plus everything already written. **The correctness ceiling is set by neither** — it is set by TLC trace-checking the implementation against the TLA+ model, which is stronger than exhaustiveness (exhaustiveness proves every case is *handled*, not that it is handled *correctly*). Compensating measure adopted: §3.6 rule 10. **Do not reopen without new information** (one checkable fact underpins it: `dyn`-compatible async traits still requiring `async-trait` — re-verify if it is ever reopened) | core §3.6 rule 10, core §11 |
| **The `boundary/` grouping segment is dropped — contexts sit at `internal/<ctx>/`.** In DDD the term is *bounded context*; "boundary" reads as the adapter **edge**, the opposite of what those directories hold. `contexts/` rejected in turn (collides cognitively with `context.Context`); no third candidate survived (`modules/` collides with Go modules, `services/` contradicts D1, `domains/` stutters against the inner `domain/`, `core/` is taken by the `Core` structs). **Settled by the observation that none of §3.6's gates depend on the prefix** — they key on the inner layer names — so it carried zero enforcement value, and flattening makes `kernelpurity` a one-item allowlist rather than a deny-list. **`boundary/lint` in §3.6 is not a path** and is deliberately unchanged | core §3.2, core §11 |
| **The §3.6 gates are custom `go/analysis` analyzers, not linter configuration.** `depguard` cannot be unit-tested, so it cannot be *shown* to bite; `forbidigo` needs inverted path-exclusion logic that silently stops covering any context added later; and rule 4 cannot be configuration at all — "no value here is a float" needs type information. Each analyzer ships `analysistest` fixtures that MUST produce diagnostics, and each was bite-checked by hand-applied mutation | core §3.6 gate status |
| **Rule 9 `kernelpurity` added** (`platform/` imports only stdlib + `platform/`): catches the domain leak (a kernel importing a context becomes a shared *domain* kernel) and spine inversion (`platform → engine`) with one rule. Third-party imports deliberately out of scope — `platform/observability` needs `prometheus/client_golang` | core §3.6 rule 9 |
| **Rule 5 widened to `engine/`, with a transport escape.** Not parity purity: `platform/clock`'s monotonic clamp is only a guarantee if everyone reads the same clock, so an adapter stamping `RecvTime` from the raw clock while freshness reads the injected one puts two clocks either side of a subtraction, disagreeing exactly across the NTP step backwards the clamp exists for. Rule 5's *family* scope also stated — the gate bans nine ambient functions where the text named one. Scope kept **structural** rather than an allow-list of the engine packages that run under both live and replay wiring: sharper today, silently incomplete tomorrow. `engine/` alone gets `//notimenow:allow <reason>`, because the no-escape argument (inject the Clock instead) does not transfer to a socket read deadline that must fire when the dispatcher is wedged | core §3.6 rule 5 |
| **Rule 8 exception documented — `RoundingMode` is `uint8` + `iota`.** The `type X string` shape earns its width at a persistence or wire boundary; a rounding mode is an argument, so the property that matters is an unusable zero value (`RoundInvalid = 0`) | core §3.6 rule 8 |
| **`fixedpoint` error precedence stated: invalid mode → divide-by-zero → overflow → precision loss.** Found by writing an independent `math/big` oracle, and it was a real defect — `mulDivMantissa` classified the same condition two ways depending on which side of an internal boundary it fell (`ErrPrecisionLoss` in [2^63, 2^64), `ErrOverflow` above) | core §3.6 rule 4 |
| **`TimerSource` added to §3.5 — timer consumption is pull-based**, on an interface separate from `Clock`, because a Clock pushing due timers on its own goroutine would put the arbitration that decides the replay key in two places. **Timer priority direction made normative: lower `Prio` is more urgent, so the zero value is the most urgent** — §3.5 stated the tie-break order but not its direction, and the opposite convention silently deprioritises an unset safety timer. **Waking a live consumer is unbuilt and belongs to the dispatcher** | core §3.5 |

**Milestone-1 second-slice implementation decisions (v0.9.6, 2026-07-27).** The eight edits milestone 1's *second* slice produced while building the ten-package shared kernel (`fixedpoint`, `clock`, `apperror`, `ids`, `events`, `config`, `outbox`, `observability`, `ratelimit`, `boot`). **They are all one kind of finding, and that is the interesting thing about them:** not one is a disagreement with the core — every one is a **requirement the core relied on without stating**, found by having to write the code that would have relied on it silently. Each was marked `[SPEC EDIT OWED]` in the source as it was taken, and the marker count was the release gate (`grep -rn "SPEC EDIT OWED" internal/` returned exactly eight). Full reasoning with rejected alternatives: `docs/superpowers/specs/2026-07-26-milestone-1-foundation-decisions.md` (§9h, §10b, §11b, §12c, §13a, §14b, §14d, §14e).

| Decision → resolution | Where |
|---|---|
| **The outbox's at-least-once export is also per-`Key` FIFO.** §3.2 and D2 stated a delivery-**count** guarantee and relied, unstated, on a delivery-**order** one: audit is called *a projection of the OMS journal*, and a projection that applies a fill before the order that produced it does not reproduce its source. Scoped deliberately — FIFO **within** a `Key`, nothing across two — because "ordered" unscoped reads as global FIFO, which is strictly stronger, unnecessary, and lets one poison record halt every consumer; it is the same structural rule §3.5 already states for dispatch. The cost is stated rather than hidden: **a record that fails forever blocks its own key forever**, the loud direction, where skipping past it silently reorders a hash-chained projection | core §3.2 |
| **The bootstrap phase roster is a set of constraints, not a list.** §3.2 gave the roster in a table cell and never said the positions were load-bearing. Two are, and both fail **silently**: signal disposition installed late means an operator's interrupt during a slow join kills the process on Go's default SIGINT/SIGTERM disposition — no flush, no record a shutdown was requested — and the window where a boot is slowest is the window where somebody interrupts it; observability started late means a boot that dies in the journal join dies with an **exit code as its only diagnosis**. The roster stays **open** (§11's three binaries have different phase sets), so the constraints bind on the kinds named, not on a fixed length | core §3.2 |
| **`SourceID` composes `VenueInstrument` rather than restating four flat fields.** The flat form could not express the `ContractType` §5.1 gives an instrument, so a source could not name an inverse contract at all; and duplicating the fields duplicates their validation, which is how two definitions of "a valid instrument" come to disagree. §3.5's own trailing comment already said this *is* a `VenueInstrument` stream — the edit makes that structural. Since `SourceID` keys the §3.4 mailboxes, it also carries §5.1's "exactly one spelling" requirement structurally rather than by convention | core §3.5 |
| **Hot-reload is per field and default-DENY (`config:"hot"`).** §5.2 said "hot-reloadable where safe" and stopped; "where safe" is a per-field judgement, and the two defaults are not equally wrong when forgotten — default-allow is noticed when a running process picks up a new journal path or a fencing-generation edit **mid-trade**, default-deny when a reload is refused with the field named. §3.6 rule 4's own argument, applied to a requirement left open rather than decided differently. Composites **recurse** rather than compare wholesale, so a hot budget nested in a non-hot venue map stays tunable while *adding a venue* needs the map marked. **One deliberate asymmetry: an unrecognised tag value is an error, not a silent deny** — everywhere else the safe direction is the loud one; here it would have been the silent one | core §5.2 |
| **The total graceful-shutdown budget MUST be strictly less than §7.4's host-silence threshold, refused at startup.** §7.4 gave the watchdog a trigger, §3.2/§7.5 gave the process a graceful shutdown, and nothing joined them — but they are the same wall clock. At or above the threshold an **ordinary restart is indistinguishable from a dead host**: the watchdog escalates, the actuator fences the signer mid-flush and latches a takeover epoch, and the process comes back up fenced, held in reconcile-only until an operator clears it. A routine deployment becomes an operator-cleared incident, caused by a sum of durations nothing was adding up. The bound is **allotted and accounted for, not enforced mid-shutdown** — the missing enforcement is the operator's second interrupt, and that is the correct side to err on, because **host death is a covered state and a process wedged in shutdown is not** (the watchdog reads liveness, not progress) | core §7.4 |
| **§7.5's preconditions latch or self-clear, and clearance restores the ability to pass — not the pass.** §7.5 gave every precondition the same verb (*asserted before trading resumes*) while stating, in one paragraph, two that clear differently: a journaled `MarginOp` resolves through its evidence contract, while a foreign order or unexplained position delta "alerts for operator clearance" and is "not auto-corrected". §7.4 then spells the second kind out for the fence — clearance **and** a fresh propagation test, "never health-flap re-entry". A uniform checklist would let a precondition that failed and then passed on its own retry re-admit trading: **§7.4's forbidden re-entry arriving through §7.5's door.** So clearance returns a latched precondition to *unreported*, never to *passed*, and the check must still run and succeed afterwards. Deliberately **not** an edit to SM §3's health-gated clearance `[→ Cx6-3]` or SM §4 — this is the boot checklist, their sibling | core §7.5 |
| **The rate-limit ladder: one mechanism the core stated three times.** §7.6 gave a threshold ("back off at ~70% weight") without saying *what* backs off; §7.1 and §6 gave a reservation without saying how much or what happens when it runs out. Traffic now carries a priority and each priority has its own ceiling on the same budget — `Opening` → `Routine` → `Safety`. **The ordering is not the obvious one and it is normative: monitoring outranks new exposure**, because shedding market-data and account reads to protect trading traffic would make the rate limiter itself the cause of a stale mark, which is §7.1's own escalation trigger — the governor would fire the de-risk it exists to help avoid. **Capacity is a ceiling for every class including `Safety`: an exclusive band, not a bypass** (the user's call). Past capacity the venue refuses, so an issued flatten is not a flatten; Binance escalates repeat violations from 2 minutes toward three days, so pushing lengthens the outage being pushed against; and §7.4's pre-placed protection plus the off-host actuator are the spec's *designed* answer to "cannot act at the venue". Ceilings are **absolute counts, not percentages of a percentage**. Rejected alternative, with its real merit recorded: refuse only on an *observed* ban, since the local count is an estimate. **Built with no new venue numbers** — `venue-profiles.md` records the header names as verified facts and not the budgets | core §7.6, §7.1, §6 |
| **Metric label cardinality is default-deny.** §10 requires metrics cut per venue and per symbol and says nothing about cardinality; §3.4's lossy-lane guarantee is about a stalled **consumer**. Neither covers the way a metrics layer actually takes a trading process down: a metric labelled by `ClientOrderID` allocates one time series per order, forever, in the trading process's own heap — **nothing stalls, every scrape succeeds, and the lane reports zero drops**, so §3.4 holds perfectly for the entire duration of the outage it fails to prevent. A label now declares a closed value set or carries a **written reason** its domain is open — the `//nofloat:allow <reason>` and `config:"hot"` shape a third time | core §10 |

**Milestone-1 re-judgement (v0.9.7, 2026-07-27).** Four normative statements the shared kernel's *code* makes that the core had never stated. They are **not** a third slice's findings — no code was written to produce them. They were **held out of v0.9.6 deliberately**: each carried no `[SPEC EDIT OWED]` marker, because each was judged at writing time to *implement* an existing requirement rather than deviate from one, and v0.9.6's release gate was the marker count (landing an unmarked ninth would have broken that correspondence permanently). The judgement is re-tested here against one question — **could a reimplementation get this wrong in the *unsafe* direction while still satisfying the core as written?** — and all four fail it, so all four land. The distinction from the two blocks above is worth keeping: those were found by *writing* code that would otherwise rely on an unstated requirement; these were found by *re-reading* code and asking what backs a claim it makes. Full reasoning, including why each was originally left unmarked: `docs/superpowers/specs/2026-07-26-milestone-1-foundation-decisions.md` (§15b).

| Decision → resolution | Where |
|---|---|
| **The outbox's dedup key is `Seq`, and the exported payload is the committed bytes.** D2 says the export is at-least-once to *idempotent consumers* and never says idempotent **on what** — which leaves content-hash dedup as a reading that satisfies the core and is wrong in the losing direction: two identical funding accruals on successive intervals are **distinct facts**, so hashing drops the second, and the loss surfaces as a **missing funding receipt in the §10 tax records** with nothing reporting it. The key is the store-assigned durable sequence, unique by construction. Separately, the `Payload` is the exact bytes that committed alongside the journal write, retained and republished verbatim: holding a Go object and re-marshalling at delivery time lets a **map iteration order or a struct-tag change** make exported bytes differ from committed ones, and against a hash-chained projection (§7.5/Z4) that difference does not read as a serialization bug — it reads as **tampering** | core §3.2 (+ D2 pointer) |
| **Reconcile-only is a posture, not a boot stage.** §7.5 names it as a phase of boot, but every §7.4 escalation puts an *already-running* process into it — a bot that recovers mid-takeover "stays in reconcile-only until an operator clears it". So it is a state occupiable at any time, on four rungs (halted → reconcile-only → stop-opening → trading). Two properties are the ones a reimplementation gets wrong: the **zero value is the most restrictive** (the alternative reads an unconfigured process as cleared to trade), and the reduction is the **weakest over the gates that were *required*, with halted as its identity**. That identity is the whole point — the natural way to write a minimum seeds the fold at the maximum, which makes **"nothing was checked" and "everything passed" the same answer**: §7.5's own latching defect arriving through the arithmetic | core §7.5 |
| **The header correction folds in upward only, and a venue retry hint is a floor.** §7.6 says "header-aware" and stops, so *adopt the venue's reading* is a satisfying and dangerous implementation. A reading is always **stale** — it describes the server at the moment it processed some earlier request, others already in flight behind it — so a reading *below* the local count is what staleness **looks like**, and adopting it discards the record of every request sent since; a reading *above* it is information available no other way (an underestimated endpoint weight, or another consumer on the same IP). The failure costs are asymmetric and that is the argument: ignoring a legitimate decrease costs **throughput until the window rolls**; adopting an illegitimate one costs a **ban that escalates toward three days**. Same direction for a venue's retry hint on a hard pause — the pause is the **longer** of it and the local penalty, because "60 s" is the venue stating its minimum, not granting permission for the remainder. **No new venue facts**: the header names and the 2 min → 3 days escalation are already verified in `venue-profiles.md` §2 and are cited, not introduced | core §7.6 |
| **The recording seam is integer-typed, and a sample carries its unit and asset.** §3.6 rule 4 bans `float64` under `internal/` and §10 lists metrics; neither says how a number *enters* the observability layer, and the exporter boundary is exactly where a `float64` seam gets reintroduced for the whole codebase's numbers on the argument that *it is only a metric* — while §12's kill-gate verdict is read off those panels. A value crosses as a scaled integer at `fixedpoint`'s canonical scale, tagged with its **unit** and, for money, its **asset**, refused on mismatch. The tags are the part rule 4 does not give: unlabelled, a duration recorded in milliseconds against a seconds metric and a USD figure summed with a USDC one both render as **plausible dashboard numbers** and fail nothing. Conversion to a `float64`-typed exporter happens once, at that adapter, under rule 4's annotated escape, with its exact range stated | core §10, §3.6 r4 |

**Milestone-1 third slice (v0.9.8, 2026-07-27).** What building `marketdata` — the first bounded context, scoped to the funding path — produced. **The lead item is a different kind from every row above it: a violation of the core, not a silence in it.** The three blocks above are requirements the core relied on without stating; this one opens with a rule §3.5 *did* state and the code disobeyed, while its own doc comment argued the rule did not apply. That distinction is the finding worth carrying, because it names a limit of the slice-close gate: **`grep -rn "SPEC EDIT OWED" internal/ tools/` finds claims the code marked, and cannot find a rule the code silently broke.** The reading pass beside the grep is what caught it, which is why that pass is not optional and not replaceable by the grep. Full reasoning with rejected alternatives: `docs/superpowers/specs/2026-07-26-milestone-1-foundation-decisions.md` (§18).

| Decision → resolution | Where |
|---|---|
| **A dispatched envelope carries the `AcceptOrdinal` of what triggered it — every producer, not only book reconstruction.** §3.5 states this outright, with "a reconstructed snapshot for book streams" as an *example*; the publisher read the example as the extent and stamped zero, so a published funding snapshot could not be traced back to the raw input that produced it. What made the misreading available is a real distinction the port's own doc **blurred**: this layer must not INVENT a sequence, which is true of `IngressSeq` — the replay key, assigned in consumption order by a dispatcher a producer cannot see — and false of `AcceptOrdinal`, which is stamped upstream at ingestion and merely **inherited**. Both are `uint64`, both ride the same envelope, and one sentence covering both loses the trace. §3.5 now scopes the rule to input-derived envelopes and says it binds every producer | core §3.5 |
| **The envelope derived from the ABSENCE of an input carries no ordinal, and reusing the last one is forbidden.** §3.4's silence timer publishes a re-judged verdict on an observation nothing has superseded — the one envelope on the lane whose entire news is that *nothing arrived* — and §3.5's inheritance rule had no case for it. `AcceptOrdinal == 0` is how it says so, and the value is readable as that only because a producer **refuses** a zero on an input-derived envelope; otherwise "no input" and "somebody forgot" are one value with two meanings, and the reader who cannot tell them apart is a replay. The rejected alternative is the one an implementation with nothing to put there reaches for: reuse the held observation's ordinal. It makes the capture log claim **one raw input produced two dispatched envelopes**, so a replay reconciling them reports a phantom duplicate and concludes the feed is healthy — inverting the meaning of the one event that fired to say a feed had gone quiet | core §3.5 |
| **`Envelope.Validate()` is the dispatcher's post-stamp check, not a producer's.** §3.5 said who stamps each sequence number and never *when* an envelope must carry them, which left `Validate` looking like an invariant of the type rather than of a position in the pipeline — and the unsafe reading is the natural one, since a producer validating before publishing cannot pass without minting an `IngressSeq`, a second authority over the order determinism keys on. Everything on a §3.4 mailbox is **pre-dispatch**: `AcceptOrdinal` inherited, `IngressSeq` zero. The consequence for a producer that still wants the type's full field check is a **probe** — validate a copy with both sequences forced, discard it — which keeps a field added to `Envelope` next year covered without restating the field list | core §3.5 |
| **A source's `Prio` is a per-`SourceID` constant and may not depend on content.** §3.5 makes the *direction* normative (lower is more urgent, zero most urgent) and says nothing about constancy, so a spec-compliant implementation can still escalate on a worse reading — and `FreshnessBlind` is exactly the tempting case, because it fires §5.2's `mark_blind_action` and therefore looks like safety traffic. §3.4's mailboxes are per-`SourceID` and §3.5 makes `VenueSeq` authoritative within one, so two priorities in one source are two envelopes that can drain out of sequence: the blind reading overtakes the stale one it followed, and **a consumer watching a degrading feed sees it recover**. Which constant is policy; that there is one is not | core §3.5 |
| **One freshness budget per (data class, venue), measured against two clocks.** §3.4 makes budgets per-data-class and per-venue and leaves open which clock measures against them. Both do: transport age from `RecvTime` against `Clock.Now()`, data age from `SourceTime` against `Clock.ServerNow(venue)`. The asymmetry is what makes both load-bearing — a **venue-side stall keeps updates arriving on schedule with a frozen `SourceTime`**, so the transport verdict reads perfectly fresh for the entire outage, which is the invisible failure. Reported separately because the operator response differs; reduced by **minimum** for a trade decision. Rejected: a second independent budget for the venue-side measurement — it buys a looser tolerance for a venue whose clock is known to lag, at the cost of a config field per venue and a second unreachable-rung validation, while §3.4's stated reason for per-class budgets is the *data's* tolerance for staleness, which does not change with which clock measured it | core §3.4 |
| **The `Envelope` is authoritative for provenance; a normalized payload carries none.** §4's `StreamFunding` comment lists "source, venueSeq" among payload contents, and both are `Envelope` fields §3.5 defines for every stream — read literally it asks for two copies with nothing saying which wins. The unsafe direction is reachable without an implementer noticing they chose: **gap detection reads `VenueSeq` while freshness reads the timestamps**, so a payload disagreeing with its envelope gets those two questions answered from different sources, and a venue restamping an unchanged reading becomes a fresh observation with no gap — the silent stall §3.4's timer exists to catch. A payload carries no instrument either, since `SourceID` is the mailbox key | core §4 |
| **`marketdata` does not publish `MarketView`; `engine/state` builds it.** §3.2's row said `marketdata` publishes an immutable `MarketView` while §3.3 and §3.2's own next row give that translation to `engine/state`. §3.3 wins: a context publishing a strategy-owned type would have to know cross-leg skew and the strategy's view shape, coupling the two across exactly the boundary §3.6 rule 1 and the read ports exist to keep — and it also needs the book path, which does not exist. Rejected: §3.2 winning, which would additionally reduce `engine/state` to a cache | core §3.2 |
| **Rules 1 and 10 are enforced; rule 6 is the only deferral left, and rule 10's deferral named the wrong trigger.** `hexboundary` and `exhaustiveswitch` landed the session their subjects appeared rather than after the code they police. Rule 10 had waited on a **state machine**, while what actually made it buildable was the shared kernel's **eleven enum-likes**, which had existed for several slices — it found **13 silent fallthroughs** on its first real run, every one of them reachable the whole time. A deferral must name the *construct the analyzer reads*, not the feature that motivated it, and rule 6's was re-checked against that. Separately, and normative for any future gate: **rule 1's obvious positive control is an import cycle** — `application/` importing `infrastructure/` is refused by the type-checker before the analyzer runs, so the gate scores a pass without executing. Within one context Go's no-cycles rule already covers most of rule 1; where it earns its place is the edge Go cannot see (`engine/`, another context's infrastructure) | core §3.6 |

**Milestone 4 task #177 is landed: SM §7's `PackageDelever` GENERATION LADDER
exists, so `FM-9` has a consumer and `FM-8`'s standing obligation is DISCHARGED
rather than merely permitted (v0.9.55, 2026-08-07).** Design record **§74**;
task **#177** closed, tasks **#203** (the journal adapter) and **#184** (the
single-order dispatch path's filter re-check, whose stated trigger has now
fired) carry what is not delivered. **The one `SPEC EDIT OWED` marker in the
repo is discharged**: SM §7 gains the ladder-resumption invariant, and the grep
over `internal/ tools/` is back to zero.

**The lead item is that four rulings came out of BUILDING the machine, none of
which reading the section could have produced, and each is a claim about
composition rather than about either side.** §7's prose and the model were both
consistent with the wrong answer in every case, because in every case the
question is what happens *between* the ladder and the shared ledger — the family
`FM-5`/`FM-8`/`FM-9` already names, arriving a fourth, fifth, sixth and seventh
time through the decomposition instead of through the spec. Two of them are
the same shape one turn out: a rule stated for one purpose being read for
another (the quiesce verb; the entry counter's threshold).

| Decision → resolution | Where |
|---|---|
| **The quiesce obligation needs NO cancel port, and "cancelling its reserved second leg" was this doc set's own gloss.** All three supersession rows require the reserved second leg to reach §6.7 finality *first*, worded as *cancel/quiesce*. Implementing it looked like it required a `Cancel` on §4's contract — which does not exist and never has, across three slices of venue work. **The absence is correct and load-bearing:** §6 step 6 sends nothing but marketable IOCs, so every order this system places is immediately terminal at the venue, and "quiesce" is therefore **awaiting** finality. That also answers the question the wording hides — whether the wait can hang: the two phases the obligation binds on (`SpotReduceInFlight`, the spot-side park) have `Complete` and `TerminalFlatten` among their successors and **both end the generation**, so the obligation is discharged by its subject finishing. A missing verb read as an omission would have added a cancel path to a system whose safety argument rests on not having one | SM §7 (new invariant), core §5.3 |
| **The entry gate and the standing trigger want DIFFERENT thresholds on ONE counter, so the reservation is taken BEFORE the premise is asked for.** `FM-9`'s netting removes "this generation's own entry", and the only handle the ledger offers is the **producer** — so the computation is sound at exactly one `PackageDelever` entry and must refuse at two. SM §7's entry rule ("only after the prior generation reaches §6.7 finality") is a claim about the same counter at **zero**. Asked in the natural order — premise, then reserve — a stale sibling reads as a count of one and the **sibling's** entry is silently netted out of this generation's premise: `FM-9` arriving through the decomposition. Reserving first makes the sibling the second entry and the existing refusal becomes the gate, so one check serves both rules. The cost is stated with it: the ledger is mutated before admissibility is known, so **every** refusal after the reserve must release, or an entry subtracts from every later sizing on behalf of a generation that does not exist | SM §7 (new invariant) |
| **Ending a generation RELEASES its entry; it does not amend it to zero.** Both leave the entry subtracting nothing, so the *final ledger* cannot tell them apart — which is why the rule had to be stated about the **act**. What the entry carries at the ending is real exposure the generation put on and did not take back (a superseded generation that bought one perp and sold no spot leaves the group genuinely net long by one), and releasing is what lets §2 see it and compensate. Amending to zero first reports the same ledger while asserting the exposure went somewhere, and the group reads flat while holding a leg. Mutant M2 confirms the suite separates them | SM §7 (new invariant) |
| **A supersession ends the GENERATION; whether §7.1's ladder resumes is a property of WHICH ROW fired** — the `SPEC EDIT OWED` marker. Premise-lost **resumes**: the premise dissolving means the package acquired a naked residual, §2's determined branch compensates it, and the package is then neutral again while the perp short may still be nearing liquidation — refusing would disable the one mechanism that reduces leverage on a neutral package for the life of the process, the stuck-state shape `FM-8`'s own ruling was taken to prevent. The two external arbitrations do **not**: the foundation has taken the package away from this ladder, and re-opening is the OMS re-authorising itself against an arbitration that just fired. Nothing is loosened by resuming, because the entry precondition still binds. **Model (a) does not answer this and cannot be read as doing so** — `Supersede` and `PdPlan` both require `superseded = "none"`, so one supersession per behaviour is a state-space bound of the `MAX_GEN` kind — and `ambiguous-order-model.md`'s clause map **asserted the opposite in a parenthetical** ("reachable … or via `Supersede`"), naming a route that reaches the guard while permanently disabling the action. Corrected there. Because the two failure directions are asymmetric and neither is derivable from an order journal, the path is carried on the **durable** supersession record | SM §7 (new invariant), core §5.3, `ambiguous-order-model.md` (corrected) |
| **§6.7's ledger entries have a SIGN, and the section that fixed their AMOUNT was silent on their DIRECTION.** `FM-5` established that an entry is what the reserving action has *committed and not yet realized*; it did not say which way the number points, and the two readings differ by a sign on every entry. Only **exposure removed** satisfies both halves of the sentence that uses it — a subtraction, whose purpose is that an in-flight reduction is not re-read as fresh naked exposure. Under the order's-own-change reading a group realized net long 1.0 with an in-flight sale of 0.4 computes a residual of **1.4**, and the determined branch sells 1.4 against 0.6 of naked long: **a naked leg minted by the rule that exists to prevent one**. The tell that settles it without a model run is that the same sentence describes `PackageDelever`'s entry as "the confirmed perp reduction not yet matched by that sale" — a buy-to-reduce of `x` nets to zero only at `+x` — so **the two producers would need opposite signs from one sentence**. Model (a) encodes the exposure reading for both and is the authority, because §46 §8e's rule bites here too: no invariant phrased over this ledger can see a sign error in it | core §6.7, SM §2 (new invariant) |
| **The ladder has a durable record set at GENERATION scope, and its subject is not an order.** FLAG-3 names one record; three further facts exist nowhere else once the process dies. The **leg ids**, because a recovered generation that cannot name its own orders computes the premise over a set containing its own subject — `FM-9` through recovery — and the two legs must be distinguishable, since only the reserved *second* leg carries the quiesce obligation. The **authorized size**, whose legal **zero** (a perp reduction that confirmed nothing) is the same bytes as a dropped field. The **supersession path**, which the ruling above makes load-bearing and which no order journal can derive. **One chain, not two**, because *the authorization precedes the spot order* is otherwise a discipline rather than a checkable fact; **one chain is not one key**, because the two id spaces share no namespace. The perp leg's **fills** are deliberately excluded: that quantity is the perp order's own cumulative execution, already durable at order scope, and a second copy is two stores holding one number — the number §7's spot cap is sized from — with no rule for a replay that finds them disagreeing | SM §7 (new invariant), core §5.1 |
| **SM §7's `Held/Alert` is ONE state in the table and MUST be TWO in an implementation.** The resume row says *the HELD LEG's* contract reaches finality, so the machine must know which leg parked — and the sharper reason is supersession: the reserved second leg is live at a spot-side park and absent at a perp-side one, so the quiesce obligation binds on one and has nothing to bind on at the other. §46 §10a had narrowed the perp-side park away as harmless (`pdDone = 0`) and building it refuted that: a perp leg can partially fill and *then* go ambiguous, holding a real transient and a nonzero reservation. Recorded as a **correspondence note in the table** rather than left to each implementer | SM §7 (states table) |
| **Two mutation verdicts that point opposite ways on the same signal, both recorded.** `premiseOrDischarge`'s three branches collapsed to one **SURVIVED**, and the mutant was **wrong**: the branches report, they do not enforce — `CheckAdvance` re-reads `Premise.Intact` one layer down and refuses with the same sentinel, so the claim lives in the `reassess` call *above* the branches, and the mutation that negates it is removing the supersession itself. Now a comment, because the natural reading is the reverse. `verifyLeg`'s hedge-group check deleted also survived, and that one was a **real gap**: every fixture in the file planted its legs on the group by construction — the collapse checklist's own tell, *the fixture does not contain the thing under test* — and it needed a new test | (code + tests) |
| **The `exhaustiveswitch` gate bit on a TEST FAKE, which is worth more than it looks.** It caught the fake journal's `AppendGenerationPhase` switch on its first run. An unnamed `default` there would let a later phase fall through to *recorded, no payload* while the suite stayed green and the port's totality argument had quietly acquired a hole — the gate policing the evidence rather than the subject | (tests) |

**Milestone 4 slice A is landed: §4.1's account & margin-mode posture has a
producer — `internal/risk` exists, `GateAccountPosture` crossed `ExitClass` into
the produced arm it has been declared beside since the composition root, and the
Portfolio-Margin check is ATTESTATION-backed because no section of either
document supplies a read path for it (v0.9.51, 2026-08-06).** Design record
**§73**; task **#198** closed, task **#199** opened for the one ruling this slice
deliberately did not take.

**The seam was not greenfield, and that is what made the structural deliverable
precise.** `platform/boot` had declared `GateAccountPosture` since slice G2,
§7.5's eight-gate roster listed it, and `Report.ExitClass` named it explicitly in
the arm for gates *"with no producer anywhere"* — whose own comment is the
instruction the slice followed: *"a gate added to §7.5's roster has to choose a
side here, and choosing 'not ours' by falling through is how a real refusal
becomes invisible."* So the deliverable was to give an already-declared gate its
first producer and move it across a compiler-checked switch. Checked for
**exhaustiveness**, note, not for **correct membership** — nothing but a test
says the gate now belongs in the incomplete arm, which is why one exists.

**The findings split three ways, and only the first kind is a defect in the
specification.** One is: Profiles §2 requires Portfolio Margin to be OFF and
supplies no `GET` for it, in a cell where every *other* item names one — the
check reads as covered because it sits in a list of covered checks, and the
missing element is one column of one cell. Four more are **design traps** where
the implementation a session in a hurry writes is vacuous or permissive in the
direction that computes the safety gate in our favour: a leverage pin derived
from the venue it is meant to check, an auto-add-margin check driven by the
venue's returned rows on an account that is flat at every boot, an external enum
read as a blacklist, and a borrow field whose *other* consumer reads its silence
the opposite way and is right to. The rest are the vacuity family this document
has now recorded at four levels — and the two general items §73g gained are
worth more than the instances: **a collapse checklist runs at ONE LAYER unless
it is re-run** (every end-to-end boot fixture in the repo carried an empty
roster, so §4.1's whole per-symbol half was reachable at boot and asserted
nowhere — because a *legal* degenerate shape is the one nobody notices is
degenerate), and **one-violation-per-fixture hides a ladder's ORDER** (eighteen
cases each breaking exactly one rung, and swapping two rungs left all eighteen
green).

**The closing reading pass found the one that no test could have.** The posture
probe reads `isAutoAddMargin` from `GET /fapi/v3/positionRisk`; Profiles §9's own
dated, verbatim V3 transcription lists twenty row fields and that is not among
them, while Profiles §2 cites the **V2** page for the setting. Underneath it is
the same shape as the block below on `restart_boundary`: §7.5 chose V3 on an
argument explicitly about *what a missing row means for the net-delta invariant*,
§4.1 inherited the choice for a question where V3's silence is precisely the
ambiguity, and **a narrowing carries an argument whose scope the next consumer
does not re-derive.** Both readings of the field's absence leave the gate
refusing rather than passing, so it was invisible to every fixture — and it is
now task #199's fourth candidate rather than a code change, because settling it
needs the page read that #180 has been blocked on for three sessions.

**Two operator questions are owed and nothing downstream is blocked by either
(task #199).** Roster-scoped coverage plus a setting published only on a
*position* enumeration means a flat Binance account refuses on every configured
symbol — the gate that must pass before the first position opens cannot pass
until one exists — and all four Binance probes are on the futures host, so a
spot-only deployment cannot answer §4.1 at all. **A gate that cannot pass is not
a conservative gate but a broken one**, and §5.2 already refuses by name a config
value *"that makes the permitted region unreachable"*; this is that shape
arriving through a rule. The strict reading stays implemented, with a fixture
asserting it and a comment saying the fixture must change when the ruling lands.

**The slice-closing `code_diff` review returned NO-SHIP with two majors, and
both are the same kind of thing: a defect that exists only BETWEEN two layers
whose own suites are green** (v0.9.52, 2026-08-06; record §73n). Each was
demonstrated by reading two files against each other, which is the one thing a
package-scoped test cannot do — and in the first case the two files' tests
encode contradictory premises about the same wire shape, both passing. The
second falsified a sentence this document's own previous block had recorded as
reassurance. Both are folded, with six mutants, all caught; two `NOBUILD!!`
first, both bad seds that orphaned an import — *read the compiler error*, and
`imported and not used` is the re-aim signal rather than a kill.

| Finding | Resolution | Where |
|---|---|---|
| **F1** — "Portfolio-Margin OFF" is a required check with **no read path** anywhere in the doc set, in a cell where every other item names a `GET`. It cannot default to *passed*: PM is the one setting that folds spot into futures collateral, i.e. the single account state that falsifies §4.1's premise outright | **Operator attestation, default refuse** — an inputs-not-conclusions document, a two-act `{path, sha256}` pin, invalidated by events never by time, absent → refuse by name. Profiles §2 gains a **marked unknown**, not a value: nothing is entered there on the strength of an attestation or a recollection, that being the channel which has already misreported a load-bearing number twice here. Rejected: inferring PM from the sibling classic-account settings — PM is a different **account type**, so a classic setting's value establishes nothing about it, and the argument would be recorded once and re-derived never | core §4.1, Profiles §2 |
| **F10** — the attestation copies #179's spot-baseline shape correctly and is **not equally strong**, which is exactly the kind of thing a later reader re-derives as a strength | Recorded as a limit in the normative text: #179's document is bound to a **history** (a journal chain mark a moved journal falsifies) and this one is bound to nothing, because the event that invalidates it — somebody enabling PM — **reaches no endpoint this system reads**. Every mitigation is worse than saying so; a credential is the loosening trigger | core §4.1 |
| **F3** — *"configured leverage matches pin"* has one natural implementation (read the venue's leverage at first boot, store it, compare later) which **cannot fail on the first boot**, the boot that would catch an account configured wrong before we touched it | The pin is an **operator-declared config value**, stated independently of the venue, and an **absent** pin refuses. One value feeding both the setter and the verifier stays self-consistent under every mutation | core §4.1, §5.2 |
| **F4** — auto-add-margin driven by the venue's returned positions checks **zero symbols on a flat account**, which is the state at every cold boot. Profiles §2's parenthetical *"not only open positions"* already anticipated it | **Coverage is answered from the configured roster**, and a configured symbol with no observation refuses by name — the identical ruling #179 reached for spot baselines (*"coverage answered from the configured roster rather than the journal"*), followed rather than re-derived | core §4.1 |
| **F9 + F14** — two ways that ruling makes the gate **unpassable**: a flat Binance account refuses on every configured symbol (the per-symbol setting is published only on a position enumeration), and a spot-only deployment cannot issue any of the four Binance probes, which are all on the futures host | **Left strict and OPEN (task #199)**, with the candidates written out and the strict reading executable as a fixture that must change when the ruling does. Not silently loosened: the refusal is the safe direction, and §4.1 now states the unreachable-region argument in place | core §4.1, task #199 |
| **F18** *(closing reading pass)* — the probe reads `isAutoAddMargin` from **V3**, whose own verbatim transcription in Profiles §9 does not list the field, while Profiles §2 cites the **V2** page for it. §7.5 chose V3 on an argument about what a missing row means for the **net-delta invariant**; §4.1 asks the opposite question and inherited the choice | **Marked in both places, not guessed**: §2 gains the ⚠, §9 gains the per-question note and the field-list evidence, #199 gains a fourth candidate (read the version that enumerates every symbol). Both readings leave the gate **refusing**, which is why no fixture could see it. The general rule: *a narrowing carries an argument, and the next consumer does not re-derive its scope* | Profiles §2, §9, core §4.1 |
| **F2** — one venue field (`borrowAmount`/`spotBorrow`), two consumers, **opposite** correct defaults for absence: #179's balance reader lets an empty value pin (a total is being derived), while the posture check's whole claim *is* "borrow is off" | The posture read gets **its own** predicate with its own empty arm, refusing by name. Reusing the balance reader's would import the permissive arm into the one gate whose purpose is refusing borrow, and the system would pass its borrow check on a response that said nothing about borrow | core §4.1 |
| **F5** — `unifiedMarginStatus` admits `{3,4,5,6}` and refuses `1`, leaving the unlisted `2` — and every value the venue adds later — decided by the direction of the default | **Whitelist, not blacklist**: an unlisted value refuses by name. The house direction — the failure from forgetting should be a refusal that names the value | core §4.1 |
| **F11** — "every deployment has a perpetual" was an invented narrowing; §5.2's venue block is **singular**, so the spot half of a cross-venue carry is a document with no perpetual in it, and the first cut would have outlawed a shape this project is building toward | **A carrier, not a default** — an empty roster has two meanings and neither default is safe (refuse it and a legitimate spot-only deployment can never boot; admit it and a forgotten roster passes vacuously). The requirement states which, the zero value names neither and is refused, and **both** disagreements refuse. The tell arrived before the reasoning: a six-caller fixture stopped validating, and rewriting it would have buried the finding | core §4.1, §5.2 |
| **F12** — the reconcile root's import fence listed `internal/risk` among the contexts §7.5's boot *"must not depend on"*, while `declaredGates()` in the same package had listed `GateAccountPosture` for as long | The fence's argument — importing a context is how a boot phase acquires a collaborator that **assumes** the posture — is true of marketdata, strategy, treasury and audit and **false** of the guard that *decides* it. The exemption is **exact-path, never a prefix**, and keyed to its reason: a test fails if the root imports `risk` while not declaring the gate, so the permission lapses the day its reason does | (code; core §7.5's roster unchanged) |
| **F6** — §6 step 3's pre-trade guard has **no caller**, because nothing in this system runs a trading loop | **One use case, two callers, the second lands with the trading loop** — not a second speculative code path. The debt is stated in §6.3 in place rather than left to be rediscovered: until that caller exists, *"boot and before each trade"* is half-delivered | core §6.3, §12 item 4 |
| **§73c-2** — *"fresh within a max-age TTL **and** re-verified by the pre-trade REST read"* admits two parses, and under the permissive one a position-mode flip inside the window is undetected — the exact case the next sentence says the re-verify exists to catch | The TTL is an **additional upper bound**, never a way to skip the read: every trade performs the full posture read regardless of snapshot age. The conservative parse is correct under both readings of the `and`; the permissive one under only one. Affordable for a structural reason — cash-and-carry opens are hours apart | core §6.3 |
| **§73c-3** — should the engine repair a posture it finds wrong? | **No — asserted, never repaired.** Beyond "an account-wide irreversible mutation inside an automated boot is the wrong default", the stronger ground: **a posture the bot can repair is a posture whose violation stops being evidence.** A wrong margin mode means *something happened we do not understand*; correcting it silently erases exactly that. Rejected for this slice: an operator `posture set` command — a new credentialed **write** path whose only evidence, with credentials deferred, would be a fixture | core §4.1 |
| **The gate's two refusing outcomes** are not interchangeable, and §7.5's existing text named only one of them | **Read-and-wrong LATCHES** (no retry can lift what this engine may not repair — a re-reportable refusal would be lifted by re-reading an account nobody changed, *"never health-flap re-entry"* through a deterministic retry); **could-not-be-established** is the ordinary self-clearing failure. Both were reachable and unasserted until two mutants survived on them | core §7.5 |
| **F7 → F13** — a finding whose **mechanism was wrong** while its symptom was real: adding two endpoints to the registry left the suite green, blamed on a one-directional check, when a complete-roster check twenty lines below already refused a missing budget by name | The real cause is a **positive control that derives its own expectation from the thing it checks** — the test profile's limits are built from the registry's own `Endpoints()`, so the pair stays self-consistent under any registry edit, and its subtest was still called *"all five construct"* while the registry held eight. Fixed by stating the roster **by hand**. The harm clause was wrong in the same direction and the safe one (a construction failure, not a runtime one) | (code; Profiles §4's rate-limit rows unchanged) |
| **F15** — every end-to-end posture boot fixture in the repo built on a deployment configuring **no derivative**, so the no-derivatives coverage flag had every integration witness and the ordinary production shape had **none** | Three boot fixtures with two configured derivatives carrying **different** pins, including a negative control whose violation is on the *second* symbol with the *first* symbol's pin as the venue's answer — so both wrong readings fail one fixture — and a positive control asserting the composed fold's **exact arity**. It is coverage of a **composition**, not an unguarded defect: all six probe mutations are already caught by their own layer's unit suite; what had no witness was the chain from an operator's config pin to a gate outcome | §73g item 8 (general) |
| **F16** — the attestation loader's four rungs each carry a stated argument for **where they sit**, and eighteen one-violation fixtures cannot see an **order**: swap rungs 1↔2 or 3↔4 and all eighteen still pass | A deliberate **two-violation** fixture per pair, naming which answer must win and asserting the shallower one is **not** also reported. Demonstrated rather than argued — each hand-applied reordering fails exactly one test, the new one. General: *wherever one-violation-per-fixture is the discipline, the ladder's ORDER is a separate claim needing its own fixture* | §73g item 9 (general) |
| **F19** *(code_diff review)* — both venue codecs REFUSED a symbol answered twice, which is hedge mode's documented wire shape and therefore a §4.1 **violation**. So a posture read and found wrong reported as one that **could not be read** — §7.5's retryable arm, `ExitIncomplete`, *"nothing is known to be wrong"* — and every probe already decoded was discarded with it, including Binance's account-wide `dualSidePosition` answer naming the same violation. `risk/domain` had carried `ReasonAmbiguous` for exactly this since the rules landed, with a fixture calling it *"the posture violation this rule exists to catch"*, and **no production path could reach it**: the codec's test asserts the refusal, the domain's test hand-builds the input the codec refuses, and both pass | **The codec reports, the rules judge.** Both rows travel; the domain sees two answers on one key and refuses by name, latching, beside every other check. Folding is still forbidden and was all the original argument ruled out — the codec took a stronger action its own reason did not support. Composition fixture: a hedge-mode boot latches, `PostureCause` nil, `Evaluated == 10`, and **both** failures named. General: *a layer that refuses what the layer above has a rule for makes that rule unreachable, and both suites stay green about a case the composition can no longer produce* | core §4.1, code |
| **F20** *(code_diff review)* — §4.1's probes are routed by product **family** against a transport map built one entry per **configured endpoint**, so a family with no endpoint makes the gate unanswerable for ever. F14 recorded the Binance half and reassured that *"Bybit is not symmetric — `/v5/account/info` and `/v5/spot-margin-uta/status` are account-level and **answerable regardless**"*, which is true of the exchange and **false of this routing**: two of Bybit's four probes ride spot, two ride linear-perp, so **either** one-sided Bybit deployment is short a family. This repo's own `spotOnly` fixture was such a document | **A startup refusal naming the family**, derived from the codec's own encoded steps rather than restated per venue — §5.2's unreachable-permitted-region rule reached through an omission rather than a value. The isolating fixture had to drop the family's **limits** as well as its endpoint, because a limit naming an unrouted family was already refused one check earlier. General: *a claim that reads as REASSURANCE is the one nobody re-derives*, and this one was falsified by a grep of the file the same slice wrote | core §4.1, §5.2 |
| **Three minors from the same round** — `arblab --help` said the reconcile binary *"produces four"* of §7.5's gates while its own report line printed 5, and its phase list never mentioned the posture; a nil `*venue.OrderClient` would have reached the ACL as a **non-nil interface** and panicked, the whole defence being one `r.halted` boolean set two phases earlier; and `Failure.Subject`'s documented invariant was broken by `Check` itself on the empty-enumeration failure | Help text and two stale in-code counts corrected; a named `ErrVenueClientMissing` at the root that still knows the concrete type, with the test that makes the interlock mutable; the invariant restated to cover the one per-scope failure that is about the enumeration rather than a member of it | code |
| **F18 → RESOLVED, and it was a DEFECT rather than the operator ruling it was filed as** *(task #199 item 1, record §73o)* — §4.1 read `isAutoAddMargin` from `GET /fapi/v3/positionRisk`. Binance's own spec-generated connector (`binance-connector-go`, models generated from their OpenAPI spec — raw, version-controlled, venue-published text) confirms Profiles §9's dated twenty-field transcription **exactly**: the field is not there. So the gate refused on **every** Binance account — a positioned symbol got a row with the field absent (*unstated*), a flat symbol got no row (*unobserved*) — not merely on flat ones as **§73j F9** had recorded. | **`GET /fapi/v1/symbolConfig` publishes all three per-symbol settings**, for every configured symbol whether or not it holds a position. The probe moved there and the positionRisk posture probe and its enum member were **deleted**: four probes become three, 80 weight becomes 60, and six checks now ride three calls. **F9's Binance half and the whole of F18 are dissolved rather than ruled on**, and #199's candidate 4 (*read V2*) is superseded by something cheaper than itself — an already-routed V1 call. Nine mutants, all caught. The claim that `symbolConfig` carries the field rests on the new source **alone**, and was landable before a credential because of its direction: if wrong, the field decodes as absent and the gate refuses, which is what it already did. |
| **The mechanism, which outlives the instance** — §7.5 chose positionRisk **V3** on an argument explicitly about its net-delta invariant (*a missing row must mean flat*), and §4.1 inherited that choice for the opposite question (*what is this CONFIGURED symbol's setting*). Recorded as *a narrowing carries an argument whose scope the next consumer does not re-derive* — but the instance is worse than the rule's usual form. | The inherited choice was not merely **out of scope**: the field §4.1 wanted **was never on the version it inherited**. No fixture could falsify it, because a fixture states what its author believes the venue sends ([[fixture-that-states-its-fact-twice]], one turn out) — so the falsifier had to be a field list read from the venue, and the WAF that blocks #180 is why nobody had one. A source class was the missing tool, not a sharper argument. |
| **A coverage consequence noticed rather than absorbed** — deleting the Binance posture read of positionRisk removed `ReasonAmbiguous`'s only **end-to-end** witness, since `symbolConfig` holds one row per symbol whatever the position mode is. | The reason keeps its producer (Bybit `position/list`, a genuine position enumeration) and gains a new composition test driving the **real** codec through the **real** ACL into the **real** rules. F19's own lesson is that a defect between two layers is invisible to either layer's suite, so silently dropping its composition fixture would have restored exactly the blind spot it was found in. |
| **F14/F20 → RULED, and the ruling was not the choice it was filed as** *(task #199, record §73p)* — the last open item asked whether a spot-only deployment must route the futures family, put as operator burden vs. coverage loss. Neither: the family set was derived from the **union** of a dialect's probes, which reads `Scope` (the granularity a rule is ANSWERED at) as if it were the rule's **subject** (which part of the deployment it GOVERNS). `observationKey`'s own comment already said those differ — `positionMode` is `ScopeAccount` on Binance and `ScopeSymbol` on Bybit, one subject at two granularities — and nothing had read it as a general fact about the table. | **A `subject` column on every rule** (account / derivatives leg / spot leg), with a family required iff some probe answers a **live** rule's setting, and every lapsed rule recorded in the verdict with the coverage that lapsed it. `Evaluated` excludes lapses, so the vacuity guard still refuses a verdict that evaluated nothing — an exemption can narrow what is asked and can never mint a clean answer. **One rule, opposite answers**: all five probed Binance rules govern the derivatives leg, so a spot-only Binance deployment issues **zero** probes and still enforces Portfolio-Margin (attested, no endpoint); a spot-only **Bybit** deployment still needs linear-perp, because `/v5/account/info` rides it and answers three account-subject settings. Nine mutants, all caught. |
| **The check the ruling made possible, and the claim it could not otherwise make** — deriving families from what a probe ANSWERS required the probe→setting relation, which until now lived only inside `DecodePostureStep`'s body where nothing could read it. | `PostureStep.Answers`, and with it a startup refusal when the rule table names a probed setting **no probe answers** — §7.6's fold-name check (#186) one surface over, and for the same reason: each half is internally consistent, so only the composition can see the disagreement. The coverage this check runs at is a **named constant**, because a check run at the narrow coverage still runs, still returns nil, and on Binance checks nothing — a claim that cannot be mutated where it lives ([[a-claim-that-cannot-be-mutated-wants-a-name]]). |
| **Two things the ruling exposed, neither absorbed** — (a) `Check`'s per-symbol loop claimed *"Requirement.Validate has already refused an empty roster, so this loop cannot be the vacuous one"*. **False from the session it was written**: the guard is conditional on `Coverage == RosterCoverageDerivatives`, and `RosterCoverageNoDerivatives` — added by the **same slice** — makes an empty roster valid. (b) §4.1's only end-to-end refusing-path witness violated `multiAssetsMargin` on a `spotOnly` deployment, justified in its own comment by *"it is an ACCOUNT-WIDE rule, so it fires on a deployment with no derivative instrument"* — the same conflation, in a test. | (a) Corrected in place; what makes the claim true now is the subject gate, which lapses every `ScopeSymbol` rule before the loop rather than running it over nothing. (b) The fixture moved to a deployment that trades the leg the violated rule governs, and the spot-only side gained its own pair. General: *a scope-shaped premise propagates into the fixtures, where it reads as covered because the assertion it makes is true of the rule in general.* |
| **A bound set at the harness's own limit is not a bound** *(found by re-aiming mutant M7)* — `TestProcessKillMidIOC` guarded its channel receive with `time.After(60 * time.Second)`, and `tools/mutate.sh`'s default `MUTATE_TIMEOUT` is **60s**. The two tied, the harness won, and any mutation that stopped the subprocess reaching the venue reported `TIMEOUT!!` instead of the `t.Fatalf` that prints the child's stderr and names the cause. | Reduced to 10s, matching every other bound in the repo (all ≤10s; this was the sole outlier) and ~20× the healthy path. General: *"a test that hangs is not a test that fails" has a second form — the guard exists, is correct, and the tool built to fire it can never see it do so.* |
| **What the ruling exposed and did NOT close** `[OPEN, task #201]` — Profiles §2's **Bybit** cell checks the spot leg in three places (spot-margin mode off; every coin's `borrowAmount==0` and `spotBorrow==0`); its **Binance** cell checks nothing about a spot leg at all. So a spot-only Binance deployment's whole §4.1 answer is one attestation and zero venue reads. | Filed rather than guessed: it is a question about the venue's published surface, blocked on the same WAF and missing credential as #180/#171, and §4.1's failure direction means a rule on a **guessed** field name would halt every Binance spot deployment. The asymmetry was invisible while every Binance deployment dragged in the futures probes regardless — the **mirror rule across venues**, surfaced by a ruling rather than by a review. |

**Milestone 3 task #197 is landed: §12's milestone 3 is RECORDED COMPLETE by a
closure pass that read item 3's every clause against the code, and the pass
found one clause listed with no implementation anywhere — `sizing-by-live-tier`
is reassigned to milestone 4 (v0.9.50, 2026-08-05).** Design record **§72**.

**Why the pass existed at all.** §12's item 3 carried a landing annotation for
every slice through **E (v0.9.35)** and then stopped, while the doc set had
reached **v0.9.49** — five slices and a twenty-eight-task tail unrecorded on the
roster, and no sentence anywhere saying the milestone was done. The first
question was whether the roster had been **deliberately demoted** to a summary
once ADR §3 became the traceability record, because that reading would make the
gap correct and the fix one sentence. It had not: the annotation is an explicit
close item in the **slice-C2a** record (*"§12's slice-C2a annotation"*) and in
the **slice-C2b** record (*"§12's slice annotation, ADR §3's dated block,
HANDOFF, v0.9.33 across all"*), it appears in **no** record from slice F1
onward, and **no ruling retired it**. It lapsed between two same-day sessions.
`adr-history` had already named this mechanism once, on the capture release:
roster rows left unmarked because *"no code asserts them yet"*, a state that
**expired silently**, where *"the close's reading pass is what re-derived it;
the grep, by construction, could not."*

**THE FINDING — a clause with no code, and the register knew where it belonged.**
Eight of item 3's nine clauses check out against named implementations. The
ninth, **`sizing-by-live-tier`**, has nothing: `MMR`, `maintenance margin`,
`leverageBracket`, `risk limit` and `marginTier` return **zero** non-test hits
across `internal/`, there is no `internal/risk/`, and no sizing function of any
kind exists. Four things make the disposition a reassignment rather than a debt:
**`C4`'s own traceability row in this document reads `| C4 tiered MMR | Profiles
§1, core §7.1 |`**, and §7.1 — *"Sizing for survival & the margin top-up
protocol"* — is a section §12 gives to milestone 4; `planning.go`'s `Quantity`
is a **designed** input (*"§6 step 4's sizing is responsible for having quantized
it"*) which `Plan` then **verifies** against the instrument's filters rather than
trusting, so the seam is deliberate and the single-writer OMS is the wrong home
for a producer; the live-tier MMR needs a signed `GET /fapi/v1/leverageBracket`
read that only milestone 4's account/margin-mode verification makes meaningful;
and the **tutorial's acceptance checklist had already paired them** as one
clause, *"live-tier MMR sizing + margin top-up protocol"*.

**What the finding is really about, and it is not the roster.** The venue fact
is **recorded and read by nobody** — profiles carries *"fetch `GET
/fapi/v1/leverageBracket` at runtime — table not otherwise readable `[→ C4]`"*
and its weight and auth class — and **eight comments across seven packages**
(`marketdata/application/funding.go`, `platform/events/coalesce.go`,
`platform/ids/instrument.go`, `platform/config/reload.go`,
`platform/observability/spec.go`, …) each name liq-distance as a quantity
computed *elsewhere*. Every one is locally true; together they are a chain of
deferrals with **no terminal**, and no gate can see that, because each comment
is correct and the grep polices only *marked* claims. **The failure direction is
the merciful one:** with no producer, `Plan` cannot be handed a size at all, so
nothing mis-sizes — which is exactly how the clause survived five slices, the
tail, and a formal-model campaign unnoticed. §47e finding 1 is the same defect
one milestone up (milestone 1 recorded complete with half of `engine/venue`
undelivered), which makes this the **second** instance and the roster the
artifact both were invisible on.

**Landed beside it (task #180, still open):** profiles §9 gains a **third**
unread candidate for the `positionRisk` weight, from a second AI summarizer, and
the block now records what the disagreement does and does not establish — two
independent summarizers agree on the **conditional shape** and disagree by a
factor of two on the multipliers ("5 with `symbol`, 20 without" against "1 for a
single symbol; 10 when the symbol parameter is omitted"), so the shape is
corroborated and the value is not. The engine's charged **20 is the maximum over
all three candidates**, correct-or-pessimistic under each, and **no further
candidate below 20 can move the code** — the property that makes the task safe
to leave open. The second summary's three tells are recorded so it is not
mistaken for a source later: one aggregated catalog page answering every
question, V2 and V3 consequently reported as carrying the byte-identical weight
line (which *weakens* the earlier "V2 now says 10" report rather than
corroborating it), and changelog citations that are search-engine redirects
rather than the changelog.

| Finding | Resolution |
|---|---|
| §12 item 3's annotation trail stops at slice E while the docs are at v0.9.49 — five slices and the #169–#196 tail unrecorded, and no sentence states the milestone complete | The consolidated annotation lands with per-slice task, record and version citations (F1 #165/§55/v0.9.36 · F2 #176,#178/§56/v0.9.37 · G1 #166/§57/v0.9.38 · G2 #166,#168/§58/v0.9.39 · G3 #181/§59/v0.9.40, tail v0.9.41–v0.9.49 records §60–§71), and item 3 now states completion |
| Was the roster deliberately demoted, making the gap correct? | **No** — an explicit close item in the C2a and C2b records, named in none from F1 onward, retired by no ruling. It lapsed; the annotation says so in place |
| `sizing-by-live-tier` is listed as milestone-3 content and has no implementation anywhere | **Reassigned to milestone 4** on four grounds, `C4`'s own row first. Struck from item 3's contents by annotation (the text is preserved, per milestone 1's scope-correction precedent), added to item 4 beside the §7.1 top-up protocol |
| Eight comments across seven packages defer liq-distance to a computation no package owns (the pass's own first count said "nine across five" — the ninth matched on `headroom`, not liq-distance; re-derived before the close) | The chain now terminates in milestone 4 **by name**. No code change: nothing sizes, so nothing mis-sizes — a halt, not a wrong number |
| A third unread candidate weight for `/fapi/v3/positionRisk`, from a second summarizer disagreeing with the first by 2× | Profiles §9 records all three, the shape-vs-value split, and the second summary's three tells. Charged weight unchanged at **20** = max over candidates; #180 stays open, closable only by a direct page read or the venue's `X-MBX-USED-WEIGHT-1M` header |

**Milestone 3 task #188 is landed: a package reserves its whole rate-limit
cost in ONE all-or-nothing governor acquisition before any leg's
authorization fsync — untimed occupancy, invisible to the header fold,
consumed at the last pre-send instant re-checking only the venue's word,
blanket-cancelled on every executor exit — and a credentialed transport
cannot be CONSTRUCTED without the SpendDomain-keyed flock bound to its
credential's fingerprint, which machine-checks the per-host slice of §67's
one-spender premise (v0.9.47, 2026-08-05).** Design record **§68** — eleven
adversarial design rounds to SHIP (twelve recorded rejections, §68d),
implementation built VERBATIM from Appendix A by a second-model session,
then the orchestrator's post-write review (five findings, below) and the
§68e collapse checklist run BEFORE the campaign. The spend-lock key evolved
credential → venue alias → closed signer-answered `SpendDomain` (rounds
5/7); the boot-detectable post-fsync genus grew to four members closed at
`NewSignedClient` (base-URL canonicality + `WriteProxy` dry-run,
signing-readiness, lock binding, credential control bytes); the token's
lifetime is the owned-REACHABLE-capability rule (round 9 correcting round
8's fold against the object graph). Fourteen-mutant campaign, zero
SURVIVED: 12 harness kills, 1 crash-kill re-aimed and re-verified as an
assertion kill on the single fixture, 1 multi-line swap demonstrated
red/green by hand; `holds[i]`→`holds[0]` killed by the port-contract
fixture's DISTINCT holds while remaining behavior-equivalent in the shipped
single-venue composition, recorded as such. Task **#188** closes.

| Finding | Resolution | Lands in |
|---|---|---|
| **`Hold.Consume` refused a zero-charged window after a fold past capacity (post-write review, DEFECT — Appendix A itself specified the unguarded form).** A fold ratcheting `used` past capacity on a window an entry charges ZERO units on failed the first conjunct and refused a leg whose charge cannot violate that window — manufacturing the exact post-fsync episode the slice removes. | Consume mirrors `admits`' `units == 0` guard; the §68e discrimination fixture (fold past capacity on W, consume an entry charging 0 on W and >0 on its sibling) was demonstrated red then green; A-2 is corrected in the record. | `ratelimit.Hold.Consume` |
| **A-18's line-order claim had no observer (post-write review).** The reserved lines render "after the fold-miss lines", but no fixture combined `FoldMisses` and `Reserved` in one snapshot — swapping the two render loops passed every test, and the golden had not been extended as A-18 asked. | The golden full-report fixture carries both non-zero; the loop swap was demonstrated to fail it in both directions. | `reconcile_test.TestReconcileReportGoldenNoMiss` |
| **§68e's reserved-heavy admits boundary was specified and absent (collapse checklist).** Every admission refusal fired via `used >= ceiling` or with `reserved < avail`; the `reserved >= avail` guard — whose naive replacement wraps uint64 and admits everything — had no fixture, the §66-round-6 non-underflow lesson unlearned on the new arithmetic. | Boundary fixtures at `used+reserved == ceiling−1` (admits) and `== ceiling` (refuses via the reserved clause), plus the cross-priority underflow corner (`reserved 6 > avail 4` under a lower ceiling, refused `ReasonCeiling`); the clause-deletion mutant is killed by them. | `reservation_test.TestAdmissionCountsReservedAtCeilingBoundaries` |
| **§68e's alias-pair lock fixture was specified and absent (collapse checklist).** One credential under two same-dialect `ids.Venue` aliases — the pair that BOOTS under the rejected alias key — was never constructed; the single-alias fixture proved constant-not-derived but not the collision. | Both aliases constructed over one credential, assert both answer `SpendDomainBinance` and the second acquire refuses `ErrSpendLockHeld`; the fixture guards its own collapse by asserting the aliases differ. | `spendlock_internal_test.TestAcquireSpendLockCollidesAcrossVenueAliasesInOneDomain` |
| **`MUTATE_TIMEOUT=180` — §68e's own spelling — silently breaks every run it protects (campaign).** The harness passes it to `go test -timeout`, which rejects a unitless duration at flag parse: non-zero exit, zero `--- FAIL` lines, classified `NOBUILD!!` — a verdict about the mutant issued by a run the mutant never saw. Demonstrated live on the error-arm mutant, which by hand is killed exactly as designed. | Campaign ran with `180s`; §68e's spelling corrected in the record. The genuine `NOBUILD!!` beside it (overflow-check deletion orphaning `math`) was re-aimed to an unfireable-comparison substitution and killed. | record §68e; `tools/mutate.sh` invocations |

**Milestone 3 task #183 is landed: the `ProvenanceFetched` writer §40d
anticipated exists for LOT FILTERS — `arblab reference fetch` reads all four
cells' public endpoints unsigned (`transport.PublicClient`, the Poller's
params-carrying sibling that refuses to follow redirects), binds every
response to its whole cell (base, quote, trading status, contract type;
HTTP 200 before any decode) and writes a NEW dated document under
`O_CREATE|O_EXCL`, venue decimals verbatim — and the trading-filter age is
no longer boot-only: `TradingFilters.ObservedAt` is required, and the
Executor's preflight re-judges every leg's row at ONE admission instant per
package against the declared `FilterMaxAge`, refusing stale rows and,
separately, rows from the FUTURE (v0.9.48, 2026-08-05).** Design record
**§69** — one adversarial design-review round returned five majors, all
validated and folded in (the table below); implementation split between the
orchestrator (transport, domain carrier, executor seam, wiring) and a
second-model session building the fetcher from §69b; the §69d collapse
checklist ran before the campaign (the one-instant claim needed a STEPPING
clock — on a fixed fake clock, one read and per-leg reads are
observationally identical). Twenty-mutant campaign, zero SURVIVED: 19
harness kills, one NOBUILD re-aimed (the sed orphaned the file's only
`http` use) and re-verified as a kill. Task **#183** closes; the planner's
freshness mirror and hot-reload are recorded §69b deferrals that land with
the engine's own loop, and #184's Dispatch surface is untouched.

| Finding | Resolution | Lands in |
|---|---|---|
| **A non-2xx or redirected response could earn a `fetched` stamp (design review, major 1).** The transport returns non-2xx as RESULTS by design, and the default redirect policy would attribute another host's body to the configured URL — the provenance `source` claim. | The writer requires HTTP 200 before decode (proven by a 429 carrying a poisoned body: decode-first would return the wrong sentinel); `PublicClient` surfaces redirects as 3xx results via `ErrUseLastResponse`, with the redirect target proven unfetched. | `driven.FetchLots`, `transport.PublicClient` |
| **Cell binding omitted quote, status and contract type (design review, major 2).** A SUSPENDED instrument keeps schema-valid numeric filters, so a fresh conforming row would green-light a leg the venue rejects — the asymmetric-rejection harm arriving through the freshness mechanism itself. | Decoders extract the venue's identity words; the writer refuses base/quote/status/contract-type disagreement before stamping. Failure-matrix fixtures poison each field independently. | `driven` decoders + `FetchLots` |
| **A backward clock step made stale rows read as FRESH (design review, major 3).** `now.Sub(ObservedAt)` goes negative and passes `age > bound` trivially; the executor holds a map resolved before the step and never re-consults the store's period-correct guard. | `ObservedAt.After(now)` refuses with its own sentinel (`ErrFiltersFromTheFuture`) — the fix differs from staleness: investigate the clock, not fetch a revision. | `Executor.preflight` |
| **The `-out != input` overwrite guard was path-textual (design review, major 4).** Symlinks, aliases and an unrelated existing target all bypass a string comparison. | `O_CREATE\|O_EXCL` refuses EVERY existing target identity (the `dispatch/logwriter` precedent), with partial-write cleanup; asserted against a pre-existing file whose content is proven unchanged. | `cmd/arblab reference.go` |
| **The one-instant-per-package claim had no discriminating fixture (design review, major 5).** On a fixed fake clock, `Now()` per leg and `Now()` once are observationally identical, so the mutation aimed at the claim would survive while the campaign reported it checked. | A stepping clock whose first read is inside the bound and second read outside it: the package must admit (one read), and the per-leg mutant refuses leg 2. Killed. | `execute_freshness_test.go` |


**Milestone 3 task #179 is landed: §7.5's net-delta invariant covers the SPOT
leg — a recorded baseline `baseline := B − ours` per `(venue, account, base
asset)`, recorded by `arblab baseline record` through a six-step protocol
(gauntlet over the terminal-and-concluded join, a `spot_settle` wait, a
re-read of BOTH surfaces requiring both unchanged, verify-back through the
real loader), stored as an inputs-not-conclusions document chain-marked to
the journal head, pinned by the two-act `{path, sha256}` config edit, and
asserted per baselined asset inside the join, where only a delta STABLE
across a second look latches; `UnassertedNoPositionSurface` is DELETED and
`UnassertedBaselineUnrecorded` HOLDS the posture — §7.5's must-not-hold
exemption is retired with its expired premise (v0.9.49, 2026-08-05).**
Design record **§70** — three adversarial design rounds to SHIP (5+4+5
majors plus two minors, all validated and folded in place), implementation
across three safe points, then the §70h item 1–24 audit at close — which
falsified the safe-point summaries' own coverage claim ("§70h 5–6 partly in
join_test": `Joiner.assertSpot` and the Reconciler's second look had NO
tests at all; `join_spot_test.go` and `reconcile_spot_test.go` close it) —
and the campaign: **20 mutants, zero SURVIVED** (four NOBUILDs, every one
an orphaned-reader bad sed, each re-aimed and killed; results table at
§70h's tail). The final fresh `code_diff` review then returned **four more
majors — all validated and folded (§70i)**, with an eight-mutant addendum
zero SURVIVED (campaign total 28); its four rows close this table. Task
**#179** closes.

| Finding | Resolution | Lands in |
|---|---|---|
| **"Recorded when" was a predicate and needed to be a PROTOCOL (design review, round 1 major 1).** Terminal state does not close the fill ledger — §5.3 admits a post-cancel late fill indefinitely — and the join's order read, executions sweep, and balance read are three independent calls with no common snapshot, so a one-shot record could mint a baseline straddled by a late-reporting execution. | The six-step protocol: replay → join gauntlet (Unattributed the non-negotiable refusal) → read B → wait `spot_settle` → re-read BOTH surfaces requiring both unchanged → write; every arm refuses rather than waits, and the result is verified back through the real loader. | `oms.RecordSpotBaseline` |
| **The virgin deployment PASSED vacuously (design review, round 2 major 1).** `Assert` derived its instrument sets from dispatched orders, so configured spot instruments + an empty journal + no baseline meant empty sets everywhere — the exact "satisfied on a boot that issued no query" failure §7.5's scope paragraph closes for derivatives. | `NewJoiner` takes the configured spot roster and `Assert` walks THAT: every configured spot instrument is answered for — asserted against its baseline or reported `UnassertedBaselineUnrecorded` — whether or not any order ever existed. | `Joiner.Assert` |
| **A stored scalar `ours` was an attestation, and a path-pinned document was replaceable in place (design review, round 2 major 4).** Schema-valid arbitrary bytes with a true chain mark would have loaded, and `O_EXCL` guards creation only — an edited-in-place document changed the effective baseline with no second act. | The document stores verbatim balance words and per-order rows; the loader RECOMPUTES `B`, `ours`, and the baseline, cross-checking every stored order id against the chain-marked journal prefix; the config pin is `{path, sha256}`, so replacement without the config edit is a named refusal. | `baseline.Load`; config `spot_baseline` |
| **The Binance account read is weight 20, and a summarized fetch had misread it as 10 (design review, round 3 major 1).** The wrong number under-charges the governor by half on every reconcile-boot balance read — the silent direction, toward the ban. | Settled by exact grep of the raw doc set; Profiles §4 corrected; an in-test assertion pins the charge at 20 and the 20→10 mutation is shown killed — the governor charges the caller's word, so the number is a claim, not a constant. | `balances_binance.go`; Profiles §4 |
| **A late fill visible in the wallet before the trade list would falsely LATCH at boot (design review, round 3 major 2).** The balance read and the sweeps are independent calls, so a self-clearing propagation gap froze into an operator incident — §7.5's own named failure, arriving through the new invariant. | The second look: on a non-empty delta the Reconciler waits `spot_settle`, re-runs the JOIN (idempotent by the fill ledger's dedupe), re-asserts; an identical delta latches, a changed one is `OutcomeIncomplete`. A balance-only bracket was rejected because the projection is frozen between Asserts — only a re-sweep can move `ours` to meet a settled wallet effect. | `Reconciler`; `reconcile_spot_test.go` |
| **A baselined asset that is another spot pair's QUOTE makes consideration flows invisible to `ours` (design review, round 3 major 4).** A buy on ETHBTC debits wallet BTC by the executed consideration while the BTC formula sees only fees — a fully journaled order would latch a false delta. | Default-deny at construction: roster rows carry the quote beside the base, and a baseline for any roster row's quote refuses by name; the quote-flow retention alternative is recorded as rejected, so the wrong number is inexpressible rather than computed. | `validateSpotLeg` |
| **The safe-point summaries OVERSTATED coverage, and the audit falsified them (session-105 §70h audit).** "§70h 5–6 partly in join_test" was a claim nobody had verified — every joiner fixture passed `SpotLeg{}`, so `assertSpot` and the second look had no tests while two sessions' summaries read as covered: true prose on a run that stopped short, caught by the checklist's own item-by-item pass. | `join_spot_test.go` (cell arithmetic, all three fee shapes incl. the perp fee that must NOT count, both delta signs, absence-as-zero, the shared-base cell, the quote-overlap deny with its positive control) and `reconcile_spot_test.go` (stable/moving/explained/unwired/precedence, the venue mutated INSIDE the injected Wait); the 20-mutant campaign ran only after the gaps were filled. | `internal/oms/application` tests |
| **Binance absence was read as zero while §70d's own sentence said it concludes nothing (final review, major 1).** The port declared absent-means-zero universally, so join, record and loader all minted a quantity from silence on the venue whose documented shape contains every asset — a checker consuming a value marked "not a conclusion", and the empty-body refusal's argument stopping one input short of the mechanism. | The enumeration kind is a CARRIER stamped by the codec (`AbsenceSemantics`, zero value refused at four seams): Bybit's silence stays the venue's zero; a Binance absence is `SpotUnconcluded` at assert, `ErrRecordBalanceAbsent` at record, `ErrStoredAbsenceUninterpretable` at load. The mislabelled absence-as-zero fixture is relabelled with an inconclusive twin added. | `ports.BalanceReport`; both codecs; `assertSpot`; `recordBalanceEvidence`; `baseline.Load` |
| **The second look re-asserted over a mid-bracket disappearance (final review, major 2).** The bracket checked only `Incomplete`/`Unattributed`, so an order the venue held at look 1 and not at look 2 bypassed §5.3 and could latch a delta computed over an unresolved absence — the normative join → resolve → assert ordering violated inside the construct built to prevent false latches. | `newAbsence` downgrades to `OutcomeIncomplete` for NEWLY absent orders only — an order absent at BOTH looks was resolved before the first assert and re-reports absent on every boot, so treating it as motion would leave a deployment holding one `AbsentAfterFinality` order unable to latch a real delta for ever. Each direction has its own fixture and killed mutant. | `Reconciler`; `reconcile_spot_test.go` |
| **The loader trusted stored side/base/fees while the record claimed only the cumulative was attested (final review, major 3).** A chain-valid re-pinned document could flip a side and move the recomputed baseline by twice the cumulative; the check also caught the fixture whose document side contradicted its own journal. | The prefix visitor collects side per id and a contradicting row refuses (`ErrOrderRowSideDisagrees`, both flip directions tested); §70b corrected in place — attested = cumulative, fees, base attribution. The bijection demand is rejected with the argument: an omitted row's fills stay in the live ledger, so the invariant itself latches on the omission at the next assert. | `baseline.Load`; record §70b |
| **`oms/infrastructure/baseline` violated §3.6 rule 3 unmarked (final review, major 4).** The position was chosen deliberately in §70b ("under oms BY POSITION") but the permitted-children rule was never edited — a deviation absorbed rather than marked, invisible to the OWED grep because the claim was never written down, found by the reading pass the close prescribes. | Rule 3 gains the documented exception with the load-bearing argument: under `driven/`, every root constructing §5.2's pin would acquire the whole write-adapter surface with the import — the shape rule 6 exists to make undeclarable. Scoped to ONE named package, never a kind. | core §3.6 rule 3 |

**Task #153 is ruled and landed: `restart_boundary` was one name over a PRODUCT
of two properties, and a REST-overlay source made the missing axis chronic — a
seam whose predecessor also ended at zero, written by the same `(host, pid)`, is
now `unsequenced_boundary`: still reported, still coverage, and the one finding
kind that does not flag its window (2026-08-05).** Design record **§71**. The
motivating measurement was 24 of 50 findings on a clean day, one per hour per
`fundingInfo` source, scaling with every REST overlay added — slice E settled
that a Rest record is not a Data record, so `lastSeq` never leaves zero and every
hourly rotation declares no predecessor truthfully. The classifier now reads two
facts the archive already carried and nothing consulted: what the predecessor
ended at, and the Open record's `Host`/`PID`. **No set-version change and no
normative section moves** — `restart_boundary` appears in none of the five
documents; the capture tool's taxonomy lives in records §32/§71, and §32k gains a
forward pointer so it cannot be cited for the retired reading. Twenty-two
mutants, **two SURVIVED and both were real fixture defects**, fixed and killed
(§71g). Task **#153** closes.

| Finding | Resolution | Lands in |
|---|---|---|
| **The task's own framing understated the damage: the verdict column had been wrong for months.** `assignVerdicts` flags a window on ANY finding but `window_absent`, so every REST-overlay hour reported `VerdictGapped` — §32k's answer to *what would a replay of this hour get* — for an hour in which a living writer rotated a file and lost nothing. Nobody had looked past the finding list. | The new kind is verdict-neutral, which is the TRUE answer rather than the quiet one: a replay gets everything the archive holds. `AffectsVerdict()` is an exhaustive switch with no default, so the decision is made per kind at a site the compiler and `exhaustiveswitch` both check instead of inherited from a `for` loop. | `FindingKind.AffectsVerdict`, `assignVerdicts` |
| **The identity comparison must narrow a NAME, never remove a row (rejected alternative).** Keying the rule on `(host, pid)` alone is cheaper and one premise short: PIDs are reused, so `pid == pid` across a seam consumes a non-reuse guarantee the OS does not offer, and a genuine restart landing on a recycled PID would be SILENT. | Identity is a conjunct beside the predecessor's own end. The same accident now mis-*names* a row that is still on the report, which is the difference between a weaker answer and no answer. Suppression by source class was rejected separately: it keys the archive's classification on configuration the archive does not carry. | record §71d |
| **The roster that polices producers could not police itself (§71e).** `finding_test.go`'s `declaredFindingKinds` map — whose own doc says a kind with no producer is a member no code path can reach — had drifted by FIVE members: every manifest kind was added to `finding.go` and to `Class()` and never here. A test that ranges over a map cannot see the member that is not in it. | The map becomes a `producerFor` SWITCH, and `exhaustiveswitch` is deliberately in scope for a switch in a `_test.go` file over a production enum. Demonstrated by deleting one case and watching `make lint` fail by name — not asserted. | `finding_test.go` |
| **A fixture that stated its fact twice (mutation M9).** The adopted-claim test SEALED its silent hour, and a sealed file states its sequence in both its Open claim and its Close payload — so deleting the Open-claim half changed nothing and the test passed against broken code. | The silent hour is unsealed (a crashed hour, also the realistic shape) and the Close record's half is its own table row. The collapse checklist's lesson one layer in: the fixture was varied correctly and a second source of the same fact absorbed the variation. | `verifyboundary_internal_test.go` |
| **A fixture whose RANGE excluded the state it planted (mutation M13).** The stale-identity test built three files so an identity could go stale across the middle one, then asked verify for the third window only — and verify walks exactly ONE file earlier than the range, so the first file was never read and the stale identity never existed. | The request names two windows, which also makes the readable control expect two boundaries. Both survivors share a shape: the mutation was aimed correctly and the fixture could not observe it — neither was findable by reading the test. | `verifyboundary_internal_test.go` |
| **The truncation conjunct's stated premise was weaker than the one that holds.** It was written as "the Data record may be in the unread bytes", but a successor's `prev_last_seq` comes from the WRITER's own `lastSeq`, not from the predecessor's bytes — so unread Data would already show as a non-zero claim. | The premise is restated to the one that bites: an unread tail can hold a second OPEN record (§38i's reopened file), so the identity being compared against may not be the last one written. Same rule, correct reason. | `chainState.truncated` doc |

**Milestone 3 task #187 is landed: a mapped Bybit response the API
demonstrably handled that yields no usable rate-limit observation is a
counted FOLD MISS — a monotone per-limit counter in the governor snapshot,
rendered historically by both roots beside §7.6's ban alert, with admission
untouched and the derive-nothing ruling intact (v0.9.46, 2026-08-04).**
Design record **§67** — four adversarial design rounds to SHIP, then a
code_diff round. The predicate: mapped AND no usable observation AND
(any 2xx not classified as the IP ban, OR classified as a throttle),
decided after `ClassifyResponse` on read bodies and on status alone at the
two body-failure exits (where the penalty goes unrecorded too, so the miss
is the response's only signal). Count-and-alert is scoped by a RECORDED
OPERATIONAL premise — one spender per UID — whose enforcement home is
#188 and whose reopening trigger is the first second concurrent spender.
26 targeted mutations, 26 kills (25 harness + 1 demonstrated-SURVIVED then
killed). Task **#187** closes.

| Finding | Resolution | Lands in |
|---|---|---|
| **The pure 2xx guard missed the endpoint meter's own refusal (design round 1, major).** Bybit does not document which HTTP status carries 10006; a 403+10006 with stripped headers — the co-consumer exhaustion scenario the task was filed about — left the counter at zero. | The miss decision reads the classified PENALTY: any throttle counts, whatever its status. The IP-ban track stays out on the recorded unheadered-gateway fact. | `transport.noteFoldMiss` |
| **The header-time 2xx decision counted the IP ban wearing a success status (design round 2, major).** HTTP 200 + retCode 10018 classifies `PenaltyBan` only after the body is read; deciding at header time counted the one response whose headerlessness is expected — beside the ban alert it undercuts. | The decision moved after `ClassifyResponse` with a `PenaltyBan` suppression; the header-time arm survives only on the two body-failure exits, documented. | `transport.Do` |
| **`arblab exercise` rendered the monotone counter twice (design round 1, major — pre-existing shape, made common by #187).** Boot report and post-execute snapshot each rendered rate-limit status; two cumulative counts read as two incidents. | The boot renderer split (verdict + ratelimit + diagnosis); exercise renders ONCE from the final snapshot when executed, the boot snapshot otherwise. | `cmd/arblab` |
| **The one-spender premise was implicit and #171 was the wrong trigger (design round 1, major).** Every root builds a process-local governor; nothing structural prevents two concurrent Bybit spenders, and #171's subject is signer validation, not credential sharing. | The premise is recorded as OPERATIONAL with its enforcement gap named (#188 owns it); the live-execution user decision must confirm single-spender deployment; #171's role corrected to baseline measurement. | record §67a |
| **Call-site arguments were claims no helper test could see (design round 3, major).** Either early-return call site hardcoding `false` for folded or `200` for status survived every planned test. | Per-site zero-fixtures (valid-headers → 0, non-2xx → 0) and four per-site argument mutations, all caught. | `signed_internal_test` |
| **"Byte-identical reconcile output" was a claim with no test (code_diff round, major — demonstrated live).** `"blocking: %v"` → `"blocking=%v"` SURVIVED the whole suite; substring assertions cannot see format drift. | A golden test pins the complete no-miss report string; the demonstrated mutant now fails it. | `reconcile_test.TestReconcileReportGoldenNoMiss` |
| **A rebuttal that survived: retCode-10006 provenance for the non-2xx arm (design round 2).** Excluding a headerless bare 429 would rest on an unverified venue claim to blind a diagnostic whose false positive costs a glance and whose false negative is a throttle-storm blind spot. | Rejected, recorded in §67c; #171's baseline measurement is the arbiter with a named loosening path. | record §67c |
| **The render line's tense was a claim about NOW from a historical counter (design round 3, minor).** One miss followed by clean folds would read "corrections are not arriving" forever. | The line's contract is historical ("N responses this run"), honest for short-lived roots; rate needs #171's external denominator, recency is §10's on the `LimitState.FoldMisses` seam. | `cmd/arblab.writeRateLimitStatus` |

**Milestone 3 task #185 is landed: Bybit's per-endpoint budgets are
expressible, charged, folded, and required — `ratelimit.ScopeEndpoint` is a
VENUE-level window per endpoint (family refused: the per-category counter
split is unverified, one shared window is conservative under either reading,
and a family-keyed window makes a two-family package non-atomic), the name is
path-derived through the dialect's closed `{path, name, dimension}` registry
with `Call.Limit` DELETED, and construction requires the complete
dimension-checked roster (v0.9.45, 2026-08-04).** Design record **§66** — six
adversarial design rounds BEFORE code, then a code_diff round after. Three
pre-send governor refusals (R1 unbudgeted, R2 unnamed, R3 zero-charged); the
cannot-count rule counts a matched endpoint window's own dimension (Bybit's
order meter IS the create endpoint's budget); Bybit `CheckFold`'s first real
body (roster name + registry dimension + exactly-1s window; family names may
not squat); the header fold moved ahead of the body read (a read failure
discarded the 429's own reading); the PRE-EXISTING `freeAt` uint64 wrap fixed
(round 6). 19 targeted mutations, 19 kills (17 harness + 2
demonstrated-and-reverted). **#187** and **#188** are filed. Task **#185**
closes.

| Finding | Resolution | Lands in |
|---|---|---|
| **Family-keyed endpoint windows make a two-family package NON-ATOMIC (round 3, blocker — the design-changing one).** The executor fans legs concurrently with no governor preflight, so under any single-ownership rule one leg sends while its sibling refuses deterministically: a naked leg by configuration. | Endpoint budgets are VENUE-level, keyed `("", name)`; the uniqueness rule and single-family carrier dissolved. Cross-family sharing is an executable test. | `ratelimit.Limit.validateFamily`, `governor_test` |
| **Declared-name substitution survives every request-time check (round 1, major).** An `execution_list` call wired as `order_realtime` charges the wrong budget and folds the wrong window, invisibly. | The name is DERIVED from the path by the reader's closed registry; `Call.Limit` deleted — the class is inexpressible, not checked. | `transport.LimitReader.Endpoint`, venue call sites |
| **Request-time-only completeness strands §6.7 recovery (round 3, major; round 4 added the dimension).** A profile missing `order_history` boots and trades, then R1 refuses the Safety-priority fallback at the recovery moment; a name-only roster check passes `order_history` declared as Orders and R3 refuses it at the same moment. | `LimitReader.Endpoints()` exposes the registry `{name, dimension}`; `NewSignedClient` requires all five, dimension-checked; Bybit `CheckFold` enforces the same per declared limit. | `transport.NewSignedClient` |
| **A 100ms endpoint interval over-admits 10x and header folds cannot repair it (round 1, major).** Observations age out on the same wrong interval. | Bybit `CheckFold` requires EXACTLY 1s — the venue's published windows are all `/s`. | `transport.BybitLimitReader.CheckFold` |
| **`freeAt` wraps when `used > ceiling` — a state the package explicitly produces (round 6, PRE-EXISTING).** The observe ratchet pushes `used` above any ceiling; `ceiling-used` wraps in uint64, a refused request reports RetryAfter 0 ("retry immediately", false), and `worseThan` can name the wrong worst limit. The existing table stopped at used=60/ceiling=100 — the collapse checklist's pinned-below-the-boundary tell. | The comparison rewritten in the non-underflowing direction (`used <= ceiling-need`), mirroring `admits`; four new window tests reach above the ceiling. | `ratelimit.window.freeAt` |
| **"Saturate the window on missing headers" REBUTTED (round 2); fold health FILED, not absorbed.** A guessed cap is a budget quietly rescaled — the spec's derive-nothing ruling is deliberate; self-charging continues and the upward ratchet recovers on the next read. | Filed as **#187** with the seam named: this slice's derivation is what makes "mapped call, headerless response" detectable at all. | task #187 |
| **The concurrent-leg capacity race is PRE-EXISTING and its loser is NOT a clean unsent (rounds 4–5).** Acquire-per-leg is the shipped design; the losing leg's refusal is pre-send but post-fsync — `classifyEffect` maps it to `StateSubmissionUnknown`, a durable authorized order needing resolution. | "Atomic" withdrawn from the brief; package-level reservation filed as **#188** carrying the acceptance-test obligation. | task #188 |
| **The composed test's assertion could not see the pre-send charge (code_diff round, major).** `Used == 40` after charge(1)+fold(40) is what the fold alone produces — replacing `Acquire` with `Check` in `Do` survived the suite. | A headerless-success case asserts `Used == 1` with every sibling endpoint at zero; the `Acquire`→`Check` mutation was demonstrated to fail it, then reverted. | `signed_internal_test` |
| **The latest-refusal fixture collapsed evaluation order onto worst-selection (code_diff round, minor — the collapse checklist's tenth instance, a NEW dimension: ORDER).** The envelope was both first-evaluated and latest-releasing, so "name the first refusal" and "name the worst" agreed; skipping the endpoint after an earlier refusal survived. | Two subtests, one per ordering; the skip mutation demonstrated to fail the inverse case, then reverted. | `governor_test` |

**Milestone 3 task #186 is landed: a profile limit of a headered dimension
must bear EXACTLY the name the dialect's reader derives — checked at signed-
client construction, where the reader/governor pair is formed — and the
runtime reader now canonicalizes header windows through the same renderer the
check uses, so the certified name and the minted name cannot disagree
(v0.9.44, 2026-08-04).** Design record **§65**. `transport.LimitReader` gains
`CheckFold(name, dimension, interval)`; `BinanceLimitReader` enforces
`weight_<window>`/`orders_<window>` for headered dimensions and refuses a
requests budget only under an exactly-derivable name (round-trip through the
shared parser+renderer, window-independent); `BybitLimitReader.CheckFold` is
deliberately vacuous with the argument recorded (#185 owns its real body);
`NewSignedClient` walks `Governor.Snapshot().Limits` and refuses construction
(`ErrLimitFoldUnaddressable`) — no composition root can forget a check that
runs where the pair is built, and `wire.go` gained zero code. Core **§7.6**
gains the fold-addressability clause; `Limit.Name` and `LimitConfig.Name` doc
contracts corrected. Task **#186** closes.

| Finding | Resolution | Lands in |
|---|---|---|
| **The boot check and the runtime reader derived names from DIFFERENT inputs, so the check could certify a fold that never arrives (second-model round 1, blocker).** The v1 check rendered a canonical name from the limit's interval while `Observations` preserved the header's own spelling — Binance's grammar is (intervalNum)(intervalLetter), so one minute can legally arrive as `1M` or `60S`, and a certified `weight_1m` beside a runtime-minted `weight_60s` passes boot while the fold still drops | `Observations` canonicalizes: parse the suffix to a duration, re-render through the SAME renderer `CheckFold` uses — the derived name is a function of the WINDOW, not the venue's spelling. The memory rule "a recomputed check must share the anchor", caught before the code existed | signed.go, §65b |
| **A prefix-based collision rule turns a legal guardrail into a boot outage (round 2, major).** Refusing every requests-dimension name under `weight_*`/`orders_*` refuses `weight_guardrail`, which no header can ever derive | Exact derivability: strip the prefix, parse, re-render, refuse only when the round trip reproduces the full name — precisely the set the canonicalizing runtime can mint. The predicate is window-INDEPENDENT (`weight_1m` at a 5s interval still collides — Observe checks neither dimension nor interval), with its own discriminating rows and a coupling mutation killed by them | signed.go CheckFold, §65b |
| **"All existing profiles conform" was generalized from the sites checked, twice (rounds 3–4, majors).** The exercise chaos JSON, then the TYPED `LimitConfig` fixtures — exercise `validConfig` and reconcile `spotOnly`, the latter hidden because the file's PRIMARY fixture was #173-renamed — all declared `spot_weight`-family names. Third-through-fifth live instances of the defect class: every exercise run and spot-only reconcile run silently dropped every venue reading | All three fixture families migrated (pure renames; every interval already matched). The inventory now spans all three shapes a profile takes: `ratelimit.Profile`/`Limit` literals, `LimitConfig` literals/builders, serialized documents | chaos_test.go, exercise app_test.go, reconcile app_test.go, §65b |
| **Naive unit multiplication wraps a parseable suffix into a name it never meant (round 4, minor).** `2^55+1` seconds times `time.Second` wraps to EXACTLY 1s, so header `X-MBX-USED-WEIGHT-36028797018963969S` would mint `weight_1s` instead of being dropped | Checked multiplication in the shared parser (`n > MaxInt64/unit` → not a window), asserted at the parser and through the header seam; the loosened-bound mutant dies to both | signed.go parseBinanceWindow, §65b |
| **The field contracts contradicted the new validator (round 5, major).** `Limit.Name` said "ours to choose, not the venue's: the adapter maps the header onto whatever this profile calls it" — false for headered dimensions under Binance the moment this landed; `LimitConfig.Name` echoed it | Both doc contracts corrected in the same commit; compatibility note in §7.6 (syntax unchanged, pre-#186 names now refuse at boot — the refusal is the feature, the config was already degraded; telemetry series identity noted, no deployed consumer exists) | limit.go, config.go, §7.6 |
| **The reviewer's duration-aliasing objection was REBUTTED, and the rebuttal adjudicated (rounds 5–6).** Spelling-tuple fold identity would reintroduce round 1's silent loss — the observed defect, five instances — to guard against a venue shape (two duration-equal limiters of one type+family) no verified roster contains; and if it ever occurred, the upward-only fold merges CONSERVATIVELY (max wins, refusals come sooner) while the status quo drops one header entirely | Duration identity stands; the rejection is recorded in §65c and the dual-header concurrent test banks the merge (both spellings fold to one window, which holds the max) | §65c, signed_internal_test.go |
| **One expected-equivalent mutant, documented like `Reserve()`'s.** The parser's `n == 0` refusal is unreachable harm: both consumers re-render through `binanceWindow`, which refuses a zero window one step later on every path | Kept — the guard is the parser holding its own contract ("a parsed window is a positive renderable duration") rather than leaning on the renderer's refusal; 13 targeted mutations, 12 kills + this one diagnosed equivalent, three bad seds re-aimed on the "declared and not used" discriminator | signed.go, §65d |

**Milestone 3 task #173 is landed: §7.6's "a governor MUST refuse a cost it
cannot count" is two enforced layers — the governor errors on a cost charging
a dimension its applicable budgets do not count, and a profile is rejected at
construction unless every family budgets every chargeable dimension or writes
down why not (v0.9.43, 2026-08-04).** Design record **§64**.
`ratelimit.Profile` gains `Uncounted` (family + dimension + written reason, no
wildcard) and the coverage rule; `Limit.Validate` reserves venue scope for the
request envelope; `Governor.decide` gains the cannot-count refusal
(`ErrCostUncounted`, an error like `ErrFamilyUnknown` — no waiting repairs a
profile-versus-adapter disagreement); `Cost` gains the `RequestOnly()` carrier
with the zero value still refused; the reconcile config gains the `uncounted`
passthrough block. Core **§7.6** gains the two-layer clause. Tasks **#185**
and **#186** are filed. Task **#173** closes.

| Finding | Resolution | Lands in |
|---|---|---|
| **The task's own trigger had already fired, and the debt's premise had expired unread.** #173 was filed under "NO production ratelimit.Profile is constructed anywhere", with the first production profile as its named trigger — and slice G2's reconcile root then built one without anyone re-reading the debt. From that session on, an operator config declaring weight budgets but no order budget would have admitted every Binance futures order for free (the endpoint is documented at ZERO weight, Profiles §7) | The refusal built now, not at the next deferral review; the transferable rule is the memory shape "a debt list refreshes assumptions instead of examining them" — the trigger clause of a filed task is a claim about the PRESENT that must be re-checked, not re-read | §64a |
| **The written waiver is self-attested, so construction alone recreates the bypass it closes (second-model round 1, blocker).** Nothing at Profile construction can see the costs the adapters present — that knowledge is the codecs' — so an `Uncounted` entry with a false reason would pass validation and uncount the dimension exactly as forgetting did | Two layers, the runtime one primary: `decide()` refuses any cost charging a dimension its applicable windows do not count, and a waiver NEVER suppresses it — the waiver is a claim that no such cost arrives, and the cost in hand is the claim's falsification, fail-closed before send. Construction remains the boot-time early warning | governor.go, §64b |
| **Every Bybit read through the signed client failed its own precondition, LIVE, and no suite could see it (round 1, major — verified against code).** C2b's read surfaces spelled their truthful nothing-but-the-request cost as the zero `Cost` ("nothing to charge and nothing to invent"), while C2a's `Cost.Validate` refuses the zero as the field nobody filled in, and `SignedClient.Do` validates BEFORE `Acquire`. Two slices' correct claims contradict at a seam neither suite composed — the builder tests pass, the governor tests pass, and every `Positions`/`OrderByClientID`/`Executions` call on Bybit returns `ErrCostUnset` | `RequestOnly()` — the claim with a carrier a literal cannot produce by accident: unexported mark, constructor-only, contradiction (`requestOnly` beside a charge) refused. The three call sites converted; a composed test drives a request-only call through `SignedClient.Do` against a real governor and pins the envelope-only charge | limit.go, positions_bybit.go, orderquery_bybit.go, signed_internal_test.go, §64c |
| **A venue-scoped weight or order budget validates, counts locally, and silently never receives the venue's own reading (round 1, major).** Both `LimitReader`s address a fold at `(family, name)`; a venue-scoped limit is keyed under the empty family, so its header observations report `ErrLimitUnknown` — which the observe seam deliberately drops. The governor then runs on local counting alone, the exact guess §7.6's header-awareness exists to correct | `Limit.Validate` reserves venue scope for `DimensionRequests` — the one dimension with no header to lose — and the coverage rule accepts only FAMILY-scoped limits for product dimensions | limit.go, §64d |
| **Family-scoped request counters bypass the shared IP-ban envelope (round 2, the single remaining major).** A profile with fully covered families and per-family request limits passes the naive coverage rule, yet spot and futures can each sit inside their own capacity while their sum trips the gateway ban neither reports on — the repo's own cross-family-binding test states why the envelope must be ONE counter | The venue-scoped request envelope is mandatory and NOT waivable: family request limits are additive detail, never the envelope. Its capacity is a locally chosen safety bound ("bound combined request rate"), not a venue fact somebody might lack — so "the venue documents no request limit" is a reason to choose a bound, never to have none | profile.go, §64e |
| **The reconcile fixture's own limit names could never receive a Binance fold, and nothing checks name agreement anywhere.** `BinanceLimitReader` DERIVES names from headers (`weight_1m`, `orders_10s`); the fixture said `spot_weight` — valid, locally counted, and every venue reading silently dropped. The observe seam's comment defers to "a boot-time check" that does not exist | Fixture renamed to the fold convention (it is the shape operators copy); the boot-time name-agreement check is **#186**, and Bybit's inexpressible per-endpoint budgets (declared ones cross-charged by every same-family call, undeclared ones losing their folds) are **#185** — both filed with full context, deliberately not absorbed | app_test.go, tasks #185/#186, §64f |
| **One expected-equivalent mutant, documented like `Reserve()`'s.** The `DimensionRequests` exemption in the decide-time loop is dead by construction while the envelope is mandatory — the requests dimension is always in every family's counted set, so removing the exemption changes nothing | Kept, because its content is the coupling to the construction rule, which the doc comment states in both places; 19 targeted mutations elsewhere, 19 kills, three bad seds re-aimed on the "declared and not used" discriminator | governor.go, §64g |

**Milestone 3 task #174 is landed: the Place ack's venue order id is PERSISTED
at dispatch — a fourth journal record kind, written ack-before-state — and
§5.3's gate 3 rules a swept row out against the instrument's persisted-ack set
(v0.9.42, 2026-08-04).** Design record **§63**. `ports.Journal` gains
`AppendAcked` and `ports.JournalVisitor` gains `Acked` (a fourth kind breaks
every visitor at compile time, as that port's doc promised); `OrderRecord`
gains `VenueOrderID` with `ApplyAck` its one producer (write-once, same-value
no-op, contradiction refused with `ErrVenueOrderIDConflict`);
`ports.VenueOrderIndex` is the narrow read port the Resolver consumes, served
by the same store on the `HedgeGroupOrders` precedent; `attribute` gains the
per-instrument map and two arms; the Joiner deliberately does NOT consume the
index and instead refuses a store-vs-live id mismatch
(`ErrVenueOrderIDMismatch`). Core **§6 step 6** gains the persistence clause;
Profiles **§8** gains the dated 2026-08-04 identity block. Task **#174**
closes.

| Finding | Resolution | Lands in |
|---|---|---|
| **The rule-out consumed a venue fact nobody had banked, and the two claims read identically in code.** Ruling a row out because its id belongs to another of our orders needs "same `orderId` ⇒ same order ACROSS TIME" — per-symbol non-reuse — which is strictly more than the at-query-time uniqueness the existing different-id arm needs; both are `==` on a string. The second-model review (Codex) refused the slice on exactly this: the doc held only the join-key fact | Verified from primary sources and banked verbatim — USDⓈ-M *"orderId is self-increment for each specific symbol"*, SPOT glossary *"uniquely identifies the order on a symbol"* — before the arm landed; every consumer scopes to the full instrument because the guarantee is per SYMBOL | Profiles §8, `ports.VenueOrderIndex` doc, §63b |
| **A detected ack conflict must not leave a poisoned index standing, and the first draft did.** Refusing the second owner at `ApplyAck` while dispatch continued would leave the index holding the FIRST owner — and rule 6 would rule the second order's own row out against it, a false terminal absence (Codex, blocker sub-point) | Two composing halves: live, `Outcome.AckConflict` folds into `HoldsNewTrading` so the executor's fan-in latches; crashed, the journal deliberately records both acked records and the replay's `ApplyAck` refuses to compose them — the boot that would build the index fails first. Pinned by the module-level composed test | `dispatch.go`, `replay.go`, `module_test.go`, §63c |
| **The Joiner must not consume the index — a persisted ack and a live answer are two authorities over one identity.** Rule 3 would let fills from two venue orders land on one client order, and the cumulative cross-check can be arithmetically satisfied while wrong (Codex, major) | The index stays resolver-only; `joinOne` refuses a non-empty store-vs-live mismatch before any attribution, and past that check attributes by whichever source is non-empty — the persisted id serving a surface that answered without one | `join.go`, `ErrVenueOrderIDMismatch`, §63d |
| **Ack-before-state is the journal ordering, argued from the crash between the two writes.** `authorized`+`acked` replays into an order recovery can still join on Binance; `authorized`+`state` replays into the pre-#174 world. The state is the re-derivable half (the resolving query supplies it); the ack, lost, stays lost | The ordering is asserted as an interleaving fact (`ackedBeforeState`) and against the real store in `module_test`; a venue that returns no id appends nothing — the absent id has one spelling, the record's absence | `dispatch.go`, §63e |
| **A dropped ack write holds nothing, and the asymmetry with the conflict is the design.** A lost record degrades gate 3 back to refusal-to-conclude — safe and late, the resolver's designed direction — while a contradiction poisons every rule-out premise | `AckJournaled`/`AckApplied` reported per leg (meaningful only when an id existed, the gating fact one field up), consumed by the exercise CLI's leg line and `ExitClass`; `AckConflict` alone reaches `HoldsNewTrading` | `dispatch.go`, `cmd/arblab`, `app/exercise`, §63f |
| **A new harness lesson: `NOBUILD!!` has a third cause, and it is the INPUT.** `MUTATE_TIMEOUT=180` parses as 180 nanoseconds, `go test` refuses the flag with zero `--- FAIL` lines, and the harness prints `NOBUILD!!` about a mutant that compiles and dies cleanly by hand — the verdict was about the invocation, not the mutation | Reproduce by hand before believing any `NOBUILD!!` whose printed compiler error is EMPTY: a real compile failure names a line; an empty diagnostic block means the runner never got that far. The unit (`180s`) was already in the handoff's standing notes | §63g, HANDOFF standing notes |

**The milestone 3 quantization slice is landed: §2.4's trading filters — lot
step, min quantity, price tick, min notional — are a VERIFIED property of the
order path, and two of the four members had no normative clause anywhere
(v0.9.41, 2026-08-03).** Design record **§62**. `platform/fixedpoint` gains the
`Quantity.OnStep` / `Price.OnTick` exactness predicates; `oms/domain` gains
`TradingFilters` (placed there deliberately — `singlewriter` leaves the domain
nameable by composition roots, the `InstrumentAssets` precedent); one shared
conformance check in `oms/application` is enforced by the **Planner** per leg
before journaling and by the **Executor's preflight** per package before any
authorization fsync; `oms.Deps` gains `InstrumentFilters` with the nil map
fail-closed. `strategy`'s `LotRules` gains an OPTIONAL `Tick` (zero =
unrecorded, so pre-tick journaled revisions keep decoding; the order-path
resolver refuses what the domain admits — each consumer's own safe direction).
`app/reconcile` gains the `WithInstrumentFilters` option; `app/exercise` gains
the REQUIRED `reference` block (path + max row age) and the boot-time resolver.
Core **§2.4** and **§6 steps 5/6** edited; Profiles **§5** gains the dated
2026-08-03 tick block. Task **#182** closes; **#183** and **#184** are filed.

| Finding | Resolution | Lands in |
|---|---|---|
| **The order path's quantization was a doc comment, and the spec's own mandate covered one member of four.** §40g's `TargetQty` derives an on-grid quantity — on the scoring path only — while `LegIntent.Quantity`'s doc asserted "§6 step 4 has already quantized it" with nothing making it true; and §2.4 mandated only the LOT intersection, so the tick check and min-notional refusal now enforced were **unmarked** holes: §6 fixes marketable-IOC LIMIT as the only expressible pattern, so a tick binds every order this system sends, and Profiles §5 shows min notional is the BINDING floor on Binance's perp. The OWED grep gates *marked* claims; both of these were absences, findable only by the reading pass that sits beside it | core §2.4, §6 step 5 |
| **Verify, never derive — the order path refuses and must not repair.** Rounding at plan or preflight would silently absorb a disagreement between sizing's idea of the grid and the venue's — precisely the disagreement worth surfacing — and would dispatch a quantity the caller's own records do not describe. The harm class is **asymmetric leg rejection**: each leg is judged by its own venue against its own grid, so an off-grid package sent anyway is one leg filled and one refused, naked exposure. Both enforcement points are package-atomic: refused whole with nothing journaled (plan) or nothing fsynced (preflight) | §62, core §2.4 |
| **A replayed record was conformed by whichever revision planned it, so the preflight re-verifies against THIS process's filters.** §7.5's replay rebuilds `Planned` records that never passed this process's Plan; §5.3 says replan-or-discard, never dispatch. The preflight is that doctrine made structural — a second pass AFTER the package-shape checks, so a mixed-base package keeps its sharper `ErrPackageQuantityMismatch` diagnosis. Accepted from the Codex review (adapted); the review's other accepted point is the **max row age** on the exercise resolver, because filters move server-side without notice and every local check passes against a stale row while the venue judges the current one | core §6 step 6, app/exercise |
| **Rejected, with reasons on record:** persisting the authorizing filter revision per order (overkill for a one-shot binary; the preflight re-verifies against current, which is the stronger check for this deployment); a `Dispatcher.admit` conformance check (single-order dispatch of a replayed record is doctrine-forbidden with no current caller, and the admission read sits below the fsync boundary the check belongs above — documented on `Module.Dispatch`, lands with the compensating-trade slice as #184) | §62 |
| **The filters resolver could not live in `app/reconcile`, and the fence proved it.** The first placement imported strategy from §7.5's boot root and `imports_test` refused it — the fence doing exactly its job, since the boot must not acquire a collaborator from a context whose posture it has not decided. The adapter lives in the binary that PLANS (`app/exercise`), which owns the reference document, the freshness bound and the resolution, and hands the boot a finished map through an Option — not a Config field, so a reconcile-only deployment cannot even express the dependency | §62, app/exercise |
| **`configs/reference.btcusdt.json` keeps `observed_at` 2026-07-31 while gaining ticks dated 2026-08-03 in their source strings.** The committed calibration's frozen as-of (the run_test freeze at 2026-07-31T12:00) must stay answerable — a single-revision store carries ONE date per row, and moving it forward would orphan the milestone-2 evidence chain; §40k-2's archive is the real fix. The tick's own observation date lives in the row's source string, which is the provenance the Profiles block backs | configs, §62 |
| **The $20 live test cannot run on the Binance perp as imagined.** The USDⓈ-M `MIN_NOTIONAL` floor re-confirmed live at **50 USDT** means the perp leg needs ≥50 USDT notional: the package grows to ~$55, or the perp leg moves to Bybit — blocked on #171's signer validation. An operator decision, surfaced in HANDOFF rather than absorbed. A related note is banked with it: the committed rows are MAINNET-observed, so a testnet run may meet different filters — the failure mode is a refused cycle, not exposure, and the mainnet run is the correct target | §62, HANDOFF |

**Milestone 3 slice G3 is landed: §6 step 6's parallel two-leg marketable-IOC
executor exists, `arblab exercise run` is §9's cycle driver and the first binary
in this repo that DISPATCHES, the process-kill-mid-IOC chaos test proves A–F
composed — and the in-process double-dispatch backstop had silently scoped
itself "per path" the moment a second dispatch path existed (v0.9.40,
2026-08-03).** Design record **§59**. `oms/application` gains `Executor`,
`Execution`, `DispatchRef` and `DispatchAuthority`; `oms.Module` gains
`Execute`; `app/reconcile` exports `Runner` (the boot held open for a composing
caller); `internal/app/exercise` is new — the §5.2 OMS block mounted under
`"oms"` plus a `trade` block, seven phases where reconcile has six, the execute
step AFTER the sequence and gated on `ExitClean`; `cmd/arblab` gains
`exercise run` with exit code **3** for dispatched-and-needs-a-human. Core
**§6 step 6** (the package rule) and **§11** edited. Task **#181** closes §47f's
roster.

| Finding | Resolution | Lands in |
|---|---|---|
| **The backstop that holds when the projection broke was process-wide only by accident of there being one path.** `Dispatcher.authorized` exists for exactly one case: a failed apply leaves a SENT order at `Planned`, the repository re-admits it, and only the in-process sent-set refuses the second `Place`. The executor is a second dispatch path, and a private map per dispatcher narrows the guarantee to *this path never double-Places* — with nothing failing, because the abandoned case is itself an error path. Extracted as `DispatchAuthority`: one synchronised, monotone set per process, threaded through the module's dispatcher and every per-leg dispatcher the executor confines. What it deliberately does NOT close is the same-id check-then-mark window spanning the fsync — serialisation per id stays the caller's, and the executor guarantees it structurally (distinct ids per call, one call at a time) | §59b |
| **The wiring of the shared set was itself a claim no test could check, and the battery said so.** A mutant handing the executor a fresh authority SURVIVED: the fragmentation is observable only through a broken projection, which the real repository cannot be made to produce. Diagnosis: a test gap with no reachable test — so the fix is the recorded move for a check nothing can make, a SIGNATURE. `NewExecutor` takes the `*Dispatcher` and inherits its authority, journal, placer, repository and clock; a second authority in one process stopped being expressible from outside the package, and the package-internal fork mutant is caught by the cross-path test. Slice B closed a hole the same way (`Dispatch` reads the order it sends), one slice's mechanism arriving as another's fix | §59g |
| **The package rule: every refusal a multi-leg package can earn lands before ANY leg's authorization fsync.** §6 step 6 put every caller-controllable refusal above the fsync per ORDER; the executor extends it to the package, because a refusal discovered at leg 2 after leg 1's fsync strands a durable authorization — or a sent IOC — on a package the caller was told was refused. Preflight is a read pass (ids: one trade, one attempt, distinct legs; records: expected version, dispatchable state, marketable-IOC, identical base quantity via the asset-checked `Cmp`); after the fan-out there is no composite error, only per-leg results, because a sent IOC cannot be un-sent and an aggregate error invites the `if err != nil { return }` reflex over live orders | core §6 step 6 |
| **The exercise root executes under `PostureReconcileOnly`, and the gate is the VERDICT.** Five of §7.5's eight gates have no producer, so this deployment's posture is reconcile-only on every boot — a driver gated on `PostureTrading` could never run, waiting on contexts §12 sequences after the evidence the driver exists to produce. The execute step requires `ExitClean` (every produced gate concluded, nothing latched) and runs AFTER the boot sequence, never as a phase — `boot.Sequence` enforces reconcile-last and that rule is right. The regime is stated rather than absorbed: sub-`$1K`, testnet-class targets, and the same gate tightens by itself as producers land | §59d |
| **The chaos harness is a fake venue because the evidence wanted is about OUR crash boundary.** The kill window is entered deterministically — the fake holds both IOCs open, so "both legs in flight with both authorizations provably fsynced" is a signalled state, not a race — and a real testnet can do none of that: it cannot hold an IOC open, cannot count our `Place` calls, and needs credentials #171 has not validated. §9's bar is asserted directly: recovery resolves both absences through §5.3's four gates to `AbsentAfterFinality`, the net-delta assertion explains every position, and the fake counts exactly one `Place` per `ClientOrderID` across both runs | §59e |
| **The collapse checklist's tenth instance — and the first caught by the battery instead of before it.** The identical-base-quantity fixture varied only UPWARD (0.6 against 0.5), where `cmp != 0` and the one-sided `cmp > 0` are the same predicate, and that mutant survived until the 0.4 case existed. The checklist's tell held as every time — the fixture did not contain the thing under test (a mismatch in the other direction) — and the lesson for the standing rule is that running the checklist first is necessary and not sufficient: the battery is what proved this instance, so neither substitutes for the other | §59g |
| **A config kernel's deferral met its first consumer and the mount point moved instead.** The exercise config wanted to EMBED `reconcile.Config`, and `platform/config`'s schema audit refuses embedded fields — deliberately, because promoted-field path naming and hot-tag semantics are undefined there, and this repo has already paid for encoding/json's silent-drop collision behaviour. The block mounts under `"oms"` instead: §5.2's schema keeps its one owner, and where a binary mounts it is that binary's document shape. The kernel's deferral now has a named prospective consumer rather than a hypothesis | §59i (vii) |

**Milestone 3 slice G2 is landed: the composition root exists, §5.2 has an OMS
configuration schema, `arblab reconcile run` is the ops CLI's third verb — and
§7.5's exemption for an instrument the invariant cannot cover turned out to be
argued from ONE cause over a predicate collecting TWO (v0.9.39, 2026-08-03).**
Design record **§58**. `internal/app/reconcile` is new — `Config` and its
`Validate`, §5.2's key refs resolved into `transport.Credential`, a
`venue.OrderClient` built from a file, #168's `checkJournalStorage`, a
`boot.Sequence` over §3.2's phases and a `boot.Checklist` over all eight of
§7.5's gates. `platform/boot` gains `GateJournalDurable` and `Checklist.Latched`;
`oms/application` gains `UnassertedReason`, `UnassertedInstrument` and
`HoldsPosture`; `driven.NewVenuePositions` refuses a two-venue instrument table;
`ports.PositionScope` refuses a spot scope. Core **§5.2**, **§7.5** and **§11**
edited. Tasks **#166** and **#168** are complete; **#181** (G3) remains.

| Finding | Resolution | Lands in |
|---|---|---|
| **The exemption was argued from one cause and the predicate collected two.** F2 ruled that an instrument whose exposure cannot be compared is reported and does **not** hold the posture, reasoning entirely from the permanent, venue-imposed gap — no venue publishes a position surface for spot. The predicate that implements it is *no configured enumeration covered this instrument*, which a **derivative** reaches: a scope nobody declared, one declared for another venue, or an instrument dropped from the config while the journal still holds an open order on it. So the net-delta gate reports **satisfied on a boot that issued no position query**, and the wiring documented the degenerate case — an empty scope list — as legal. Fixed by carrying the REASON and putting the exemption on it: only *no surface exists anywhere* is exempt, and *no configured scope covers it* is the **latching** refusal, because the epistemic position is identical to an unexplained delta. An unclassifiable reason holds. The document-level half is a config refusal naming the instrument and the missing field — and it is not the same check twice, since one is about the configured universe and the other about the JOURNAL's, which outlives any one config file | core §7.5, §5.2 |
| **A justification is a claim about ONE input and the bucket it governs may have several.** The general rule this earns, beside the recorded *hazard named for one input* and *reassuring claim generalized from one input*: when a rule EXEMPTS something, the thing to check is not whether the exemption is sound but **what else lands in the bucket it is written over**. The tell was in the test rather than the code — the fixture for the exemption planted exactly one order, on the spot leg, so every instrument that could reach the bucket was a spot one and the two causes were the same answer. That is the collapse checklist's own tell (*the fixture does not contain the thing under test*) run on a REASON instead of on a value | — |
| **#168, and the condition is DURABILITY rather than locality.** §11/§13 forbid NFS and network filesystems and schedule the check to §7.5's list. It lands as `GateJournalDurable`, a gate of its own for §7.5's own rule that a gate is reported about its own evidence — and merging it into the replay's is not merely untidy: the storage check runs first and LATCHES, and `Report` refuses a latched gate the replay's own opening `OutcomePending`, so one gate would turn a correct sequencing into an error from the phase that must not produce one. It widened on the way in: `tmpfs` satisfies every word §13 writes, honours every fsync, and loses the whole log at the reboot §7.5 exists for, so a volatile filesystem is refused under its own name — and an unrecognised filesystem and an uninspectable host are refusals too, because an unverifiable claim about durability is not a verified one | core §7.5, §11 |
| **The POSTURE cannot be a reconcile tool's verdict, and the question that can be had no read-only accessor.** `Checklist.Posture` reduces over the REQUIRED gates, which is §7.5's default-deny mechanism, and this root correctly declares five gates whose producers live in contexts that do not exist — so the posture is reconcile-only on a boot that found nothing and on one that found a foreign order. `Blocking` has the same problem one layer down. What separates them is whether anything **latched**, which is §7.5's own distinction between self-clearing and operator-clearance preconditions — and `latching` was unexported with no read-only accessor, so the only way to discover a latched gate was to call `Clear` and see whether it succeeded: to answer the question by performing the act that answers it. `Checklist.Latched` is that accessor, and `Report.ExitClass` is the verdict | core §7.5 |
| **A venue derived from a map's first key is a coin flip, and the config file is what makes it reachable.** `driven.VenuePositions` derives its venue from the instrument table rather than storing it, justified as *"every instrument in the table shares a venue in every deployment this repo has"* — a claim about deployments, not about the map. The first deployment configured from a FILE is the one that could break it, and the failure is per-boot: Go's randomized map iteration decides which venue the adapter believes it is, so the scope check below admits a scope on one boot and refuses it on the next with no input changed. Refused at construction, naming both venues in a stable order | — |
| **`ports.PositionScope` accepted a spot scope that `venue.PositionScope` refuses.** The same normative fact — no venue publishes a position surface for spot — was stated on one of two mirrors, so a spot scope was constructible, survived `NewJoiner`, and was refused two layers down by the codec: a configuration mistake turned into a FAILED net-delta gate on every boot, halting a healthy process for a line in a file | — |

**Milestone 3 slice G1 is landed: §7.5's reconcile-only boot is ONE use case in
the one order its phases are correct in — and the fused reading asserted the
net-delta invariant over a quantity the join had DECLINED to rebuild (v0.9.38,
2026-08-03).** Design record **§57**. `oms/application` gains `Reconciler`,
`Reconciliation` and `UnprovedOrder`; `Joiner.Join` is split from a new
`Joiner.Assert` and the `Assertion` report; `UnprovedReason` gains
`RetryMayLift`; `platform/boot` gains `OutcomeIncomplete`; `oms.Module` gains
`Reconcile`. Core **§7.5** is edited. Tasks **#175** and the recovery half of
**#169** are discharged; **#166** is re-scoped as **G2** (the composition root
and the `arblab reconcile` binary, which is what would construct a
`boot.Checklist` in production) and **G3** (parallel-IOC + the
process-kill-mid-IOC chaos test).

**THE FINDING is a contradiction between two clauses of one use case, and only
the composition could see it.** `Join` records an order in `Absent` precisely
because §5.3 forbids inferring *"no executions"* from an absence — it does not
sweep it, and it leaves the zero cumulative the replay gave it — and `assert`,
one function later, read that zero as a fact. The venue's own standing position
is the other side of the subtraction, so the whole of any fill such an order took
reported as an **unexplained position delta**: §7.5's *alert for operator
clearance*, which **latches**. The boot it fires on is the one §7.5 exists for —
a dispatch that reached the venue, a crash seconds later, and a propagation bound
that has merely not elapsed — so a condition that discharges itself inside
`MaxLanding + SweepMargin` became an incident requiring a human, which is §7.4's
*"never health-flap re-entry"* run backwards. **Neither half is wrong on its
own**, which is why four slices of review passed over it: the join's refusal to
infer is correct, and asserting the invariant before trading resumes is correct.
What is wrong is that nothing owned the ORDER, because §7.5's three phases were
three unwired use cases and the sequence lived in no code. The fix is the
sequence made into a use case — replay → join → §5.3's rule on every absence →
assert — plus a **gate**: the assertion runs only where the rebuild concluded,
and where it does not run its gate stays **unreported**, which is the honest
value and not the permissive one because the posture folds over the *required*
gates. The rejected order is kept as a **control that runs**
(`TestAssertingOverAnUnrebuiltAbsenceLatches`), on the standing habit of encoding
the rejected reading rather than arguing it.

**THE SECOND HALF: the outcome vocabulary's THIRD empty cell, and the tabulation
that found the second one was drawn on too few axes.** The fixed reading needs an
outcome meaning *"the check ran, concluded, and left work a retry may finish"* —
reconcile-only, because halting forbids the reconciliation writes that finish the
work, and non-latching, because demanding operator clearance for a timer is the
halts-itself-with-every-light-green failure. F2's table showed that cell as
occupied by `OutcomeUnreported`/`OutcomePending`, and **both of those are
PRE-VERDICT**: every other row had a member meaning *"it ran and it refuses"* and
this row had none. So the property the tabulation was missing is **whether the
outcome reports a result at all**, and the general rule is one turn past F2's — *a
product is only as complete as the properties it is tabulated on, so a cell filled
by a member answering a DIFFERENT question from its row-mates is the tell that an
axis is missing, not that the cell is occupied.* Reported as `OutcomePending`
instead, a concluded-and-refusing boot is indistinguishable from a hung one: the
process stays alive, audible and below stop-opening, so it may not reduce
exposure, nobody escalates because the boot "has not finished", and §7.4's
watchdog reads liveness and does not escalate either — the **alive-but-wedged**
uncovered failure, reached through a vocabulary.

**Task #175 is the same finding's third face, and it belongs to the REASON.**
§5.3's `UnprovedReason` constants each already committed, in prose, to whether
waiting could ever help — *"this one resolves itself"*, *"it never resolves
itself and gets strictly worse with time"*, *"permanent, and fixed by
configuration rather than by waiting"* — and nothing read those sentences.
`RetryMayLift` makes them a value the checklist consumes, so an absence that can
never be proved is the **latching** refusal while one waiting on a clock is the
re-reportable one. It is a method on the reason rather than a set of reasons the
boot recognises, because the permissive default is the one that costs: a gate
added later and defaulting to *"a retry may lift it"* would be reported as
ordinary boot progress for ever.

**The battery is 23 mutations and 23 kills.** Six first attempts returned
`NO-OP!!` or `NOBUILD!!` and every one was a bad sed rather than a claim — a
duplicate `case`, an orphaned `declared and not used`, a line-number prefix
inside a command the harness already scopes to a line, and two wrong line
numbers — each matching the recorded discriminator, so each was re-aimed rather
than recorded as a non-result. The two that
first **SURVIVED** are worth recording because they are the *switch-default hides
its own group* trap arriving on a new member: dropping `OutcomeIncomplete` from
`posture()`'s and `latching()`'s case lists falls through to the trailing
`return PostureHalted` / `return true`, and `platform/boot`'s own suite had no
fixture for the new member at all. Both became kills once the posture table and
`TestIncompleteRefusesWITHOUTLatching` covered it — which asserts **both** halves,
because the posture alone is already provided by `OutcomePending` and the
non-latching alone by `OutcomeFailed`, so a fixture checking either half would
pass against the wrong member. The collapse checklist also caught a **fake with a
second opinion** before the mutations: `fakeOrders.ApplyPlanned` handed back an
UNTAGGED zero cumulative where the real repository registers the order in the
fill ledger and the ledger's zero is tagged at `Register` — invisible until a use
case does arithmetic on it, which is exactly what the assertion does to every
order the replay rebuilt and nothing filled.

**Milestone 3 slice F2 is landed: §7.5's venue join is executable, the fill
ledger is rebuilt from venue truth, and the net-delta invariant turns out to
cover ONE LEG of a two-leg package (v0.9.37, 2026-08-03).** Design record
**§56**. `engine/venue` gains the position surface (`PositionScope`, `Position`,
`PositionReport`, `PositionCodec` on both venues' codecs) and `OrderCodec`
embeds it; `oms/application/ports` gains `VenuePositions` and widens
`VenueExecution` to carry price and fee; `driven` gains the position ACL;
`oms/application` gains `Joiner` and `Join`, and `attribute` gains its second
key; `oms/domain` gains `OrderState.MayExistAtVenue`; `Recovery` gains
`Dispatched`; `platform/boot` gains `OutcomeProjectionStale`. Core **§7.5** is
edited; Profiles gains **§9**. Tasks **#176** (`Fill.Account`'s producer) and
**#169** (§6 step 7's hold, as a vocabulary and a predicate) are discharged as
far as an unbuilt boot sequence permits — the mapping that reports either
outcome waits on the composition root, exactly as `ChainBroken` has since F1.

**The join is an ENUMERATION, and that is a requirement rather than a shape.**
§7.5 alerts on *foreign orders or unexplained position deltas*, and a
per-instrument position read can only ask about instruments this system already
knows it traded — the one set that cannot contain a foreign one. So the
positions call names no symbol, and a symbol the account holds that this
deployment does not trade IS the alert. The endpoint VERSION then becomes part
of the requirement: Binance offers two `positionRisk` endpoints whose absent
rows mean opposite things (V3 returns *"only symbol that has position or open
orders"*; V2 returns every symbol), so only V3's silence carries the conclusion
*flat* — and V3's silence, on the same page, is also how a foreign resting
ORDER becomes visible.

**The executed quantity is rebuilt from the venue, with TWO attribution keys and
a cross-check between two surfaces.** F1 left every replayed order at a
cumulative of zero, and zero is not a missing number there — §6.7 folds it into
a residual, a group of zeroes is *Determined* and flat, and flat is the verdict
that PERMITS a compensating trade. The join therefore resolves every order the
venue may hold, attributes its executions by client order id **or by the venue's
own order id returned by that same query** — the only key one venue offers,
since its trade lists carry none — and compares the order surface's stated
cumulative against the rows it could attribute. It needs no transaction, and the
reason is a property rather than an exemption: the join advances no cursor, so
it is **re-derivable**, and the fill dedupe identity makes the repeat a no-op.
*Idempotence is what the live path buys with a transaction and this one gets
from the venue.*

**FINDING 1 — the invariant is stated over a hedge GROUP and venue truth exists
for only one of its legs.** Neither venue has a position surface for spot; the
balance that stands in its place contains holdings this system never bought, so
comparing it to our own exposure needs a baseline nothing records. Both
directions are now normative: the limit is **reported on every boot**, because a
clean report is otherwise read as a complete one, and it deliberately does **not
hold the posture**, because a leg whose invariant can never be asserted would
keep a correct system below trading for ever — the halts-itself-with-every-light-green
failure `ContributesPossibleDelta` exists to avoid, arriving from the other end.

**FINDING 2 — the outcome vocabulary's SECOND empty cell, one slice after the
first was filled without tabulating the rest.** F1 found *halted + latching*
missing, added the member its own producer needed, and stopped — leaving the
stop-opening row in the state the halted row had just been rescued from. §6 step
7's rule falls in it: a dispatch that reached the venue and whose apply failed
holds new trading, which is **stop-opening** (the order may be executing, so the
safety verbs must stay available) and **latching** (the check that would lift it
recomputes from the projection that is known stale — and the stale reading is
the permissive one, since the order sits at `Planned`). A re-reportable outcome
would be lifted by an invariant agreeing with its own corrupted input. General
form: **an outcome vocabulary is a PRODUCT of the properties it decides, and a
missing member is found by tabulating it rather than by waiting for a
producer.**

**The battery: 31 mutations, 30 killed by the suite and 1 by the gate, with one
real survivor that changed the code.** The survivor was the position lookup
returning its FIRST match from a Go map, so deleting the product comparison — a
perp row matching the spot instrument with the same symbol, which every
cash-and-carry deployment declares — was caught only when the randomized
iteration happened to visit spot first. *The answer to a coin-flip test is not
more trials; it is to remove the coin flip*: the lookup now refuses a second
match, which is also a real configuration check, since an unvalidated
`ContractType` splits one instrument across two keys carrying different base
assets (`domain.FillKey` records the same hazard). Three trials confirm the
mutation is now deterministic. The one gate-covered mutation is the recorded
trailing-return trap on `Outcome.latching`, verified by running `make lint`
against the mutant with a clean positive control rather than asserted.

**Milestone 3 slice F1 is landed: §7.5's replay half is executable, the anchor
has a journal record kind of its own, and the boot vocabulary was missing the
outcome that HALTS AND LATCHES (v0.9.36, 2026-08-03).** Design record **§55**.
`oms/application/ports` gains `JournalReplay` + `JournalVisitor` and
`ErrJournalChainBroken`; `oms/application` gains `Replayer` and `Recovery`;
`driven` gains the walk, the per-record chain verification and the
`journal_authorized` table; `ports.Journal` gains `AppendAuthorized` and
`AppendState` refuses `StateDispatchAuthorized`; `platform/boot` gains
`OutcomeEvidenceUnusable`. Core **§5.3**, **§7.5** and State-Machines **§1** are
edited. §47f's slice F is **split**: the venue join and the net-delta invariant
are task **#178**, on the seam `boot.Gate` already draws and because
`Positions`/`FundingLedger` have no venue client yet.

**The anchor's home is the mirror rule on an axis this repo had not used it on.**
Slice D made the anchorless authorization inexpressible in the **projection**
and the **journal** kept the anchorless spelling as its only one — so the
projection could not hold an unanchored authorization, the log could not hold an
anchored one, and a crash-recovered order arrived at the crash boundary with no
anchor **however carefully the live path had stamped it**. The mirrors here are
not two venues and not two directions of travel; they are *journal and
projection*. General form: **when a rule is enforced by making a shape
inexpressible, ask which OTHER stores can express it.**

**The replay rebuilds THROUGH the write path**, so the recovered projection can
only reach states a live process could have produced — SM §1's edges re-checked,
the OCC version advancing as it did the first time, an impossible history
**refused rather than installed**. Verification is per record and precedes the
apply; its three claims fail differently, and the sequence-density one catches
what nothing else does — a dropped **prefix re-anchored to genesis**, where
every remaining link verifies perfectly. A **duplicate delivery is not a corrupt
history** (§5.3 puts the same execution on two streams), and **no fill is
journaled**, so after this phase an order's state is authoritative and its
executed quantity is not yet a claim — which is why the net-delta gate is
separate. Stated limit: **truncation at the tail is undetectable** without an
independently durable head.

**THE FINDING — §7.5's preconditions vary on TWO independent axes and the
outcome vocabulary had one.** How restrictive a refusal is, and whether a re-run
may lift it, are independent; three of the four combinations had a value and
*halted + latching* did not. Reported as an ordinary failure, a broken chain is
freely re-reportable, so a deterministic retry re-admits trading — §7.4's
*"never health-flap re-entry"* through §7.5's door. Reported as an ordinary
operator matter it latches at **reconcile-only**, which permits `ActReconcile` —
and §7.5's `DeactivationPending` re-drive fires a **terminal, non-reactivatable** tombstone for whichever plan the corrupt log appears to name — after its ack no `Replicate` for that `TradeID` is accepted and `ListActive` never returns it, so a trade that is in fact still open is left **permanently without a takeover path**, with its hedge-group lease released for a successor. **An act is exposure-neutral by construction only where its INPUTS are
trustworthy, and this is the one gate whose failure means they are not** — the
recorded *an invariant cannot audit its own input* shape, one layer out, on the
classification of an action rather than on a number.

**Milestone 3 slice E is landed: §6.7's classifier is executable, and `FM-5`,
`FM-6` and `FM-7` are now claims the TYPES make (v0.9.35, 2026-08-03) — and the
hedge-group key it computes over had a component nothing in the system carried.**
Design record **§54**. `oms/domain` gains `HedgeGroup`, `PossibleDelta`,
`ReservedReductions` and `Classification`; `oms/application` gains `Classifier` and
`Verdict`; `ports` gains `HedgeGroupOrders`, a **one-method read port**. Core **§5.1**,
**§6 step 7** and **§12** and State-Machines **§2** are edited.

**The classifier decides and cannot act, which is what §6.7 asks for.** Six clients
route through this gate — the one-leg-failure handler, ADL/liquidation, the kill-switch
panic-MARKET, protection-disarm, `PackageDelever`/`EmergencyClosePackage`, and actuator
takeover — and every one of them wants to trade. `Classifier` holds one read method and
no writer, no placer and no journal, so there is no method through which a compensating
trade could be issued: `Resolver`'s no-placer argument, applied to the other end of the
same section.

**`FM-5` is inexpressible rather than forbidden.** `Reserve` takes **no amount**, because
`AmbiguousOrder.tla`'s nominal reading is literally `Pend(0, "pkgspot", FALSE)` and the
control that *is* `FM-5` (`RESERVE_AT_CREATION = TRUE`) is the branch that books the
target. An entry becomes nonzero only through `Amend`, which is a producer stating what
it has **committed and not yet realized**. The one producer nonzero from its first
instant — the `IntervalFloorReduction` — reserves and then amends, and that two-step is
§6.7's own ordering made visible rather than a tax. The rejected middle option,
`Reserve(committed, realized)`, is **refuted by the module**: `PdAuthorizeSpot(s)` sets
the entry to `s`, not to `pdDone − pdSold`, so the two part company for the whole second
half of the ladder.

**`FM-6` gets a guard the spec's stated inputs never needed, and the asymmetry is the
point.** The subtrahend's mutators live on the classifier and each **returns a fresh
verdict**, so a ledger change that does not re-classify is a call that does not exist.
Inputs 1 and 2 keep the ordinary shape, because they already had a written re-run rule.
**A re-run rule that is written down has a reader; the missing one has none** — so the
input the spec forgot is the input that gets a type.

**FINDING 1 — §5.1's hedge-group key has THREE components and an order record carries
TWO.** `(account, venue, symbol)` is defined once, in §5.1, and §5.1's own `Order` field
list has no account in it; the only `Account` in the system was `Fill`'s, which has a
real consumer (the dedupe key) and **no producer at all**. The failure direction is the
rare one in this kernel that **concludes rather than halts**: keyed on `(venue, symbol)`
alone, a naked long in one subaccount nets against a naked short in another to a residual
of **zero** — positions and margin are per subaccount, so none of that netting is real —
and the group reads flat, no `ExposureReduction` is sized, both legs stay naked, and
every invariant computed over the group agrees. Taken: the group is a value object with
the zero value unusable, the store holds **one** account and refuses another's group, and
§5.1 records that a second subaccount takes a second store or an edit to its own field
list.

**FINDING 2 — the reading pass caught a figure one step before it reached the code.**
State-Machines §2 cited its interval-floor defence-in-depth proof as **1,086,199**
distinct states; that is the *model*'s slice E measurement on the pre-`FM-8` graph, and
the current suite reproduces **1,093,953**. The design record froze the old figure
deliberately, as a record of what that release checked — right for a record, wrong for a
normative sentence a reader tries to reproduce — and by the time this slice read it, the
stale number had reached six documents and was about to be copied into a Go doc comment.
**A figure no run reproduces reads exactly like one that does**: the citation shape this
project keeps finding, arriving on a number rather than on a section.

**34 mutations, 34 kills; two survived the first pass and both were one shape.** Deleting
either episode-reset flag left every test green — the deadline flag is invisible at the
moment of resolution, because a collapsed range already returns `Determined` whatever the
flag says, and the floor flag is invisible when the next call sets the deadline itself.
**The observable difference exists only on the FOLLOWING incident, and no test had one**:
a fresh ambiguity inheriting a stale deadline verdict would hold new trading and admit a
floor with no deadline ever having blown, and a spent floor flag would leave a second
incident with no admissible safe action for ever. The collapse checklist had already
forced the fixtures — unequal legs, so a hedge's own sign convention is testable; a
straddle table sitting **on** both boundaries; three producers at three different amounts,
so one producer's contribution is not the sum — and it did not reach this, because the
variation needed was in the **sequence**, not in a value.

**Milestone 3 slice D is landed: §5.3's stable-not-found rule is executable (v0.9.34,
2026-08-03) — and the anchor it measures from had no home, because slice A's
rejection of it was a claim about the future.**
Design record **§53**. `oms` gains `ports.OrderResolver` (the read-only recovery port),
the `Resolver` use case, and the read-half ACL adapter; `OrderRecord` gains
`AuthorizedAt`, and the write repository gains `ApplyAuthorized`, which is the only
method that can reach `StateDispatchAuthorized`. Core **§4**, **§5.3** and **§12** and
State-Machines **§1** are edited.

**The rule is COMPOSED, because §5.3 names it twice and never states its steps.**
State-Machines §2's four-step finality contract supplies three gates and §5.3's
retention paragraph the fourth: the **landing horizon** has passed, so an authorized
send can no longer land after the query; the order is **absent from every surface** the
venue offers; an **executions sweep proved coverage** past that horizon and contains no
row this system cannot rule out; and the answering surface's **published reach** still
covers the order. Every gate that fails yields a **named refusal and no state change**,
and that asymmetry is the design — each conclusion needs a positive answer from a venue
rather than the absence of a negative one, so the type can refuse to conclude and
cannot conclude wrongly. `Resolver` holds no `OrderPlacer`, so §5.3's *"never
resubmitted"* is a method that **does not exist** rather than a rule someone remembers;
§4 now binds that narrowing on the consumer's port while leaving `VenueAdapter`'s wide
capability set alone.

**Finding 1 — the anchor had no home, and the reason it was rejected had EXPIRED.**
Both of the rule's time bounds count from the instant this system committed to the
single `Place`, and nothing in the OMS recorded it: not `domain.Order` (the immutable
plan-time half, which does not know when dispatch will happen), not the order record,
and **not the journal** — whose codec *deliberately* refused a timestamp field in slice
A, on the recorded ground that it would add *"a value the chain covers but **nothing
reads**"* and that *"when an event time is genuinely needed it is the venue's, and **it
arrives on a fill**"*. Both clauses were true when written. Both are false one slice
later: v0.9.33 made elapsed time since authorization load-bearing, and the case that
needs it is an order with **no fill and no venue record at all** — there is no fill for
the venue's time to arrive on, and a not-found answer carries no created time either.
This is the recorded *rejected-alternative-is-an-unchecked-claim* shape on a second
axis: **the rejection's REASON was a claim about the future, and nobody re-derives a
rejection when the future arrives.** No grep can find it, because what is wrong is a
clause that is **absent** rather than one that departed from the spec — the same
blindness as C1's citation-to-a-nonexistent-section, arriving from the opposite
direction. **The consequence is a quiet halt, not a wrong number:** the projection is in
process memory, so a crash-recovered order has no anchor and `AbsentAfterFinality` is
**unreachable for exactly the order §5.3 says recovery resolves that way** — the order
stays in a state that contributes a possible-delta interval, its hedge group is
`Ambiguous` for ever, new trading is held, and every light is green. Taken:
`AuthorizedAt` as a venue-frame `clock.ServerTime` stamped **above** the fsync, with the
journal column deferred to **slice F and its replayer**, on the
schema-field-frozen-before-its-consumer rule.

**Finding 2 — State-Machines §1's query-resolution set had no `Canceled`.** §6 step 6
sends nothing but marketable IOC limits, so the commonest thing a *resolving* query can
find is an order the venue already terminated unfilled (Binance `EXPIRED`, Bybit
`Cancelled`) — and that answer had **no legal target** from `DispatchAuthorized` or
`SubmissionUnknown`. Worse than stuck, it **misdirects**: the nearest available terminal
is `AbsentAfterFinality`, which is precisely the evidence-class conflation **FLAG-2**
exists to prevent, so recording it there tells §7.5's reconcile-only boot that we never
found an order we in fact found. **A missing edge that leaves a wrong edge as the only
legal one is worse than a missing edge**, and no gate can see it — the transition set is
total either way and each individual edge is legal. v0.9.11's venue-initiated-
termination fix added exactly this edge for the two **live** states and stopped there:
**the mirror rule's second axis again, live-vs-recovery this time.**

**Two rulings, recorded so they are not silently reopened.** Gate 4 compares against
**`min(unfilled, filled)`** retention, not against the field whose name fits: at the
moment the rule asks whether an absence is explicable by purging it does not know which
kind of order it was — that is what the absence *means* — so the absence must be
inexplicable under every hypothesis. Both venues publish the shorter figure for unfilled
orders today, which is exactly why taking a field by **name** rather than by **role** is
the unsafe habit. And an **unpublished retention licenses only what an operator
configures**: "no cliff" proves absence for ever, while "never provable" makes every
Binance-**spot** lost-ack IOC unresolvable — and the spot leg of a cash-and-carry is a
BUY, so that is half of every trade — so both defaults are unsafe, the carrier is
forced, and an unpublished-and-unconfigured surface yields a refusal that names itself.
The check that would be **in time** — refuse to trade an instrument whose absences could
never be proved — belongs to §7.5's boot list, because the irreversible commitment is
the `Place` and it has already happened by the time this rule runs.

**The collapse checklist ran BEFORE the battery and found dead code in the code just
written.** `attribute()` took a venue order id, and its only caller is the absent path —
where a venue reporting no such order has by definition supplied none. Two unreachable
arms, reading as handled cases. Removing them **priced the consequence honestly**: on
Binance **any** execution row inside the window makes the absence inconclusive, because
neither trade list returns a client order id and the venue id is the only other key.
Closing that means persisting `Ack.VenueOrderID` and reconciling against the set of our
own orders — §7.5's reconciler, and a raised task rather than a hidden cost.

**28 mutations, 28 kills — after eight survived the first pass, and every one was a real
gap.** Two are **the mirror rule's second axis inside a single rule**: gate 1 had a
fixture placing the clock *inside* the uncertainty band, the only window where the
band's edges differ; gate 4 — the same band, the deliberately **opposite** edge — had
every fixture hours from its boundary, and the test file's own fixture header named the
pair while citing one test. Two are **assertion**-shaped rather than fixture-shaped:
`ProvenThrough != horizon` is satisfied by **zero**, so a sweep that recorded no
watermark at all passed, and `calls < hardStop` is satisfied by every wrong page bound
below it — both replaced by positive assertions against known exact values. Three are
the quietest of all: `AuthorizedAt`, the field this slice exists to add, was **written by
every repository test and read back by none**, and the dispatcher's own
uncalibrated-venue refusal was masked by the repository refusing the same instant one
layer down — so only the **ordering** claim (nothing journaled, nothing sent) could see
it. The ninth verdict was a `NOBUILD!!` naming an orphaned `ok`, which is a bad sed
rather than a claim, and was re-aimed.

**The reading pass found two more, and both are citation-shaped.** (1) Ten sites cited
*"§6.7 step 2"* / *"§6.7 step 3"* — **core §6.7 has no numbered steps.** The four steps
are State-Machines §2's, and §6.7 says so in terms (*"the contract's four steps:
[mechanism: State-Machines §2]"*). Worse than imprecise: `Executed is §6.7 step 2's
"authoritative final cumulative executed quantity"` is a **verbatim quotation
attributed to a section that does not contain the phrase at all** — it appears nowhere
in the core. This is C1's citation-to-a-nonexistent-section tell, inside the same file
that already cited *"SM §2 step 1"* correctly two paragraphs above, and a reader
checking gate 3 against §6.7 would find no step 3 and could conclude the gate was
invented. Retargeted; the genuinely-§6.7 quotations (*"a point-in-time not-found cannot
collapse an in-flight order"*) were left, and the contrast is what makes the file
readable. (2) **State-Machines §1 stated two different evidence rules for the SAME
target state.** The FLAG-1 rows reach `Canceled` by *"explicit venue-terminal cancel +
the final executions sweep"*; the **venue-initiated-termination** row — added by
v0.9.11, and the one that fires on this system's *ordinary* outcome — named no sweep at
all. §2 admits no per-source exemption (*"the ONE canonical boundary … never a weaker
cancel-ack-only variant"*), and the code already refuses one: `Resolver.present` sweeps
before recording **every** terminal, on the argument that a per-state exemption would be
a second rule about evidence living in one function. A partially-filled IOC is where the
gap bites — its status is terminal before its last fills have been read. The row now
carries the rule; nothing in the code changed, because the code was already stricter
than the row it implements.

**Milestone 3 slice C2b is landed: the order READ path exists (v0.9.33, 2026-08-03)
— and §4's `Cursor` cannot be either venue's cursor.**
Design record **§52**. `engine/venue` gains `OrderByClientID` across all five order surfaces, the
normative Bybit realtime→history fallback, the bounded executions sweep, and the normalized
status/execution DTOs; **Profiles gains §8**, the read-surface verification. **§47e finding 1 is now
fully closed** — milestone 1's undelivered `engine/venue` account/order half has both its write and
its read side. **C2b kept its own number on §51a's own argument**: folding it into slice D produces a
slice *larger* than "build C2 whole", which §51a rejected because what a long session cuts is the
**evidence**, not the code — an argument that rejects the smaller merge cannot admit the bigger one.
The fold's real objection (the query methods have no consumer) is answered by **where the slice
stops**: nothing is added to `oms/application/ports`, so slice D declares the port **with** its
consumer, exactly as C2a's `ports.OrderPlacer` landed beside `dispatch.go`.

**THE FINDING: the obvious `Cursor` satisfies §4 completely and makes §6.7 inexpressible.** §4
declares `Fills(VenueInstrument, since Cursor)` and says nothing about what a `Cursor` is. The three
execution surfaces offer an **ascending trade id** (both Binance families) and an **opaque descending
page token** (Bybit), and **none of the three can be compared to a time** — while §6.7 step 3 sweeps
*"through a venue/server watermark strictly later than the order's cancellation-effectiveness / max
landing horizon"*, which is a comparison against a horizon. So a `Cursor` defined as *"whatever the
venue hands back"* — the idiomatic choice, and the one each venue's documentation invites —
implements the signature **correctly on all three surfaces** and leaves the one clause that consumes
it unwritable. **The cursor is therefore a venue server instant**, and the venue's own handle is
demoted to within-sweep paging. The venues corroborate the same conclusion independently: on **both**
Binance families `fromId` and the time window are **mutually exclusive**, so a watermark-bounded
sweep cannot also resume by id. **The transferable shape is that a cursor's DIRECTION is not in its
type** — Bybit's `nextPageCursor` walks into **older** data, so storing it as the next `since`
resumes at the oldest row of the previous sweep and re-reads that window forever **while every
request succeeds, every page decodes, and dedup silently absorbs the duplicates**. Nothing fails.

**The derivation is what found it, and it was found before a line was written.** The DTOs were
derived from State-Machines §2's four-step finality contract rather than from the endpoints' response
shapes — the deliberate mitigation for freezing a schema whose Go consumer arrives in slice D. Two of
this slice's four findings came from asking the *contract*, not the venues.

**Not-found is a RESULT, not an error — and the venues put it in opposite layers, inverted from
C2a's finding.** Binance states an absence in an **error envelope** (`-2013 NO_SUCH_ORDER`); Bybit
states it in a **success** body with an empty list. C2a found Bybit stating a *refusal* inside a
success status; this is the same seam with the venues swapped. `-2013` on a 4xx classifies
`EffectNotApplied`, which is true about the **query** and a category error about the **order**, and
propagated as an error it is indistinguishable at the call site from a rate-limited or timed-out
query — the inference §5.3 forbids in terms. **An absence is a value; only an unanswered query is an
error.**

**The stable-not-found rule has an EXPIRY DATE — and the reading pass found HALF of it already
recorded, which makes the finding worse.** Profiles §4's history-retention row has said *"some
unfilled-cancelled orders purged in ~3 days"* since **2026-07-20**, self-flagged *"corroborated —
browser-confirm exact windows"*: the fact was written down, was marked as needing exactly the
verification Profiles §8 has now performed, and **its consequence went underived for a fortnight and
several slices**. What hid it is *where* it sits — §4's row is about **history enumeration for the
§7.4 actuator**, so a reader carrying §5.3's stable-not-found question never had cause to open it,
and the composition that bites needs both halves at once. **This is C1's citation-to-a-section-that-
does-not-exist tell INVERTED**: there the cited section was missing; here the section exists, says the
thing, and nobody re-read it. Profiles §8 is now authoritative for the figures and states that §4's
are the actuator's approximations — the verbatim condition is **conjunctive**
(`CANCELED`-or-`EXPIRED` **and** no filled trade **and** created + 3 days), and a rule reading it as
*"orders vanish after 3 days"* would discard evidence it still has. The finding stands as follows.
Binance USDⓈ-M drops a
`CANCELED`/`EXPIRED` order with **no filled trade** after **3 days**; Bybit's history drops
`Cancelled`/`Rejected`/`Deactivated` after **24 hours**. §6 step 6 sends only marketable IOCs, so an
IOC that did not fill — terminal immediately, no execution — **is** that row: **this system's
commonest terminal outcome is the one that becomes unqueryable soonest on both venues.** Past the
cliff the query cannot separate *"existed and expired unfilled"* from *"never existed"*, while §5.3
maps a stable not-found to `AbsentAfterFinality`, whose whole meaning is **proved** absence. **The
spec's rule was already right and what is new is that it is not always satisfiable**: §6.7's
composition holds *inside* the retention envelope and has no meaning outside it. §5.3 is edited to
bound the claim; #163 builds it. **The exposure reading is unchanged either way**, which is exactly
why it is easy to dismiss — both readings contribute zero delta, nothing is mis-hedged, no invariant
fires, and what breaks is the **evidence class**. It is
`[[documented-zero-meets-zero-is-unusable]]`'s family: a **venue-published bound** colliding with a
repo rule written as though the quantity were unbounded.

**On Binance a swept execution carries NO client order id, and a shipped comment said that could not
matter.** Neither `/fapi/v1/userTrades` nor `/api/v3/myTrades` returns a `clientOrderId` or an
`origClientOrderId`, so the venue's `orderId` is the **only** join key; Bybit's `execution/list`
returns `orderLinkId` on every row. C2a's `ports.Ack.VenueOrderID` said *"nothing in this system
REQUIRES the venue's id"*, **justified by a parenthetical naming the one venue where that holds** —
C2a's own finding 1 arriving in the **reassuring** direction, which is worse, because a hazard note
at least announces that something needs checking. The composition still closes, and only by a second
venue fact (Binance retains a filled order 90 days), which makes `Fills` and `OrderByClientID` a
**composition** where §4's signatures suggest independence — and leaves the sweep useless alone in
exactly the scenario it exists for, a lost ack. §4 is corrected.

**Evidence: 26 mutations, 26 kills after fixes; five survivors, four real gaps.** The one to carry is
`bybitExecutionSide("Buy") → Sell`, which is **C2a's own §51g survivor one slice later on the mirror
path**: C2a fixed the identical shape on the write path and checked the mirror across **venues**; the
mirror it missed is the other **direction of travel**. Every execution fixture *sold*, because the
flagship's perp leg sells — and the spot leg of every cash-and-carry trade **buys**, so the unreached
arm was half of all real fills. **The mirror rule has a second axis.** The fifth survivor was an
**equivalent mutant**, and diagnosing it was the work: `AbsenceFromError`'s effect check is
unreachable because every `apperror` constructor that sets a `Code` also pins `Effect` to
`EffectNotApplied`. The guard is **kept** and now reached via `apperror.New`, because its necessity
rests on an unstated invariant of a *different* package — a constructor for "a 5xx that carried a
venue code" would make the branch live the day it lands.

**Milestone 3 slice C2a is landed: the order write path exists (v0.9.32, 2026-08-03)
— and one venue states a REFUSAL inside a success status.**
Design record **§51**. `engine/venue` gains `Place` for both venues across spot and perp, the
normalized request DTO, both venues' create-order codecs and the response envelope reader;
`oms/infrastructure/driven` gains the real `ports.OrderPlacer` ACL adapter, so the OMS module test
no longer fakes the port. **C2 was split again by layer** — C2a is the write path, C2b the read path
(task #172) — because C2 as scoped measured larger than C1, and what a long session cuts is the
**evidence**, not the code. The rejected *fold-into-slice-D* alternative is recorded with its
objection **adopted**: the query methods have no consumer until slice D declares one, which is the
schema-field-frozen-before-its-consumer shape, so #172's ports land with their consumer either way.
**Profiles gains §7** — the submission-surface verification, the other half of what §47f's
non-existent-§7 citation claimed was banked. *(§50's block below says "there is no §7"; that was true
when it was written, and this is the section that answers it.)*

**THE FINDING: Bybit answers a REFUSED request with HTTP 200, and the classifier read it as an
acceptance.** Verified live on public testnet endpoints, no credential involved: a malformed Bybit
query returns `HTTP 200` with `{"retCode":10001,...}` while the equivalent Binance query returns
`HTTP 400`. C1's classifier switched on status alone. **The worse outcome is one step further on**: a
Bybit refusal body carries no `orderId`, so the ack decode fails — and a failed decode *after a
success status* is classified `EffectApplied`, meaning "the order is live and our record of it is
broken". An **explicit venue rejection** would have been recorded as an order that **definitely
exists**, collapsing §5.3's two evidence classes (proved absence vs. explicit rejection) into their
opposite. **Profiles §6 had already named this exact mechanism** one layer over, for Bybit's
zero-length 401 body — *"the parse failure is then what classifies the request's effect"* — and the
second instance landed anyway. **A hazard written down for one INPUT is not thereby covered for the
others**, which is the transferable form. The seam was even pre-built and unwired: `ClassifyStatus`
already took a `code` parameter and every caller passed `""`.

**The composition has a DIRECTION, and the general arm matters as much as the Bybit one.** A body may
**contradict** a success status; it may **never soften** a failure one — a 5xx that classified
`EffectUnknown` keeps that reading even where the body documents a clean refusal, because a venue
that could not answer is a venue whose account of what it did is in doubt. And an error body beside a
2xx with **no venue rule for its code** is `EffectUnknown`, not an acceptance: without that arm the
fix would have had a hole of exactly the shape it was written to close. It is reachable and tested on
**Binance**, not on the venue that motivated it.

**§7.6's shed classes had NO CARRIER anywhere, and both defaults are unsafe unequally.** The class of
an order is a property of the §5.3 **command** that produced it, and neither the `Order` aggregate
nor its projection records that command — so the adapter is the layer that must state the class and
the only layer that cannot derive it. `Opening` sheds a **flatten** during exactly the incident the
reserve exists for; `Safety` spends the reserve on ordinary trading and needs a second coincidence
before it costs anything. **A silent default would have been the first kind**, so `Place` and
`Dispatch` take it explicitly, checked **above** the `DispatchAuthorized` fsync — refusing after that
fsync would leave a durable authorization for an order nothing ever sent. Core **§4, §5.3 and §7.6
are all edited**; §5.3's table is total over the command set, and `Opening` turns out to contain
**exactly one command**. **The better long-term shape is recorded and deliberately not taken**: a
field on the `Order` aggregate naming its originating command would also decide §5.3's
`stop-opening` exemption — one missing carrier serving two rules — and it is a §5.1 aggregate change
reaching the journal codec, the planner and recovery. Cheap only because §6's orchestrator has no
production caller yet, and it would not have been later.

**A documented venue ZERO met this repo's zero-is-unusable default.** Binance documents
`POST /fapi/v1/order` verbatim as *"0 on IP rate limit(x-mbx-used-weight-1m)"* while spot's is
*"Weight: 1"*, so the truthful cost for placing a futures order was the one `Cost.Validate` refused —
and the refused endpoint is the most important one in the system. The argument survived and only its
subject was wrong: the check is now *"charges no dimension a caller fills in"*. **The collision is
invisible because both readings are "a zero in a required field"**, and the tell was reading the
weight line verbatim instead of assuming it. Where the **venue** publishes a zero, the rule inverts.

**The reading pass found one, and it is the zero-fix's own consequence (task #173).** A `Cost`
charging a dimension the profile declares no budget for is **silently uncounted** — the governor
walks the *profile's* windows, so a dimension with no window is never visited, and a governor that
counts nothing admits everything. Accepting the documented zero is what made that reachable: it left
Binance's futures new-order as **the one call whose entire cost rides on a single dimension**, so a
profile with weight budgets and no order-count budget charges *nothing* per futures order against a
venue that bans on order counts. **Nothing is broken today** — no production `ratelimit.Profile`
exists anywhere — so it is recorded with its **trigger named as the construct**, per §3.6's deferral
rule: the first production profile. §7.6 states the requirement from this release. **The shape is
the finding**: a correct fix **relocated the premise** onto a different object that nothing checks —
`FM-5`'s family at the configuration layer, where *a validator cannot audit the thing it is
validated against* — and the marker grep is blind to it because the clause is **absent** rather than
departed from.

**An assumption CLOSED and a narrowing recorded.** `ids.ClientOrderID`'s standing *"confirm before
milestone 3 places a real order"* is discharged: both venues' id rules are verified, the minted
format is inside both with **one character of headroom**, and the rules **differ** (`.` is legal on
Binance, illegal on Bybit) — which is what gave the tests a discriminating fixture they otherwise
could not have had. Separately, `OrderRequest` has **no `Type` and no `TimeInForce`**: §6 step 6's
marketable-IOC-limit rule is enforced by the *type*, and the argument for the narrowing is recorded
because a narrowing carries one — an enum whose only legal value is the one we emit **declares a
member nothing emits**, satisfying the compiler, `exhaustiveswitch` and switch coverage while leaving
it unreachable. The narrowed-away case is a resting order, and it arrives with its consumer or not
at all. **20 mutations, 20 kills.** The one survivor is §50's finding one layer down, in the tests
rather than the venues: `binanceSide(Buy)` was mutated to return `"SELL"` and everything passed,
because no Binance assertion ever read the buy fixture's side — while the Bybit mirror was covered
for free by a **byte-for-byte body comparison**. **A whole-artifact assertion covers fields you never
thought to check; a field-picking assertion covers exactly the ones you listed** — so the collapse
checklist runs against the assertions, not only the fixtures.

**Milestone 3 slice C1 is landed: `engine/venue` has a signed REST transport (v0.9.31, 2026-08-02)
— and the two adapters' EVIDENCE is not symmetric even though their code is.**
Design record **§50**. §47e finding 1's first half is closed: credentials that cannot leave their
package, both venues' verified signature compositions, a clock-injected timestamp, the §7.6 governor
attachment, and §5.3's effect classification. **Slice C is split by LAYER rather than by venue**, on
§47f's own standing hazard — splitting Binance-then-Bybit would put the two clients in different
sessions, which is exactly the mirror failure that hazard names. C2 carries `Place`,
`OrderByClientID`, `Fills` and the Bybit history fallback.

**The scope note's premise was false, and the tell is cheap.** §47f's slice-C row cites *"profiles
§1/§4/§7"* for facts it says are already recorded. **There is no §7** — the document has ended at §5
throughout — and the string each venue signs was recorded nowhere in the doc set. This is §47e's own
shape one layer out: §47e caught a normative claim the code silently failed, this is a **verification
claimed as banked when it was not**, and both are invisible to the marker grep because the clause is
*absent* rather than *departed from*. **A citation to a section that does not exist is the tell**, and
it is the same mechanism as an unchecked rejected alternative applied to a pointer instead of an
argument. Profiles §6 is the verification it assumed.

**THE FINDING: the evidence does not mirror, even where the code does.** Binance publishes **three**
reproducible HMAC vectors (two documents, two secrets, all three re-computed and matching); Bybit's
worked example **masks its secret**, and the one hypothesis that would rescue it — that the secret is
the literal placeholder — was **tested and refuted**. The credential-free differential probe (sign
with an invented key twice, correctly and incorrectly, and read the venue's complaint) is **refuted
on both venues**: Binance answers `-2015` to both and Bybit answers HTTP 401 with a zero-length body
to both, because each checks the credential *before* the signature. **That refutation is only
trustworthy because Binance ran first as its positive control** — on Bybit alone the observation is
two identical 401s, readable equally as "no complaint, so it is right", with nothing to arbitrate.
So `BybitSigner` has **no offline oracle at all** while `BinanceSigner` has three, the two will look
symmetric in the code, and their passing test counts are **not comparable evidence**. §47f's mirror
rule covers fixes propagating to a mirror; **its inverse does not follow from it**. Task **#171**
carries the only thing that can close it: a real testnet credential.

**§7.6 was wrong about Bybit's header, and is corrected.** It folded *"the reported consumption
(`X-MBX-USED-WEIGHT-*`, `X-Bapi-Limit-*`)"* upward-only — but `X-Bapi-Limit-Status` is verbatim
*"your **remaining** requests"*, the complement. Folding it directly **inflates** the local count when
the venue reports room (90 remaining > 30 used → adopted) and **discards** the reading when the venue
reports almost none (2 remaining < 30 used → ignored) — and the discarded direction is the silent one,
and the one that ends in the ban the rule exists to prevent. Fixed to
`used = X-Bapi-Limit − X-Bapi-Limit-Status`, deriving **nothing** when the cap is absent. It was
invisible because both headers are integers about the same budget in similarly-named fields: the
**format-collapse family one layer up**, where the collision is between two fields' *meanings* with
types, shapes and ranges all identical.

**A design decision worth carrying: the acceptance window is asymmetric and only one side is
configurable.** Both venues test `timestamp < serverTime + 1000 && serverTime - timestamp <=
recvWindow`. `recvWindow` widens only the *behind* side; the ahead side is a fixed 1000 ms no
parameter can raise. So the documented knob for timing rejections repairs one direction and cannot
touch the other, and a host whose clock runs fast keeps failing intermittently with the fix
apparently applied. Encoded as a named constant, composed with `platform/clock`'s uncertainty band —
signing is refused when the band does not fit inside `min(aheadTolerance, recvWindow)`. And
`recvWindow` **unset is refused** rather than defaulted, because the venue's 5000 ms default silently
adopts the venue's policy, and `recvWindow` bounds how long a delayed non-idempotent `Place` can
still execute after we stopped waiting for it.

**Reading pass:** §7.6's *"surface 418 / IP ban as immediate operator alerts"* is unmet end-to-end —
the classification produces `PenaltyBan`, `platform/ratelimit` states it does not deliver alerts, and
nothing reads `Snapshot.Bans`. Unmarked, because the consumer is absent rather than wrong. Task
**#170**. **18 mutations, 18 kills.** Three survivors, and one is a **new shape**: the credential
redaction's fixture was varied correctly (key and secret given different lengths) while the
**assertion** — a substring containment — could not see the variation. The collapse checklist's tell
gains a sibling: *the assertion does not distinguish the thing under test*.

**Milestone 3 slice B is landed: the planning use case exists (v0.9.30, 2026-08-02) — and what closed
`module.go`'s stated hole was a SIGNATURE, not a check.**
Design record **§49**. §6 step 5 now has an implementation: deterministic `ClientOrderID` minting,
every leg's intent fsynced to the write-ahead journal before any submission, `ApplyPlanned`'s first
caller, the OCC version threaded into `Dispatch`, and `Deps.Journal` converted from a port field to a
path — the conversion `module.go` predicted and `singlewriter` forced, because with the adapter
inside a policed package the only remaining way a root could fill that field was by writing a second
write path to order state.

**`Dispatch` takes an id and a version and READS the order from the write repository.** Until this
slice there was no alternative — no planning path existed, so the caller was the only source of an
order. Keeping the old `Dispatch(domain.Order)` would have left two holes open with the mechanism to
close them one field away: an order that was **never planned** could be dispatched (no journal
record, nothing for §7.5 to correlate), and an order could be **mutated between planning and
dispatch**, so the journal describes one order and the venue receives another. Neither is expressible
now, and the property is enforced by the type system rather than by a check — which also means no
mutation can test it, so it is recorded as a claim the signature *is*. One consequence points the
other way and is worth the same weight: `admit`'s `Order.Validate` had to be **removed**, because
with the order arriving from a repository whose `ApplyPlanned` refuses a malformed one, it became a
branch no test could reach — an unreachable sentinel reading as a handled case. Its removal is only
sound because the refusal is now a **port contract** rather than one adapter's behaviour.

**The `TradePlanned` record is deliberately not written, and the argument is the reusable part.**
§6 step 5 names *"`TradePlanned` + intents"*; this writes the intents. The easy reason is
insufficient: §5.1's aggregate has no `Trade` type. What makes the omission **safe** is a *different
clause of the same sentence* — *before any submission*. Because every leg is journaled before `Plan`
returns anything dispatchable, the only state a partially-journaled plan can reach is one where
nothing was sent, and §5.3 prices that exactly: *"a `Planned`-only order was never dispatched (no
venue order can exist) → safe to replan or discard"*. The question a trade-level record would answer
has no consequence attached to it. That argument has a premise about code, so the premise is a
**run** rather than a paragraph (`FM-7`'s lesson, `P6`'s habit): `Plan` is phase-wise, and the test
breaks the *last* leg's journal write and requires the repository to be **entirely empty**. The
change that would break the argument is named in the spec — a planner that dispatches leg 0 before
journaling leg 1.

**THE FINDING: `FM-5`'s shape arrives in Go, at the projection layer.** §6's write path is
journal-then-apply, and an apply that fails *after* its `Place` has run leaves the order at
`Planned` in the current-state store while the venue holds it — and `Planned` is the one non-terminal
state contributing **no** possible-delta interval. A sent order therefore reads to the §6.7
classifier as contributing nothing, which turns an **ambiguous** residual into a **determined** one
and unblocks precisely the ≤5 s compensation the ambiguity existed to hold. The projection does not
merely lag; **it lags in the direction that unblocks the held action.** This is the family's
signature — *a rule evaluated on a quantity read from a store that a different store is the authority
for* — and it inherits the family's consequence unchanged: **no invariant computed from the
projection can detect it** (§46 §8e, one layer out of TLA⁺). Landed as a refuse-to-trade clause in
§6 step 7, recoverable because the journal is written first; task **#169** carries the response to
slice E/F. It carried **no** marker and the grep could never have found it — nothing departs from a
clause, the clause was absent — which is the reading pass beside the grep doing the job it exists
for.

Seventeen mutations, seventeen kills. All four `NOBUILD!!` were bad seds, and the discriminator is
now stated generally: *"does not implement"* is §3.6 rule 2's assertion working (a kill);
*"declared and not used"* is Go reporting that the mutation deleted a value's only reader (re-aim).
The collapse checklist found a **format** collapse rather than a cardinality one — `ClientOrderID`
renders leg and attempt as two decimal digits each, so at attempt 0 a field-order bug produces the
identical id, and the tell was the *format*, not the data. `singlewriter` was re-verified as §47f
demanded — by **running** the isolating probe (a context root importing the port, where rule 1 is
silent), not by reading the analyzer — and no new §3.6 gate is due.
| core §6 step 5, core §6 step 7, core §6 sequence diagram, core §12 milestone 3

**Milestone 3 slice A is landed: the OMS journal is real (v0.9.29, 2026-08-02) — and the engine that
holds it was chosen by a requirement that binds two slices later.**
Design record **§48**. `ports.Journal` had no adapter (§47e finding 2), so §5.3's
fsync-before-network crash boundary — the single most safety-critical ordering in the OMS — was
proven only against a test fake. It now has one: append-only, hash-chained, one fsync per record,
on SQLite in WAL mode at `synchronous=FULL`, **verified at open rather than assumed**.

**The store was the real fork, and it was invisible from inside the slice.** §47f described slice A
without naming an engine, and neither `AppendPlanned` nor `AppendState` needs a transaction — so a
bespoke hash-chained log file is the smaller implementation and the one a reader reaches for. What
forbids it is **§5.3**, not §13: the fill apply, the per-instrument REST cursor advance and the
journal/outbox append must share **one transaction**, and `driven/doc.go` had already recorded the
consequence — *a transaction spanning two stores cannot be added to one of them alone*. The log file
would be rewritten at slice D/F, or §5.3 would go unmet quietly. Generalised: **a component's
storage engine can be decided by a constraint that binds none of its own methods.**

Three findings came out of building it. **(1) The predicted fork was mis-premised.** Session 73's
handoff named the first design question as whether to reuse `engine/capture`'s codec, *"already
fuzzed, already has a hash chain"* — and `engine/capture` has no hash chain. It has per-record
CRC32C, a dense **sequence** chain, and whole-file `.sha256` sidecars. Reuse would have supplied
framing; the chain would have been new code either way. The benefit credited to a rejected — or
here, a *proposed* — alternative is asserted, and nobody re-derives it. **(2) A shared constant made
the obvious test vacuous.** `journalDSN` and `verifyPragmas` both read `wantSynchronous`,
deliberately, so the request and the check cannot drift; the cost is that mutating the constant is
**self-consistent** — the DSN asks for NORMAL, the verification demands NORMAL, they agree, and
every *Open succeeds* test passes against a journal that no longer fsyncs on commit. This is
capture's own note about a wrong CRC polynomial (*"a checksum that is self-consistently wrong
verifies its own output perfectly"*) arriving as a **configuration constant**, and the repair is the
same: the required values are stated as **literals** in the test, never read from the constants under
test. Normative for any self-asserted configuration. **(3) A collapse warning that stopped one
short.** §47f warned that a chain of length 1 collapses *"verify against the previous record"* into
*"verify against the first"*. True, and the repair that sentence suggests is a chain of **two**,
where the two readings are **still** the same answer — record 1 is both. Three is the shortest chain
that separates them, and the inequality is now asserted rather than left implied. A debt worded
around a specific number pre-answers which number fixes it.

Twelve mutations, twelve kills — fsync and the chain link bite-checked **separately**, as §47f
required — with the collapse checklist run before them, not after. Scope held: `Deps.Journal` stays
a port field (its conversion is slice B's, per §47f), chain **verification** is construction's
complement and belongs with §7.5, and §13's **third** durability precondition — local filesystem
only, never NFS — is **not checked**, carries no `SPEC EDIT OWED` (nothing departs; it is scheduled),
and is placed on §7.5's refuse-to-trade posture list rather than in an adapter constructor
(task #168). | core §12 milestone 3, core §13, core §5.1/§5.3

**Milestone 3 is admitted before strategy #2's gate (v0.9.28, 2026-08-02) — because the record that
rejected it contains the claim that refutes the rejection.**
Ruling record **§47**. §42d rejected *"proceed to milestone 3 now"* on the ground that the OMS would
be **"built 'on spec' for a strategy whose economics are unmeasured"** — and rejected a roster
rewrite, one sentence later, on the ground that **"milestones 3–6 are strategy-agnostic controls and
survive unchanged"**. The second is the load-bearing claim of the entire post-kill re-sequencing:
it is *why* the roster was annotated rather than rewritten. If milestones 3–6 survive a **complete
strategy replacement**, a gate verdict on strategy #2's economics cannot invalidate milestone 3, and
the first rejection's stated ground has nothing to bite on. **Neither rejection was ever read against
the other** — the precise failure mode a rejected alternative invites, since the benefit a record
credits a rejected option with is asserted and nobody re-derives a rejection.

The normative core sides with the second claim twice — **§3.1** (*"the foundation runs every Intent
through the **same** risk gate → OMS pipeline **regardless of strategy**"*) and **§1.1** (the
single-writer OMS is *"foundational, not tiered"*) — and **the practice had already departed from
the rule**: §12 milestone 3 names the ambiguous-order state machine as *"this milestone's safety
kernel"*, and sessions 68–71 built exactly that pre-gate as formal model (a), finding `FM-5`…`FM-9`.
The operative rule was never "no milestone-3 work before the gate" but "no milestone-3 work **in
Go**", which no record defends, and which leaves nine findings as corrections to prose that no
implementation has been measured against.

**The partition is by invalidation, not by milestone number**, which is why milestone 4 splits: the
`Fence`, `FenceReady` monitoring, kill-switch ladder and `audit` projection are per-venue and
per-authority, while the **`EmergencyPlan` contents and §4.1's posture-verification leg roster are
leg-shaped** — §1.1's $1K tier names *"the Binance perp and both spot IOC legs"*, which is
**cash-and-carry's** shape, not strategy #2's perp-perp. Milestones 5–7 stay hard-gated: §41r
caveat 1 makes backtest fidelity per-strategy by normative text, and milestone 6 fixes BTC-only
canary scope. **The gate's kill authority over live capital is untouched — construction order moves,
promotion order does not.**

**The residual risk is project mortality, not rework, and it is priced rather than argued away**
(§47d): if strategy #2's gate also fires the project stops, and the base rate is **1 gate fired out
of 1 gate run**. Against it — nothing built is strategy-specific, so a *third* strategy inherits all
of it; the work converts `FM-1`…`FM-9` from prose into checked claims; and the wait is not free, at
11.165 GiB/day with the gate at day 59 of an 85-day disk, on the roster's longest-lead item.

**Two scope findings came out of drawing the boundary (§47e), and both change milestone 3's real
size.** (1) **Milestone 1's `engine/venue` account/order half was never delivered** — the package
holds market data only, and §12 records the milestone complete; this is a normative claim the code
silently does not satisfy, which the `SPEC EDIT OWED` grep is structurally blind to. (2)
**`ports.Journal` has no adapter**, so §5.3's fsync-before-network crash boundary — the single most
safety-critical ordering in the OMS — **is today proven only against a test fake**. `module.go` is
honest about the write repository being unwired and argues the absence well; the journal's absence is
the same shape one layer down and was stated nowhere.

**Milestone 2b step 2's disk figure is measured (v0.9.27, 2026-08-02) — and the derived bound it
checks is wrong at both ends, for a reason worth more than the number.**
The §32m-D ritual ran on the expanded capture three hours inside its 48-hour deadline
(capture-expansion record §44e). The measured day is **24 whole hours, 2026-08-01T12:00Z →
2026-08-02T12:00Z** — deliberately *not* the 11:13Z restart anniversary, because that hour holds
~11 minutes of the previous process for the eight BTC sources and only ~47 minutes of life for the
32 new alt sources, so a tier split over it is biased by construction in the direction that flatters
the tier being tested (~1%, sized rather than hand-waved). **11.165 GiB — BTC tier 4.314, alt tier
6.852, per shortlisted symbol 0.387 (MUSDT) to 1.743 (BEATUSDT)** — over an archive that testifies
to its own honesty: the full-day verify reports **0 integrity / 50 coverage**, and all **960
per-hour source lines read `alive 60/60`**, so this is a fully covered day rather than the
lid-window under-measure the 2026-07-31 figure was. A second verify over the restart hour reports
the restart exactly as §44d promised — **8 `process_restart` gap records** (the pre-existing BTC
sources) and **32 `alive_short`** (the new alt sources at 47/60), the tier split reporting itself.

**The finding is that §42c's aggregate was right while its premise was false.** The derived
~0.6–1.1 G/day/symbol rested on "the two fixed-rate Binance streams contribute ~450M/day regardless
of symbol activity; the two Bybit streams scale down". The eight-symbol total landed inside the
derived +5–9 G/day band (6.852) and the mean per symbol inside 0.6–1.1 (0.856) — and **no symbol-level
claim survived**: MUSDT is 35% below the floor, BEATUSDT is 1.58× the ceiling, and BEAT's Bybit book
(959.5 MiB) *exceeds BTC's* (678.9), so "alts likely lower" fails too. The cause was measured rather
than inferred, by splitting bytes/day into **records/day × bytes/record**: bytes/record is constant
across the nine perp streams (966–1132 B for a 20-level book) while the delivery **rate** over those
same nine runs 2.68/s to 8.99/s against the venue's 10/s ceiling — 9.98/s on BTC spot, the closest
anything comes to saturating it. A venue's documented **update speed is a maximum, not a cadence** —
both venues' book streams push only when the book changed. Verified as venue behaviour and not local
loss, which is the claim that matters since both produce the same byte count: over one hour of the
quietest symbol the **`pu` chain has zero breaks**, the minimum inter-push interval is **102 ms** (the
cap, honoured exactly) and the maximum silence is **22.5 s** (94.8 s over the full day). Landed in the profiles' 2026-08-02 dated
block, because the next consumer to assume "fixed rate" would be the read path — and a 22.5-second-old
book on a quiet alt is normal venue behaviour, so a staleness rule keyed on it is a liquidity
judgement, not fault detection. **The same refuted premise turned out to underwrite an operational
setting**, which is the part carrying a to-do: §44b copied BTC's 10 s `first_data_timeout` to all 32
new sources *because* depth20 was believed fixed-rate, and Binance partial depth — unlike Bybit's
topics — sends no subscribe-time snapshot, so on the quietest alt the stream is silent longer than
that deadline for 3.42% of the day (max 94.8 s) and ~1 reconnect in 100 would trip it while behaving
perfectly. It self-recovers and has not bitten; it is recorded rather than fixed, because the setting
is restart-required and a restart is an operational act with its own gap record. **`markPrice@1s` is the one genuinely fixed-cadence stream** (exactly
86,400 records/day on 7 of 9 symbols) and is therefore the positive control for the whole reading:
the class §42c assumed exists does exist, and the depth stream is not in it.

**That same stream turned out to be an instrument, and it is the transferable part.** Because its
expected count is exact, its byte or record count *measures coverage*: it prices the two reconnect
hours at **103 s and 48 s of lost data inside hours the coverage metric scores `alive 60/60`** (not a
verify defect — `alive` counts minutes touched — but a limit §45's decision-grid joins must know);
applied backwards it re-reads the 2026-07-31 day as **~61% covered**, where the "naive scale-up to
full wall coverage" that produced 3.6–3.8 G/day had assumed ~1.2×; and it caught a defect in this
session's own reading tooling, which had silently dropped the `gapped` hour lines and would have
published a false 91.67% coverage figure for one symbol. **A one-number scale-up cannot separate
coverage from activity** — which is the whole reason the ritual measures.

**§38f's deferral was re-argued on the measured clock and does not survive intact (ruling §42f-3's
named revisit point).** 954 GiB free, 11.165 GiB/day: **85 days to zero free, 76 to a 100 GiB
floor** — and the gate the whole schedule waits on (BCHUSDT, ~2026-09-30) is **at day 59**, by which
point ~678 GiB of un-backfillable archive sits single-copy with ~295 GiB of disk left. §42c's "~3
months of local disk" survives as an order of magnitude and dies as a plan. **Books are 93.6% of the
daily bytes**, so the un-backfillable half is essentially the whole burn, and §38f's own premium
estimate ("~6 GB/day and a one-line rsync") was **half the real rate**: the rsync line is unchanged,
the sizing is not — **≥2 TB**, not the 512 GB a session-64 note assumed. The recommendation to the
user is to take the target now rather than at a third deferral; compression and retention stay
refused on §38g's original grounds. Record: capture-expansion §44e; profiles' 2026-08-02 dated block;
core §12 milestone 2b step 2's landed-figure annotation.

**Formal model (a) found a fifth spec defect (v0.9.24, 2026-08-02) — `FM-5`, and it lives in a composition neither machine owns.**
`docs/formal/AmbiguousOrder.tla` composes SM §2's ambiguous-order classifier and finality contract
with SM §7's `PackageDelever` generations on **one shared reservation ledger**, which was the whole
reason to model them together (design record §46 §2.1: model (b)'s headline defect was also a
composition defect, and here the ledger has three producers). Slices A–D are built, verified and
documented — **75 runs** through `docs/formal/run.sh` as of slice F, a companion artifact
(`docs/formal/ambiguous-order-model.md`) mapping every clause of both machines' Invariants sections
to a construct or to a stated omission. **Slices E and F followed** — the adversarial round and
`FM-8`'s encoding — and between them the model has now found **five** defects (`FM-5`…`FM-9`), each
with a normative edit landed **and** a run that demonstrates it. **The defects are below**; the model's own methodological
results are recorded in §46 and are not normative.

| Finding → resolution | Where |
|---|---|
| **FM-9 a supersession rule whose trigger names a quantity that INCLUDES ITS OWN SUBJECT. v0.9.25 landed FM-8's standing precondition with the trigger stated as "the §2 classifier leaves `Determined` for this hedge group". But the generation's OWN perp reduction is an exposure-changing order on that group: `PdSubmitPerp` reclassifies with the leg's interval in the sum, so `cls` leaves `Determined` on the generation's FIRST STEP — and §7 DEPENDS on that, because it is how §2 refuses every compensating trade until the leg reaches finality. Read literally the row therefore fires on the generation's own machinery. Control `NC22` (the literal sentence, encoded) reaches it 4 STATES FROM `Init` WITH NO EXTERNAL EVENT AT ALL: `realized = 0`, nothing outstanding, ledger empty — a generation opened on a genuinely, physically flat package and aborted one step later by itself. A spurious abort is not bookkeeping: §7's generations exist because the perp short is nearing liquidation, so cancelling one cancels the mechanism that was reducing the risk, and does it with a reserved leg out. RESOLUTION — LANDED v0.9.26: the trigger is the ENTRY TEST RE-EVALUATED NET OF THE GENERATION'S OWN LEGS — this generation's unsold transient removed from the realized delta and its own ledger entry removed from `outstanding_reserved_reductions`, both sides of the same subtraction. That quantity is INVARIANT under every step of the generation's own ladder (perp fills raise the transient and amend the entry by the same amount — FM-5's fix stated as a quantity), so it moves only when something EXTERNAL moves it, which is what "the premise dissolved" means. Checked as `INV10`, whose negative control is the spec's own literal sentence — a first for either model. THE RULING IS UNTOUCHED: standing-vs-entry was not reopened; what the model supplied is the quantity that makes the ruling coherent. THIRD FINDING IN ONE FAMILY: FM-5 (an entry read before its backing exists), FM-8 (an entry's premise outliving it), FM-9 (a rule evaluated on a quantity including its own subject) — all three are §2 and §7 disagreeing about what the shared ledger means, and none was visible to any invariant phrased over the ledger** | SM §7 (transition row + standing-condition invariant), `docs/formal/` — **CONSUMER LANDED v0.9.55 (task #177)**: the netted premise is `Classifier.PremiseFor`, computed by the *same* fold and subtraction as the ordinary verdict with the generation's legs and its own entry held out, so the two cannot drift; the trigger is discharged by `Deleverer.Reassess` rather than reported. The build found the decomposition's own instance of this defect — the netting identifies the generation's entry by **producer**, so with two live entries a **sibling's** would be netted out, which is why the entry gate counts and why the reservation is taken before the premise is asked for |
| **FM-8 §7's "no naked residual" is enforced ONLY as an entry guard, so a live generation can hold a premise that has dissolved. `PdPlan` opens a `PackageDelever` generation on `TrueResidual = 0`, which is RESERVATION-NETTED (`realized − outstanding_reserved_reductions`) — so it admits a package that is neutral *pending somebody else's in-flight reduction* rather than physically flat, while core §5.3 says PackageDelever reduces leverage "on a STILL-NEUTRAL package" and §7's invariants say "the package has NO NAKED RESIDUAL". Probe `PC27` reaches the opening itself at 3 steps (`realized = −2`, both legs zero: physically short, exactly offset). Probe `PC28` reaches the harm at 4: the offsetting reduction's OWN result comes back ambiguous, SM §2 row 4 RELEASES its ledger entry as specified, and the state is a live generation over a package that is physically short AND no longer determined, with nothing reserved against it. NOT A MODEL ARTIFACT — the release is exactly what §2 row 4 mandates; what is missing is anything that tells the GENERATION. It is FM-5's mirror as §8f predicted: FM-5 is an entry counted BEFORE ITS BACKING EXISTS, FM-8 is an entry's PREMISE OUTLIVING THE ENTRY. Invisible to every invariant in the module because INV5 polices the generation's own legs (`pdSold <= pdDone`), never the package premise it opened under. §8f asked WHICH neutrality; the probes say the sharper question is WHEN it is checked — even the strong physical reading, applied at entry only, can be falsified the instant afterwards. RESOLUTION — RULED by the user 2026-08-02 and LANDED in v0.9.25: the STANDING PRECONDITION. "No naked residual" is a standing condition, not only an entry test, so the §2 classifier leaving `Determined` for the hedge group SUPERSEDES an active generation, quiescing its reserved second leg to §6.7 finality first — a third row beside `EmergencyClosePackage` and `FenceReadyLost`, policed by the SAME quiesce invariant rather than a new one. **CORRECTED BY `FM-9` (v0.9.26): the trigger as worded here — the classifier's verdict — is a quantity THE GENERATION ITSELF MOVES, and the row must read the premise NET OF THE GENERATION'S OWN LEGS. The ruling is unchanged; the quantity is.** The reservation-netted ENTRY reading is retained (a reserved reduction genuinely does offset the exposure it reduces); what changes is that it is no longer sufficient alone. The rejected alternative — tightening `PdPlan` to physical `realized = 0` — was NOT adopted because it MOVES THE WITNESS RATHER THAN REMOVING THE CLASS: a generation may open on a genuinely flat package and have a floor issued underneath it a step later, dissolving the premise identically. It is kept on record as an available hardening, not as a refuted idea. **THE MODEL ENCODING AND ITS NEGATIVE CONTROL ARE OWED (slice F, sketched in §46 §11f): the ruling is normative from this release, but the run showing the hole closes has not been performed — a fix argued is not a fix checked, which is this slice's own repeated lesson** | SM §7 (transition table + two invariants), `docs/formal/` — **encoding LANDED v0.9.26, slice F: `NC21` violates `INV9`, `PC29` shows the row fires, `W5` shows the harm reaches a nonzero spot sale** |
| **FM-7 a rule whose stated justification assumes what the adjacent sentence forbids. SM §2's interval-floor rule says the floor is "never issued twice (else, for [0.4, 1.0], two uncertain 0.4 reductions could reach −0.4 and defeat the proof)" — but that counterexample REQUIRES the reservation not to be counted, and the neighbouring sentence requires it to be. Control `NC20` (NC2's constants, INV2 removed, INV1 alone) PASSES over the COMPLETE graph — 8,770,801 generated, 1,086,199 distinct, 0 left on queue: double issuance cannot overshoot the sign while reservation-aware sizing works. THE RULE IS RIGHT AND STAYS; its justification was the defect. Resolution: the parenthetical is retained as the statement of WHAT THE RULE PROTECTS AGAINST — the FM-5 world, where an entry is not backed at the instant it is read — and the rule is recorded as deliberate defence in depth over two independent mechanisms, the single-issue rule being the one that survives a defect in the other. Why it matters that this was fixed rather than left: NOBODY RE-DERIVES A RECORDED JUSTIFICATION, so a later reader checking the [0.4, 1.0] story against the sizing this spec mandates would find it does not hold and could retire a rule doing real work. HOW IT WAS FOUND, and the reason it needed a new run: §7d had already reported `overshoot` FALSE throughout NC2's counterexample — but NC2 lists TypeOK and INV2 only, and INV2 IS `floorCount <= 1`, so TLC halts at the first second-dispatch state and `overshoot` was never a checked invariant in that run at all. Non-vacuity cost no run for the same reason: NC2 violating INV2 already proves double issuance is reached. RE-MEASURED ON THE SLICE-F MODEL (v0.9.26): NC20 still PASSES over the complete graph — 8,948,671 generated / 1,093,953 distinct / 0 left on queue — the figures differing from v0.9.25's because slice F changed the state graph. The 8,770,801 / 1,086,199 figures quoted in v0.9.25's status line are left as measured then rather than rewritten: they record what that release checked. A finding's evidence being re-established on every suite run is the point of keeping its control** | SM §2 (interval-floor rule), core §6 step 4 |
| **FM-6 a verdict goes stale because the LEDGER changed and no order did — and the spec's own wording names it. §2 Scope says "classification is a function of TWO inputs and re-runs when either changes" (fills/position events move the realized delta; order-lifecycle/finality events collapse an interval). But the quantity it gates on is `realized_delta − outstanding_reserved_reductions`, which has THREE inputs, and the third is absent from the re-run rule — so the subtrahend, which decides the verdict exactly as the minuend does, moves on events that are not order events (a generation opening, amending as its perp leg confirms, completing, terminal-flattening, being superseded; a floor retiring). Model-checked signature is a DEAD END, not a violated invariant: every counter-leg interval collapsed, the verdict still `DeadlineExceeded`, its one permitted action (the interval floor) spent by FLOOR_ONCE, a determined naked residual and dispatch budget in hand — and nothing enabled. Resolution: three inputs, re-run on any; the machine may not wait for an order event when the ledger alone changed the residual, which is the argument §2 already makes for the cancel-with-zero-executions case, owed to the third input. WHY NO EARLIER SLICE COULD FIND IT: every path out of a generation runs through `PerpReduceFinal`, which reclassifies — the perp-side `Held/Alert` park is the unique route that bypasses it, and SM §7 has had that transition since v0.9 while the model did not (found by building it, §46 §10a→§11b). It is FM-5's sibling: FM-5 is "the ledger must be backed when read", FM-6 is "a change to the ledger must be observed" — both are §2 trusting `outstanding_reserved_reductions` as an input it never re-derives. Control NC19; the harness gained a `deadlock` verdict so a control that STRANDED the model cannot read as one that CAUGHT something** | core §6 step 7, SM §2, SM §7, `docs/formal/` |
| **FM-5 a reservation is counted before its counterpart exists. SM §7's FLAG-3 ruling ("the reservation stays generation-scoped *from creation*; the §2 classifier subtracts it *throughout*") composed with §2's *unconditional* reservation-aware sizing lets a generation book its TARGET into the ledger at `Planned`, before its perp leg has bought anything — whereupon `realized − reserved` reports a phantom SHORT on a package that is NEUTRAL, and §2's determined branch buys back exactly the exposure the generation's own perp leg is about to buy. TLC exhibits the unbacked ledger at 2 steps and the resulting dispatch at 3; the end position (net long by the target) follows arithmetically from the model's own transitions and is recorded as a derivation, not as a checked property. Resolution: FLAG-3's words are kept and its silence is closed — what it left under-determined is the reservation's AMOUNT, not its existence. The entry is created at `Planned` as ruled, but carries the generation's OUTSTANDING NET INTENT: zero until the perp leg confirms, amended as it fills (§2's own "coalesce / amend / cancel-replace, never a fresh command per event", applied to the reservation rather than to the order), so the ledger is exactly backed at every instant. Why four rounds of human and machine review and the model's own slice A all missed it: the invariant that would object is phrased over quantities COMPUTED FROM the ledger, so a corrupt ledger corrupts the check too and the phantom buy computes as non-overshooting — `overshoot` is FALSE in the violating state. AN INVARIANT BUILT ON THE LEDGER CANNOT AUDIT THE LEDGER. The literal reading is negative control `NC7`** | core §6 step 7, SM §2, SM §7 (FLAG-3), `docs/formal/` |

**Strategy #2's gate is code-complete (v0.9.23, 2026-08-01) — the procedure commit landed and the §45f-2 seal is lifted.**
The cross-venue gate design record's slice D closed (§45p, commit `7ccadec`): the `Proc*` roster, the
two admission judgements, the gate artifact schema, the runner and `arblab crossvenue gate` landed
together, because parameters are only pre-registered if the code that may read post-cutoff evidence
arrives with them. The roster's inputs are CHECKED against their sources rather than transcribed —
entry levels against the committed `shortlist.json` on every run, sizes by one stated rule (250 USDT
notional per leg, rounded up, lot-aligned on both venues, which at 4× is §8's $1K tier divided by the
roster) with the venues' own ledgered filters verified where the raw tree is present. **Two numbers
are deliberately READ rather than pinned:** §2.4 forbids hardcoded fee constants and §2.1 a hardcoded
hurdle, and the committed reference revision already answers both, so the runner reads them as-of the
declared bound and the assembly takes them as arguments — a roster carrying its own fee fields would
assemble with them at zero, and a zero taker fee is a free round trip. **One normative change:** §2.1
gains a clause permitting a reporting denominator other than `C_allocated` when it is explicitly
declared and demonstrably at least `C_allocated` — the prohibition it qualifies names only smaller
denominators (posted margin, leg-capital floor) because the risk it bounds is a flattered return, and
this gate reports on the per-leg notional, which is the larger number at these sizes and applies no
leverage uplift at all. The deviation and its clause landed in one commit; marker grep zero. The gate
then REFUSED three times on first contact with the live archive, each a different guard: §41e's
execution binding on a dirty tree, H2's manifest rule on the still-open capture hour (so a gate window
can only span complete days), and finally the §45f-1 end rule by name — exit 2, no artifact written,
because §45f-3's pre-declared portfolio means a thin symbol refuses the run rather than being dropped
from it. That run also measured the continuity carry on live data at ~99.7% joined coverage against
the ~9% ruling §45n-1 rejected. **The gate now waits on evidence, not code:** 180 matched post-cutoff
settlements per symbol, with BCHUSDT binding at half the others' rate on its 8h grid.

**Milestone 2b step 2 landed (v0.9.22, 2026-08-01) — the capture runs the shortlist.**
The §42c expansion executed the same day the shortlist was measured (capture-expansion design
record §44): 32 new sources — eight symbols × both venues' perp funding + perp books, every shape
its BTC counterpart's with only the symbol substituted (§44b: nothing per-symbol was chosen;
scripted generation with a uniqueness assertion; BTC's eight streams byte-identical) — and the
capture restarted 11:13Z as **PID 8495, 40 sources** (config `1c362feffe4a`). The pre-config item
cleared first (§44a; the profiles' dated ceilings block): Bybit's 500-connections-per-5-minutes
per domain (restart burst 19), Binance spot's 300-per-attempt-per-5-minutes (we open 1), and the
finding with teeth — **Binance futures states no numeric per-IP ceiling at all**, so the 19
connections to `fstream` rest on the behavioral reading (ban-on-churn) plus the per-source backoff
ladder being exactly the pacing it asks for. The restart is recorded, not hidden (§44c): ritual
verify first over the full accumulated range (**COVERAGE, 0 integrity, 639 coverage** — the known
honest thin windows), clean SIGTERM stop, **~8 seconds of recorded gap**, and every pre-existing
source minted its `process_restart` gap record at reopen — §32j's restart-required trade doing its
job. The alt streams' history simply begins at 11:13Z; §42a already named everything earlier as
un-backfillable by anyone. **Deliberate absences unchanged from §42c** (no spot legs, no per-alt
diff evidence, no third venue, no broad funding tier) — re-affirmed, not re-decided. Due next, with
dates: the **§32m-D per-tier one-day figure by 2026-08-03T11:13Z** (lands as §44e), which is also
**§38f's named revisit point** (ruling §42f-3 — the off-host decision gets re-argued on a measured
disk clock); then §42e's read-path + simulator + gate procedure commit, parameters pinned there.

**Milestone 2b step 1 executed (v0.9.21, 2026-08-01) — the screening ran, and the shortlist is measured.**
The §42b screening step turned into committed code and a committed dataset the same day (screening
design record §43). The harness — `internal/strategy/infrastructure/screening` + `arblab screen run` —
is committed rather than a scratch clone, and the difference is principled: §42b requires its
parameters pinned "at its own commit", which only a commit can do, and the screening mutates nothing
governed (§43a). Two of the calibration fetcher's guarantees were deliberately NOT reused because
they are wrong here, not merely unneeded: H1's era refusal is replaced by the narrower permanent seal
(**BTCUSDT never fetched**, tested as "no BTC funding URL is ever requested"), and H3's declared grid
is replaced by a grid **measured from the assembled rows** — vindicated when young listings showed
interval-move counts in the hundreds. The run (§43c): 465/465 symbols, zero failures; 1.79M + 1.89M
venue rows → **1,572,157 matched instants, zero pairing collisions** at nearest-minute rounding;
eligibility partitions the 466 exactly (61 ranked / 367 below the 5M floor / 35 mismatched-grid /
2 thin / 1 sealed). The ranking (§43d): best-entry annualized net over EPISODES after the 21bp
two-venue round trip — **shortlist LABUSDT, BEATUSDT, KAITOUSDT, MMTUSDT, MUSDT, SOONUSDT,
ESPORTSUSDT, BCHUSDT** (71.6%/yr down to 1.5%/yr; cut at #9 LTCUSDT 0.93%; seven of eight are 2025
listings on the 4h grid — the veterans cluster under 1%, so the dislocation spread is a young-listing
phenomenon; every win rate is below half, so persistence is what pays, measured in the continuation
column). The census (§43e): **2,972 events ≥ 25bp across 365 symbols, 29% multi-settlement, 2.4k in
2025–26**; extremes 3.01% single print / 23 settlements / 39.1% cumulative (HUSDT, June 2026).
**Carried forward on purpose:** the selection-contamination statement (§43b — the shortlist is
selected on pre-cutoff history; clean gate evidence accumulates post-cutoff), and §43g's boundary —
nothing in step 1 is gate evidence; the census shows settled prints existed, never that a book was
there to execute against. Verification (§43f): the fixture-collapse checklist ran BEFORE the
mutations and found three `<`-vs-`<=` boundary collapses plus an uncovered sign guard; nine aimed
mutations, nine caught; the end-to-end battery asserts on REQUESTS as well as outputs (the seal, and
resume-issues-no-duplicate-requests). Rejected alternatives live in §43a/§43b with the parameters
they shaped.

**Strategy #2 scoped, §12 re-sequenced (v0.9.20, 2026-08-01) — milestone 2b before the OMS.**
The §41r mandate turned into a roster edit the same day, on measured facts (scoping design
record §42). The organizing observation is the **backfillability asymmetry**: settled funding
history is free public REST on both venues, so candidate screening runs *now* with no capture
dependency — while books and live-estimate streams in our own format are the un-backfillable
half (§3.5 defines parity as replaying *our own* log), so §41r's "time-sensitive" pressure lands
on the screened shortlist's streams, not on capturing broadly today. Measured universe
(2026-08-01, both venues' public instrument endpoints): **466 both-venue USDT perps; 431
same-grid pairs (332 at 4h — twice BTC's settlement frequency, doubling both revenue events and
the per-settlement drag `R`), 35 mismatched-grid**; Binance's `fundingInfo` returned interval
overrides for all 466, so the existing hourly overlay already records interval provenance
venue-wide; same interval ≠ same phase, so screening pairs settlements by venue-published
instants. **Rulings (all four §42f forks, 2026-08-01):** shortlist **~8 symbols** for the
four-stream tier (~+5–9G/day derived; the one-day §32m-D measurement ritual re-runs within 48h);
**no broad funding-only tier** until the dislocation census — computable from backfillable data —
prices what breadth earns its disk; **§38f re-deferred, dated**, with the ~3×-compressed clock
recorded (~3 months of local disk at the expanded rate); **the roster edit signed off**.
**Rejected alternatives, with reasons:** per-alt diff evidence streams (890M/day each for a
pipeline claim BTC's stream already holds); alt spot books ("optionality" at ~1G/day/symbol when
§2.3 is perp-perp and the §38f clock is the scarce resource); a third venue (new transport
protocol + a full profiles verification campaign for a universe the two-venue gate does not
need — revisit only on a measured thin-space trigger); Binance's all-symbol `!markPrice@arr@1s`
(~6G/day derived for all 530 symbols — strictly worse than per-symbol streams below ~500);
proceeding to milestone 3 against a killed v1 edge (the roster's own preamble forbids it); a
wholesale roster rewrite (milestones 3–6 are strategy-agnostic controls and survive unchanged —
a minimal dated edit is auditable, a rewrite is drift wearing a redesign's clothes); running
strategy #2 without a milestone row (the gate needs a pre-registered home in the core, or its
kill authority is a promise in a side record); a second capture process for the new symbols (the
writer's single-owner model and close-time manifest truth were designed for one process — two
writers is §5.1's forbidden shape transplanted to the archive; the expansion is a config edit +
an honest `process_restart` gap record). The gate's shape inherits the §41 discipline whole —
procedure commit pins parameters, holdout declared before scoring, refusals are governance
questions, kill authority real — with the two §41r caveats built in: measured alt depth and the
persistence-measured edge bar.

**Milestone-2 post-kill fork: RULED (v0.9.19, 2026-08-01) — (c), the alt/dislocation niche.**
The decision the kill left open, taken the same day on the same record — and the record is what
made three of the four forks decidable without building anything. §41p's decomposition
(`clearing(h) ≈ A/h + R`; A ≈ 39bp round trip, R ≈ 3.2bp/settlement recurring drag) sorts §12's
four nominally-parallel forks into refuted / measured / mispriced / untested: **(a) maker-entry**
attacks only A, and a 40% cut leaves the short-hold corner at ≈10.8bp clearing against the era's
8.8bp all-time maximum print while moving long-hold clearing ≈0.2bp — refuted as a fork, retained
as a **standing tactic** for whatever eventually trades; **(b) regime-selective** is the corner
the deployment sweep already bounded, because the conservative estimator *is* regime-selectivity
priced and it measured 11.5% utilization with 4 entries in three years (the unbounded reading —
*predictive* regime detection — is a different research program, not this fork); **(d) tuition**
was priced at 60% utilization, so at the measured 11.5% the sub-hurdle spread buys standby
management rather than live education, and its real value — the platform build — is captured by
(c)'s path anyway; **(c) alone changes the funding distribution itself** rather than shaving
costs around BTC's (median 0.74bp, max 8.8bp — costs irreducibly ≈2–3bp/settlement at best).
**Two caveats are normative for the scoping:** §41p's "the cost is fees, not depth" is a BTC
measurement that does not transfer (alt books are thinner; depth re-enters the cost model), and
§2.3's edge bar — measured spread persistence, not the headline spread — stands; strategy #2
gets its own pre-registered economic gate under the same kill discipline that produced this
verdict. The rejected alternatives are recorded with their numbers because a rejection nobody
can re-derive is an unchecked claim: each declined fork's outcome is computable from §41p's
table, which is what a kill that produces a *decomposition* rather than a verdict buys. Next:
a new design record scoping strategy #2 (roster re-sequencing, the alt/cross-venue capture
surface — time-sensitive, self-capture cannot be backfilled — candidate-universe selection).

**Milestone-2 economic gate: FIRED (v0.9.18, 2026-08-01).** The decision §12's milestone-2 row
exists to force, taken on the day its machinery delivered the evidence. The governed derivation
(calibration design record §41, slices A–C) refused on the real era — `ErrNoExcursions` from two
byte-identical stamped clean-checkout invocations: at the pre-registered v1 procedure the entry
clearing level (0.0016/settlement) sits at **1.8× the era's all-time maximum funding print**
(3,381 settlements, 2023-01→2026-01), so the strategy never enters and the exit band has nothing
to measure. The read-only sensitivity diagnostic (§41p + addendum) then **bounded the admissible
space**: the clearing level is capital-scale-invariant to the ninth digit (every frame cost is
proportional to notional), deployment moves only the hurdle-on-`C_allocated` term, and the best
admissible corner — 98% deployment, 30-day minimum hold — reaches **11.5% utilization with 4
entries in three years**, which §8's own LCB scoring could only score as a kill. **The user's
dated ruling (§41q): kill by governance rather than a v2 ceremony that measures the same verdict**
— milestone 2 closes with NO calibration artifact, no attestation, and step 5 unbuilt, exactly
the "explicit pass/fail deliverable, not a vibe-check" §12 promised. Two consequences landed with
the close: §8's pre-registered latency clause (declared bound authorized for the $1K gate,
measured latency before capital — the marker's own text, so **ADR §3 gains no edit row for it**;
reasoning in the design record) and §12's dated outcome sentence. **The §12 fork choice —
maker-entry / regime-selective / alt-niche / tuition — is deliberately NOT taken here**; it is
the open user decision, and §41q maps each fork onto the measured cost decomposition (≈39bp
round trip + ≈3.2bp/settlement recurring drag at the committed frame) so the discussion starts
from numbers rather than hope. The H1 holdout era (post-2026-02-01) remains unpolluted for any
fork that revives a calibration.

**Milestone-2 capture slice E (v0.9.14, 2026-07-30).** One edit, and it is the v0.9.13 rows-1/4 lesson repeating one session later: slice E's config test began asserting an **eighth** capture stream in `internal/` — the hourly `fundingInfo` REST overlay — while the core's two scope sentences stopped at seven. Caught by asking "does the core know this?" at the close, not by the marker grep, which by construction cannot see an unmarked claim.

| Decision → resolution | Where |
|---|---|
| **The capture set records the hourly `fundingInfo` REST overlay.** §4 makes the adapter's 8h default conditional on "the overlay was fetched and came back empty" being a **held fact**, and "we asked and it was empty" is indistinguishable from "we never asked" unless the ask itself is recorded — so the overlay is captured with the same file discipline as the seven WS streams, empty responses, non-2xx answers and failed asks included (a failed ask is a Rest record with the transport's words, not a Gap — nothing is *missing* after a failed poll; the ask is fully recorded). Stated in §8 and §12.2 together, the pair v0.9.13 established. Rejected: leaving the overlay as design-record-only scope — the config test asserts it normatively in `internal/`, which is precisely the state that expired silently last release | core §8, §12.2 |

**Milestone-2 capture release (v0.9.13, 2026-07-30).** What building the capture tool (design record §32, slices A–C: codec, writer, transport, composition root, `arblab capture run`/`capture verify`) accumulated across four sessions, landed at the close of slice C — the slice whose "done when" is capture *running*. **Six edits and one code fix.** Five sat on the design's own §12 roster from the day the design was written — the roster is what let three sessions carry them as "expect exactly four markers, unchanged" without re-deriving what was owed — and the sixth (the module rename) was found by building. One roster nuance is the process finding worth the block: rows 1 and 4 were unmarked because "no code asserts them yet", **and that state expired silently** — the schema's positive-control test now asserts §32c's seven-source capture set in `internal/`, which is §32a's widening of §12.2, unmarked because at writing time it read as loading a config rather than asserting a scope. The close's reading pass is what re-derived it; the grep, by construction, could not.

| Decision → resolution | Where |
|---|---|
| **The capture lane's counters are per-`(Source, Topic)`, not per-`Source`.** The lane stores wire bytes and one `SourceID` has more than one wire representation — §32c's partial-depth stream and diff evidence stream are the same `SourceID`, and the realistic accident is an operator editing one topic's cadence in config six weeks in. A per-`Source` counter spanning two wire feeds has **no hole when one of them stops**, which inverts the lane's one guarantee. The finer key preserves every hole the coarser spelling would have shown, so §3.5's drop discipline is unchanged; departure is strictly finer | core §3.5 |
| **`Wall`'s monotonic clamp is stated, and it has an observation seam.** §3.5's Clock block never said the live clock clamps a backwards NTP step, and §3.6 rule 5 mentioned the clamp only as an argument for policing `engine/`. The clamp is right for the engine (a timer firing late beats a settlement evaluated twice) and **wrong for anything writing an archive**: a clamped run of identical instants squashes the replay time axis, §3.4's latest-value cache accepts an equal `RecvTime` by design, and a capture archive is unrecoverable, so the corruption is permanent and silent. `Backsteps()` (count, total absorbed) is the seam, and an archival consumer MUST record an engaged clamp as data. Rejected: a second caller-held monotonic reference — a tool whose job is to detect a clock defect must not hold two clocks and privately arbitrate them | core §3.5 |
| **Rule 8 gains its second documented integer-enum exception: `capture.RecordKind`.** It crosses a persistence boundary — a byte in every archive frame — so rule 8's letter says `type X string`. Two grounds against: §32e's frame is a **fixed-width header** in the one part of the format that must stay decodable from a corrupt file, and rule 8's self-description rationale is already delivered one level down ("the archive is the fact; readability is a tool over it" — `capture cat` is where a kind becomes a name). What the integer keeps is the rule's other half: a refused zero value, so a zero-filled region — sparse hole, page allocated before a crash — cannot decode as a valid record | core §3.6 rule 8 |
| **§11's backend roster follows the 2024 module rename: `nhooyr.io/websocket` → `github.com/coder/websocket`.** Recorded rather than absorbed because "we chose differently from the spec" and "the spec names something that no longer exists" need different edits, and only the second is true. Also the engine module's first runtime third-party dependency (zero transitive), which §11 permits for exactly this | core §11 |
| **Self-capture is funding AND books (+ one evidence stream), not "both funding streams".** §32a's argument: §3.5 defines parity as replaying *our own* log, §8's fill model walks book depth, and Profiles closes the spot door (no mark, no funding — the spot leg is priced from its own book). Capturing only funding proves the parity claim on the easy half — a scalar per second — and asserts it on the reconstruction path it actually exercises, with the discovery landing at milestone 5 when the missing months are unrecoverable. Stated in both places that carried the old scope, §12.2 and §8, because one statement updated and one left behind is the v0.9.9 two-statements failure built by hand | core §12.2, §8 |
| **Code fix, no spec edit: a citation naming the wrong rule.** `app/capture`'s config schema justified its integer percent field with "§3.6 rule 1 forbids float64" — rule 1 is the hexagonal boundary; the float ban is rule 4. Found by the closing reading pass checking each core citation against the section it names; harmless today, but a wrong pointer survives every gate this repo has, because no analyzer reads prose | Process — no core change |

**Milestone-1 seventh slice (v0.9.12, 2026-07-27).** What building `engine/state` — §3.2's in-memory latest-value cache and §3.4's cross-leg skew — produced, and the close that leaves **§12 milestone 1's roster with no unbuilt item**. **Four edits and one code fix.** Two arrived carrying `[SPEC EDIT OWED]` markers; **the other two were found by the closing reading pass**, which is v0.9.8's lesson recurring for the third release running — a claim the code merely *relies on* is never marked, because at writing time it reads as implementing an existing requirement rather than departing from one. Note on ordering: this block set runs **ascending through v0.9.8 and descending after it**, so the newest block is the one below the v0.9.8 table rather than the one at the bottom.

| Decision → resolution | Where |
|---|---|
| **A cached entry carries NO freshness verdict; the re-judgement happens at the READ.** §3.4 requires a silence timer because "a feed that simply stops emits no event", and names that failure **only for the feed**. A latest-value cache is where it reappears: `marketdata` computes the transport and data verdicts at *accept* time, an observation then sits in the cache and **ages while nothing arrives to re-judge it**, and an entry that stores what it was handed reports the dead feed `FreshnessFresh` for the entire outage — **every light green**, which is the same invisible failure the timer exists to catch, one hop past where the timer reaches. The rule is **structural rather than procedural** because the two are not equally safe when somebody forgets: the entry has no freshness field *to* read, so the stale verdict is unrepresentable rather than merely unused, and every age is derived from the stored watermark against an instant the **caller** supplies. Rejected: storing the verdicts beside re-derived ones (keeps a field whose only remaining use is to be read by mistake); and re-deriving them **inside** the cache (a second freshness authority holding a second copy of §5.2's budgets — the two-mechanisms cost this project has already paid once). Bite-checked by a **reflection test over the struct fields** rather than by a comment, because the field somebody adds next year will look like a helpful convenience | core §3.4 |
| **Cross-leg skew is measured over `RecvTime`, and the core named both timestamps without choosing.** §3.5 carries `SourceTime` and `RecvTime` "for freshness, cross-leg-skew metrics, and offline analysis" — both, for that list of three — and §3.4 requires a skew limit beside sub-second book budgets. **This is a silence of a kind no earlier block records:** not an omission (v0.9.6), not a statement made twice incompatibly (v0.9.9), but a question the core *raises* and then declines. The answers differ in the direction that matters: a cash-and-carry package is spot on one venue and the perp on another, so a `SourceTime` skew across its legs **is** the unsynchronized-clock subtraction §3.5 already forbids — and it fails quietly, since the number is the venues' clock *offset* rather than any property of the market, indistinguishable from real skew against a sub-second limit, so the gate either blocks a healthy pair forever or is widened until it admits a real one. Rejected: a `ServerNow`-corrected `SourceTime` skew — `ServerTime` carries an uncertainty **band**, and a measurement whose error bar is the size of the threshold it feeds is not a measurement. `RecvTime` is stamped by this process from the one clock both legs share, which yields an identity worth keeping — **skew = \|`Age(a)` − `Age(b)`\| at any instant**, since the instant cancels — so skew and freshness cannot contradict each other, the function takes **no clock at all**, and the identity is the implementation's own test oracle. What made the question visible is a **forward reference**: `marketdata/domain.Watermark` refuses the skew field and says "it is computed where the legs meet", and arriving at the place that comment points to is what surfaced the unanswered half | core §3.4, core §3.5 |
| **A latest-value store REFUSES a strictly-earlier `RecvTime`, ACCEPTS an equal one, and compares `RecvTime` rather than `VenueSeq`** — three halves, each failing in its own direction, and none stated. "Latest wins" is ambiguous exactly where it costs: read as *last to arrive wins*, an out-of-order write **is** the last to arrive, and storing it makes the cache report data **staler than it was a moment ago on a feed that just delivered**, having overwritten the newer reading that would have explained it. The mailboxes are FIFO per `SourceID` so this should not arise, and "should not" is load-bearing — that invariant lives inside §3.5's dispatcher, which is explicitly *not yet built*, and a consumer relying on an unbuilt component's guarantee without checking fails silently the day it ships with a different one. The equality half fails **in the backtest only**: under a `TestClock` every observation dispatched between advances shares a `RecvTime` exactly, so a strict *not-after* check drops all but the first update at each instant, a parity divergence no test of the live path can see. And the sequence half inverts: `VenueSeq` is authoritative only *within* a `SourceID` and a full snapshot **replaces** rather than follows, so a resnapshot recovering a desynced book legitimately carries a lower sequence — a sequence check would reject **the one write this cache most needs to accept**. Found by the closing reading pass | core §3.4 |
| **§5.2's configuration schema gains §3.4's freshness family — the per-(data class, venue) budgets and the cross-leg skew limit.** The schema carried the mark's max-age, inside the stale-mark escalation policy, and neither the book nor funding budgets nor the skew limit §3.4 requires in the same breath. **One member of a per-venue set with a config field while the others have none is how the others become constants** — and §3.4's stated reason for per-class budgets is the data's tolerance for staleness, which is a per-venue judgement by construction. Each is validated the way §7.4's ladder is: a value making the permitted region unreachable is **refused at startup, by name**. The skew limit is the one somebody sets to zero by accident, and zero admits only two legs delivered in the very same instant — under a wall clock every package is refused and **the engine simply never trades with nothing reporting an error**, because each individual refusal is the gate working; under a `TestClock` it is reachable, so the backtest would pass a configuration that halts the live engine. Found by the closing reading pass, from the direction the code asserts: the slice introduced a typed, `Validate()`-ing skew limit that the schema had no home for | core §5.2 |
| **Code fix, no spec edit: a sentinel that nothing could return, documented as an assertion that did not exist.** `state.ErrInstrumentMismatch` was declared with the comment "it cannot happen through the exported API, which derives the key from the payload; **it is the assertion that keeps that true**" — and no assertion existed, nor could one be written: both write paths derive the key from the payload one line before storing it, and the watermark carries no instrument of its own to disagree with. So `errors.Is(err, ErrInstrumentMismatch)` compiled, always answered false, and **read as a handled case**. This is the fixture-collapse tell this project has now caught nine times in tests — a branch pinned to a value that makes the distinction it draws unreachable — relocated into the **error taxonomy**, where no coverage tool and no mutation harness objects, since an unused package-level `var` is legal Go and killing it changes no test. The contrast is what names it: `oms.ErrFillInstrumentMismatch` and `marketdata.ErrBookInstrumentMismatch` are each returned by real code and pinned by a test, because there the two identities genuinely arrive from different places. Deleted rather than given an assertion, since an assertion whose condition is unreachable is permanently uncovered by construction | Process — no core change |

**Milestone-1 sixth slice (v0.9.11, 2026-07-27).** What building `oms` — §5.3's lifecycle and fill ledger, §5.1's order intent, the driven ports, §6 step 6's dispatch use case, the write repository and `module.go` — produced. **Seven edits and one code fix**, and the block mixes the kinds v0.9.10 first mixed, with a new one at the end: a rule the code silently did not meet, in a slice that was already closed.

| Decision → resolution | Where |
|---|---|
| **SM §1's `DispatchAuthorized` gains the five query-resolution edges.** The table's "no-return / crash" row named the resolution ("resolved by query only") in its ACTION column while the target column pointed back at the same state, so read as the table alone a crash-recovered order had **no way out**. The requirement was stated twice elsewhere — core §5.3's "resolved only by `OrderByClientID`/`Fills`" and this section's own crash-recovery invariant — and neither statement reached the table. Cost when implemented literally: not a wrong order but a **stall with every light green**, since the state contributes a §6.7 possible-delta interval and the hedge group holding it is `Ambiguous` forever. | SM §1 transitions |
| **SM §1 gains `DispatchAuthorized` → `SubmissionUnknown` on an ambiguous transport result**, as a row DISTINCT from "no-return / crash". Core §6 step 6 states it in words and the order it moves is this one, since the fsync happens "immediately before the single `Place`". Neither existing path could carry it without recording something untrue: `Submitted` journals that the call "returned a transport *result*" — which §7.5 replays as evidence the request reached the venue — while "stays" claims crash recovery for a call that returned an error. It is FLAG-1's `CancelPending → CancelUnknown` symmetry one step earlier, and the asymmetry was its **absence**. | SM §1 transitions |
| **SM §1 gains a venue-initiated `Acknowledged`/`PartiallyFilled` → `Canceled` row** — IOC expiry, GTD expiry, venue cancel-all, self-trade prevention — with no cancel request of ours, so `CancelPending` is never entered. Not an edge case: core §6 step 6 mandates marketable-IOC limits and §2's finality contract makes an IOC immediately terminal at the venue, while the table's only paths into `Canceled` left `CancelPending`. So **the only order type this system sends could never terminate on its ordinary outcome**, and the one-leg-failure handler never issues its `ExposureReduction` — a naked leg stays naked while the classifier waits for fills on an order the venue closed milliseconds after the send. | SM §1 transitions |
| **Core §5.1's `Fill` field list drops `FeeAsset`.** The list predates the money kernel; a `fixedpoint.Money` carries its asset tag as a runtime-checked field per §3.6 rule 4, so a separate column is a second copy of a fact the first field states — the shape this repo already paid for once. The failure is specific rather than cosmetic: fee attribution reads one field and the tax export the other, and a Binance fill paid in BNB against a USDT notional reconciles to two different numbers with nothing failing. | Core §5.1 |
| **Core §3.6 rule 3 is a PERMITTED-children rule, not a required-children one.** "Contains exactly `driven/` + `driving/`" admits two readings, and requiring both halves of a context that only has one forces an empty package — the same mistake as a lint over an empty tree, a shape that looks compliant while asserting nothing. The rule's actual purpose is to stop `infrastructure/` growing a third category where the adapter *direction* stops being visible in the path. | Core §3.6 rule 3 |
| **Core §3.6: a rule identified BY POSITION reads a Go external test package as the package it tests.** An external test package is the one Go construct where a package's import path and its directory **disagree** — `internal/oms_test` is outside `internal/oms/` by path and inside by directory — so rules phrased as "packages outside X" did not decide the case. Two gates had already answered it independently and **oppositely, both correctly** (`hexboundary` polices, `singlewriter` exempts), with no rule to consult. Measured cost, not hypothetical: `singlewriter` shipped without the normalisation and reported `oms`'s own `module_test.go` as a second writer to order state, because a test double for a port must name that port's types. The shape that hid it is worth keeping — an external test package of a *subpackage* sits below the exempt path and is covered by a prefix comparison for free, so only the **owner** package's own external test exercises the rule. | Core §3.6 |
| **Core §5.1's OCC `Version` gains both of its rules: it starts at 1, and only a write that CHANGES the record increments it.** The section said "a `Version int64` (OCC)" and stated neither, and the natural implementation gets each wrong in a **different** direction. Base-1 is the zero-value-is-unusable discipline applied to a version: an `expected` of 0 is what a caller that never loaded a record passes, so it must match nothing — a 0-based counter makes that blind write succeed against a real record and race whatever is there, while satisfying the sentence completely. No-op-does-not-bump is the duplicate-fill case: §4 puts the same execution on two streams and §5.3 has REST recovery re-reading fills the private stream delivered, so a duplicate is the **ordinary** case several times per fill, and a version bumped to record that nothing happened invalidates every other reader's expectation for nothing. **Found by the slice-closing reading pass, not by the marker grep** — the v0.9.8 shape, since a claim the code merely *relies on* is never marked. | Core §5.1 |
| **Code fix, no spec edit: §3.6 rule 2 was silently unmet in the CLOSED `marketdata` slice, on both halves, for five sessions** — ports flat in `application/ports.go` rather than `application/ports/`, and no `var _ ports.X = (*Impl)(nil)` anywhere in its `infrastructure/driven/`. The spec was right and the code quietly was not, so **no edit is owed and none is taken**. What it adds is a third limit on the slice-close gate, beside v0.9.8's (the grep cannot see an unmarked claim) and v0.9.10's (nor a rule the code silently breaks): **rule 2 has no analyzer, and the closing reading pass did not object either, because a rule the code silently does not meet looks exactly like a rule it meets.** What surfaced it was `oms` implementing the same sentence, at which point one repo held two contexts disagreeing — so **a second implementation sees what neither the grep nor the reading pass can**. Recorded as decision-record §24f, carried three sessions, fixed rather than absorbed. | Process — no core change |

**Milestone-1 fifth slice (v0.9.10, 2026-07-27).** What completing `marketdata`'s **order-book path** — the `engine/venue` book DTOs, the driving dispatch, the lane publisher and the wiring — produced. **This is the first block that mixes all three kinds**, and the mixture is itself the item worth recording: two **silences** in the core (§3.4 defined an overflow policy for the post-reconstruction lane and none for the hold buffer, whose correct policy is the *opposite* one; §4 said a payload carries no provenance without saying what happens when the envelope structurally cannot carry a field — a delta spans a *range* of sequences and an envelope holds one), one **violation by the code** of a rule the core already stated correctly (`platform/events` declared six `StreamKind`s and not `StreamOrders`, which §4's `OrderGateway` and §3.5's product table both name — the second such instance after v0.9.8's `AcceptOrdinal`, and the second demonstration that the marker grep cannot find this class by construction), and one **verification owed to Profiles rather than to the core**: the span design cites an asymmetry between the two venues' depth-update IDs, and Profiles records the book topics without recording either feed's update-ID shape, so the citation is presently unverified. The previous two blocks were each homogeneous — all silences, or lead-violation — and the practical cost of assuming that continues is that the odd item gets filed under the wrong process: a Profiles verification is not a spec edit, and the slice-close grep answers only for the latter. Full reasoning with rejected alternatives: `docs/superpowers/specs/2026-07-26-milestone-1-foundation-decisions.md` (§22).

**Milestone-1 fourth slice (v0.9.9, 2026-07-27).** What building `engine/venue` — the normalized funding DTO, which retired the `driving.FundingUpdate` placeholder `marketdata` had been holding — produced. **All three are silences rather than violations**, the v0.9.6 shape rather than the block above. But two of them are a **sharper kind of silence, and it is worth naming**: the core does not merely omit these, it **states them twice**. The market-data seam is declared in §3.5 *and* in §4 with shapes that no single type satisfies; the per-stream product constraint is stated in §4 and dropped where §3.5 defines the identity carrying both of its fields. A gap is visible to a reader of one section. **A contradiction is visible only to a reader holding two sections at once** — which is what a slice's reading pass does and what no grep performs, so this block and the one above name the same limit from opposite sides: the previous slice found a rule the code silently broke, this one found rules the *spec* silently disagreed with itself about. Full reasoning with rejected alternatives: `docs/superpowers/specs/2026-07-26-milestone-1-foundation-decisions.md` (§20).

| Decision → resolution | Where |
|---|---|
| **The market-data seam is `MarketDataSource`, and §4's per-stream shape is authoritative.** §3.5 gave `DataSource{ Subscribe(VenueInstrument) <-chan Envelope }`; §4 gave `StreamBook`/`StreamFunding`/`StreamMark`. Same position in the pipeline — both are what the dispatcher reads and what §8 substitutes a replay adapter for — and a type satisfying one does not satisfy the other, so an implementer picks by which section they happened to read. §4 wins on three grounds: a `SourceID` is `(instrument, stream)` and §3.4 gives each its own mailbox, so one channel per instrument merges three and forces the subscriber to **re-derive a key the seam could have kept**; the per-stream product constraints are **inexpressible** in `Subscribe(VenueInstrument)`, so a spot subscription silently yields a third of the streams with nothing naming which third; and §7.6 makes venue capacity a priority-laddered budget, so a funding-only consumer forced to open the high-rate book feed spends that budget on nothing. The tell that §4 was the current one is structural rather than a preference: **every other line of §3.5's seam block is written as a reference to §4** — `ExecutionVenue` is a composition of §4's four interfaces and says "see §4" inline — while `DataSource` was the one written as an independent definition, which is the line that falls behind. Profiles §3 was **already** writing the name as `MarketDataSource.StreamMark`, so the rename aligns §3.5 to the spelling the rest of the set had. It is deliberately **not** resolved by declaring the interface in code: no client implements it and no dispatcher calls it, and an interface with no implementer and no caller is a declaration nothing can disagree with | core §3.5, §4 |
| **A `SourceID` MUST refuse an `(instrument, stream)` pair no venue can produce, and the full per-stream table is stated — including the rows that are NOT constrained.** §4 marks funding and mark perp-only per stream; §3.5 defines `SourceID`, gives it a `Validate`, and never carries the constraint across — so a spot instrument paired with the funding stream validated cleanly, as did every envelope built from it. The rule binds at the identity for the reason `ids.VenueInstrument` already checks its product against its contract type: **this is the type that holds both fields**, and a rule about two fields agreeing has one place it can be enforced rather than remembered. Placement decides coverage — on `SourceID` every producer gets it, including those not yet written, while on a helper only callers who remember the helper do. The hole was **covered only by accident**: `marketdata`'s own domain refuses a spot funding observation, so the one built path was safe while every other consumer of the same source was not, and the coverage sat three layers downstream of the transport that had already accepted it. §3.4 makes `SourceID` the mailbox key, so an impossible source is a **mailbox that exists, can be subscribed to, and never delivers** — indistinguishable downstream from a venue gone quiet, which §3.4's silence timer would then faithfully report. The table states the negative rows explicitly (the private streams and control are **not** product-constrained, since §4.2 makes spot a full API family with its own user-data stream) because the constraint *reads* like a market-data property and is not one — inferring it would silently unsubscribe half of a cash-and-carry trade. An unclassified stream is treated as perp-only: **deny the source nobody classified rather than admit it**, matching §5.2's and §10's default direction | core §3.5 |
| **The units discipline covers TIME, not only money.** §4 requires numeric fields to cross as dimensional `fixedpoint`, never `float64` — a rule about money, and a good one — while saying nothing about the venue fields that are **durations**. `fixedpoint` puts a money field's dimension in its *type*; a duration has no such tag, and the unit of a venue's duration field lives only in its **name** (Bybit's `fundingIntervalHour`). So a conforming reimplementation can multiply it by `time.Minute`, satisfy every sentence §4 had, and hand §2.4 a 480-minute interval: the settlement enumeration divides by it, returns a plausible count, and **§12's kill-gate carry figure is wrong by a factor of sixty with nothing failing** — on the milestone whose entire output is a threshold comparison. This is **§10's own v0.9.7 argument** — a duration recorded in the wrong unit renders as a plausible dashboard number and fails nothing — applied one layer earlier, at the boundary where the duration is *created*. The rule is a conversion that states the unit plus refusal of an implausible value; the bound is a **unit check, not venue policy** (every perp on either MVP venue settles at 1/2/4/8h, Profiles §1, so a generous ceiling still rejects minutes, seconds, or a truncated `time.Duration`). A **zero is refused, never defaulted**: it arrives from three situations a fallback answers identically — the overlay genuinely reported nothing, the overlay was never fetched or a decode dropped the field, and a Bybit **delta** whose unchanged field is simply absent (Profiles §1: "absent field = unchanged, never null-coerce") — two of which are a wiring failure wearing a correct BTC reading's clothes, and the third of which **absorbs the C8 flip** the whole interval-is-live rule exists to catch. The venue's documented default is the *adapter's*, applied from the positive fact that an overlay was fetched and came back empty `[→ C8]` | core §4 |

**Not tagged, and that is deliberate.** No milestone-1 edit in any block above carries a `[→ …]` tag: **ten** (v0.9.5) + **eight** (v0.9.6) + **four** (v0.9.7) + **eight** (v0.9.8) + **three** (v0.9.9) + **three** (v0.9.10) + **seven** (v0.9.11) + **four** (v0.9.12) + **six** (v0.9.13, the first milestone-2 addend — same standing reason, and five of the six were pre-registered on the capture design's own §12 roster) + **one** (v0.9.14, the overlay-scope edit — the same one-edit-two-places shape as v0.9.13's row 1/4 pair) — with one exception that proves the rule rather than breaking it: the time-units row cites the **existing** `C8` tag, because it resolves a finding this project already tracks rather than introducing a new family. **This tally was itself stale, and how is the point of stating it per-release.** It read "thirty-three … the five blocks above" through v0.9.10, which was correct at v0.9.9 and silently wrong the moment the fifth-slice block landed — a block whose three core edits nobody added. A total with no per-release breakdown cannot be checked against the document it summarises, so it goes stale in the one direction that reads as complete. The breakdown above is the fix: each addend names the release that owes it, so the next omission is a subtraction somebody can see. A `M1-#` family was considered and rejected at v0.9.6: it would have covered eight of what were then eighteen milestone-1 edits, and a greppable scheme that silently omits the rest reads as complete when it is not — the failure mode this project has already paid for elsewhere. The count is now **fifty-four** — forty-seven milestone-1 plus v0.9.13's six plus v0.9.14's one — which only sharpens the argument. Traceability for all fifty-four lives in the ten blocks above, each row or paragraph naming its section — and "above" is doing less work than it looks, since the block set runs ascending through v0.9.8 and descending after it, so the newest sits in the middle.

## 4. v0.8 doc-split map (what moved where)

The trimmed core keeps its §-numbers; this maps each v0.7.1 block to its v0.8 home (design `docs/superpowers/specs/2026-07-19-doc-split-design.md`, plan `docs/superpowers/plans/2026-07-19-doc-split-execution.md`).

| v0.7.1 location (frozen blob, `6d211cd`) | v0.8 home |
|---|---|
| Convergence narrative (Status ¶ + Codex/review-round summary) | ADR §1 |
| Appendix — finding→resolution traceability table | ADR §3 (this table) |
| §4 quirk matrix (perp/futures venue facts) | Profiles §1 |
| §4.1 account & margin posture tables | Profiles §2 |
| §4.2 spot-leg surface table | Profiles §3 |
| Review-record §2 dated verification citations | copied to Profiles §4 |
| §5.3 order-state list + transition table | SM §1 |
| §5.3 `reduceOnly`-by-instrument-position naked-leg cases | SM §2 |
| §5.3 `PackageDelever` generation mechanics | SM §7 |
| §6.7 (core §6 step 7) ambiguous-order classifier mechanics | SM §2 |
| §7.4 actuator takeover protocol (fence latch → quiesce → reconcile → reduce) | SM §4 |
| §7.4 `Fence`/`FenceReadyLost` differentiated-state branches | SM §3 |
| §7.4 protective-order disarm (`Armed → DisarmPending → Disarmed`) | SM §5 |
| §7.4 `EmergencyPlan` lifecycle (replicate/generations/deactivate) | SM §6 |

The requirement text, its `[→ …]` tag, and a `[mechanism: …]` pointer stay in the core at every row above — only the state list / transition table / dated fact table moved.

**Flagged ambiguities surfaced by the split — all seven resolved in v0.9 by the round-4 (R1d) reviewer rulings** `[→ R1d-F1…F7]`. The v0.8 flag texts are preserved (struck through in spirit) with each ruling:

- **FLAG-1** — SM §1 — *was:* the entry edge into `CancelUnknown` (and any cancel-lost-race exit) never stated. **Resolved (R1d-F1):** ambiguous cancel transport result → `CancelUnknown` (symmetry with `SubmissionUnknown`); explicit terminal cancel + final executions sweep → `Canceled`; cancel-lost-to-fill → `Filled`.
- **FLAG-2** — SM §1 — *was:* the terminal state of a stably-not-found order never named. **Resolved (R1d-F2):** new terminal **`AbsentAfterFinality`**; `Rejected` is not overloaded (explicit venue rejection only); a discovered execution supersedes it.
- **FLAG-3** — SM §7 — *was:* a distinct state between perp-reduce finality and spot submission under-determined. **Resolved (R1d-F3):** durable **`SpotReduceAuthorized`** journal record (crash-boundary discipline at generation scope).
- **FLAG-4** — SM §7 — *was:* exit from `Held/Alert` on later resolution never stated. **Resolved (R1d-F4):** nonterminal — eventual finality resumes the remaining target, or `TerminalFlatten` under the liquidation policy.
- **FLAG-5** — SM §3 — *was:* no stated branch for a backup-fence-readiness-only loss. **Resolved (R1d-F5):** **`FenceDegraded`** (alert/repair, openings continue on the intact primary — not a wind-down); dimension space re-cut in the same round (`ActuatorReady` added, durably-fenced reclassified, R1d-4).
- **FLAG-6** — SM §3 — *was:* exits from the branches never stated. **Resolved (R1d-F6):** one-way per incident — restored health never aborts a started branch; escalate-only re-arbitration; recovery = terminal outcome + operator clearance + fresh propagation test.
- **FLAG-7** — SM §6 — *was:* deactivation for a trade flattened *by the actuator* never stated. **Resolved (R1d-F7):** actuator-side tombstone gated on two-leg close + zero reconciled per-leg allocations + final surfaces, with a durable `ActuatedFlat`/`DeactivationPending` record until the tombstone is durable.
