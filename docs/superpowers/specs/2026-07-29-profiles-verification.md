# §35 — the Profiles verification round: §13's five debts, and standing debt #1 (2026-07-29)

**Status:** done. No code changed. `venue-profiles.md` gains **§5** plus row
corrections in §1 and §3; `2026-07-29-capture-tool-design.md` §13 and §32c are
updated in place. **Slice C is unblocked.**

This was not a build slice. It is the research task §13 named, held open for three
sessions while slices A and B were built around it, and by session 31 it was the
critical path for the whole milestone. This record holds **what the verification
changed about the questions**, because in three of six cases the debt as written
was mis-stated, and a debt that closes without correcting itself teaches nothing.

| Debt | Asked for | Answer |
|---|---|---|
| §13-1 | Binance per-symbol funding topic | `btcusdt@markPrice@1s`, on `/market` |
| §13-2 | Binance futures book host, path, cadences | `/public`; 5/10/20 @ 100/250/500 ms — **and two more hosts** |
| §13-3 | Bybit linear public WS URL | `wss://stream.bybit.com/v5/public/linear` |
| §13-4 | The **two** venues' depth update-ID shapes | **Three** shapes, split by product not venue |
| §13-5 | Public vs authenticated for the REST overlays | `fundingInfo` public; one sub-item left open on purpose |
| standing #1 | (same as §13-4) | cleared with §13-4 |

---

## §35a — the finding that unblocked the other five: the SPA has a server-rendered twin

`venue-profiles.md` §4 carries three blocks marked *"⚠ not verbatim:
`developers.binance.com` is a client-rendered SPA; **browser-confirm before
implementation**"*. That caveat is the reason two of §13's debts were phrased as
research tasks rather than lookups, and it has stood since 2026-07-20.

**`developers.binance.com/docs/…` returns an empty JavaScript app shell.
`developers.binance.com/en/docs/catalog/…` returns the same documentation as
server-rendered text.** Every Binance fact in Profiles §5 came from the second
path, verbatim, with no browser involved.

**§4 already cited one catalog URL** — the history-retention line, fetched
2026-07-20 — and it is marked *"corroborated … browser-confirm exact windows"*
like its neighbours. So the escape hatch was already in the document, used once,
and **recorded as if it had the same evidentiary weight as the shell fetches
around it.** Nobody noticed that one of the five citations in that block was a
different *kind* of source that had actually worked.

> **A source that worked is invisible when it is filed beside four that did not.**
> The block's summary caveat was applied to all of its rows, and the row that
> did not need it lost the distinction. The tell is a source list whose entries
> share a caveat but not a domain path.

The practical consequence is larger than this session: **§4's three
`⚠ browser-confirm` blocks are now removable by re-fetch**, not by opening a
browser. They are left as they are — this session verified what §13 asked for and
did not re-open closed rows — but the note is in Profiles §5's method paragraph so
the next reader does not repeat the reasoning.

**A second, smaller instance of the same shape:** the catalog is split by URL
prefix, so `…/ws-streams/public` contains **no mark-price section at all**. Read as
a fetch failure that is three wasted attempts; read correctly it is *corroboration
of the routing split* — mark price is a `/market` stream, so its absence from the
`/public` page is the page being right. **A page's silence is evidence when you
know what the page is scoped to.**

---

## §35b — §13-2 and §13-3 asked for one host and one URL; both questions were under-specified

§13-2 asked for *"Binance futures book stream host, path and available cadences"*.
The answer needs **three hosts**, and the debt could not have surfaced that,
because it was written from §32c's source table — which records **topics only**.

| Source | Host + path | Speeds |
|---|---|---|
| Binance perp **funding** | `wss://fstream.binance.com/market/ws/…` | 3 s or 1 s |
| Binance perp **book** | `wss://fstream.binance.com/public/ws/…` | 100 / 250 / 500 ms |
| Binance **spot** book | `wss://stream.binance.com:9443/ws/…` | **1000 / 100 ms** |

The three-prefix split (`/public` high-frequency, `/market` regular, `/private`
user data) is real, dated **2026-03-05**, and the legacy `/ws` and `/stream` paths
were decommissioned **2026-04-23** — Profiles §1's existing row was correct and is
now verbatim-sourced rather than carried. What §1 did **not** say is that the spot
host is outside the scheme entirely and has a **different set of update speeds**.
A `@depth20@250ms` config that works on futures is invalid on spot.

§13-3 has the same shape one turn smaller. Profiles §1 said *"single WS host"* for
Bybit, which is **true and useless**: the host is single and the **path** is
per-category (`/v5/public/linear` vs `/v5/public/spot`). Sources 5 and 6 use the
same topic string, `orderbook.50.BTCUSDT`, and are distinguished **only** by URL.

> **Recording a topic without its endpoint records the half a reader can guess.**
> §32c's source table listed seven topics and zero URLs, and two of its rows are
> byte-identical topics on different hosts. The table has gained a `Connect to`
> column.

---

## §35c — §13-4 asked for two shapes; there are three, and the split is not by venue

Standing debt #1 has been open since session 20, and `engine/venue`'s book-span
design (core §4's *"a book delta payload carries `FirstSeq` and never `LastSeq`"*)
was built against the claim that the two venues are *asymmetric*. They are. But
the debt's framing — *"the two venues' depth update-ID shapes"* — encodes an
assumption that **a venue has a shape**, and Binance has three.

| Feed | Shape | Gap-detectable from its own bytes? |
|---|---|---|
| Binance **futures** partial **and** diff | `U` … `u` span **+ `pu`** back-link | **Yes** — `pu` ≠ previous `u` is a gap |
| Binance **spot** diff | `U` … `u` span, **no `pu`** | Only across consecutive events |
| Binance **spot** partial | **`lastUpdateId` only** — no span, no event time | **No** |
| Bybit linear **and** spot | single `u` (**resets to 1 on service restart**) + `seq` | `u` monotonicity, which a restart breaks |

Two consequences, and the second is the one that costs something.

**1. Binance futures partial depth is self-sequencing.** The partial-book stream
carries the *same* `U`/`u`/`pu` fields as the diff stream — which is not true on
spot, and is not what a reader assumes from the name "partial book depth"
(a snapshot-shaped feed that nonetheless carries delta bookkeeping). Source 3
therefore needs no help to be gap-checked.

**2. Source 4 — Binance *spot* partial depth — cannot be gap-checked at all.**
`{lastUpdateId, bids, asks}` and nothing else: no span, no back-link, **no event
time**. Capture's `CaptureSeq` (§32d) becomes the only ordering evidence that feed
has, and losslessness for it is asserted by the capture layer rather than
checkable against the venue's own numbering. That is a **reconstructability limit
on the spot leg's fill model**, which feeds the kill gate's execution-cost input.
It is a venue fact, not a tool defect, and it is now recorded in Profiles §3
rather than discovered during step 5.

**Source 7's job has changed accordingly.** It was specified as *"evidence for
standing debt #1"* — measuring the Binance update-ID shape that documentation did
not record. Documentation now records it. Stream 7 is worth keeping as live
confirmation, but the open question it should be pointed at is **source 4's**, and
§32c has been amended to say so.

> **A debt that names a count has already answered part of the question.**
> "The **two** venues' shapes" is a schema, and it survived four rounds of review
> and five sessions because re-reading it re-reads the schema.

This is §33c's failure one layer earlier. There (slice A, corrected in session 31)
a **rejected alternative's stated benefit** went unchecked for two sessions because
it was written down confidently. Here an **open question's stated shape** went
unchecked for five. **Both are assertions that arrive pre-worded and get re-read
instead of re-derived** — and a debt list is more dangerous than a decision record,
because re-reading it is what you are *supposed* to do with it.

---

## §35d — §13-5, and the one sub-item deliberately left open

`GET /fapi/v1/fundingInfo` is **public**: the catalog page marks it with no
security type, and a live unauthenticated request returned **HTTP 200 with a full
body**. Weight 0, sharing a 500 / 5 min / IP budget with `GET /fapi/v1/fundingRate`.

Binance's security taxonomy is now recorded verbatim in Profiles §5 because
"authenticated" is not one thing here: `TRADE`/`USER_DATA` need **key + signature**,
while `USER_STREAM`/`MARKET_DATA` need **key only**. A design that says "needs
credentials" has not said which blast radius it means (§7.3).

Of §32h's **deferred** overlays, `GET /fapi/v1/leverageBracket` is `USER_DATA` and
`GET /v5/account/fee-rate` is signed. That is worth recording as more than
bookkeeping: §32h's **first ground for deferring them was credentials**, and until
now that ground was *assumed*. It is now verified, so the deferral is an informed
decision rather than a plausible one.

**`GET /fapi/v1/commissionRate` is NOT verified.** The catalog page truncated
before its row, and it is recorded as unverified in both Profiles §5 and §13-5
rather than closed with its two neighbours. Nothing is built on it — it is
deferred — so this blocks nothing. It is stated because **the cheapest moment to
lose a gap is while closing the four around it**: a debt list that goes from five
items to zero reads as complete, and one that goes to zero-with-a-footnote reads as
what it is.

---

## §35e — the live probe, and three ways `fundingInfo` is not what the design assumed

§32h's design rests on one property: **absent means 8 h**, so *"we asked and it was
empty"* and *"we never asked"* must be distinguishable, which is why the `Rest`
record stores the body verbatim. One unauthenticated GET on 2026-07-29 returned
**742 rows**, and three things in it are worth the section.

**1. BTCUSDT is present, not absent.**
`{"symbol":"BTCUSDT","adjustedFundingRateCap":"0.00300","adjustedFundingRateFloor":"-0.00300","fundingIntervalHours":8,"disclaimer":true,"updateTime":null}`
— identical for ETHUSDT. The cap **confirms Profiles §1's ±0.3% BTCUSDT row from a
second direction**, which is a free corroboration of a fact recorded in 2026-07-18.
But the *absent* branch — §32h's entire motivating example — **does not fire for
the one symbol we trade**. The design is still right (other symbols are absent, and
recording the fetch is right regardless), but its load-bearing instance is a
**populated row**, and that was not known when the reason was written.

**2. 8 h is not the common case.** `fundingIntervalHours` across the 742:
**4 h × 442, 8 h × 296, 1 h × 4.** Profiles §1 records the Binance side as
*"absent ⇒ 8h default"* and the Bybit side as *"dynamic 8h↔1h auto-switch"* — **no
4 h case appears anywhere in the document**, and on Binance it is the majority of
*listed* symbols. The 8 h default is safe here only because BTCUSDT is listed and
says 8. **The moment the strategy widens past BTC, "absent ⇒ 8 h" is a coin flip
dressed as a default** — and core §4's rule that a zero interval is *refused rather
than defaulted* is the guard that survives it, which is an argument for that rule
that its author did not have.

**3. `updateTime` is `null` on BTCUSDT** while other rows carry an epoch-ms
integer. A decoder typing it as a non-nullable integer parses 740 rows and **fails
on exactly the symbol we trade**. It is the "exemption leaks past its reason" shape
inverted: not a check skipped for one caller's sake, but a *type* that holds for
99.7% of the data and fails on the one row the whole system exists to read. A
fixture built from any other symbol makes it invisible.

**On the evidence class.** These three are **live observations, not documented
guarantees**, and Profiles §5 labels them that way. A re-measurement can change any
of them; nothing may be built on them that a re-measurement would invalidate. They
are recorded because a measurement that contradicts a design's stated premise is
worth more than one that confirms it, and this one did both.

---

## §35f — what this round did NOT do

- **No code changed.** The facts are recorded; wiring them into config and the WS
  transport is slice C's work, and doing it here would have merged a research task
  into a build slice.
- **§4's three `⚠ browser-confirm` blocks are untouched.** §35a shows they are
  re-fetchable, but they are outside §13's scope and re-opening verified rows on
  the way past is how a scoped task becomes an unscoped one. **Recorded as
  available work, not done.**
- **The doc set was not version-bumped.** This is not a slice release, and
  Profiles' own line — *a Profiles verification is not a spec edit* — is the
  reason. The change is marked in the Status line as a dated post-release addition
  so the next set version absorbs it rather than hiding it.
- **`commissionRate`'s security type** — §35d.
- **Bybit's spot depth-level roster** is recorded only for level 50 (20 ms), the
  one we use. Two independent reads of the page returned linear's and spot's
  rosters as identical, including a level 1000 that Profiles §3 had not recorded;
  **two reads through one summarizer are one read**, so the roster is not asserted
  and §3's existing `{1|50|200}` stands.

---

## RESUME HERE — end of the verification round

**Slice C (§32m) is unblocked and is the next work.** Raw WS transport, subscribe +
ack + first-data assertion, reconnect-with-gap, `arblab capture run`, config,
observability, and the minimal `capture verify` with its positive control.

**Four things slice C should carry out of this record:**

1. **Config must express host, path prefix and topic separately** (§35b). Three
   sources, three hosts, and two sources whose topic strings are byte-identical.
2. **Source 4 has no wire-level gap check** (§35c). Its `capture verify` arm cannot
   assert what the other six can, and the positive control must not be written in a
   way that hides the asymmetry.
3. **`fundingInfo`'s `updateTime` is nullable** (§35e-3). Capture stores bytes
   verbatim so the writer does not care — but `capture verify` and anything at
   step 5 that parses the overlay does.
4. **The live `Mono` source is still open** (§34c) — a `Mono` method on
   `platform/clock.Wall` or an adapter in `internal/app/capture`, and **it must not
   be `Now() - start`**.
