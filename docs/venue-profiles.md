# Venue Capability Profiles — Binance USDⓈ-M / Bybit V5

**Status:** v0.9.51 (2026-08-06) — **§2 gains a ⚠ block marking TWO gaps in the Binance posture row rather than filling them, and §9's `positionRisk` block gains the note that the VERSION choice is PER QUESTION** (milestone 4 slice A, task #198, record §73). **No new venue fact is entered — that is the point of both edits.** First: *Portfolio-Margin OFF* is a required check with **no read path anywhere in this doc set**, in a cell where every other item names a `GET`, so core §4.1's gate is **attestation-backed until one is verified** and nothing is recorded here on the strength of an operator attestation or an assistant recollection — the channel that has already misreported a load-bearing number twice in this project. Verification is blocked by the same AWS WAF challenge as **#180**. Second, and found by the slice's closing reading pass: §4's Binance block cites `isAutoAddMargin` from the **V2** page with an unverified *"(or v3)"*, while **§9's dated verbatim V3 transcription lists twenty row fields and neither `isAutoAddMargin` nor `autoAddMargin` is among them** — and the engine reads it from V3. Either §9's list is an abbreviation or V3 dropped the field with the rows it also dropped; both readings leave core §4.1's gate **refusing** rather than passing, which is why no fixture could see it, and one page read settles which. §9 now also records **why the two versions cannot be chosen once for all callers**: §7.5 picked V3 on an argument explicitly about what a missing row means for the **net-delta invariant** (*flat* must be a conclusion), while §4.1 asks *what is this configured symbol's setting*, where V3's silence is precisely the ambiguity and V2's row-for-every-symbol enumeration answers directly — so task **#199** gains a fourth candidate. Third, restated as a marked unknown rather than assumed either way: **whether Bybit `GET /v5/position/list` returns a row for a configured-but-never-traded symbol is established nowhere**, and all three of Bybit's per-symbol posture settings are answered from it — **#171**'s credential settles it in one call, a loosening trigger and never a correctness prerequisite. Previous: v0.9.50 (2026-08-05) — **§9's ⚠ `positionRisk` block records a THIRD unread candidate weight and, more usefully, what the disagreement establishes: two AI summarizers, run on different renderings, both report a CONDITIONAL weight (cheap with `symbol`, expensive without) and disagree by a factor of two on the multipliers — "5 with `symbol`, 20 without" against "1 for a single symbol; 10 when the symbol parameter is omitted"** — so the **SHAPE is corroborated by independent agreement and the VALUE by nothing, and the shape is the load-bearing half** because §7.5's enumeration always takes the expensive arm. The engine's charged **20 is the MAXIMUM over all three candidates**, hence correct-or-pessimistic under each, and **no further candidate below 20 can change the code** — the property that makes #180 safe to leave open. The second summary is recorded as not closing the task, with its three tells: every answer taken from ONE aggregated catalog page, V2 and V3 consequently reported as carrying the BYTE-IDENTICAL weight line (which *weakens* rather than corroborates the earlier "V2 now says 10"), and changelog citations that are `google.com/search?q=…` redirects rather than the changelog (v0.9.50 is **milestone 3 task #197**, the §12 closure pass; record §72). Previous: v0.9.49 (2026-08-05) — set-version alignment; this doc's #179 facts landed mid-slice as §4's dated 2026-08-05 **spot balance surfaces** block and are verified, not new with the version: Binance `GET /api/v3/account` at weight **20** (settled by exact grep of the raw docs after a summarized fetch misread it as 10 — the correction is design-review round 3's finding 1) with `free`/`locked` documented **partitions of holdings** so `total = free + locked` is the venue's word; Bybit `GET /v5/account/wallet-balance` as the coin-less UNIFIED enumeration whose absent coin means zero, with the **walletBalance/locked NAMED INFERENCE** (no sentence states inclusion; pinned only at `locked == 0`, **#171 the loosening trigger**, never a correctness prerequisite) and the **50/s per-UID `wallet_balance` row** beside §4's verified table (v0.9.49 is **milestone 3 task #179** — the spot leg's recorded exposure baseline; record §70). Previous: v0.9.48 (2026-08-05) — **§5's lot/tick block gains the dated 2026-08-05 re-probe note (task #183, the fetcher's own read of all four cells): every value unchanged, Bybit's `lotSizeFilter`/`priceFilter` parent keys pinned live on both categories, and the venues' distinct decimal padding recorded as a verbatim-storage fact.** Previous: v0.9.45 (2026-08-04) — **§4 gains the dated Bybit per-endpoint rate-limit block: the verified per-UID-per-second table (create 10/s inverse/linear/option and 20/s spot, both upgradable; all four read surfaces 50/s), the verbatim rolling-window, batch-consumption and configuration-independence sentences, the WS-vs-API citation caveat (the page's "counted separately" sentence is about WEBSOCKET connection limits and must not be cited for API budget separation), and the named per-category-counting assumption recorded as a LOOSENING trigger for #171, never a correctness prerequisite** (milestone 3 task #185, record §66). Previous: v0.9.42 (2026-08-04) — **§8 gains the `orderId` identity block: per-symbol uniqueness AND non-reuse, both verbatim from the venue's own pages** (v0.9.42 is **milestone 3 task #174** — the Place ack's venue order id persisted at dispatch, and §5.3's gate 3 ruling swept rows out against the instrument's persisted-ack set; record §63). The banking order is the finding: the rule-out was designed against "same `orderId` ⇒ same order across time", a NON-REUSE claim this doc did not hold — §8 recorded only the weaker join-key fact — and the second-model review refused the slice until the premise was verified or the arm withdrawn. USDⓈ-M's Query Order page states *"orderId is self-increment for each specific symbol"* and the SPOT glossary *"uniquely identifies the order on a symbol"*; both are per SYMBOL, which is why every consumer scopes to the full instrument. Previous: v0.9.41 (2026-08-03) — **§5 gains the dated 2026-08-03 block: all four BTCUSDT tick sizes observed live, every 2026-07-31 lot/notional row re-confirmed, and three probe findings banked** (v0.9.41 is the **milestone 3 quantization slice** — §2.4's trading filters enter the order path; record §62). The ticks: Binance spot **0.01**; Binance perp, Bybit spot and Bybit perp all **0.1** — a 10× asymmetry inside Binance's own BTCUSDT, which is the price axis of the naked-leg case the order path now refuses. The probe findings: fapi's `exchangeInfo?symbol=` is **IGNORED** (the full ~1 MB roster answers, so a fetcher must filter client-side); Binance spot's `MARKET_LOT_SIZE` publishes a literal **ZERO** `stepSize` (`LOT_SIZE` vs `MARKET_LOT_SIZE` are now distinguishable in this doc — the v0.9.23-era note conflating them is superseded); and the perp's min-notional arrives under a **fourth spelling** (`notional`, beside `minNotional`/`minOrderQty`/`minOrderAmt`). The re-confirmed **50 USDT** USDⓈ-M min-notional floor is the fact that blocks the $20 live test on the Binance perp. Previous: v0.9.40 (2026-08-03) — set-version alignment, no change in this doc (v0.9.40 is **milestone 3 slice G3**: the parallel-IOC executor, `arblab exercise run`, and the process-kill-mid-IOC chaos test; record §59). The chaos fixture speaks the Binance shapes this doc already records — `-2013 NO_SUCH_ORDER` as the one documented not-found, the per-family hosts, `positionRisk` — and adds no venue fact, because a fake venue is not evidence about a venue. Previous: set-version alignment, no change in this doc (v0.9.39 is **milestone 3 slice G2**: the composition root, §5.2's OMS schema, `arblab reconcile run`, and §7.5's exemption traced back to the one cause it was argued from; record §58). Previous: **Status:** v0.9.38 (2026-08-03) — set-version alignment, no new venue facts in this doc (v0.9.38 is **milestone 3 slice G1**: §7.5's reconcile-only boot composed into one use case; record §57). §8's retention figures are what the slice's finding turns on — an unfilled terminal order purged after **3 days** on Binance USDⓈ-M and **24 hours** in Bybit's history — because they make an absent order the ORDINARY answer rather than an edge case, and the boot was asserting its net-delta invariant over exactly those orders' unrebuilt zero. Previous: v0.9.37 (2026-08-03) — **§9 lands: the POSITION surface**, read for the first time by §7.5's venue join (**milestone 3 slice F2**; record §56). Both venues' endpoints, parameters, direction encodings and scopes are recorded, and three facts are load-bearing rather than descriptive: Binance's **V2 and V3 `positionRisk` differ in what their SILENCE means** — V3 returns *"only symbol that has position or open orders"*, so an absent row is a flat account, while V2's absent row means the symbol does not exist — **the two venues encode direction in different layers** (Binance signs `positionAmt`; Bybit's `size` is *"always positive"* with the sign in `side`, which is empty for a flat row), and **Bybit's linear enumeration is illegal without a `settleCoin`** when no symbol is sent, which makes its scope narrower than Binance's. One ⚠ is recorded as evidence about the source rather than as a fact: the **V2 endpoint page still publishes weight 5 while the venue's own changelog raised it to 10** (2024-08-07), so an endpoint page can lag its changelog and the direction of that error is the one that ends in an IP ban. Previous: **Status:** v0.9.36 (2026-08-03) — set-version alignment, no change in this doc (v0.9.36 is **milestone 3 slice F1**: §7.5's journal replay, the authorization anchor's own record kind, and the boot outcome that halts AND latches; record §55). Nothing here moved because the replay reads a LOCAL store: the slice touches no venue surface, and the venue facts the anchor is compared against — the retention cliffs in §4 and §8 — are unchanged. Previous: **Status:** v0.9.35 (2026-08-03) — set-version alignment, no change in this doc (v0.9.35 is **milestone 3 slice E**: the §6.7 classifier made executable, with §5.1's hedge-group key gaining the account component no order record carried; record §54). Nothing here moved because the classifier is venue-agnostic by construction — it reads order state and a fill ledger, both already normalized by the read path this doc's §8 verified. Previous: **Status:** v0.9.34 (2026-08-03) — set-version alignment, no change in this doc, and the ABSENCE of a change is the point (v0.9.34 is **milestone 3 slice D**, the venue-specific stable-not-found rule; record §53). The rule needs a retention bound for **Binance SPOT**, whose Query-order page publishes none — and the missing figure does **not** become a §8 row. §8 records **verified venue facts**, and what stands in for an unpublished one is an **operator's assumption**, which is a local configuration with an owner rather than a fact about the venue; recorded here it would be indistinguishable at read time from the four §8 figures that were actually checked. §8's one *assumed* row is labelled as such for the same reason. What slice D does consume from this doc is §8 unchanged: the two published retentions per surface, of which the rule takes the **shorter** — an absence must be inexplicable under every hypothesis, and at the moment it is evaluated we do not know whether the order filled. Previous: **Status:** v0.9.33 (2026-08-03) — **A NEW §8 lands — the order READ surface**, the half §5.3's recovery and §6.7's finality contract actually consume (§7 was the submission half). It records **five order surfaces for two venues**, Bybit's normative realtime→history fallback, and the **four values that differ between Binance's own two families** — order-status weight 1 vs 4, executions weight 5 vs 20, order retention published vs not, and **the executions window cap at 7 days on USDⓈ-M against 24 HOURS on spot**, the same venue with the parameter spelled identically. It records that **`fromId` and the time window are mutually exclusive on both Binance families** (futures says so; spot's supported-combination whitelist contains no such pair), which is what forbids a venue resume handle from being §4's `Cursor`; that **Bybit's `nextPageCursor` walks into OLDER data** because its execution list is *"sorted by `execTime` in descending order"*, so a paging handle stored as a resume point re-reads one window forever **while every request succeeds**; that **`execTime` is not unique** and Bybit recommends a composite tiebreak; that **neither Binance trade list carries a client order id at all**, leaving the venue `orderId` as the only join key; that **not-found lives in the ERROR envelope on Binance (`-2013`) and in a SUCCESS body on Bybit** — §7's finding with the venues swapped; and the **retention cliffs** — an unfilled `CANCELED`/`EXPIRED` order dropped after **3 days** on USDⓈ-M and after **24 hours** in Bybit's history, which is precisely §6 step 6's marketable-IOC outcome. **One fact is an ASSUMPTION and is labelled as one**: that Bybit reports an unknown `orderLinkId` as `retCode` 0 with an empty list needs #171's credential, and the codec is written so the unverified branch **fails closed**. Previous: v0.9.32 (2026-08-03) — **A NEW §7 lands — the order submission surface**, the other half of what §47f's non-existent-§7 citation claimed was banked (§6 was the first half). It records both venues' create-order parameter tables; the **shape asymmetry** (Binance routes by host+path, Bybit by a `category` field inside a signed body — and both mis-routes are *accepted orders on the wrong product*); the **`newOrderRespType` enum-and-default asymmetry**, where the family defaulting to the largest response is the one whose response we least want (a spot LIMIT defaults to `FULL`, carrying a `fills` array on the submission path); the published `newClientOrderId`/`orderLinkId` rules **with the note that Binance SPOT publishes neither, so the futures rule is applied to it as a stated assumption**; the per-endpoint costs **including the documented futures ZERO**; the three testnet base hosts, all three probed alive 2026-08-02 **and again 2026-08-03** — including **`testnet.binance.vision`, the Binance SPOT base, which was recorded nowhere in this doc set**; and **the HTTP-200-with-`retCode` fact with both live probe transcripts** — Bybit states a refusal inside a success status, so a status-only classifier reads a rejection as an acceptance, which is §6's zero-length-401 seam reached by a **second input**. Bybit's `create-order` limit is 10/s, and **the HTTP status accompanying `retCode 10006` is NOT documented**, which is why it is classified in both the status arm and the code table. `ids.ClientOrderID`'s standing pre-milestone-3 budget assumption is **closed here**, and the two venues' id rules **differ** — `.` is legal on Binance and illegal on Bybit. Previous: v0.9.31 (2026-08-02) — **A NEW §6 lands — the private REST surface**: both venues' verified signature compositions, three reproducible Binance test vectors, the asymmetric acceptance window, the refuted credential-free probe, the error→effect semantics and the rate-limit header polarity. This is the verification §47f's slice-C scope note assumed was already banked (v0.9.31 is **milestone 3 slice C1**: `engine/venue` has a **signed REST transport** at last — credentials that cannot leave their package, both venues' signature compositions, the clock-injected timestamp, the §7.6 governor attachment and §5.3's effect classification. §47f's slice C is **split by LAYER, not by venue**, because §47f's own mirror hazard forbids putting the two venues' clients in different sessions; C2 carries `Place`/`OrderByClientID`/`Fills`. **THREE FINDINGS.** (1) The slice's scope note claimed the venue facts were *"already recorded (profiles §1/§4/§7)"* — **there is no §7**, and how either venue composes the string it signs was recorded NOWHERE; a citation to a section that does not exist is the tell, and it is §47e's shape one layer out, invisible to the marker grep because the clause is absent rather than departed from. (2) **The evidence is not symmetric even though the code is:** Binance publishes three reproducible vectors (all re-computed and matching), Bybit's masks its secret (the literal-placeholder hypothesis was TESTED and refuted), and the credential-free differential probe is **refuted on both venues** — each checks the credential before the signature — a refutation trustworthy only because Binance ran first as its **positive control**. So writing Bybit's signer "the same way" inherits none of Binance's assurance (task #171). (3) **§7.6 was WRONG about Bybit's header and is corrected**: `X-Bapi-Limit-Status` is *"your remaining requests"*, the COMPLEMENT of Binance's used-weight, so the upward-only fold inflates the count on good news and **discards the venue's warning when it is nearly banned** — the silent direction, and the one that ends in the ban. Also: the acceptance window is **asymmetric** — `recvWindow` widens only the behind side while the ahead side is a fixed 1000 ms no parameter can raise, so the documented knob repairs one direction and cannot touch the other. 18 mutations, 18 kills; three survivors, one of which is a NEW shape — the fixture was correctly varied and the **assertion** could not see the variation. Record §50). Previous: v0.9.30 (2026-08-02) — set-version alignment, no change in this doc (v0.9.30 is **milestone 3 slice B**: §6 step 5's planning use case exists and `module.go`'s stated hole is CLOSED — deterministic `ClientOrderID` minting, intents fsynced to the write-ahead journal before any submission, `ApplyPlanned`'s first caller, the OCC version threaded into `Dispatch`, and `Deps.Journal` converted to a path as `singlewriter` forced. **`Dispatch` now takes an id and a version and READS the order from the write repository**, so an order that was never planned cannot be dispatched and one mutated after planning cannot be submitted in place of the one that was journaled. **`TradePlanned` is deliberately unwritten** — §5.1's `Trade` aggregate has no domain type — and what makes that safe is the same sentence's *all legs before any submission* clause, since every partially-journaled plan is then `Planned`-only and §5.3-discardable. **THE FINDING is `FM-5`'s shape at the PROJECTION layer:** a journal-then-apply failure *after* the send leaves a sent order at `Planned`, the one non-terminal state contributing no possible-delta interval, so §6.7 reads an **ambiguous** residual as **determined** — refuse-to-trade, landed in §6 step 7, task #169, and as in that family no invariant computed from the projection can detect it. 17 mutations, 17 kills; the collapse checklist found a **format** collapse — leg and attempt both render as two digits, so at attempt 0 a field-order bug is invisible. Record §49). Previous: v0.9.29 (2026-08-02) — set-version alignment, no change in this doc (v0.9.29 is **milestone 3 slice A is landed** — `ports.Journal` has an adapter at last: a durable, append-only, hash-chained write-ahead journal on SQLite (WAL + `synchronous=FULL`, **verified at open rather than assumed**), closing §47e finding 2, so §5.3's fsync-before-network crash boundary is no longer proven only against a test fake. Record **§48**. **The store is SQLite on §5.3's ground, not §13's** — neither journal method needs a transaction today, so from inside the slice the engine reads as a free choice and it is not: the fill apply, the REST cursor advance and the journal/outbox append must share **one transaction**, and `driven/doc.go` had already recorded that a transaction spanning two stores cannot be added to one of them alone, so a bespoke log file would be rewritten at slice D/F or the rule would go unmet. **Three findings.** (1) Session 73's predicted design fork was **mis-premised**: `engine/capture` has a *sequence* chain plus per-record CRC32C and whole-file sidecars — **no hash chain** — so reuse would have supplied framing and the chain would have been new code regardless. (2) The pragma check hid a **self-consistent mutation**: `journalDSN` and `verifyPragmas` deliberately share `wantSynchronous` so they cannot drift, which means mutating the constant makes the request and the check agree and every *Open succeeds* test passes against a journal that no longer fsyncs — capture's wrong-CRC-polynomial note arriving as a **configuration constant**, and the fix is the same: the required values are stated as **literals** in the test. (3) §47f's collapse warning stopped **one short** — it named a chain of length 1, and at length **2** *link to the previous record* and *link to the first record* are still the same predicate; three is the shortest chain that separates them. Twelve mutations, twelve kills, fsync and the chain link checked separately; the collapse checklist ran before them. `Deps.Journal` stays a port field (slice B's, per §47f) and chain **verification** is not implemented — only construction). Previous: v0.9.28 (2026-08-02) — set-version alignment, no change in this doc (v0.9.28 is **milestone 3's pre-gate admission**, ruling record §47: the milestone-2b hard gate narrows to milestones 5–7 plus milestone 4's leg-shape-dependent half, kill authority over live capital untouched. **Relevant to this doc going forward:** milestone 3 slice C builds `engine/venue`'s never-delivered **account/order half** — signed REST, `Place`, `OrderByClientID`, `Fills`, and Bybit's `GET /v5/order/history` fallback — **testnet only, and nothing unverified enters here**; §1/§4/§7's private-surface facts are its normative input). Previous: v0.9.27 (2026-08-02) — **one new dated block: book-stream "update speed" is a CEILING, not a cadence**, measured live over 24 whole hours × 9 symbols × both venues, with the local-loss alternative ruled out by a zero-break `pu` chain (v0.9.27 is milestone 2b step 2's **§32m-D per-tier disk figure, measured 2026-08-02 inside its 48-hour deadline**: the day is **11.165 GiB over 24 whole hours with all 40 sources `alive 60/60` and verify 0 integrity — BTC tier 4.314, alt tier 6.852, per shortlisted symbol 0.387 to 1.743**. §42c's derived ~0.6–1.1 G/day/symbol is refuted **at both ends and in its premise**: the venues' documented **update speed is a ceiling, not a cadence** — measured `pu`-chain breaks zero, minimum inter-push 102 ms, maximum silence 22.5 s in the hour checked and 94.8 s over the day — so every book leg is activity-driven and no symbol-independent floor exists. Books are **93.6%** of the burn, so the un-backfillable half is the whole cost, and **§38f's off-host destination is a live user decision again on a measured clock**: 85 days to a full local disk, the ~2026-09-30 gate at day 59 with ~678 GiB single-copy). Previous: v0.9.26 (2026-08-02) — set-version alignment, no change in this doc (v0.9.26 is formal model (a)'s slice F: `FM-8`'s standing-precondition ruling is **encoded and model-checked** — `NC21` violates the new `INV9`, so the hole is shown to close — and encoding it found **`FM-9`**, a defect in the clause that landed one release ago. The v0.9.25 transition row named **the §2 classifier's verdict** as the trigger, and the generation's own perp leg moves that verdict by design, so control `NC22` aborts a generation on a **physically flat package 4 states from `Init` with no external event at all**. The fix is the same test evaluated **net of the generation's own legs**; the ruling itself is untouched. Nothing here changes — the safety kernel's machines are milestone-3+ machinery, hard-gated on strategy #2's own economic gate, and serve it unchanged when it fires). Previous: v0.9.25 (2026-08-02) — **FORMAL MODEL (a)'s SLICE E IS COMPLETE, and it produced two further findings: `FM-7` and `FM-8`, both LANDED here (`FM-8` by user ruling; its model encoding is owed).** **`FM-7`:** SM §2's interval-floor rule *"never issued twice"* is **sound and stays** — but its stated justification (*"else, for `[0.4, 1.0]`, two uncertain `0.4` reductions could reach `−0.4` and defeat the proof"*) **assumes the reservation is NOT counted, which the adjacent sentence requires it to be.** Control `NC20` passes over the **complete** state graph (8,770,801 generated / 1,086,199 distinct / **0 left on queue**): double issuance cannot overshoot the sign while reservation-aware sizing is working. The rule is therefore recorded as **deliberate defence in depth over two independent mechanisms** — it is the one that survives a defect in the other — and the historical counterexample is retained as *what the rule protects against* (the `FM-5` world, an entry not backed at the instant it is read) rather than as a hazard reachable under the sizing this set mandates. It was fixed rather than left because **nobody re-derives a recorded justification**: a later reader checking the reason would find it does not hold, and the natural next move is to retire a rule that is doing real work. **`FM-8` — RULED by the user and LANDED here; only its MODEL ENCODING is owed (slice F):** §7's *"the package has no naked residual"* is enforced **only as an ENTRY GUARD**, and on a **reservation-netted** quantity — so probes `PC27`/`PC28` reach, at **3 and 4 steps from `Init`**, a **live `PackageDelever` generation over a package that is physically short and no longer even `Determined`, with nothing reserved against it.** The offsetting reduction's own ambiguous result releases its ledger entry exactly as SM §2 row 4 mandates; what is missing is anything that tells the **generation**. It is **`FM-5`'s mirror** (*an entry counted before its backing exists* / *an entry's premise outliving the entry*), and it is invisible to every invariant in the module because `INV5` polices the generation's **own legs** and never the premise it opened under. **The ruling is the STANDING PRECONDITION:** "no naked residual" is a standing condition, not only an entry test, so the classifier leaving `Determined` **supersedes** an active generation, quiescing its reserved leg to §6.7 finality first — a **third** row beside `EmergencyClosePackage` and `FenceReadyLost`, policed by the **same** quiesce invariant rather than a new one. The rejected alternative (tighten `PdPlan` to physical `realized = 0`) **moves the witness rather than removing the class** — a floor issued a step after the generation opens dissolves the premise identically — and is kept on record as an available hardening, not as a refuted idea. **What is NOT yet done, and is marked as such: the model encoding and its negative control (slice F, sketched in §46 §11f). The clause is normative from this release; the run that shows the hole closes has not been performed, and a fix argued is not a fix checked.** **Also this release: model (b)'s five negative controls were asked the witness-degeneracy question for the first time** — six probes (`FenceTakeover_W1`–`W6`), **all REACHED** — and the degeneracy was **systematic rather than four accidents**: every harm sits exactly one `VEntryFill` past the witness TLC reported, because breadth-first search always finds the cheap surrogate for exposure before exposure itself. It surfaced two suite defects invisible from a green run: **`I2_HostIndependentCoverage` had never been violated by any run in the suite** (listed in two configs, both halting on a shallower `I3` before the `I2` state exists to be found), and **`fence-takeover-model.md`'s own prose carried harm its runs never showed** — claims that were *true about the system*, which is exactly why four review rounds passed over them. Previous: v0.9.24 (2026-08-02) — set-version alignment, **no new venue fact** (v0.9.24 is `FM-5` AND `FM-6`, formal model (a)'s fifth and sixth spec defects: the reservation ledger's entries must carry outstanding net intent rather than target, and a change to that ledger must re-trigger the §2 classifier — both are claims about our own bookkeeping, resting on no venue behaviour, so nothing here is touched or owed). Previous: **Status:** v0.9.23 (2026-08-01) — set-version alignment, no new venue fact (v0.9.23 is strategy #2's cross-venue gate procedure commit, §45p). One venue observation from pinning the roster's sizes, deliberately NOT frozen here because the instrument roster moves: the shortlist's lot steps DIFFER between venues on two of eight names (KAITOUSDT 0.1 on Binance vs 1 on Bybit; BCHUSDT 0.001 vs 0.01) and Binance's BCHUSDT row carries a 20 USDT minimum notional against every other row's 5 — read 2026-08-01 from the ledgered `exchange_info`/`instruments` files and pinned in the roster's own battery with their provenance. Binance also serves both `LOT_SIZE` and `MARKET_LOT_SIZE` filters, each with a `stepSize`; for all eight symbols the two agree, so that data cannot distinguish which one a reader consulted — recorded because the next roster may not be so lucky. The §41r-caveat-1 scope item is unchanged: measured alt-pair depth facts land here only when §42c books have accumulated enough to measure. Previous: **Status:** v0.9.22 (2026-08-01) — **the per-IP WebSocket connection-ceiling verification LANDS** (dated block at the end of §5), discharging the scope item the v0.9.20 note opened and the v0.9.21 note marked due with the §42c expansion: Bybit ≤500 new connections per 5 minutes per WebSocket domain, Binance spot 300 per attempt per 5 minutes per IP, and **Binance USDⓈ-M futures states no numeric per-IP connection ceiling at all** — its bound is behavioral (repeated-disconnect ban), which makes the capture's per-source backoff ladder the compliance mechanism rather than a courtesy. Read against the expansion's 39 concurrent connections (19 fstream / 19 Bybit / 1 Binance spot), every documented numeric bound clears by an order of magnitude; a broad funding tier re-runs the arithmetic at 2N sockets and stays gated. The remaining §41r-caveat-1 scope item is unchanged: measured alt-pair depth facts land here only when §42c books have accumulated enough to measure. Previous: v0.9.21 (2026-08-01) — set-version alignment, no new venue fact (v0.9.21 is milestone 2b step 1 executed, screening design record §43: the screening ran over both venues' settled-funding REST history — 465/465 symbols, zero fetch failures, and **zero pairing collisions at nearest-minute rounding across 1.57M matched instants**, which is §7's publication-jitter note holding venue-wide, measured). The v0.9.20 scope notes discharge as follows: per-candidate history depth/grid integrity is now MEASURED — listing dates, modal intervals, and interval-move counts (running into the hundreds for young listings) live in `research/screening/ranking.json` as measurements over the assembled rows, deliberately not frozen here (the moving-roster rule); the per-IP WebSocket connection-ceiling verification is now DUE with the §42c capture expansion, before any config lands; measured alt-pair depth facts still land here only when §42c books flow (§41r caveat 1 — the ranking priced published base-tier taker cells, not depth). Three live observations from the screening fetch (2026-08-01, both venues' public REST): Binance `fundingRate` rows now carry a `rateType` field (`"Regular"` observed) — the screening parser deliberately ignores it, and any future consumer should treat an unfamiliar value as a question, not a default; Binance's millisecond publication jitter is live again on alt rows (`…001` stamps, 1ms past grid — the §7 note, still true); and Bybit V5's paged funding shapes hold for alts exactly as §7 records them (newest-first pages, string timestamps). Previous: v0.9.20 (2026-08-01) — set-version alignment (v0.9.20 is the strategy-#2 scoping + §12 re-sequencing, scoping record §42), and a scope note for this document's future: the scoping's screening step will verify per-candidate funding-history depth/grid integrity (the §7 history rows are BTC verifications and do not transfer), the capture expansion's venue per-IP WebSocket connection ceilings are a verification item **before** any broad tier lands, and §41r caveat 1 still means measured alt-pair depth facts land here rather than inheriting the BTC "fees, not depth" row. The 2026-08-01 instrument-roster measurement (466 both-venue USDT perps; 431 same-grid, 332 at 4h; Binance `fundingInfo` covering all 466) lives in scoping record §42b — a moving roster, re-fetched at screening time, deliberately not frozen into this document. Previous: v0.9.19 (2026-08-01), the §12 post-kill fork ruling — (c), strategy #2 early (calibration design record §41r). Previous: v0.9.18 (2026-08-01), milestone 2's economic-gate kill. Previous: §1's Bybit cap/floor row and the dated §5 block record the **2026-07-30 `instruments-info` live measurement**: upper/lower funding clamps `±0.005`, exactly symmetric and equal to the ticker's `fundingCap` — the ground the replay decoder's floor mirror rests on, with the live clients obligated to replace the mirror with the polled pair — and `fundingInterval: 480`, MINUTES, the units-in-the-name trap live beside the same venue's `fundingIntervalHour`.
**Doc set:** normative core `build-spec.md` · venue facts `venue-profiles.md` · kernel machines `state-machines.md` · history/traceability `adr-history.md` · teaching companion `tutorial.md`.
**Role:** per-venue facts + quirk matrices behind the core's venue-contract requirements (`build-spec.md` §4). **Citation coverage, stated honestly `[→ R1d-11]`:** every fact carries its verification *date* (2026-07-18 / 2026-07-19 / 2026-07-20); the **safety-kernel rows carry full source citations in §4 here**; the remaining §1/§3 rows' per-row citations live in the v0.6/v0.7 review records (`docs/review/`), not inline — treat any fact older than a venue API cycle as due for re-checking.

## 1. Per-venue quirk matrix (perp/futures)

**Per-venue quirk matrix the adapter MUST encode (verified 2026-07-18):**

| Concern | Binance USDⓈ-M | Bybit V5 linear |
|---|---|---|
| WS routing | Base `wss://fstream.binance.com`. **Three prefixes, split by data rate** — `/market` (regular market data: **mark price**, aggTrade, kline, tickers, liquidations, mini-tickers, composite index), `/public` (high-frequency: **depth**, bookTicker, `!bookTicker`), `/private` (user data). Raw stream `…/<prefix>/ws/<stream>`; combined `…/<prefix>/stream?streams=<a>/<b>`, or connect to `…/<prefix>/stream` and send `SUBSCRIBE`. **Legacy `wss://fstream.binance.com/ws` and `/stream` decommissioned 2026-04-23** `[→ 7.2]` *(re-verified verbatim 2026-07-29, §5)* | **`wss://stream.bybit.com/v5/public/linear`** (USDT/USDC perp + futures); also `/v5/public/spot`, `/v5/public/inverse`, `/v5/private`, `/v5/trade`; testnet is `stream-testnet.bybit.com`, same paths. Topics `tickers.<sym>`, `orderbook.50.<sym>`, private. **Client sends ping ≈ every 20 s**; ≤ 500 connections per 5 min per domain; public `args` ≤ 21 000 chars *(verified 2026-07-29, §5)* |
| Funding estimate | **Per-symbol `<sym>@markPrice@1s`** (or `<sym>@markPrice` = 3 s) on the **`/market`** prefix; all-symbol `!markPrice@arr@1s`. Payload → `r` (funding rate), `p` (mark), `i` (index), `P` (estimated settle price, *"only useful in the last hour before the settlement starts"*), `ap` (mark price moving average), `T` (next funding time), `st` (symbol type, post-CM-migration: 1=UM, 2=CM), plus `e`/`E`/`s`. **No interval field.** *(per-symbol form + `ap`/`st` verified 2026-07-29, §5)* | `tickers.<sym>` → `fundingRate`, `nextFundingTime`, **`fundingIntervalHour`, `fundingCap`** (new Oct-2025; pin client ≥ Oct-2025) |
| Interval source | REST `GET /fapi/v1/fundingInfo` overlay (absent ⇒ 8h default); formula scales `/(8/N)` | from ticker `fundingIntervalHour`; **dynamic 8h↔1h auto-switch on alts, no notice** `[→ C8]` |
| Cap/floor | BTCUSDT ±0.3% (=0.75×MMR); alts ±2% default; overlay via `fundingInfo` | ticker `fundingCap` / `instruments-info.upper/lowerFundingRate`; read live. **BTCUSDT measured 2026-07-30: upper `0.005` / lower `-0.005`, exactly symmetric and equal to the ticker's `fundingCap`** (§5) |
| Book | On the **`/public`** prefix. Partial: `<sym>@depth<5\|10\|20>@<100\|250\|500>ms`; diff: `<sym>@depth@<100\|250\|500>ms`. Omitted speed = **250 ms** default *(levels + all three speeds verified 2026-07-29, §5)* | `orderbook.<1\|50\|200\|1000>.<sym>` on the **linear** URL; **level 50 pushes at 20 ms** (1 → 10 ms, 200 → 100 ms, 1000 → 200 ms). Snapshot+delta; delete on size "0"; reset on snapshot *(level-50 cadence verified 2026-07-29, §5)* |
| **Depth update-ID shape** `[→ standing debt #1, CLEARED 2026-07-29, §5]` | **Futures partial and diff depth carry the SAME shape** — `U` *"First update ID in event"*, `u` *"Final update ID in event"*, `pu` *"Final update Id in last stream (ie `u` in last stream)"*, plus `E` event time and `T` transaction time. A **span with a chain link**: a gap is detectable from the wire alone, because `pu` must equal the previous event's `u`. **Binance SPOT does not share this — see §3.** | **A single `u` per message** (*"Update ID … is used to identify the data version"*), plus `seq` (cross sequence) and `type` ∈ {`snapshot`, `delta`}. **No span, no previous-event link**, so continuity is `u` monotonicity alone and a `snapshot` is the reset signal. **`u` is reset to 1 on a service restart** — a decoder treating `u` as globally monotonic breaks across one |
| Ticker deltas | full snapshot each message | snapshot then delta; **absent field = unchanged, never null-coerce** |
| Private streams | `/private` user-data WS (listenKey lifecycle): order, execution, `ACCOUNT_UPDATE`, margin-call, ADL | private topics `order`, `execution`, `position`, `wallet`; auth handshake |
| Forced-close reason `[→ R1d-7]` | liquidation/ADL closes surface on `ORDER_TRADE_UPDATE` via **system `clientOrderId` prefixes** — `autoclose-` (liquidation), `adl_autoclose` (ADL); REST recovery **`GET /fapi/v1/forceOrders`** (`autoCloseType` `LIQUIDATION`\|`ADL`) *(corroborated 2026-07-20 — not verbatim-fetched, §4; browser-confirm before implementation)* | **no `closeType` field exists on any V5 private stream** (the v0.7.1 row was wrong). Verified 2026-07-20: **execution `execType` ∈ {`BustTrade` (liquidation), `AdlTrade` (ADL)} is the authoritative signal** (carries qty + `execId` — dedups through the standard fill ledger); corroboration: order `createType` ∈ {`CreateByLiq`, `CreateByTakeOver_PassThrough`, `CreateByAdl_PassThrough`}, position `positionStatus` ∈ {`Liq`, `Adl`} + `adlRankIndicator`. Recovery: **`GET /v5/execution/list`** (accepts + returns `execType`); closed-pnl distinguishes only `BustTrade`, **not** ADL. **No cross-topic arrival order is documented** — join fills↔position via `seq`+symbol (§4) |
| Order query | `GET /fapi/v1/order` by `origClientOrderId`; `userTrades` for fills | `GET /v5/order/realtime` (open + last ~500 closed, **cleared on server restart**) → fall back to **`GET /v5/order/history`** by `orderLinkId`; `/v5/execution/list` for fills `[→ R2-#1]` |
| Attached protection | **No atomic attach-on-entry**: New Order carries no TP/SL param; conditional types are **separate Algo-Service orders** (`POST /fapi/v1/algoOrder`, since 2025-12-09; sending via `/order` returns **-4120**); **no futures OCO/bracket** → a pre-armed **external actuator is MVP-mandatory** on the perp leg for the first-fill→ack window (core §7.4). SPOT brackets via `orderList/otoco` (LIMIT working order only) | **Venue-native TP/SL attaches atomically** on `POST /v5/order/create` — but the create response is an **async accept**; the fill *and* the armed protection are confirmed on the **private WS** (`order`/`execution`), not the HTTP ack. Baseline host-independent cover on the **perp** `[→ R1-F2, R2-#4, R2b-B2]` |
| Fees | `GET /fapi/v1/commissionRate` (+`rpiCommissionRate`) | `GET /v5/account/fee-rate` |
| Position mode | Hedge Mode: **`reduceOnly` cannot be sent** (param-level restriction; -2022 "ReduceOnly Order is rejected" is the *generic* error, not the Hedge mechanism); close via opposite side + matching `positionSide`. One-way uses `reduceOnly`; **read one-way via `GET /fapi/v1/positionSide/dual`.dualSidePosition==false** | `positionIdx` 0/1/2 + `reduceOnly`; one-way = `positionIdx 0` (read `position/list`; set `position/switch-mode 0`) |
| Market order | `MARKET` has **no price cap** — use LIMIT+IOC | converts to IOC limit w/ venue band; optional user `slippageTolerance` |
| Tiered MMR | **fetch `GET /fapi/v1/leverageBracket` at runtime** — table not otherwise readable `[→ C4]` | `instruments-info` risk limits |
| Rate limit | `X-MBX-USED-WEIGHT-*`/`ORDER-COUNT-*`; 429→418 IP ban (2 min→3 days) | `X-Bapi-Limit*` headers **and** unheadered IP gateway ban ~600/5s (retCode 10018) |

## 2. Account & margin architecture (posture facts)

**Per-venue posture facts + assert-or-refuse read paths (core §4.1):**

| Concern | Binance USDⓈ-M (classic account) | Bybit UTA (linear) |
|---|---|---|
| Account model | Segregated **spot wallet ≠ futures wallet** (separate pools; funds transferred in). **Portfolio Margin / Unified MUST be OFF** — it would fold spot into futures collateral | **UTA** unified account (UTA2.0 for new accounts; classic unavailable since the Dec-2024 forced migration) — the load-bearing property is **Isolated Margin mode**, not the UTA *version*; segregation is per-position, not separate wallets |
| Perp margin mode | **ISOLATED** — `POST /fapi/v1/marginType {marginType:ISOLATED}` | **ISOLATED_MARGIN** (account-level under UTA2.0) — `POST /v5/account/set-margin-mode {setMarginMode:ISOLATED_MARGIN}` |
| Collateral policy | **Single-Asset Mode** — `POST /fapi/v1/multiAssetsMargin {multiAssetsMargin:false}` (USDT-only margins BTCUSDT). Multi-Assets forces **Cross** and is mutually exclusive with Isolated → MUST be off | Per-coin collateral switch exists **only in Cross/Portfolio, not Isolated** — do **not** rely on "switch BTC off"; Isolated per-position liquidation *is* the segregation. `spotHedgingStatus` MUST be **OFF** (spot-hedging lets spot offset derivatives, re-entangling the legs) |
| Spot borrow/leverage | plain spot buy (no margin) | **`isLeverage=0`** hard-coded on every spot `POST /v5/order/create`; spot-margin mode **OFF** (`/v5/spot-margin-trade/switch-mode 0`) |
| Position mode & leverage `[→ R2b-5]` | **One-way** — `GET /fapi/v1/positionSide/dual`.dualSidePosition==false; configured leverage per symbol pinned (drives liq distance). Hedge mode forbids `reduceOnly` (param-level) | **One-way** — `positionIdx==0` (read `GET /v5/position/list`; set `POST /v5/position/switch-mode 0`); configured leverage per symbol pinned |
| Spot seizable for a perp loss? | **No** — futures liquidation is computed on the futures wallet only; the margin engine never sees spot BTC | **No, under Isolated** — per-position loss is capped at that position's margin; there is **no account-level MMR liquidation** in Isolated mode. Requires **auto-add-margin OFF** so the position can't silently pull funds |
| Rescue margin lives in | **staged in the futures wallet, *outside* the isolated position** — applied by an explicit **`positionMargin {type:1}` top-up** when liq distance breaches rebalance (Isolated ⇒ free wallet is *not* seen by the position's liq engine; spot-wallet USDT is invisible to the futures engine, and spot→futures transfers degrade under stress); core §7.1 | **staged in the UTA, *outside* the isolated position** — applied by an explicit **`position/add-margin` top-up** on the same trigger (Isolated ⇒ per-position margin only; auto-add-margin OFF); core §7.1 |
| Posture assert-or-refuse (boot + pre-trade) | `GET /fapi/v1/multiAssetsMargin`==false; `GET /fapi/v1/symbolConfig`.marginType==ISOLATED (or `positionRisk`); Portfolio-Margin OFF (**⚠ no read path in this document — attestation-backed, see the ⚠ block below**); `positionRisk.isAutoAddMargin`==false (**read-only** — no REST setter, off by default; **⚠ which VERSION publishes this field is unsettled — see the ⚠ block below**); **one-way** `positionSide/dual`.dualSidePosition==false; configured leverage matches pin | `GET /v5/account/info`: **marginMode==ISOLATED_MARGIN** (the load-bearing check), unifiedMarginStatus ∈ {3,4,5,6} = any UTA (classic **1** refused; 5/6 expected for new accounts), spotHedgingStatus==OFF; `GET /v5/spot-margin-uta/status`==off; `GET /v5/account/wallet-balance` every coin borrowAmount==0 ∧ spotBorrow==0; **auto-add-margin OFF for every configured linear symbol** (not only open positions); **one-way** `positionIdx==0` (`position/list`); configured leverage matches pin |

### ⚠ Two gaps in the row above, marked rather than filled (2026-08-06, task #198, record §73)

This document takes **verified facts only**, so what the §4.1 posture slice could not verify is
recorded as a marked unknown instead of as a value. Both gaps are in the Binance column, and both
leave the gate **refusing** rather than passing, which is the safe direction and also why neither is
visible to any test.

- **Portfolio-Margin OFF has NO READ PATH in this document, and the check is ATTESTATION-BACKED until
  one is verified.** Every other item in the Binance assert-or-refuse cell names a `GET`; this one
  names a requirement, and the account-model row above states it (*"Portfolio Margin / Unified MUST
  be OFF"*) without supplying an endpoint. Nothing has been entered here on the strength of an
  operator attestation or a recollection — candidate read paths exist in assistant recollection and
  are deliberately **not recorded**, that being the channel which has already misreported a
  load-bearing number twice in this project (`/api/v3/account`'s weight; §9's three disagreeing
  `positionRisk` candidates). Verification is blocked by the same AWS WAF challenge that has blocked
  **#180** across sessions. Core §4.1 specifies the operator attestation that stands in meanwhile —
  inputs-not-conclusions, two-act `{path, sha256}` pin, events-not-time invalidation, absent →
  refuse — and states its limit: it is bound to no history, so nothing falsifies it.
- **`isAutoAddMargin` is cited above from the V2 page, and the engine reads it from V3, whose own
  verbatim field transcription in §9 does NOT list it.** §4's Binance posture block records the
  setting as read-only *"via `GET /fapi/v2/positionRisk` (**or v3**)"* — the parenthetical is
  unverified — while §9's dated V3 read transcribes twenty row fields and neither `isAutoAddMargin`
  nor `autoAddMargin` is among them. Both readings are open: §9's list may be an abbreviation, or V3
  may have dropped the field with the rows it also dropped. **The consequence differs per reading and
  neither is a wrong pass:** if the field is present the gate works as designed; if it is absent, the
  observation decodes as *unstated* and every configured symbol refuses — the same unreachable-gate
  shape core §4.1 records under task #199, arriving from a second cause. It is settled by one read of
  the two endpoint pages (the #180 blocker again) or by one credentialed call.
- **Whether Bybit `GET /v5/position/list` returns a row for a CONFIGURED-BUT-NEVER-TRADED symbol is
  not established anywhere**, and all three of Bybit's per-symbol posture settings are answered from
  it. If it omits such symbols, the flat-account refusal core §4.1 records for Binance holds on Bybit
  too, for `positionIdx`, leverage and auto-add-margin at once. **#171's credential settles it with a
  single call** — a loosening trigger, never a correctness prerequisite.

## 3. Spot-leg surface

**Spot-leg facts — separate hosts/streams/filters; no mark/funding exists on spot (core §4.2):**

| Spot concern | Binance SPOT (`api.binance.com`) | Bybit SPOT (`category=spot`) |
|---|---|---|
| Order / query / fills | `POST /api/v3/order`, `GET /api/v3/order`, `/api/v3/openOrders`, **`/api/v3/myTrades`** (fills) — *not* the futures `userTrades` | V5 `/v5/order/*` with `category=spot`; `/v5/execution/list` |
| User-data stream | **separate listenKey** — `POST /api/v3/userDataStream` on the spot host (+PUT keepalive ~30 m, DELETE), distinct from `/fapi/v1/listenKey`. **Deprecated →** new path is WS-API `userDataStream.subscribe` needing **Ed25519 `session.logon`** (pin the auth path) | private `order`/`execution`/`wallet` on the spot private WS |
| Trading filters | `GET /api/v3/exchangeInfo` → `LOT_SIZE` (stepSize), `PRICE_FILTER` (tickSize), `NOTIONAL`/`MIN_NOTIONAL` | `instruments-info category=spot` (lot/tick/min-notional) |
| Book stream | `<sym>@depth`/`@depth<5\|10\|20>` on `wss://stream.binance.com:9443` (or `:443`) — *not* fstream, and **no `/public` prefix**: the spot host kept its own base. Partial-depth speeds are **1000 ms or 100 ms** (*not* futures' 100/250/500). Init snapshot `GET /api/v3/depth` *(verified 2026-07-29, §5)* | `orderbook.<1\|50\|200>.<sym>` on `wss://stream.bybit.com/v5/public/spot` (category set by URL path); level 50 at 20 ms as on linear |
| **Depth update-ID shape** `[→ standing debt #1, §5]` | **Two shapes, and neither is the futures one.** Spot **diff** depth carries `U` + `u` but **no `pu`** — a span with no chain link. Spot **partial** depth carries **`lastUpdateId` and nothing else** — no span, no `pu`, no event time `E`. So a Binance spot partial-depth feed is **not gap-detectable from its own wire content**; ordering evidence has to come from outside the payload | Same as linear — single `u` (reset to 1 on service restart), `seq`, `type` ∈ {`snapshot`, `delta`} |
| Native bracket | `orderList/otoco` (OTOCO) / `oco` — atomic spot bracket, **LIMIT working order only** | TP/SL on the spot order where supported |
| Mark / index / funding | **none — spot has no mark or funding.** Price & freshness-check the spot leg from its **own book (best bid/ask / last)**; `StreamMark` is **perp-only**. Bybit spot ticker's `usdIndexPrice` is a USD-conversion ref, **not** a mark | same — no mark/funding on spot |

So `MarketDataSource.StreamMark` is defined **only for perp instruments** (below), and the spot leg's valuation/freshness reads its book. Rate-limit governors are **per `(venue, API family)`** — `(binance, spot)` and `(binance, futures)` report and enforce *separate* used-weight/order budgets via their own `exchangeInfo`, so they are distinct **weight/order** buckets, not one venue bucket — though the **IP-ban envelope is treated as shared per venue** (inferred, not documented; core §7.6).

## 4. Verification record (dated sources)

Verified against official docs 2026-07-18 / 2026-07-19 (from the v0.7 review record §2, frozen there):

### Binance USDⓈ-M — isolated margin, posture, keys
- **Isolated margin:** only the position's allocated isolated margin is at stake; **free futures-wallet USDT does not passively back an isolated position** — rescue is an active add-margin call. → https://academy.binance.com/en/glossary/isolated-margin · https://www.binance.com/en/support/faq/how-to-adjust-margin-balance-in-cross-isolated-margin-mode-360038447311
- **Add isolated margin:** `POST /fapi/v1/positionMargin` (`type` 1=add, 2=reduce). → https://developers.binance.com/docs/derivatives/usds-margined-futures/trade/rest-api/Modify-Isolated-Position-Margin
- **Auto-add-margin:** `isAutoAddMargin` is **READ-only** via `GET /fapi/v2/positionRisk` (or v3 — **⚠ the parenthetical is UNVERIFIED and §9's dated V3 field transcription does not list the field; see §2's ⚠ block, task #198**) — **no public REST setter** on USDⓈ-M; off by default → read-verify `false`. → https://developers.binance.com/docs/derivatives/usds-margined-futures/trade/rest-api/Position-Information-V2
- **Position mode read:** `GET /fapi/v1/positionSide/dual` → `dualSidePosition` (true=Hedge, false=One-way). Hedge mode: `reduceOnly` **cannot be sent** (param-level restriction; -2022 "ReduceOnly Order is rejected" is the *generic* error, not the Hedge mechanism). → https://developers.binance.com/docs/derivatives/usds-margined-futures/account/rest-api/Get-Current-Position-Mode · https://developers.binance.com/docs/derivatives/usds-margined-futures/trade/rest-api/New-Order
- **Margin-mode events:** `ACCOUNT_CONFIG_UPDATE` fires on leverage (`ac`) and multi-assets (`ai`); **isolated/cross margin-type** change surfaces via `ACCOUNT_UPDATE` reason `MARGIN_TYPE_CHANGE`. **Position-mode (one-way/hedge) change emits NO event → REST poll.** → https://developers.binance.com/docs/derivatives/usds-margined-futures/user-data-streams/Event-Account-Configuration-Update · https://developers.binance.com/docs/derivatives/usds-margined-futures/user-data-streams/Event-Balance-and-Position-Update
- **Key management / fence:** a **regular account has NO public REST endpoint to disable/delete/IP-edit its own key** (UI + 2FA only; read-only `GET /sapi/v1/account/apiRestrictions`). API-automatable **only via the sub-account program with a MASTER key**: `DELETE /sapi/v1/sub-account/subAccountApi`, permission `POST /sapi/v1/sub-account/subAccountApiPermission`, IP `POST /sapi/v2/sub-account/subAccountApi/ipRestriction`. → https://developers.binance.com/docs/wallet/account/api-key-permission · https://developers.binance.com/docs/sub_account/api-management

### Bybit V5 — isolated margin, posture, keys
- **Isolated margin (UTA):** loss capped at the position's margin; **no auto-transfer from available balance** unless Auto-Margin Replenishment (auto-add-margin) is on. → https://www.bybit.com/en/help-center/article/UTA-Trading-Rules · https://medium.com/bybit/what-is-the-isolated-margin-mode-how-to-add-auto-margin-replenishment-to-a-position-3d9bd61ccf2
- **Add/reduce margin:** `POST /v5/position/add-margin` (doc slug `manual-add-margin`; **path is `add-margin`** — a known slug/path mismatch). `margin` positive adds, negative reduces. → https://bybit-exchange.github.io/docs/v5/position/manual-add-margin
- **Auto-add-margin:** `POST /v5/position/set-auto-add-margin` (`autoAddMargin` 0=off). → https://bybit-exchange.github.io/docs/v5/position/auto-add-margin
- **Position mode:** read `GET /v5/position/list` `positionIdx` (0=one-way); set `POST /v5/position/switch-mode` (0=one-way, 3=hedge — enum differs from `positionIdx`). Isolated/cross must be read from **`GET /v5/account/info` `marginMode`**, NOT the deprecated position `tradeMode` (always 0). → https://bybit-exchange.github.io/docs/v5/position/position-mode
- **create-order ack:** the response is an **async accept** (returns `orderId`/`orderLinkId`); status/fills must be confirmed via the private WS — *"This request is asynchronous so please use the websocket to confirm the order status."* → https://bybit-exchange.github.io/docs/v5/order/create-order
- **Spot TP/SL + IOC:** spot **Limit** orders support attached TP/SL; spot supports IOC; **combining attached-TP/SL with an IOC spot order is doc-silent → integration-test item** (neither confirmed nor denied). → https://bybit-exchange.github.io/docs/v5/order/create-order
- **Sub-key fence:** modify `POST /v5/user/update-sub-api` (slug `modify-sub-apikey`), delete `POST /v5/user/delete-sub-api`. **Requires an authority holding ONE OF `Account Transfer` / `Sub Member Transfer` / `Withdrawal`** (a sub key may modify itself; a master key may manage its sub) — a **high-privilege, transfer/withdrawal-class** credential. → https://bybit-exchange.github.io/docs/v5/user/modify-sub-apikey · https://bybit-exchange.github.io/docs/v5/user/rm-sub-apikey

### Bybit V5 — forced-close evidence (verified 2026-07-20, official docs fetched verbatim) `[→ R1d-7]`
- **No `closeType` field exists** on the private order / execution / position streams — the v0.7.1 quirk-matrix row claiming it was wrong. → https://bybit-exchange.github.io/docs/v5/websocket/private/order · https://bybit-exchange.github.io/docs/v5/websocket/private/execution · https://bybit-exchange.github.io/docs/v5/websocket/private/position
- **Execution stream (authoritative):** `execType` enum — `Trade`, `AdlTrade` ("Auto-Deleveraging"), `Funding`, `BustTrade` ("Takeover liquidation"), `Delivery`, `Settle`, `BlockTrade`, `MovePosition`, … ; `execId` (the fill dedup key); `seq` — "Cross sequence, used to associate each fill and each position update … use seq + symbol to check unique". → https://bybit-exchange.github.io/docs/v5/enum · https://bybit-exchange.github.io/docs/v5/websocket/private/execution
- **Order stream (corroboration):** `createType` incl. `CreateByLiq` ("laddered liquidation"), `CreateByTakeOver_PassThrough` ("closed at bankruptcy price"), `CreateByAdl_PassThrough` ("Auto-Deleveraging(ADL)"). **Position stream (corroboration):** `positionStatus` = `Normal` / `Liq` / `Adl`; `adlRankIndicator`. → (order/position pages + enum, above)
- **Recovery:** `GET /v5/execution/list` **accepts an `execType` request param** and returns `execType` + `execId`; `GET /v5/position/close-pnl` carries a **reduced** `execType` set (`Trade`, `BustTrade`, `SessionSettlePnL`, `Settle`, `MovePosition`) — it can flag a liquidation but **not ADL**; use `execution/list` for ADL. → https://bybit-exchange.github.io/docs/v5/order/execution · https://bybit-exchange.github.io/docs/v5/position/close-pnl
- **No cross-topic arrival-ordering guarantee is documented** for order/execution/position; `seq`+symbol is the documented fill↔position correlation key, never an assumed arrival order.

### Binance USDⓈ-M — forced-close evidence (corroborated 2026-07-20 — ⚠ not verbatim: `developers.binance.com` is a client-rendered SPA; **browser-confirm before implementation**) `[→ R1d-7]`
- Liquidation/ADL closes surface on `ORDER_TRADE_UPDATE` via **system `clientOrderId` prefixes**: `autoclose-` (liquidation), `adl_autoclose` (ADL) (+ `settlement_autoclose-` for delisting/settlement). → https://developers.binance.com/docs/derivatives/usds-margined-futures/user-data-streams/Event-Order-Update
- REST recovery: **`GET /fapi/v1/forceOrders`** ("Query User's Force Orders") with **`autoCloseType` ∈ {`LIQUIDATION`, `ADL`}** — the explicit liq-vs-ADL distinguisher. → https://developers.binance.com/docs/derivatives/usds-margined-futures/trade/rest-api/Users-Force-Orders
- `ACCOUNT_UPDATE` event-reason (`m`) reportedly includes `ADL`; `MARGIN_CALL` is a *warning*, not a close confirmation — **both unverified here; confirm in-browser**.
- **History retention (recovery-relevant, `[→ Cx5-3]`):** order + algo-order history ≈**90 days** (some unfilled-cancelled orders purged in ~3 days); trade (execution) history ≈**6 months**; **no server-side client-ID-prefix filter** — prefix matching is client-side over enumerated pages. → https://developers.binance.com/en/docs/catalog/core-trading-derivatives-trading-usd-s-m-futures/api/rest-api/trade *(corroborated 2026-07-20 — browser-confirm exact windows)*.

### Margin-op & transfer resolution evidence (verified 2026-07-20; Binance response bodies corroborated, not verbatim — SPA) `[→ R1d-6]`
- **Bybit add/reduce margin returns only a position snapshot — no operation/transaction id**: `POST /v5/position/add-margin` response = `category, symbol, positionIdx, riskId, …, positionIM, positionMM, …, updatedTime` (no op id; the only resolution signal is the **`positionIM` delta** vs a before-state). → https://bybit-exchange.github.io/docs/v5/position/manual-add-margin
- **Bybit transaction-log has NO margin-adjustment type**: `GET /v5/account/transaction-log` records carry a unique `id`, but the ~100-value `type` enum (`TRANSFER_IN/OUT`, `TRADE`, `SETTLEMENT`, `LIQUIDATION`, `ADL`, …) contains **no value for a manual position-margin add/reduce** — whether a top-up emits a record at all is **undocumented; MUST NOT be relied on**. → https://bybit-exchange.github.io/docs/v5/account/transaction-log · https://bybit-exchange.github.io/docs/v5/enum
- **Binance margin-change history exists but records have no unique id**: `GET /fapi/v1/positionMargin/history` (`symbol`; `type` 1=add / 2=reduce; `startTime`/`endTime`/`limit` default 500) → rows `{amount, asset, symbol, time, type, positionSide}` — resolution matches on **symbol + type + amount + time window**, never an id. `POST /fapi/v1/positionMargin` responds `{amount, code, msg, type}` — **no op id**. → https://developers.binance.com/docs/derivatives/usds-margined-futures/trade/rest-api/Get-Position-Margin-Change-History · https://developers.binance.com/docs/derivatives/usds-margined-futures/trade/rest-api/Modify-Isolated-Position-Margin
- **Bybit transfers have a CLIENT-SUPPLIED idempotency key**: `POST /v5/asset/transfer/inter-transfer` **requires a client-generated `transferId` UUID** ("Please manually generate a UUID"); `GET /v5/asset/transfer/query-inter-transfer-list` queries **by that UUID** (`status`: `STATUS_UNKNOWN`/`SUCCESS`/`PENDING`/`FAILED`) — a timed-out create is idempotently resolvable, and a same-UUID retry is safe. → https://bybit-exchange.github.io/docs/v5/asset/transfer/create-inter-transfer · https://bybit-exchange.github.io/docs/v5/asset/transfer/inter-transfer-list
- **Binance transfers have NO client-supplied id**: `POST /sapi/v1/asset/transfer` returns a **server-generated** `tranId` only; resolution of a timed-out create = `GET /sapi/v1/asset/transfer` (keyed by required `type` + time window; `status` `CONFIRMED`/`PENDING`/`FAILED`) **before any retry** — a naive retry is not idempotent and can **double-transfer**. → https://developers.binance.com/docs/wallet/asset/user-universal-transfer

### SQLite durability (Linux target)
- **`PRAGMA fullfsync` is macOS-only** (`F_FULLFSYNC`); a **no-op on Linux**. → https://sqlite.org/pragma.html#pragma_fullfsync
- **WAL durability:** `synchronous=NORMAL` in WAL is durable across an *app* crash but **may lose the last transaction(s) on power loss**; **`synchronous=FULL` fsyncs the WAL on every commit** → power-loss durable. → https://sqlite.org/pragma.html#pragma_synchronous · https://sqlite.org/wal.html
- **Durability ≠ checkpoint** (a committed txn is durable in the fsync'd WAL before checkpoint; `wal_autocheckpoint` default 1000 pages). SQLite assumes the OS/hardware honor fsync; **network filesystems (NFS) are documented as corruption-prone** → local ext4/xfs with barriers only. → https://sqlite.org/atomiccommit.html · https://sqlite.org/howtocorrupt.html
- **Production (Linux):** `journal_mode=WAL; synchronous=FULL;` local fs; boot-verify pragmas.

### Bybit V5 — per-endpoint API rate limits (verified 2026-08-04, official docs fetched verbatim) `[task #185, record §66]`
- **The meter:** verbatim *"The API rate limit is based on the rolling time window per second and UID."* → https://bybit-exchange.github.io/docs/v5/rate-limit
- **The verified table (per UID, per second; the create rows are marked upgradable per account):** `POST /v5/order/create` **10/s** (inverse, linear, option) and **20/s** (spot); `GET /v5/order/realtime`, `GET /v5/order/history`, `GET /v5/execution/list` **50/s** (all categories); `GET /v5/position/list` **50/s** (inverse, linear, option — the table has **no spot cell** for this surface).
- **Batch consumption formula, verbatim:** *"Number of consumption = number of requests * number of orders included in a single request"* (batch APIs; a plain create consumes 1). This is why `order_create`'s local budget counts ORDERS: the venue's own formula counts orders carried, so `Cost{Orders: N}` rides the existing units rule with no special case.
- **Configuration independence, verbatim:** *"the configuration of business lines is independent of each other"* (context: the trade-limit consumption note; the rate-limit table itself has per-category columns).
- **⚠ CITATION CAVEAT:** the page's *"counted separately for Spot, Linear, Inverse, and Options markets"* sentence is about **WEBSOCKET CONNECTION limits**, not API request budgets — it must NOT be cited for per-category API budget separation.
- **Named assumption (deliberately not relied on in either direction):** whether a spot call and a linear call to the SAME endpoint decrement different counters is an **inference** from the configuration-independence quote plus the per-category table columns — no sentence states it for API budgets. The local model is therefore ONE venue-level shared window per endpoint (`ratelimit.ScopeEndpoint`, family refused): exact if the venue shares the counter, slightly conservative if it splits it. **#171's testnet measurement is a LOOSENING trigger** (split per family for throughput if separation is measured), never a correctness prerequisite. Where the venue's caps are unequal per category (create), the shared window's ceilings are stated from the MINIMUM across the categories the profile enables — operator arithmetic, normative in core §7.6.

### Spot balance surfaces — the two balance endpoints (verified 2026-08-05, official docs fetched verbatim) `[task #179, record §70]`

**Binance SPOT `GET /api/v3/account`** (SIGNED; source: `binance/binance-spot-api-docs` GitHub, master, fetched 2026-08-05 — the repo serves the same doc set the SPA renders, as raw markdown):
- Request weight **20** (verified against the raw markdown by exact grep — a first summarized fetch misread it as 10, and the second-model design review caught the disagreement; the number is load-bearing at the governor, which charges exactly the caller-stated cost); params `omitZeroBalances` (BOOLEAN, default **false**), `recvWindow`, `timestamp`.
- `balances[]` rows `{asset, free, locked}` with the field meanings verbatim: `free` = *"Amount not currently committed to an open order"*, `locked` = *"Amount currently committed to an open order"* — the two are documented **partitions of holdings**, so `total = free + locked` is the venue's word, not an inference.
- With `omitZeroBalances` left false, every asset comes back — a row's **absence carries no conclusion and none is needed**. The adapter leaves the parameter unset.

**Bybit `GET /v5/account/wallet-balance`** (source: `bybit-exchange.github.io/docs/v5/account/wallet-balance`, fetched 2026-08-05):
- `accountType=UNIFIED` **required**; `coin` optional, and the adapter **never passes it**: verbatim *"If not passed, it returns non-zero asset info"* — the unscoped call is the enumeration kind whose silence carries a conclusion (**absent coin = zero balance**, §9's version-is-part-of-the-requirement rule arriving as a parameter). A `coin=`-scoped call flips the absence semantics to the uninterpretable kind.
- Per-coin fields verbatim: `walletBalance` = *"Wallet balance of coin"*, `locked` = *"Locked balance due to the Spot open order"*, `borrowAmount` = spot + derivatives liabilities. `availableToWithdraw` deprecated for UNIFIED since 2025-01-09.
- **⚠ NAMED INFERENCE (not relied on):** no sentence states whether `walletBalance` includes `locked`. The equity formula (*"Asset Equity = … = walletBalance − spotBorrow + unrealisedPnl + Asset Option Value"*) never adds `locked`, which **implies** inclusion — record §70d refuses to bet: the total is computed under **both readings**, a row is pinned only where they coincide (`locked == 0`), and a `locked != 0` row is reported unpinned rather than asserted. **#171's testnet session is the loosening trigger** (measure walletBalance against a resting spot order), never a correctness prerequisite.
- Per-endpoint budget: the rate-limit page's Account table row is **`/v5/account/wallet-balance` … 50/s** (per UID, per second, same meter as §4's verified table).

## 5. Capture-source stream facts (verified 2026-07-29, official docs fetched verbatim)

**Why this section exists and why it is separate from §4.** §4 is frozen at the v0.7 review record. This
block clears the five verification debts the capture tool's design raised (design record §13) plus the
standing depth-update-ID debt open since v0.9.10, and it is dated separately so the two rounds are never
conflated.

**The method matters, because it is what unblocked three rows §4 could only mark "browser-confirm".**
`developers.binance.com/docs/…` is a client-rendered SPA and returns an empty app shell to a fetcher —
which is why §4 carries `⚠ not verbatim` on the forced-close and margin-op blocks. **There is a
server-rendered twin at `developers.binance.com/en/docs/catalog/…`** that returns the same documentation
as real text. §4 already cited one such URL without noticing it generalised. Everything below is
verbatim from that path or from Bybit's static docs; the one live-probe fact is labelled as such.

**The catalog is itself split by URL prefix**, which is corroboration rather than an inconvenience: the
`ws-streams/public` page contains **no** mark-price section, because mark price is a `/market` stream.
The page's silence is evidence for the routing split, not a failed fetch.

### Binance USDⓈ-M — websocket routing and the two decommissioned paths `[→ design §13 debt 2]`
- **Three prefixes on `wss://fstream.binance.com`:** `/public` (high-frequency public market data —
  Individual Symbol Book Ticker, Partial Book Depth, Differential Book Depth, All Book Tickers),
  `/market` (regular market data — Aggregate Trades, **Mark Price**, Klines, Tickers, Liquidations, Mini
  Tickers, Composite Index), `/private` (user data). **Legacy `wss://fstream.binance.com/ws` and
  `wss://fstream.binance.com/stream` were decommissioned 2026-04-23.** The prefixes were added to every
  stream page as a `URL PATH` section on **2026-03-05**. → https://developers.binance.com/en/docs/products/derivatives-trading-usds-futures/websocket-market-streams/Important-WebSocket-Change-Notice · https://developers.binance.com/docs/derivatives/change-log
- **Path forms:** raw `wss://fstream.binance.com/<prefix>/ws/<stream>`; combined
  `wss://fstream.binance.com/<prefix>/stream?streams=<a>/<b>`, or connect to `…/<prefix>/stream` and send
  a `SUBSCRIBE` request. → https://developers.binance.com/en/docs/catalog/core-trading-derivatives-trading-usd-s-m-futures/api/ws-streams/public
- **`wss://stream.binancefuture.com` is documented as a second base endpoint** alongside `fstream`. Not
  used, recorded so nobody re-derives it.

### Binance USDⓈ-M — the per-symbol mark-price (funding) stream `[→ design §13 debt 1]`
- **`URL PATH: wss://fstream.binance.com/market/ws/<symbol>@markPrice@<updateSpeed>`.** Stream name
  `<symbol>@markPrice@<updateSpeed>` — e.g. **`btcusdt@markPrice@1s`**; the bare `<symbol>@markPrice`
  form is the **3-second** cadence. **Update Speed: 3 s or 1 s** — there is no other cadence.
- **Payload:** `e` event type, `E` event time, `s` symbol, `p` mark price, `i` index price, `P`
  *"Estimated Settle Price, only useful in the last hour before the settlement starts"*, `r` funding
  rate, **`ap` mark price moving average**, `T` next funding time, **`st` *"(After CM migration) Symbol
  type: 1 = UM, 2 = CM"***. **`ap` and `st` were not previously recorded here.**
- **No funding-interval field on the stream** — confirmed against the field list, which is what §1's
  "Interval source" row and core §4's refuse-a-defaulted-zero rule both rest on.
- All-symbol form `!markPrice@arr@<updateSpeed>`, same 3 s / 1 s. → https://developers.binance.com/en/docs/catalog/core-trading-derivatives-trading-usd-s-m-futures/api/ws-streams/market

### Binance USDⓈ-M — book depth streams and their update-ID span `[→ design §13 debts 2 and 4]`
- **Partial Book Depth:** `<symbol>@depth<levels>@<updateSpeed>`; **levels 5, 10, 20**; speeds
  *"250ms or 500ms or 100ms"*. **Diff. Book Depth:** `<symbol>@depth@<updateSpeed>`, same three speeds.
  Both on the **`/public`** prefix.
- **Both carry the identical update-ID shape** — `U` *"First update ID in event"*, `u` *"Final update ID
  in event"*, `pu` *"Final update Id in last stream (ie `u` in last stream)"*, plus `E` event time, `T`
  transaction time, `ps` and `st` post-CM-migration. **The partial-book stream is therefore
  self-sequencing on futures**, which is *not* true of the same-named spot stream (below). →
  https://developers.binance.com/en/docs/catalog/core-trading-derivatives-trading-usd-s-m-futures/api/ws-streams/public

### Binance SPOT — book depth, and the shape that has no span at all `[→ design §13 debt 4]`
- Base endpoint **`wss://stream.binance.com:9443`** or **`wss://stream.binance.com:443`** — the spot host
  kept its own base and takes **no `/public` prefix**.
- **Partial Book Depth:** `<symbol>@depth<levels>` or `<symbol>@depth<levels>@100ms`; levels **5, 10, 20**;
  update speed **1000 ms or 100 ms** — *not* futures' 100/250/500.
- **Payload is `lastUpdateId` + `bids` + `asks`, and nothing else** — no `U`, no `u`, no `pu`, **no event
  time**. **Diff. Depth** carries `e`, `E`, `s`, `U`, `u`, `b`, `a` — a span, but **no `pu`**.
- **Consequence, stated because it is load-bearing:** a Binance spot partial-depth feed **cannot be
  gap-checked from its own wire bytes**. Ordering and loss evidence for that source must come from
  outside the payload. → https://github.com/binance/binance-spot-api-docs/blob/master/web-socket-streams.md

### Bybit V5 — public websocket URLs and the orderbook topic `[→ design §13 debts 3 and 4]`
- **Mainnet public:** `wss://stream.bybit.com/v5/public/linear` (USDT/USDC perpetual **and** futures),
  `…/v5/public/spot`, `…/v5/public/inverse`, `…/v5/public/option`, `…/v5/public/spread`,
  `…/v5/public/rfq`. **Private:** `wss://stream.bybit.com/v5/private`. **Order entry:**
  `wss://stream.bybit.com/v5/trade`. Testnet mirrors every path on `stream-testnet.bybit.com`.
- **Connection discipline:** *"Avoid establishing over 500 connections within a 5-minute window"* per
  domain; public `args` array ≤ 21 000 characters (spot: 10 args per request); client ping ≈ every
  **20 s**. → https://bybit-exchange.github.io/docs/v5/ws/connect
- **Orderbook topic `orderbook.<depth>.<symbol>`.** Linear depth/push-frequency: **1 → 10 ms, 50 → 20 ms,
  200 → 100 ms, 1000 → 200 ms**. Fields: `type` ∈ {`snapshot`, `delta`}, **`u` update id — *"resets to 1
  during service restarts"***, **`seq` cross sequence**, `b`/`a` as `[price, size]`. **Size 0 means
  delete the entry; a new snapshot means reset the local book.** Level-1 repeats its snapshot every 3 s
  with an unchanged `u`. → https://bybit-exchange.github.io/docs/v5/websocket/public/orderbook
- **Orderbook JSON value types, observed from our own capture (2026-08-01, LABUSDT linear + BTCUSDT
  spot frames, config `1c362feffe4a`):** the envelope is `{topic, type, ts, data, cts}` with `ts`/`cts`
  epoch-ms JSON **numbers**; `data` is `{s, b, a, u, seq}` with `u` and `seq` JSON **numbers** — unlike
  the tickers topic, whose funding fields are all strings — and `b`/`a` as `[price, size]` **string**
  pairs. A delta may carry an empty side array; a live delete (`["63052","0"]`) was observed in the
  BTCUSDT spot stream. Snapshot depth on both products: 50 levels per side, bids descending, asks
  ascending. *(own-archive observation, not a doc quote — labelled as such deliberately)*
- **Orderbook `u` advances by EXACTLY 1 per delta within a connection, observed from our own capture
  (2026-08-01, all 10 Bybit book streams then recorded — BTCUSDT linear + spot across 47 hour-files
  each, 8 linear alts):** of 8,463,452 deltas, every one with an in-connection predecessor had
  `u = prev + 1` — zero larger jumps, zero backward or repeated `u`. Snapshots appeared **only** as a
  subscribe/reconnect's first frame (zero mid-stream snapshots), and `seq` was strictly increasing
  across every consecutive emitted pair. **Consequence the crossvenue accumulator builds on:** a delta
  with `u ≠ prev + 1` is a sequence break — re-anchor at the next snapshot, never absorb; monotonic-but-
  gapped `u` is NOT a healthy feed. The docs do not state contiguity; this is a measurement, and a
  re-measurement that finds a legitimate jump shape must land here before any consumer absorbs it.
  *(own-archive observation, not a doc quote — labelled as such deliberately)*

### The two venues' depth update-ID shapes — the standing debt, and it is THREE shapes, not two `[→ standing debt #1, CLEARED]`
The debt was recorded as *"the two venues' depth update-ID shapes"*. That framing is wrong, and the
correction is the finding:

| Feed | Shape | Gap-detectable from its own bytes? |
|---|---|---|
| Binance **futures** partial **and** diff depth | `U` … `u` span **+ `pu`** chain link | **Yes** — `pu` ≠ previous `u` is a gap |
| Binance **spot** diff depth | `U` … `u` span, **no `pu`** | Only by comparing consecutive events |
| Binance **spot** partial depth | **`lastUpdateId` only** | **No** |
| Bybit linear **and** spot orderbook | single `u` (**resets to 1 on service restart**) + `seq` | Only by `u` monotonicity, and a restart breaks it |

**Binance is not one venue for this purpose** — it is a futures shape and two different spot shapes, and
the split is by *product and stream type*, not by venue. This is what core §4's *"a book delta payload
carries `FirstSeq` and never `LastSeq`"* has to absorb: a single-sequence feed sets `FirstSeq` from its
one number, and the spot-partial feed has no second number to lose.

### REST overlay endpoints — public vs authenticated `[→ design §13 debt 5]`
- **Binance security types, verbatim:** `NONE` — *"Endpoint can be accessed freely."*; `TRADE` and
  `USER_DATA` — *"requires sending a valid API-Key and signature"* (both are `SIGNED`); `USER_STREAM` and
  `MARKET_DATA` — *"requires sending a valid API-Key"* (key, no signature). → https://developers.binance.com/docs/derivatives/usds-margined-futures/general-info
- **`GET /fapi/v1/fundingInfo` is PUBLIC.** The catalog page marks it with no security type, and a
  **live unauthenticated probe on 2026-07-29 returned HTTP 200 with a full body** *(live probe, not a doc
  quote — labelled as such deliberately)*. Description: *"Query funding rate info for symbols that had
  FundingRateCap/FundingRateFloor / fundingIntervalHours adjustment"*. **Weight 0**, sharing a
  **500 / 5 min / IP** limit with `GET /fapi/v1/fundingRate`. → https://developers.binance.com/en/docs/catalog/core-trading-derivatives-trading-usd-s-m-futures/api/rest-api/market-data
- **`GET /fapi/v1/leverageBracket` is `USER_DATA`** (signed) — IP weight 1. **`GET /v5/account/fee-rate`
  requires Bybit authentication** (`X-BAPI-SIGN` / `X-BAPI-API-KEY` / `X-BAPI-TIMESTAMP` /
  `X-BAPI-RECV-WINDOW`); required param `category`, optional `symbol`/`baseCoin`. Both are on the
  design's **deferred** overlay list, and their being credential-bearing is exactly the deferral's first
  stated ground. → https://developers.binance.com/en/docs/catalog/core-trading-derivatives-trading-usd-s-m-futures/api/rest-api/account · https://bybit-exchange.github.io/docs/v5/account/fee-rate
- **`GET /fapi/v1/commissionRate`'s security type is NOT verified** — the catalog page truncated before
  its row. **Not asserted.** It is deferred anyway, so nothing built depends on it; recorded here so the
  gap is visible rather than assumed closed with its neighbours.

### The `fundingInfo` overlay, observed live — three facts that were not what the design assumed
A single unauthenticated `GET https://fapi.binance.com/fapi/v1/fundingInfo` on **2026-07-29** returned
**742 rows** of `{symbol, adjustedFundingRateCap, adjustedFundingRateFloor, fundingIntervalHours,
disclaimer, updateTime}`. *(Live observation. It is a measurement, not a documented guarantee, and
nothing may be built on it that a re-measurement could invalidate.)*

1. **BTCUSDT is PRESENT, not absent.** `{"symbol":"BTCUSDT", "adjustedFundingRateCap":"0.00300",
   "adjustedFundingRateFloor":"-0.00300", "fundingIntervalHours":8, "disclaimer":true,
   "updateTime":null}` — identical for ETHUSDT. The cap **confirms §1's ±0.3% row from a second
   direction**, and the interval is 8 h. But the *"absent ⇒ 8 h default"* branch **does not fire for our
   one traded symbol**; the overlay answers positively.
2. **8 h is not the common case.** `fundingIntervalHours` across the 742: **4 h × 442, 8 h × 296,
   1 h × 4.** §1 records no 4 h interval at all. "Absent ⇒ 8 h" is a statement about the *unlisted*
   symbols, and the majority of *listed* ones are 4 h — so the default is safe only because BTCUSDT is
   listed and says 8.
3. **`updateTime` is `null` on BTCUSDT** while other rows carry an epoch-ms integer. A decoder typing it
   as a non-nullable integer parses 740 rows and **fails on the one symbol we trade**.

### Bybit `instruments-info` funding clamps, observed live (2026-07-30) — the floor the ticker never states

A single unauthenticated `GET https://api.bybit.com/v5/market/instruments-info?category=linear&symbol=BTCUSDT`
on **2026-07-30** returned `upperFundingRate: "0.005"` and `lowerFundingRate: "-0.005"` — **exactly
symmetric, and equal to the live ticker's `fundingCap`**. *(Live observation, dated; a re-measurement
that finds asymmetry invalidates everything resting on the mirror below.)*

1. **This is the fact the replay decoder's floor mirror rests on.** The `tickers.<sym>` stream carries
   `fundingCap` and **no floor**; the floor exists only here, and the capture set does not poll this
   endpoint. The replay decoder therefore emits `Floor = -Cap`, justified by this measurement alone —
   and the live venue clients (milestone 3) MUST replace the mirror with the polled pair rather than
   inherit it.
2. **The same response carries `fundingInterval: 480` — MINUTES.** The ticker spells its interval field
   `fundingIntervalHour` ("8", hours); this endpoint spells the same 8-hour fact as 480 minutes with the
   unit only in the docs. Core §4's units-in-the-name rule, observed in the wild on one venue's two
   spellings of one number: an adapter that reads either field into a bare integer computes a carry
   figure wrong by a factor of sixty with nothing failing.

### BTCUSDT lot/notional filters, observed live (2026-07-31) — the four `(venue, product)` cells `[→ design §40g, step 3 slice A]`

Four unauthenticated probes on **2026-07-31 ~05:09 UTC** (Bybit response `time` 1785474562995), one per
cell of the milestone-2 universe. *(Live observations, dated; these are the rows the committed
milestone-2 reference revision cites, and step 5's period-correct archive replaces them the day a
re-measurement disagrees.)*

| Cell | Endpoint | Step / min qty | Max qty | Min notional |
|---|---|---|---|---|
| Binance SPOT | `GET /api/v3/exchangeInfo?symbol=BTCUSDT` | `LOT_SIZE` stepSize **0.00001**, minQty **0.00001** | 9000 | `NOTIONAL` minNotional **5** (USDT, `applyMinToMarket: true`) |
| Binance USDⓈ-M perp | `GET /fapi/v1/exchangeInfo` → BTCUSDT | `LOT_SIZE` stepSize **0.001**, minQty **0.001** | 1000 (market: 120) | `MIN_NOTIONAL` **50** |
| Bybit SPOT | `GET /v5/market/instruments-info?category=spot&symbol=BTCUSDT` | basePrecision **0.000001**, minOrderQty **0.000001** | 230 (market: 120) | minOrderAmt **5** (USDT) |
| Bybit linear perp | `GET /v5/market/instruments-info?category=linear&symbol=BTCUSDT` | qtyStep **0.001**, minOrderQty **0.001** | 1500 (market: 150) | minNotionalValue **5** |

1. **The two perps agree on 0.001 BTC steps; the two spots do not agree with each other** (10⁻⁵ vs
   10⁻⁶). §2.4's intersection rule therefore binds at the PERP step for either venue's package — a
   quantization that used the spot step would emit perp-unroutable quantities a thousandth of a BTC at
   a time.
2. **Min-notional is the binding floor on Binance's perp** (50 USDT ≈ 0.0008 BTC at ~65k, above the
   0.001 lot floor's ~65 USDT — close enough to flip as price moves; the sizing must check both).
3. **Bybit spot's `quotePrecision` (0.0000001) is finer than fixedpoint's scale-9 tick only in
   appearance** — it is a PRECISION, not a step; the order-quantity constraint is `basePrecision`, and
   nothing here requires sub-scale-9 arithmetic.

### BTCUSDT tick sizes, observed live (2026-08-03) — the PRICE half of the trading filters `[→ task #182]`

Four unauthenticated probes on **2026-08-03 ~18:45 UTC** (Bybit response `time` 1785782725618), the
same four cells. The 2026-07-31 table above records the QUANTITY filters and no tick; every planned
order is a marketable-IOC **limit** (§6 step 6), so the tick binds every order this system sends, and
the order path's §2.4 verification consumes these beside the lot rows. *(Live observations, dated;
the committed reference revision's `tick` members cite this block.)*

| Cell | Filter read | tickSize |
|---|---|---|
| Binance SPOT | `PRICE_FILTER` (minPrice 0.01, maxPrice 1000000) | **0.01** |
| Binance USDⓈ-M perp | `PRICE_FILTER` (minPrice 556.80, maxPrice 4529764) | **0.10** |
| Bybit SPOT | `priceFilter` (tickSize only — no min/max published on spot) | **0.1** |
| Bybit linear perp | `priceFilter` (minPrice 0.10, maxPrice 1999999.80) | **0.10** |

1. **The two spots disagree on tick by 10× (0.01 vs 0.1), exactly as they disagree on lot step** — so
   a limit price valid on Binance spot (50000.05) is off-grid on every other cell, which is the price
   axis of the asymmetric-rejection hazard §2.4's intersection rule exists for on the quantity axis.
2. **Every 2026-07-31 lot/notional value was re-read in the same four responses and is unchanged** —
   steps, min quantities, max quantities and all four min-notional spellings (including the Binance
   perp's 50 USDT floor, whose JSON field is spelled `notional`, a **fourth** spelling of
   min-notional across the four cells).
3. **`GET /fapi/v1/exchangeInfo?symbol=BTCUSDT` IGNORES the symbol parameter** — the response was the
   full ~1.0 MB roster (spot's same-shaped query returned 5.3 KB, honoring it). A fetcher that pages
   the futures filters per symbol pays the full-document cost every call and should not be written to
   expect a narrowed body.
4. **Binance spot's `MARKET_LOT_SIZE` carried `stepSize "0.00000000"`** — a live zero step on a
   filter that does not apply to limit orders. Two consequences, both banked: the v0.9.23 note that
   LOT_SIZE and MARKET_LOT_SIZE "cannot be distinguished by data that agrees" no longer holds — on
   this read they DIFFER (1e-5 vs 0), so a reader that consulted the wrong filter now has data that
   convicts it — and a zero step arrives from real venue output, not only from a forgotten field,
   which is why `fixedpoint.ErrStepNotPositive` refuses it rather than reading it as "no constraint".

**2026-08-05 re-probe (task #183, the fetcher's own read of all four cells):** every value in the two
tables above re-confirmed unchanged. Two payload-shape facts banked for the fetcher, both from the
live bodies: **Bybit's lot fields live under a `lotSizeFilter` parent object and the tick under
`priceFilter`, on BOTH categories** (spot: `basePrecision`/`minOrderQty`/`minOrderAmt`; linear:
`qtyStep`/`minOrderQty`/`minNotionalValue` — the per-category field split the 2026-07-31 block
records, now with the parent keys pinned); and the venues PAD decimals differently (spot
`"0.00001000"`, fapi `"0.001"`, Bybit mixed `"0.1"`/`"0.10"`), so a fetcher storing venue words
verbatim writes strings that are numerically equal but textually distinct from the hand-recorded
rows — validation is by conversion, and goldens must carry the padded forms.

### Base-tier fee tables, observed live (2026-07-31, rendered pages) — the four fee cells `[→ design §40d, step 3 slice A]`

Read from the two venues' official fee pages **rendered in a real browser on 2026-07-31** (both are
client-rendered SPAs; a plain fetcher gets an empty shell or times out — the §40d rejection of a
programmatic fee-page importer, observed while verifying). Undiscounted base columns quoted; each page
also shows a token-discount column (BNB −25%/−10%, MNT −25%/−10%) which is NOT the base rate and is
recorded here only so a later reader knows which column the numbers came from.

| Cell | Tier row | Maker | Taker | Source |
|---|---|---|---|---|
| Binance SPOT (USDT pairs) | Regular User | **0.100%** | **0.100%** | https://www.binance.com/en/fee/trading — "Spot & Margin" tab |
| Binance USDⓈ-M perp (USDT pairs) | Regular User | **0.0200%** | **0.0500%** | https://www.binance.com/en/fee/futureFee — "USDⓈ-M Futures" tab |
| Bybit SPOT (crypto-crypto) | Non-VIP | **0.1000%** | **0.1000%** | https://www.bybit.com/en/announcement-info/fee-rate/ — "Spot" tab |
| Bybit linear perp | Non-VIP | **0.0200%** | **0.0550%** | same page — "Perpetual & Futures" tab |

1. **§2.4's spot-vs-perp premise holds at both venues**: spot taker is 2× (Bybit ~1.8×) the perp
   taker, so reusing the perp round-trip figure for cash-and-carry understates cost by roughly half —
   the exact trap the sentence names.
2. **These are the modeled-tier (VIP0/non-VIP) PUBLISHED rows, not any account's fees.** §40d's
   two-clause §2.4 edit authorizes scoring the pre-capital gate on them, labeled; account-real
   authenticated revisions (`/v5/account/fee-rate`, auth — §4 above) remain a precondition of any
   live order. VIP0 is the worst published tier, so the proxy errs against the gate.
3. **Bybit's page states a caveat worth carrying**: "Actual fees charged depend on the displayed fee
   rates applicable to your account after login" — one more reason the provenance class of these rows
   is `hand_recorded`, never `fetched`.

### Family-1 REST history endpoints, observed live (2026-08-01) — the slice B fetch facts `[→ design §41c/§41f, step 4 slice B]`

Verified by the `arblab calibrate fetch` runs that committed the four `datasets/` files (unauthenticated
GETs from this host, pages stored verbatim in the files — the observations below are re-derivable from
the committed bytes):

| Series | Endpoint | Page limit | Shape facts |
|---|---|---|---|
| Binance funding history | `GET https://fapi.binance.com/fapi/v1/fundingRate?symbol=&startTime=&endTime=&limit=` | 1000 | rows ASCENDING; `fundingTime` ms epoch **with millisecond jitter** (observed `1672560000008`, 8 ms past the 08:00 grid instant); `fundingRate` decimal string; `markPrice` empty on old rows; serves 2023-01-01 onward with a complete 8h grid through 2026-01-31 |
| Bybit funding history | `GET https://api.bybit.com/v5/market/funding/history?category=linear&symbol=&startTime=&endTime=&limit=` | 200 | V5 envelope (`retCode`/`result.list`); rows **NEWEST-FIRST**; `fundingRateTimestamp` is a STRING of ms; grid exact (no jitter observed); complete 8h grid 2023-01-01 → 2026-01-31 |
| Binance spot klines | `GET https://api.binance.com/api/v3/klines?symbol=&interval=1h&startTime=&endTime=&limit=` | 1000 | array-of-arrays, index 0 open-time ms, index 1 open-price string; **2023-03-24T13:00Z DOES NOT EXIST** (that day's spot halt — the completeness refusal fired live on the full range, so the committed kline era starts 2023-04-01); complete thereafter |
| Binance mark-price klines | `GET https://fapi.binance.com/fapi/v1/markPriceKlines?symbol=&interval=1h&startTime=&endTime=&limit=` | 1500 (1000 used) | same array shape, volume fields zero; **no 2023-03-24 hole** (perp mark kept printing through the spot halt) — the shared 2023-04-01 era start is the SPOT series' constraint, adopted by both because the basis derivation subtracts them pairwise |

1. **The jitter tolerance is an observed necessity, not caution**: exact-grid matching would refuse
   real Binance funding history on the 8 ms row above. `driving.GridJitterTolerance` (1 minute,
   inclusive) is 1/480th of the funding interval — no row can be ambiguous between grid points.
2. **§38f's "funding is re-fetchable" now has a measured floor**: both venues served the full
   three-year 8h grid with zero gaps. Klines are re-fetchable too, with the one spot-halt hole.

### Per-IP WebSocket connection ceilings, verified 2026-08-01 — the §42c pre-config item `[→ scoping §42c, capture expansion]`

The v0.9.20 scope note made this verification due **before** the shortlist config lands. Verified
2026-08-01 from the official docs (Binance futures via the rendered `/en/docs/products/…` path; Binance
spot via the GitHub source markdown, which cannot be an app shell; Bybit's static docs):

| Venue / domain | Connection ceiling | Streams per connection | Lifetime | Inbound msg rate |
|---|---|---|---|---|
| Binance USDⓈ-M `fstream.binance.com` | **No numeric per-IP ceiling stated.** The stated policy is behavioral: "A connection that goes beyond the limit will be disconnected; IPs that are repeatedly disconnected may be banned." | 1024 | 24h, disconnect expected at the mark | 10/s |
| Binance spot `stream.binance.com` | **300 connections per attempt every 5 minutes per IP** | 1024 | 24h | 5/s |
| Bybit V5 `stream.bybit.com` (per WebSocket **domain**, spot+linear combined) | **"Do not build over 500 connections in 5 minutes"** | spot ≤10 args per subscription request; futures/spread no args limit; 21,000-char args cap per public connection | no fixed lifetime; server cuts the connection after 10 min without ping-pong or data (docs recommend ping every 20s) | — |

→ https://developers.binance.com/en/docs/products/derivatives-trading-usds-futures/websocket-market-streams/Connect
· https://github.com/binance/binance-spot-api-docs/blob/master/web-socket-streams.md
· https://bybit-exchange.github.io/docs/v5/ws/connect

**Read against the §42c expansion (one source = one connection = one topic):** the shortlist adds
8 × 2 = 16 connections per venue's perp domain, so a full restart opens **39** concurrently —
19 `fstream.binance.com`, 1 `stream.binance.com`, 19 `stream.bybit.com` (18 linear + 1 spot). Every
documented numeric bound clears by an order of magnitude: Bybit's restart burst is 19 of 500/5min,
Binance spot's is 1 of 300/5min. The one venue with no numeric ceiling (Binance futures) bounds by
behavior, and the capture's backoff ladder (1s→60s, per source) is the mitigation it asks for —
reconnects are paced, never hot-looped. Streams-per-connection limits are untouched by construction:
the capture never multiplexes. The 24h Binance disconnect is already an observed operational fact
(reconnect + `reconnect` gap record, running since 2026-07-30). **A broad funding tier is a different
arithmetic** — 2N sockets against Bybit's 500/5min restart burst and Binance's behavioral bound —
and stays gated on its own §42c pricing, plus the multi-topic capture mode's design record if ruled
wide enough to need it.

### Book-stream "update speed" is a CEILING, not a cadence — measured live (2026-08-02) `[→ capture-expansion §44e]`

The rows above record each stream's documented **update speed** (Binance partial depth 100/250/500 ms;
Bybit linear `orderbook.50` 20 ms). Measured over **24 whole hours × 9 symbols × both venues**
(2026-08-01T12:00Z → 2026-08-02T12:00Z, 23.8M data records, verify 0 integrity), the documented figure
is the **minimum interval between pushes**, and every book stream on both venues delivers **only when
the book changed**:

| Stream | Documented | Measured delivery rate (this window) |
|---|---|---|
| Binance `<sym>@depth20@100ms` (cap 10/s) | 100 ms | **2.68/s** (SOONUSDT) … **9.98/s** (BTCUSDT spot). No symbol saturates it |
| Binance `<sym>@depth@100ms` diff (cap 10/s) | 100 ms | 9.75/s (BTCUSDT) |
| Bybit `orderbook.50.<SYM>` (cap 50/s) | 20 ms | **3.33/s** (MUSDT) … **30.53/s** (BEATUSDT); BTCUSDT 28.10/s |
| Bybit `tickers.<SYM>` | push-on-change | 0.83/s (MUSDT) … 7.11/s (BEATUSDT) |
| Binance `<sym>@markPrice@1s` | 1 s, fixed | **exactly 86,400/day** on 7 of 9 symbols — the one genuinely fixed-cadence stream |

**Verified as venue behaviour rather than local loss**, which is the claim that matters, since "we
dropped messages" and "the venue did not send" produce the same byte count. On one hour of the
quietest symbol (MUSDT `@depth20@100ms`, 6,387 records): **`pu`-chain breaks = 0** — every message's
`pu` equals its predecessor's `u`, so nothing was lost between the venue and disk — while the
**minimum observed inter-message interval is 102 ms** (the cap, honoured exactly), the median is
156 ms, p90 is 1,254 ms and the **maximum silence is 22.5 s**. The per-message update-ID span runs
from **1 to 1,297,023**, i.e. each push carries everything accumulated since the last one.

Three consequences, all outside this file's own scope but each a place the wrong reading bites:

- **Disk cost per symbol is activity on all four legs**, with no venue-imposed floor. This refuted
  §42c's derived per-symbol bound in both directions (capture-expansion §44e).
- **A stale book is not necessarily a broken one.** 94.8 s between pushes on a quiet alt is normal
  venue behaviour, so a read-path staleness rule keyed on "no update in N seconds" is making a
  liquidity judgement, not detecting a fault.
- **The venues differ at subscribe, and that asymmetry is what a first-data deadline rests on.**
  Bybit pushes a snapshot on subscribe/reconnect for both topics (§5's 2026-07-31 measured fact), so
  its first message is guaranteed; **Binance partial depth has no subscribe-time snapshot** and waits
  for the next change. Measured on the quietest shortlisted symbol, MUSDT's Binance book is silent
  longer than 10 s for 3.42% of the day (max **94.8 s**), so a 10-second first-data deadline on that
  stream is a bet on activity, not a fault detector — capture-expansion §44e sizes it at ~1 reconnect
  in 100.
- **A fixed-cadence stream is an instrument.** Because `markPrice@1s` delivers exactly 86,400/day
  when nothing is wrong, its byte or record count measures coverage directly — §44e uses it to
  price two reconnects at 103 s and 48 s of lost data inside hours the coverage metric scores
  `alive 60/60`.

→ measured from this repo's own archive (`capture verify` + `capture cat`), not from the docs; the
documented figures above are unchanged and still correct as ceilings.

## 6. Private REST surface — authentication, testnet, and what checks it (verified 2026-08-02)

**Why this section exists.** §47f's slice-C scope note states that the private-surface
facts are *"normative and already recorded (profiles §1/§4/§7)"*. **They were not, and
there is no §7 in this document** — it has ended at §5 since v0.9.x. Across the whole doc
set the only authentication fact was one line in §5 naming Bybit's four header names, and
**how either venue composes the string it signs was recorded nowhere.** That is the single
most load-bearing venue fact a signed REST client encodes. This section is the verification
the scope note assumed was banked; the citation was a pointer to a section that does not
exist, which is the tell that nobody re-derived it `[→ M3 slice C1]`.

### Binance USDⓈ-M — SIGNED endpoint composition (docs fetched 2026-08-02)

- **Header:** *"API-keys are passed into the Rest API via the `X-MBX-APIKEY` header."*
  *"API-keys and secret-keys are case sensitive."*
- **Algorithm:** *"Endpoints use `HMAC SHA256` signatures. The `HMAC SHA256 signature` is a
  keyed `HMAC SHA256` operation."* The HMAC **key is the secret key**; the HMAC **value** is
  *"the `query string` concatenated with the `request body`"* — concatenated **without a
  separator**, per the spot doc's wording of the same rule.
- **Placement:** *"Please make sure the `signature` is the end part of your `query string`
  or `request body`."* The `signature` parameter is therefore **not itself part of the
  signed payload**.
- **Timing:** `timestamp` is *"the millisecond timestamp of when the request was created and
  sent."* `recvWindow` is optional, milliseconds, *"If `recvWindow` is not sent, it defaults
  to 5000."* Maximum `recvWindow` is **60000 ms** (spot doc). Server-side test, verbatim:

  ```
  if (timestamp < serverTime + 1000 && serverTime - timestamp <= recvWindow) {
     // process request
  } else {
     // reject request
  }
  ```

- **Base endpoints:** production `https://fapi.binance.com`; **testnet
  `https://demo-fapi.binance.com`** — the *documented* host. `https://testnet.binancefuture.com`
  **also answers** (live probe 2026-08-02: both returned HTTP 200 to `GET /fapi/v1/ping`), so
  the legacy host a reader recalls from memory works, and picking it produces no failure to
  learn from. Take the documented one.
- → https://developers.binance.com/docs/derivatives/usds-margined-futures/general-info ·
  https://github.com/binance/binance-spot-api-docs/blob/master/rest-api.md

**THREE REPRODUCIBLE TEST VECTORS, all three re-computed here 2026-08-02 and matching.**
These are an *oracle*, not an illustration — the repo's oracle discipline is satisfiable
offline on this venue:

| Source | Secret | Payload | Expected signature |
|---|---|---|---|
| Futures general-info | `2b5eb11e…bb4d9` | `symbol=BTCUSDT&side=BUY&type=LIMIT&quantity=1&price=9000&timeInForce=GTC&recvWindow=5000&timestamp=1591702613943` | `3c661234138461fcc7a7d8746c6558c9842d4e10870d2ecbedf7777cad694af9` |
| Spot rest-api, ASCII | `NhqPtmdS…ATj0j` | `symbol=LTCBTC&side=BUY&type=LIMIT&timeInForce=GTC&quantity=1&price=0.1&recvWindow=5000&timestamp=1499827319559` | `c8db56825ae71d6d79447849e617115f4a920fa2acdcab2b053c4b2838bd6b71` |
| Spot rest-api, non-ASCII | (same) | `symbol=%EF%BC%91…%EF%BC%96&side=BUY&…&timestamp=1499827319559` | `e1353ec6b14d888f1164ae9af8228a3dbd508bc82eb867db8ab6046442f33ef3` |

The non-ASCII vector is the one worth keeping: it fixes that the signed bytes are the
**percent-encoded** form, so a signer that signs pre-encoding agrees with the other two
vectors and disagrees only here.

### Bybit V5 — signed request composition (docs fetched 2026-08-02)

- **Headers:** `X-BAPI-API-KEY`, `X-BAPI-TIMESTAMP` (*"UTC timestamp in milliseconds"*),
  `X-BAPI-SIGN`, and `X-BAPI-RECV-WINDOW` — **optional, defaulting to 5,000 ms**. The
  signature travels in a **header**, not in the payload; Binance puts it in the payload.
- **String to sign — and the order is the fact:**
  GET → `"timestamp + API key + recv_window + queryString"`;
  POST → `"timestamp + API key + recv_window + jsonBodyString"`.
  So Bybit signs the **raw JSON body verbatim** on POST — the bytes actually sent, not a
  re-serialization — and the api key appears **inside** the signed string as well as in its
  own header.
- **Algorithm:** *"Use the **HMAC_SHA256** or **RSA_SHA256** algorithm to sign the string"*,
  rendered as a *"lowercase HEX string for HMAC_SHA256"*.
- **Timing:** `server_time - recv_window <= timestamp < server_time + 1000`.
- **Base endpoint:** testnet `https://api-testnet.bybit.com` (live probe 2026-08-02:
  HTTP 200 to `GET /v5/market/time`).
- → https://bybit-exchange.github.io/docs/v5/guide

**THERE IS NO USABLE TEST VECTOR, and this is the asymmetry that matters.** Bybit publishes
a worked example — string to sign
`1658384314791XXXXXXXXXX5000category=option&symbol=BTC-29JUL22-25000-C`, signature
`410e0f387bafb7afd0f1722c068515e09945610124fa11774da1da857b72f30b` — but **the secret is
masked**, so the signature cannot be reproduced. The one hypothesis that would rescue it
(that the secret is the literal placeholder `XXXXXXXXXX`) was **tested and refuted**
2026-08-02: it yields `c00720f96c5934ca…`, not the published value.

### The credential-free discrimination probe — REFUTED on both venues (live, 2026-08-02)

The obvious way to check a signing composition without holding a credential is a
**differential probe**: sign a request with an invented key/secret twice, once with the
correct composition and once with a deliberately wrong one, and see whether the venue's
error distinguishes them. If it does, the venue itself is the oracle.

**It does not, on either venue** — both check the credential *before* they check the
signature:

| Venue | Correct composition | Wrong composition |
|---|---|---|
| Binance testnet `GET /fapi/v2/balance` | `{"code":-2015,"msg":"Invalid API-key, IP, or permissions for action"}` | **identical** |
| Bybit testnet `GET /v5/account/wallet-balance` | **HTTP 401, `content-length: 0`** — no `retCode`, no body | **identical** |

**The refutation is only trustworthy because Binance ran first as its positive control.**
On Bybit alone the result is two identical 401s, which is readable as *"no complaint, so the
composition is right"* — and nothing in the observation arbitrates. Running it first on the
venue whose answer is independently known (the three vectors above) is what makes this a
refutation rather than an ambiguous reading.

**Two consequences that outlive the probe:**

1. **Bybit's signer has no offline oracle and Binance's has three.** The two adapters will
   look symmetric in the code — one interface, mirrored implementations — while the
   *evidence* behind them is not symmetric at all. §47f's standing hazard ("a fix applied in
   one place gets applied to its mirror") does **not** cover this and its inverse does not
   follow from it: writing Bybit's signer the same way inherits none of Binance's assurance.
   **Validating Bybit's composition requires a real testnet credential**, and that is a named
   gap rather than an assumption.
2. **A Bybit auth failure carries no `retCode` and no body at all.** An error classifier that
   reaches into the body for `retCode` finds zero bytes on precisely the failure an
   unattended process is most likely to hit (an expired or IP-blocked key). The parse failure
   is then what classifies the request's effect — which is the §5.3 seam, reached by a path
   nobody designed.

### The acceptance window is ASYMMETRIC, and only one side is configurable

Both venues state the same server-side test, and the shape is the fact:

| Venue | Verbatim rule |
|---|---|
| Binance | `if (timestamp < serverTime + 1000 && serverTime - timestamp <= recvWindow)` |
| Bybit | `server_time - recv_window <= timestamp < server_time + 1000` |

`recvWindow` widens **only the behind side**. The ahead side is a **fixed 1000 ms**
that no parameter can widen — on either venue, and Bybit's maximum `recvWindow`
and Binance's 60000 ms ceiling are both irrelevant to it.

**Why this is a trap rather than a detail.** The symptom of a mis-set clock is the
venue rejecting requests on timing grounds, and the documented knob for that symptom
is `recvWindow`. So the natural operator response — raise `recvWindow` — repairs the
*behind* direction and leaves the *ahead* direction at 1000 ms, untouched and
unmentionable in the config. A host whose clock runs fast therefore keeps failing
intermittently while the fix looks applied and the knob looks maxed. The two
directions are not symmetric and the configuration surface implies they are.

**Consequence for `platform/clock`.** `ServerNow(venue)` carries an *uncertainty band*.
Signing is only reliable while that band fits inside the window — and because the
ahead side is fixed, the binding constraint is **`uncertainty < 1000 ms` regardless of
`recvWindow`**. A venue-imposed constant, not a tuning parameter.

**Consequence for `recvWindow` as a policy.** Absent, it defaults to 5000 ms on both
venues. That default is a *convenience* for a client that does not care, and this one
does: `recvWindow` bounds how long a **delayed non-idempotent `Place` can still
execute** after we stopped waiting for it. A large window converts requests we have
given up on into orders that fill; a small one makes the venue refuse a stale
submission, which is a clean `EffectNotApplied` instead of an `EffectUnknown` that
resolves to `Applied` later. So the safe direction is **small**, the venue's default
is larger than a trading client should want, and an unset field must be **refused**
rather than silently taking the venue's policy.

### Error semantics → `apperror.Effect`, and the venues do not document this equally

**Binance states the unknown-execution case explicitly** (USDⓈ-M general-info,
fetched 2026-08-02). On a `503`, verbatim: *"the API successfully sent the request but
not get a response within the timeout period. It is important to **NOT** treat this as
a failure operation; the execution status is **UNKNOWN** and could have been a
success."* Also verbatim: `4XX` — *"used for malformed requests; the issue is on the
sender's side"*; `5XX` — *"used for internal errors; the issue is on Binance's side"*;
`429` — *"used when breaking a request rate limit"*; `418` — *"used when an IP has been
auto-banned for continuing to send requests after receiving `429` codes."*

**Bybit states no equivalent anywhere** (V5 error-code page, fetched 2026-08-02). Its
codes are causes only — `10016` *"1. internal server error; 2. Service is restarting"*,
`10002` *"The request time exceeds the time window range"*, `10006` *"Too many visits.
Exceeded the API Rate Limit"*, `10018` *"Exceeded the IP Rate Limit"* — and **no
sentence anywhere addresses whether a failed or timed-out request may nevertheless
have executed.** → https://bybit-exchange.github.io/docs/v5/error

The asymmetry does **not** license different treatment. `EffectUnknown` is the safe
default precisely where evidence is missing, so Bybit's silence makes the conservative
classification *more* load-bearing, not less — the venue that documents the hazard is
the venue where we could in principle do better, and the one that says nothing is the
one where nothing may be inferred.

### Rate-limit response headers report OPPOSITE quantities (verified 2026-08-02)

| Venue | Header | Verbatim meaning |
|---|---|---|
| Binance | `X-MBX-USED-WEIGHT-*`, `X-MBX-ORDER-COUNT-*` | consumption so far in the window |
| Bybit | `X-Bapi-Limit` | *"your current limit for current endpoint"* |
| Bybit | `X-Bapi-Limit-Status` | *"your **remaining** requests for current endpoint"* |
| Bybit | `X-Bapi-Limit-Reset-Timestamp` | *"the timestamp indicating when your request limit resets if you have exceeded your rate_limit"* |

**Bybit reports the COMPLEMENT of what Binance reports**, and §7.6's upward-only fold
is stated over consumption. Folding `X-Bapi-Limit-Status` directly inflates the local
count when the venue says there is room and **discards the reading when the venue says
there is almost none** — the silent direction, and the one that ends in a ban. The
conversion is `used = X-Bapi-Limit − X-Bapi-Limit-Status`, and with `X-Bapi-Limit`
absent **no observation may be derived at all**. §7.6 is corrected accordingly.

Nothing in the values distinguishes the two: both are integers about the same budget,
in headers named alike, differing only in the direction the venue's prose points them.

**Also from this page (2026-08-02):** exceeding the endpoint limit returns
`"retCode": 10006, "retMsg": "Too many visits!"`. The HTTP IP limit of **600 requests
per 5 seconds** returns **"403, access too frequent"**, after which *"you must wait at
least 10 minutes for the automatic ban to be lifted"* — the page attributes **403** to
the IP limit and does **not** name `10018` there, while the error-code page does list
`10018` as *"Exceeded the IP Rate Limit"*. Both spellings are therefore treated as the
IP-ban track. → https://bybit-exchange.github.io/docs/v5/rate-limit

---

## 7. The order submission surface — create-order parameters, id rules and costs (verified 2026-08-02)

**Why this section exists.** §6 was written because §47f's slice-C scope note cited a "§7"
that did not exist. This *is* that §7, and it holds the other half of what the note claimed
was banked: §1 and §3 name the create-order **endpoints** and record nothing about their
parameters, their response shapes, their id rules or their costs. Every fact below was read
verbatim from the venue's own document on the date given, and each one is encoded in
`internal/engine/venue/order_{binance,bybit}.go` `[→ M3 slice C2a]`.

### The two venues have DIFFERENT SHAPES, and the shape is the first fact

| | Binance | Bybit |
|---|---|---|
| Create-order path | **two**: `POST /api/v3/order` (spot), `POST /fapi/v1/order` (USDⓈ-M) | **one**: `POST /v5/order/create` |
| Host | **two**: `api.binance.com`, `fapi.binance.com` | **one**: `api.bybit.com` |
| Product family selected by | the **host + path** | the **`category` body field** (`spot` / `linear`) |
| Payload | query string (signed, signature last) | JSON body (signed **verbatim**, §6) |
| Side spelling | `BUY` / `SELL` — **upper** | `Buy` / `Sell` — **title case** |

**The two shapes fail differently, and neither failure has a status code.** On Binance a
mis-route sends the order to the wrong *host*, where the same symbol is a real, executable
instrument on a different product — a leveraged short where a cash purchase was intended.
On Bybit the same mistake is a **mis-set string inside a signed body**, where nothing about
the URL looks wrong. In both cases the venue **accepts** the order. Hence the normative
rule in §4: an unconfigured or unrecognised product family is **refused, never defaulted**.

### Binance — `POST /fapi/v1/order` (USDⓈ-M) and `POST /api/v3/order` (spot)

| Parameter | Futures | Spot | Note |
|---|---|---|---|
| `symbol`, `side`, `type`, `quantity`, `price` | required | required | `type=LIMIT` + `timeInForce=IOC` is §6 step 6's marketable IOC |
| `timeInForce` | required for `LIMIT` | required for `LIMIT` | |
| `newClientOrderId` | optional, **rule published** | optional, **no rule published** | see below |
| `reduceOnly` | **futures only** | **does not exist** | *"cannot be sent in Hedge Mode"* (§1); spot has no position to reduce |
| `newOrderRespType` | `{ACK, RESULT}`, default **`ACK`** | `{ACK, RESULT, FULL}`, *"MARKET and LIMIT order types default to **FULL**"* | **the asymmetry below** |
| `recvWindow`, `timestamp` | §6 | §6 | ahead-side is a fixed 1000 ms on both |

**COST — and the futures endpoint is documented as weight ZERO.** Verbatim from the USDⓈ-M
New Order page: *"1 on 10s order rate limit(X-MBX-ORDER-COUNT-10S); 1 on 1min order rate
limit(X-MBX-ORDER-COUNT-1M); **0 on IP rate limit(x-mbx-used-weight-1m)**"*. Spot's is
*"Weight: 1"*, and spot meters order counts as well. So **the most important endpoint in
the system charges zero on the IP weight budget**, and a shared cost for "the create-order
endpoint" would be wrong on one family whichever number it named. This zero is a real venue
fact, not an unfilled field, and it collided with this repo's *zero-is-unusable* default —
`ratelimit.Cost.Validate` had to be corrected from *"weight must be non-zero"* to *"charges
no dimension a caller fills in"* `[→ M3 slice C2a]`.

**`newOrderRespType` — the enum AND the default differ between the families, and the
direction is the hazard.** The family that defaults to the **largest** response is the one
whose response we least want: a spot **LIMIT** order's default `FULL` response carries a
**`fills` array**, handing this system executions on the *submission* path — at the moment
§4 makes the private stream the single authority over fills and §5.3 has no transaction
open. They would be dropped, which is correct and is the fragile kind of correct: the data
sits in the response, one edit from a reader who thinks applying it early is an
optimization. **`ACK` is therefore stated explicitly on both families**, which converts a
discipline into an absence.

**`newClientOrderId` — the rule is published for futures ONLY, and spot inherits it as a
STATED ASSUMPTION.** Futures, verbatim: *"Can only be string following the rule:
`^[\.A-Z\:/a-z0-9_-]{1,36}$`"*. **The spot document states neither a charset nor a length
for the field.** The rule enforced on spot is therefore the *futures* rule, chosen because
it is the only published one — **not because spot was checked**. If spot in fact permits
less, this check passes an id spot rejects and the failure arrives as a venue rejection at
submission time. Recorded rather than assumed, because that is exactly the shape
`ids.ClientOrderID` carried unverified for several slices (below).

→ https://developers.binance.com/docs/derivatives/usds-margined-futures/trade/rest-api ·
https://github.com/binance/binance-spot-api-docs/blob/master/rest-api.md

### Bybit V5 — `POST /v5/order/create`

| Field | Value / rule | Note |
|---|---|---|
| `category` | `spot` \| `linear` | **selects the market**; `inverse` is served but out of scope (no sizing arithmetic here is written for coin-margined quantity) |
| `symbol`, `side`, `orderType`, `qty`, `price`, `timeInForce` | `Limit` + `IOC` | `side` is **title case** |
| `orderLinkId` | *"A max of **36** characters. Combinations of numbers, letters (upper and lower cases), dashes, and underscores are supported."* | **narrower than Binance's** |
| `reduceOnly` | perp only | |
| `isLeverage` | **`0` on every spot order — normative (§2)** | **and this is the trap** |
| Response | *"an async accept (returns `orderId`/`orderLinkId`)"* (§1) | status/fills confirmed via the private WS |

**`isLeverage=0` and `omitempty` — the value that MUST be sent is a zero.** §2 makes
`isLeverage=0` normative on every Bybit spot order: it is what keeps the spot leg an
**unborrowed cash purchase** rather than a margin position, which is the premise of the
whole *"spot cannot be seized to cover the perp loss"* property. The obvious JSON struct tag
drops exactly zero, so the field that must never be missing is the one the idiomatic
encoding silently removes — and the resulting order is a **perfectly valid margin-enabled
spot buy** that no status code objects to. The test asserts the field is **present**, not
that it decodes to `0`, because a dropped field also decodes to `0`.

**COST — Bybit publishes no per-endpoint WEIGHT.** Its trade limits are **requests per
second per endpoint**: `create-order` is **10/s**. So there is no weight to charge, and the
cost is one order slot plus the one request every acquisition charges by construction.

**`retCode 10006`'s accompanying HTTP status is NOT documented**, which is why it is
classified in **both** the status arm and the code table — see below.

→ https://bybit-exchange.github.io/docs/v5/order/create-order ·
https://bybit-exchange.github.io/docs/v5/rate-limit

### BYBIT ANSWERS A REFUSED REQUEST WITH HTTP 200 (live probes, 2026-08-02)

Verified on public testnet endpoints, **no credential involved**:

```
GET api-testnet.bybit.com/v5/market/tickers      -> HTTP 200
  {"retCode":10001,"retMsg":"Illegal category","result":{},...}
GET demo-fapi.binance.com/fapi/v1/depth?symbol=  -> HTTP 400
  {"code":-1102,"msg":"Mandatory parameter 'symbol' was not sent, ..."}
```

**The refusal lives in the BODY on Bybit and in the STATUS on Binance.** A classifier that
switches on status alone therefore reads a Bybit rejection as an acceptance — and the worse
outcome is one step further on: a Bybit refusal body carries **no `orderId`**, so the ack
decode fails, and a failed decode *after a success status* is classified `EffectApplied`,
meaning *"the order is live and our record of it is broken"*. An **explicit venue
rejection** would have been recorded as an order that **definitely exists**, collapsing
§5.3's two evidence classes — proved absence (`AbsentAfterFinality`) and explicit rejection
(`Rejected`) — into their opposite.

§6 had already named this exact mechanism one layer over, for Bybit's zero-length 401 body:
*"the parse failure is then what classifies the request's effect — which is the §5.3 seam,
reached by a path nobody designed."* **The seam was described and the second instance
landed anyway** — a hazard written down for one *input* is not thereby covered for the
others `[→ M3 slice C2a]`.

**The normative consequence.** The effect classification MUST read the body's status code
alongside the HTTP status, per venue, and the composition has a **direction**: a body may
**contradict** a success status, but it may **never soften** what a failure status
established. A 5xx that classified `EffectUnknown` keeps that reading even where the body's
code documents a clean refusal — a venue that could not answer is a venue whose account of
what it did is the thing in doubt. **An error body beside a success status with no venue
rule for its code is `EffectUnknown`**, not an acceptance.

**Bybit's code table is default-deny**, and the cost of that is the point:

| `retCode` | Classified | Source |
|---|---|---|
| `10001`, `10002` | **`NotApplied`** — the venue parsed the request and refused it | error page (§6) |
| `10006` | rate limit → throttle | *"Too many visits!"*; **HTTP status undocumented** |
| `10018` | IP rate limit → ban track | §6 |
| `10016` | **`Unknown`** — Bybit's analogue of the 5xx Binance documents | *"internal server error; Service is restarting"* |
| everything else | **`EffectUnknown`** | **no Bybit document anywhere states whether a failed request may have executed** (§6) |

So an ordinary *"insufficient balance"* costs a resolving query and a trading hold. That is
the right way round, and the cost is the point: **the other default is silent when it is
wrong.** On Binance the mirror rule is narrower — only a **negative** `code` is read as an
error, because non-negative `code` fields appear on Binance responses that mean success.

### Testnet base hosts — all three probed alive

| Family | Host | 2026-08-02 | 2026-08-03 |
|---|---|---|---|
| Binance USDⓈ-M | `demo-fapi.binance.com` | HTTP 200 `/fapi/v1/ping` | HTTP 200 |
| **Binance SPOT** | **`testnet.binance.vision`** | HTTP 200 `/api/v3/ping` | HTTP 200 |
| Bybit (both) | `api-testnet.bybit.com` | HTTP 200 `/v5/market/time` | HTTP 200 |

**`testnet.binance.vision` was recorded nowhere in this doc set** before this section. §6
recorded only the futures testnet host, which is the half a *futures*-shaped reading of the
system notices is missing — and the spot leg is half of every cash-and-carry trade. The
legacy `testnet.binancefuture.com` also answers (§6): the host a reader recalls from memory
works, so picking it produces no failure to learn from. Take the documented one.

### The `ClientOrderID` budget is CLOSED, and the two rules are not the same

`ids.ClientOrderID` carried this since it was written: *"the 36-character budget and the
`[A-Za-z0-9_-]` character set this format fits inside are NOT recorded in
venue-profiles.md … Confirm before milestone 3 places a real order."* **Both rules are now
verified above**, and the minted format is **35 characters of `[a-zA-Z0-9-]`** — inside
both, **with one character of headroom**.

**Binance's class is strictly wider than Bybit's**: `.`, `:` and `/` are legal on Binance
and illegal on Bybit. That asymmetry is what makes the per-venue checks *testable* — a
fixture using only ids both venues accept cannot distinguish two correct per-venue checks
from one shared check written to whichever rule the author happened to remember. A shared
check would have to be the intersection, and would then reject an id one venue accepts for
a reason belonging to the other.

---

## 8. The order READ surface — resolution, executions, retention (verified 2026-08-03)

**Why this section exists.** §7 recorded the *submission* surface; this is the half §5.3's
recovery and §6.7's finality contract actually consume — `OrderByClientID` and the
executions sweep. Every fact below was read verbatim from the venue's own document on
2026-08-03 and is encoded in `internal/engine/venue/orderquery_{binance,bybit}.go`
`[→ M3 slice C2b]`. **One fact is an assumption and is labelled as one**, at the end.

### The surfaces, and there are FIVE of them for two venues

| | Order resolution | Executions |
|---|---|---|
| Binance USDⓈ-M | `GET /fapi/v1/order` | `GET /fapi/v1/userTrades` |
| Binance spot | `GET /api/v3/order` | `GET /api/v3/myTrades` |
| Bybit | `GET /v5/order/realtime` **→ `GET /v5/order/history`** | `GET /v5/execution/list` |

Bybit is the venue with **two** order surfaces, and §5.3 already made the fallback normative:
realtime holds live orders plus, verbatim, *"the most recent 500 orders"* of the closed ones,
and *"after a server release or restart, filled, cancelled, and rejected orders of Unified
account should only be queried through order history"*. **An absence from realtime is also
what a Bybit deploy looks like**, so it is not an absence.

### THE FOUR VALUES THAT DIFFER BETWEEN BINANCE'S OWN TWO FAMILIES

| | USDⓈ-M | spot |
|---|---|---|
| Order-status weight | *"IP Weight 1"* | *"Weight: 4"* |
| Executions weight | *"Weight: 5"* | *"Without `orderId` 20 · With `orderId` 5"* |
| **Executions window cap** | ***"cannot be longer than 7 days"*** | ***"can't be longer than 24 hours"*** |
| Order retention | published (below) | **not published** |

This is §7's `newOrderRespType` finding on four more parameters: *the same venue, the other
family, the value spelled identically*. **The window cap is the one that fails silently.** A
sweeper written once against "the 7-day window" is wrong on spot by a **factor of seven**,
and the failure is not a refusal — it is a truncated sweep that reports success, i.e. a
**false watermark**, which is the quantity §6.7 step 3 collapses a possible-delta interval
on. The codecs therefore **refuse a too-wide window rather than truncating it**, and chunking
is the caller's.

### `fromId` and the time window are MUTUALLY EXCLUSIVE — on both Binance families

Futures says it directly: *"The parameter `fromId` cannot be sent with `startTime` or
`endTime`."* Spot says it by omission — it publishes a **whitelist** of supported parameter
combinations (`symbol`; `symbol`+`orderId`; `symbol`+`startTime`; `symbol`+`endTime`;
`symbol`+`fromId`; `symbol`+`startTime`+`endTime`; `symbol`+`orderId`+`fromId`) in which **no
`fromId`+time combination appears**. So a sweep bounded by a §6.7 watermark **cannot also
resume by trade id** on either family. This is what forces §4's `Cursor` to be a *server
instant* rather than a venue resume handle: the two cannot both be sent, and only one of them
can be compared to a horizon.

### Executions: the resume handles point in DIFFERENT DIRECTIONS

| Surface | Resume handle | Sort order | Comparable to a time? |
|---|---|---|---|
| Binance USDⓈ-M `userTrades` | `fromId` (trade id) | ascending | **no** |
| Binance spot `myTrades` | `fromId` — *"it will get trades >= that `fromId`"*, **inclusive** | ascending | **no** |
| Bybit `execution/list` | `nextPageCursor` (opaque) | ***"sorted by `execTime` in descending order"*** | **no** |

**Bybit's "next page" walks into OLDER data.** A `since` is forward in time; a
`nextPageCursor` on a descending list is backward. The two are spelled the same way — a
string you hand back to the next call — and nothing in either one's *type* records its
direction. Storing a `nextPageCursor` where a `since` belongs resumes at the **oldest** record
of the previous sweep and re-reads that window forever, **while every request succeeds, every
page decodes, and the fill ledger's dedup silently absorbs the duplicates**.

**`execTime` is NOT unique, and Bybit says so:** when `execTime` matches, *"it is recommended
to sort according to `execId+OrderId+leavesQty`"*. A time-only watermark therefore either
drops the tied remainder or re-reads it — and §5.1's dedup key
(`UNIQUE(account, venue, product_category, symbol, execution_id)`) prices that choice: a
re-read costs one row check, a gap costs an execution the net-delta invariant is computed
without. **The watermark rewinds to the start of the tied instant.** Binance spot's inclusive
`fromId` is the same choice made by the venue.

**Window defaults do not prove coverage.** All three surfaces answer with the time parameters
omitted — USDⓈ-M *"the last 7 days' data"*, spot *"the most recent trades"*, Bybit *"return 7
days by default"* — and each of those gets you executions while proving coverage **through**
no particular instant. §6.7 step 3 wants exactly that proof, so the horizon is **sent**, never
defaulted.

### Execution-row fields — and on Binance the client order id is ABSENT

| Surface | Row fields (verbatim) |
|---|---|
| USDⓈ-M `userTrades` | `buyer, commission, commissionAsset, id, maker, orderId, price, qty, quoteQty, realizedPnl, side, positionSide, symbol, time` |
| spot `myTrades` | `symbol, id, orderId, orderListId, price, qty, quoteQty, commission, commissionAsset, time, isBuyer, isMaker, isBestMatch` |
| Bybit `execution/list` | includes **`orderLinkId`** on every row, plus `execId, orderId, symbol, side, execQty, execPrice, execFee, feeCurrency, execTime` |

**Neither Binance list carries a `clientOrderId` or an `origClientOrderId`**, so on Binance
the venue's own `orderId` is the **only** join key between a swept execution and one of our
orders. The composition that rescues it is a *second* venue fact — Binance retains an order
**with fills** for 90 days, so `OrderByClientID` can always recover the `orderId` — which
makes `Fills` and `OrderByClientID` a **composition** where §4's signatures suggest
independence, and leaves the sweep useless alone in exactly the scenario it exists for: a lost
ack, where §6.7 runs precisely because the `Place` response was never read. §4's
`Ack.VenueOrderID` had recorded the opposite, justified by a parenthetical naming the one
venue where it holds; the claim is corrected there `[→ M3 slice C2b]`.

### `orderId` identity semantics — uniqueness AND non-reuse, per symbol (verified 2026-08-04) `[→ task #174, record §63]`

| Family | Verbatim statement | Source |
|---|---|---|
| Binance USDⓈ-M | *"orderId is self-increment for each specific symbol"* | Query Order (`GET /fapi/v1/order`) page notes, developers.binance.com, fetched 2026-08-04 |
| Binance SPOT | *"orderId: A field in the order response that uniquely identifies the order on a symbol."* | SPOT API Glossary, developers.binance.com, fetched 2026-08-04 |

**Why this needed banking when the join key was banked a section up.** The block above records
that `orderId` is the only join key on Binance — a claim consumed at **query time**, where the
venue names an id and a row either matches it or does not. Task #174's rule-out consumes a
**stronger** claim: an id persisted from a Place ack **still names the same venue order when a
later sweep meets it** — non-reuse **across time**. The two read identically in code (`==` on a
string) and rest on different guarantees, which is how the stronger one was nearly consumed
unbanked; a self-incrementing per-symbol counter is precisely a non-reuse statement, and the
spot glossary's "uniquely identifies" cannot survive a reuse either. **Both statements are per
SYMBOL**, so every consumer scopes to the full instrument — the same number legitimately names
different orders on the spot and perp books of one symbol, and an index keyed on the id alone
would rule out (or claim) a row across namespaces, in the direction that records a false
`AbsentAfterFinality`. **What a violation looks like and where it is caught:** a venue handing
two of our orders on one instrument the same id is refused at the projection's ack apply
(`ErrVenueOrderIDConflict`), holds new trading via the executor's fan-in, and leaves a journal
whose two acked records refuse to replay — the failure is a halt with the operator named, never
a quiet rule-out.

### NOT-FOUND IS IN A DIFFERENT LAYER ON EACH VENUE — inverted from §7's finding

| | Binance (both families) | Bybit |
|---|---|---|
| Order absent | error **`-2013 NO_SUCH_ORDER`**, *"Order does not exist."* | `retCode` 0 with an **empty `list`** *(assumed — see below)* |
| Which layer | the **envelope** — §6's classifier already makes it an `*apperror.Error` | the **payload** — the envelope reader sees a clean success |

§7 found **Bybit** stating a *refusal* inside a success status. This is the same seam
**inverted**: **Binance** states a *semantic answer* inside a transport *error*. `-2013` on a
4xx classifies `EffectNotApplied`, which is a true statement about the **query** — it was
refused, nothing happened — and a category error about the **order**. Propagated as an error
it is indistinguishable at the call site from a query that was rate-limited or timed out, and
§5.3 forbids that conflation in terms: *"'not submitted' is never inferred from a transient
not-found"*. **So an absence is a VALUE and a failure is an error**, and they leave by
different paths.

### RETENTION — the stable-not-found rule has an expiry date

| Surface | Drops an order when |
|---|---|
| Binance USDⓈ-M `GET /fapi/v1/order` | *"order status is `CANCELED` or `EXPIRED` **AND** order has NO filled trade **AND** created time + 3 days < current time"*; unconditionally at *"order create time + 90 days"* |
| Binance spot `GET /api/v3/order` | **no retention published** — recorded as unverified, **not** as "no cliff" |
| Bybit `GET /v5/order/realtime` | *"the most recent 500 orders"* — a **count**, not a duration; cleared for closed orders on a server restart |
| Bybit `GET /v5/order/history` | *"last 7 days"* for closed statuses; ***"last 24 hours"* for `Cancelled`, `Rejected`, `Deactivated`**; beyond 7 days *"supports querying orders which have fills only"* |

**Compose that with §6 step 6.** This system sends *"parallel marketable-IOC limit orders
(never raw MARKET)"* and nothing else. A marketable IOC that does not fill is terminal
immediately **with no filled trade** — precisely the row USDⓈ-M drops after **three days** and
precisely the row Bybit's history drops after **twenty-four hours**. **This system's single
most common order outcome is the one that becomes unqueryable soonest, on both venues.** Past
that horizon the query cannot separate *"the order existed and expired unfilled"* from *"the
order never existed"*, while §5.3 maps a stable not-found to `AbsentAfterFinality` — **proved**
absence. The exposure reading is identical either way, which is what makes it easy to dismiss:
what breaks is the **evidence class**. The normative consequence is in core §5.3.

**§4 ALREADY CARRIED BINANCE'S CLIFF, AND THAT MAKES THIS WORSE RATHER THAN BETTER.** The slice's
record claimed neither venue's retention was written down anywhere in the doc set. That is **false**:
§4's history-retention row has said *"order + algo-order history ≈90 days (**some unfilled-cancelled
orders purged in ~3 days**); trade (execution) history ≈6 months"* since **2026-07-20**, and it even
flagged itself *"corroborated — browser-confirm exact windows"*, which is precisely the verification
this section performs. **So the fact was recorded, was marked as needing confirmation, and its
consequence was never derived** across the fortnight and the several slices in between. What hid it
is *where* it sits: §4's row is about **history enumeration for the §7.4 actuator** — prefix-complete
association over paginated order history — so a reader arriving with §5.3's stable-not-found question
was never looking at that row, and the composition that matters (this system emits nothing but
marketable IOCs, §6 step 6, whose commonest outcome **is** the purged row) needs both halves at once.
It is C1's *citation-to-a-section-that-does-not-exist* tell **inverted**: there the cited section was
missing, here the section exists and says the thing, and nobody re-read it.

**This section is authoritative for the retention figures and §4's row is the approximation.** §4
gives ≈90 days / ~3 days / ≈6 months as corroborated magnitudes for the *actuator's* enumeration
budget; the table above gives the venue's **verbatim conditions**, which is what a rule about proved
absence has to be evaluated against — the 3-day drop is conjunctive (`CANCELED`-or-`EXPIRED` **and**
no filled trade **and** created + 3 days), and a rule reading it as "orders vanish after 3 days" would
be wrong in the direction that discards evidence it still has. Two statements of one quantity is the
hazard this doc set already names elsewhere; the split is kept because the two answer different
questions, and this note is which one wins.

Because retention differs **per surface**, an absence must be reported with the surface that
produced it — "absent from Bybit realtime" and "absent from Bybit history" are different
claims, and only the composed one is admissible input to the stable-not-found rule.

### The terminal statuses do not normalize the way they read

| The real event | USDⓈ-M / spot | Bybit linear | Bybit spot |
|---|---|---|---|
| An IOC that did not fill in full | `EXPIRED` | `Cancelled` — *"in derivatives, orders with this status may have an executed qty"* | `PartiallyFilledCanceled` — *"only spot has this status"* |

**Bybit has no expired-by-rule status at all**, so a normalized `Canceled` / `Expired` split
would put the *same real event* in different members depending on which venue answered. The
distinction that survives is the one §5.3 needs and both venues state in one word: an
**explicit rejection** (Binance `REJECTED`, Bybit `Rejected`), which is an evidence class
rather than a cause.

### Costs

| Surface | Cost |
|---|---|
| Binance USDⓈ-M order status / executions | weight **1** / weight **5** |
| Binance spot order status / executions | weight **4** / weight **20** (5 with `orderId`) |
| Bybit all three read surfaces | **no published weight** — Bybit meters **requests per second per endpoint**; both order surfaces are **50/s** |

Bybit's two order surfaces are charged against **separate** §7.6 limit names, because folding
them into one budget would let the fallback spend the primary's allowance during exactly the
incident — a Bybit restart — that makes every open order need the fallback at once.

### THE ONE ASSUMED FACT, named so it can be closed

**That Bybit reports an unknown `orderLinkId` as `retCode` 0 with an empty `list`** — rather
than as a non-zero `retCode` — **is assumed, not verified.** The order surfaces are all
signed, and the credential-free probes that settled §6 and §7 reach public endpoints only.
**Task #171** owns the testnet credential that closes it.

The codec is written so the answer does not change its safety: an empty list yields *absent*,
and a non-zero `retCode` never reaches the decoder at all — the envelope classifier turns it
into an error, and Bybit's `AbsenceFromError` declines to read an absence out of any error. So
the unverified branch **fails closed**: if Bybit does signal absence by a code, this codec
reports *a query that could not be answered* rather than *an order that is not there* — the
conservative half of §5.3's rule, and the half a wrong guess should land on.

→ https://developers.binance.com/docs/derivatives/usds-margined-futures/trade/rest-api ·
https://github.com/binance/binance-spot-api-docs/blob/master/rest-api.md ·
https://bybit-exchange.github.io/docs/v5/order/order-list ·
https://bybit-exchange.github.io/docs/v5/order/execution

## 9. The POSITION surface — enumeration, direction, and the version that decides what silence means (verified 2026-08-03)

**Why this section exists.** §7.5 asserts a net-delta invariant against `Positions` before trading
resumes, and slice F2 is the first code to read one. The method note applies: `developers.binance.com/docs/…`
is a client-rendered SPA, so the two Binance pages below were read through a server-side rendering
proxy of the same URLs and **cross-checked against the venue's own changelog**, which is what turned
up the contradiction in the weight row. Bybit's static docs were fetched directly and are verbatim.

### Binance USDⓈ-M — two versions of one endpoint, and their silences mean opposite things

- **`GET /fapi/v3/positionRisk`** — IP weight **UNSETTLED, see the ⚠ block below (task #180)**;
  parameters `symbol` (optional), `timestamp`
  (required), `recvWindow` (≤60000). Response rows carry `symbol`, `positionSide`, `positionAmt`,
  `entryPrice`, `breakEvenPrice`, `markPrice`, `unRealizedProfit`, `liquidationPrice`,
  `isolatedMargin`, `notional`, `marginAsset`, `isolatedWallet`, `initialMargin`, `maintMargin`,
  `positionInitialMargin`, `openOrderInitialMargin`, `adl`, `bidNotional`, `askNotional`,
  `updateTime`. **Verbatim, and it is the fact the invariant rests on:** *"only symbol that has
  position or open orders will be returned"*.
- **`GET /fapi/v2/positionRisk`** — the same shape, and it returns a row for **every** symbol
  (the page's own example is a flat one, `"positionAmt": "0.000"`). Its changelog entries, verbatim:
  **2024-07-24** *"`GET /fapi/v3/positionRisk`: Compared to `GET /fapi/v2/positionRisk`, this endpoint
  only returns symbols that the user has positions or open orders in."* and **2024-08-07**
  *"`GET /fapi/v2/positionRisk`: 5->10"*.
- **The version is a safety decision, not a preference.** On V2 a missing row means the symbol does
  not exist on the family; on V3 it means the account is flat there. §7.5 reads this surface to
  decide whether the venue holds exposure nothing here can account for, so *flat* has to be a
  CONCLUSION — and V3 is the version whose silence carries it.
- **…and the decision is PER QUESTION, which the paragraph above did not have to say and the next
  consumer needed (2026-08-06, task #198, record §73m).** The argument for V3 is scoped to §7.5's
  net-delta invariant, where a missing row must mean flat. Core §4.1's posture asks the opposite
  question — *what is this CONFIGURED symbol's setting* — and there V3's silence is exactly the
  ambiguity, while V2's row-for-every-symbol enumeration answers directly; the engine's posture probe
  nonetheless issues **V3**, so a flat account has no row for any configured symbol and the gate
  refuses on all of them (core §4.1, task #199, which now carries this as a fourth candidate). Note
  also that **the twenty V3 fields transcribed above contain neither `isAutoAddMargin` nor
  `autoAddMargin`**, which is the whole of the evidence behind §2's second ⚠ mark — either this list
  is an abbreviation or V3 dropped the field, and one page read settles which. The general shape is
  worth more than the instance: *a narrowing carries an argument, and the argument's scope is not
  re-derived by the next section that inherits the choice.*
- **⚠ THE V3 WEIGHT IS NOT VERIFIED, and this block records that state rather than a number
  (2026-08-05, task #180 — still open).** What is verified, from the 2026-08-03 read: the V2 endpoint
  page stated IP weight 5 while the venue's own 2024-08-07 changelog entry raised it to 10 (verbatim:
  *"`GET /fapi/v2/positionRisk`: 5->10"*), so an endpoint page can lag its own changelog for years.
  What is NOT verified: **V3's weight, in any of its three candidate readings** — a flat **5**, which
  is what this section recorded on 2026-08-03 through a server-side rendering proxy; a CONDITIONAL
  **"5 with `symbol`, 20 without"**, reported 2026-08-05 by a browser-side AI summary; and a
  CONDITIONAL **"1 for a single symbol; 10 when the symbol parameter is omitted"**, reported later the
  same day by a second, different AI summarizer (operator-run, quoted as verbatim page text).
  None is a source under this document's rule: the first came through a rendering proxy of a
  client-rendered page, the other two through summarizers — **the class of read that already misreported
  `GET /api/v3/account` as weight 10 when the raw docs say 20** (v0.9.49's own correction). A summary
  is not evidence about a number; the number has to be read. **Two summarizers disagreeing by a factor
  of two on the same line is the strongest statement yet of why**, and it is worth more than either
  number they produced.
  - **What the disagreement DOES establish, and it is not nothing: the SHAPE.** Two independent
    summarizers, run through different tooling on different renderings, both report a **conditional**
    weight — cheap when `symbol` is supplied, expensive when it is omitted — and disagree only on the
    two multipliers (5/20 against 1/10). Neither reports the flat row this section recorded in 2026-08-03.
    The conditional shape is the load-bearing half for this deployment, because §7.5's enumeration
    always takes the expensive arm; the multiplier decides only how pessimistic the charge is. So the
    shape is corroborated by independent agreement and the value is corroborated by nothing.
  - **The second summary carries all three of the tells recorded against the first, which is why it
    does not close the task.** It answered every question — V2, V3, the enumeration sentence, the
    parameter list — from **one aggregated catalog page** (`…/api/rest-api/trade`), where "which
    endpoint owns this Weight line" is a judgement rather than a reading and V2 and V3 sit adjacent;
    it consequently reported **V2 and V3 as carrying the byte-identical weight line**, which is the
    signature of a reader that could not separate them, and which *weakens* rather than corroborates
    the earlier report that V2's page now says a flat 10; and its changelog citations are
    **`google.com/search?q=…` redirects** rather than the changelog URL, i.e. that answer came from a
    search result and not from the page. Its one negative claim — no `positionRisk` changelog entry
    after 2024-08-07 — rests on the same search and is recorded as unverified in both directions.
  - **The readings are NOT symmetric for this deployment, because §7.5's enumeration sends no
    `symbol`** — every one of them charges this system its expensive arm. The engine charges **20**,
    which is the maximum over all three candidates and therefore correct-or-pessimistic under each:
    exact under the 5/20 reading, pessimistic by 10 under 1/10, by 15 under the flat 5, in every case
    once per reconcile pass. Under-charging a weight is the direction that ends in the IP ban §7.6
    exists to avoid, so the charge is set by the worst candidate and not by the newest one. The
    conservative charge is a code decision made against uncertainty, **not a venue fact**, and it is
    recorded as such in `positions_binance.go`. **A new candidate reading below 20 changes nothing
    about the code** — which is the property that makes this task safe to leave open.
  - **A direct read of the published line is a LOOSENING trigger**, never a correctness prerequisite:
    a page that says a flat 5 drops the charge to 5 with its citation, one that says 10-without drops
    it to 10, and nothing else in the system depends on which reading is true.
  - **Method note, and it retires the technique this section was written with.** As of 2026-08-05
    `developers.binance.com` answers curl and every headless fetcher with an **AWS WAF JavaScript
    challenge** (HTTP 202, empty body, `awsWafCookieDomainList` shell), and the URLs below **302** into
    a restructured catalog tree
    (`/en/docs/catalog/core-trading-derivatives-trading-usd-s-m-futures/api/rest-api/trade#position-information-v3`).
    The SSR-proxy route named at the head of this section no longer reaches the page, so the remaining
    channels are a real browser or the venue's own `X-MBX-USED-WEIGHT-1M` response header — the latter
    being the venue's actual accounting rather than its prose, and available on testnet with the
    credential #171 is waiting on.
  - Unverified in the other direction too: the same 2026-08-05 summary reported that V2's page now
    says **10**, i.e. that the two-year discrepancy above may since have been resolved at the source.
    Also unread, also pending the same look.
- **Direction is in the SIGN.** `positionAmt` is signed; `positionSide` is `BOTH` in one-way mode
  (§4.1 pins one-way as refuse-to-trade). A hedge-mode account answers with a `LONG` row and a
  `SHORT` row for one symbol, whose net is a subtraction no field states.
- Error codes used by this surface's reasoning, verbatim: **`-1121 BAD_SYMBOL` "Invalid symbol."**
  — which is why a malformed scope comes back as an error rather than as an empty list.

→ https://developers.binance.com/docs/derivatives/usds-margined-futures/trade/rest-api/Position-Information-V3 ·
https://developers.binance.com/docs/derivatives/usds-margined-futures/trade/rest-api/Position-Information-V2 ·
https://developers.binance.com/docs/derivatives/change-log ·
https://developers.binance.com/docs/derivatives/usds-margined-futures/error-code

### Bybit V5 — the direction is OUTSIDE the number, and the scope needs a settlement asset

- **`GET /v5/position/list`** — `category` required (`linear` / `inverse` / `option`); for linear,
  verbatim, *"either `symbol` or `settleCoin` is **required**. `symbol` has a higher priority"*. Rate
  limit **50 requests/second, per UID** (the API's limits are *"based on the rolling time window per
  second and UID"*), so there is no weight to charge — the request itself is the cost.
- **`size` is verbatim *"Position size, always positive"*, and `side` is *"'Buy': long, 'Sell': short"*
  with an *"empty string `""` for an empty position"*.** This is Binance's encoding inverted: a
  decoder written once against either venue reads the other's short as a long, with the magnitudes
  agreeing and nothing failing.
- **A flat row is an ANSWER.** Verbatim, when `symbol` is passed *"it returns data regardless of
  having position or not"*, so `side: ""` beside `size: "0"` is the documented empty position and
  must be admitted. The same empty side beside a NONZERO size is a direction the venue did not
  state, and both available defaults report a live position as flat or as pointing the wrong way.
- `positionIdx` **0 is One-Way Mode** (1/2 are the two-way sides), which §4.1 already reads for the
  posture check.
- **The enumeration is narrower here than on Binance.** One call covers one settlement asset, so a
  position settled in an asset the scope did not name is not in the answer and is not evidence about
  anything — the caller's guarantee across the two venues is this one.

→ https://bybit-exchange.github.io/docs/v5/position ·
https://bybit-exchange.github.io/docs/v5/rate-limit
