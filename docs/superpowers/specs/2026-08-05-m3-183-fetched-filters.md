# M3 task #183 — fetched-provenance trading filters + freshness for the long-running engine (§69)

**Status:** CLOSED (2026-08-05, docs v0.9.48) — one adversarial design-review
round (five majors, all accepted and recorded in place below), implementation
split orchestrator/second-model, §69d checklist run before the campaign,
twenty mutations zero SURVIVED (one NOBUILD re-aimed: the sed orphaned the
file's only `http` use). Deferred with arguments, not dropped: the planner's
freshness mirror and hot-reload land with the engine's own loop (§69b);
#184's Dispatch surface untouched.
**Task:** #183 (follow-up from #182, Codex review major #1, adapted; record §62's revisit
trigger — "Revisit when the long-running engine (#183) makes plan-to-dispatch gaps real").
**Reads:** core §2.4 (trading-filter conformance bullet), §40d (fetched facts become
revisions), record §62 (quantization slice), record §60g (`admitPackage` is the seam),
venue-profiles §5 (the 2026-08-03 dated filter block).

## §69a — the decision, in three parts

**(1) The `ProvenanceFetched` writer §40d anticipated, scoped to LOT FILTERS.** A new
`arblab reference fetch` subcommand reads both venues' public filter endpoints unsigned,
and writes a NEW reference document whose `lots[]` rows carry `class: "fetched"`, the
fetch instant as `observed_at`, and the request URL as `source` — while `fees[]` and
`hurdle{}` are carried forward from the input document unchanged. Fees stay
hand-recorded on §40d's own boundary: the fee endpoints answer for *the authenticated
account's* tier and this project holds no credential, whereas the filter endpoints are
public and venue-published — and for venue-published facts the venue's own answer *is*
the definition. The account-real precondition on live orders (§40d) keys on fee rows and
is untouched by this slice.

**(2) Venue words verbatim, validated by conversion.** The fetcher extracts the venue's
decimal STRINGS and writes them into `LotRow` unchanged — no fixedpoint round-trip at
fetch time. `Document.Validate()` already validates every row by conversion through the
domain validators (the one authority), so an unusable venue answer refuses the write and
names the row. This also preserves the venue's own formatting (`"0.00001000"` on spot,
`"0.001"` on fapi) as evidence of what was actually answered.

**(3) Freshness at the executor's admission region, not boot-only.** `oms/domain.
TradingFilters` gains a required `ObservedAt` — the carrier of the age question, which
today exists nowhere in the OMS vocabulary, so nothing past boot can even ask it. The
Executor takes a declared `FilterMaxAge` (threaded through `oms.Deps`) and its preflight
refuses a leg whose filters row is older than the bound at this package's admission
instant, with nothing fsynced or sent. The boot-time check in `exercise/filters.go`
stays — it names the FILE before any run starts; the admission check names the
INSTRUMENT at the moment the age matters. Same comparison direction at both points
(`age > bound` refuses; equality passes): a recomputed check must share the anchor.

## §69b — the mechanism

### Transport: `PublicClient` (engine/venue/transport/public.go)

`rest.go`'s own doc anticipated this consumer ("milestone 3's fee-tier reads will want
the same poller"). Neither existing type fits: `PollerConfig` refuses query strings by
validated design (§32h's endpoint takes none), and `SignedClient` always signs. The new
sibling:

```go
type PublicConfig struct {
    Scheme, Host, Path string   // Poller's rules, verbatim
    Params  Params              // ordered; rendered by Params.Encode (the signing path's
                                // percent-encoding, which has its own oracle); may be empty
    Timeout time.Duration
    ReadLimit int64
}
type PublicClient struct{ ... }
func NewPublicClient(cfg PublicConfig) (*PublicClient, error)
func (c *PublicClient) URL() string
func (c *PublicClient) Fetch(ctx context.Context) (PollResult, error)
```

Poller's semantics carry over exactly: one GET per Fetch, non-2xx is a RESULT, body over
`ReadLimit` is a refusal (read limit+1 so at-the-limit and over it are distinguishable),
no retry (§37b: the next call is the retry), timeout on the client so no ambient-time
member appears at a call site. `Params` keys must be non-empty; empty `Params` is legal
(fapi takes none).

**One departure from the Poller, and it is the provenance argument (review round 1,
major 1): redirects are not followed.** The Poller archives what the venue answered and
a 3xx is an answer; this client's URL becomes a `source` claim on a row that gates
orders, and following a redirect would attribute a different host's body to the
configured URL. `CheckRedirect` returns `http.ErrUseLastResponse`, so a redirect
surfaces as its own 3xx result — which the writer refuses as it refuses any non-200,
naming the status.

**No pagination, and the narrowing carries its argument:** every call this slice makes
is symbol-scoped where the venue honors scoping (Bybit both categories, Binance spot) or
unscoped-but-single-page (Binance fapi, ~1 MB full roster — inside a 4 MB ReadLimit).
The cursor-paged shape exists only on Bybit's UNSCOPED instruments-info, which this
fetcher never issues. When a multi-symbol universe outgrows per-symbol calls, the
cursor loop in `screening/universe.go` is the precedent — and a cursor is a resume
watermark, not a page token, so it must not be added casually.

### Decode + write: strategy/infrastructure/driven/reference_fetch.go

Placement: the schema (`LotRow`) lives in `driven` beside its store, and screening
already set the precedent of a strategy-infrastructure package parsing these exact
endpoints. `hexboundary` permits infrastructure → engine. Four pure decoders, one per
cell, each `func(body []byte, symbol string) (LotRow, error)` — pure so fixtures
exercise them without a server:

| Cell | Request | Fields read (verified live 2026-08-05; venue-profiles §5) |
|---|---|---|
| binance spot | `GET /api/v3/exchangeInfo?symbol=BTCUSDT` (honored) | `filters[]`: `LOT_SIZE`.stepSize/minQty — **never `MARKET_LOT_SIZE`**, whose stepSize is a live literal zero; `PRICE_FILTER`.tickSize; `NOTIONAL`.minNotional |
| binance linear_perp | `GET /fapi/v1/exchangeInfo` — `?symbol=` **ignored**, full roster answers; select client-side | `LOT_SIZE`.stepSize/minQty; `PRICE_FILTER`.tickSize; `MIN_NOTIONAL`.**notional** (the fourth spelling) |
| bybit spot | `GET /v5/market/instruments-info?category=spot&symbol=BTCUSDT` | `lotSizeFilter`.basePrecision (the spot lot constraint — a precision, not a step field)/minOrderQty/minOrderAmt; `priceFilter`.tickSize |
| bybit linear_perp | `...?category=linear&symbol=BTCUSDT` | `lotSizeFilter`.qtyStep/minOrderQty/minNotionalValue; `priceFilter`.tickSize |

Bybit bodies go through the V5 envelope (retCode 0 or refuse — `screening`'s rule).
Every decoder refuses an absent symbol, an absent filter object, and an empty field —
absence must never become a permissive zero (§40d's "unpopulated reads as free" rule;
`LotRules.Validate` backs it up at conversion, but the decoder's refusal names the
venue answer rather than the assembled row).

**A decoded row is bound to its whole cell, not just its symbol (review round 1,
major 2).** Each decoder also extracts and the writer refuses on: the venue's own
base/quote asset words (`baseAsset`/`quoteAsset`, `baseCoin`/`quoteCoin`) against the
input row's `base`/`quote`; the trading status (`TRADING` / `Trading` — the same
predicate `screening/universe.go` applies); and the contract type where the venue
publishes one (`PERPETUAL` / `LinearPerpetual`). The failure this closes is not
hypothetical drift: a SUSPENDED instrument keeps schema-valid numeric filters, so
without the status bind a fresh, conforming, provenance-stamped row would green-light
a leg the venue will reject — the asymmetric-rejection harm again, arriving through
the freshness mechanism itself. And a non-200 status refuses BEFORE decode: the
transport hands back a 429/503 body as a result on purpose, and a cached or
error-page body must never earn a `fetched` stamp (review round 1, major 1).

The writer `FetchLots(...) (Document, error)` returns the input document with each
configured instrument's lots row replaced, `class: "fetched"`, `observed_at` =
clk.Now() RFC3339 UTC, `source` = the request URL. All fetches and the full
`Document.Validate()` succeed before ANY bytes are written — an nth-fetch failure
produces no file. The CLI opens `-out` with `O_CREATE|O_EXCL` (the
`dispatch/logwriter` precedent): every existing target refuses, which subsumes
"don't overwrite the committed calibration-pinned input" (§62g) without trusting a
path comparison that symlinks and spelling defeat (review round 1, major 4). A
revision is a new file, always.

### Freshness: oms/domain + Executor

- `TradingFilters` gains `ObservedAt time.Time`; `Validate()` refuses zero
  (`ErrFilterObservedAtUnset`). Zero-value-unusable, and in the direction that is right
  when somebody forgets: an unset date reads as infinitely stale and refuses, never as
  fresh forever.
- `NewExecutor(d, filters, maxAge, reserver)`: `maxAge > 0` required iff the filters map
  is non-empty; a non-empty map with no bound and a bound with no map are both refused
  at construction (`ErrFilterMaxAgeInvalid`) — the pair is one policy and half of it is
  a wiring error with a name.
- `Executor.Execute` stamps ONE `now := e.clk.Now()` per package; preflight's second
  pass (the same loop as `conformsToFilters`, §60g's admission region — before any
  reserve, fsync or send) refuses a leg whose `now.Sub(f.ObservedAt) > e.filterMaxAge`
  with `ErrFiltersStaleAtAdmission`, naming instrument, observed-at, age and bound.
  One instant per package so a two-leg package cannot straddle the bound and refuse
  asymmetrically — the package-atomicity rule §62d already bought.
- **A row from the future refuses too (review round 1, major 3).** A negative age
  passes `age > bound` trivially, and it has a real producer: a backward wall-clock
  step after boot, exactly the event `platform/clock`'s clamp exists for — but the
  executor holds an already-resolved map and never re-consults the store's
  `ErrNotPeriodCorrect` guard. `ObservedAt.After(now)` refuses with its own sentinel
  (`ErrFiltersFromTheFuture`) rather than riding the staleness one, because the
  operator's fix differs: a stale row wants a fresh revision, a future row wants the
  clock investigated.
- `oms.Deps` gains `FilterMaxAge time.Duration`, documented as the same declared
  operator tolerance `Reference.MaxAge` states at boot; the exercise root passes one
  value to both. `exercise/filters.go` populates `ObservedAt` from
  `lots.Provenance.ObservedAt`.

### What this slice does NOT change

- **The Planner gets no freshness check.** It has no clock, and its one caller today is
  a one-shot binary whose boot check runs seconds earlier. The mirror is real and
  recorded, not silently absorbed: when the long-running engine's planning loop lands,
  plan-time freshness lands with it (the #184 pattern — the check joins its first real
  caller). The executor covers the gap meanwhile: every planned leg passes preflight
  before money moves, including replayed ones.
- **No hot-reload wiring.** `config.Store` can already reload and `ReferenceStore`
  reads through `Current()`; the consumer that re-reads mid-process is the engine
  binary, which does not exist. Recorded as that binary's trigger, same rule as above.
- **No fee fetching.** §40d's boundary (authenticated, account-real) stands.
- **#184's single-order Dispatch surface** is untouched and still owed to its own task.
- **`ProvenanceFetched`'s doc comment** said "authenticated programmatic fetch" — 
  written when fees were the only anticipated writer. Widened to name both writer
  shapes (authenticated for fees; unsigned for public filter facts). `[SPEC EDIT OWED]`
  markers on the widened comment and the §2.4 admission-freshness sentence, landed in
  the same session (see §69e).

## §69c — rejected alternatives (recorded so nobody reopens them)

- **Extend `PollerConfig` with a `Params` field.** Its doc refuses query strings with an
  argument ("a field nothing uses is a field nothing validates") that dissolves for the
  new consumer but NOT for the old: §32h's endpoint still takes none, capture's config
  still constructs the type, and the field would be validated-but-forbidden there —
  a conditional validation surface on a type whose whole design is "every field
  required". A sibling type keeps both surfaces unconditional.
- **Parse venue decimals to fixedpoint at fetch time, write canonical strings.** Destroys
  the venue-verbatim evidence (padding differences are provenance), duplicates the
  validation the store already does by conversion, and puts a second decimal authority
  beside the one the domain owns — the stored form would contain the answer.
- **Overwrite the committed reference document in place.** §62g deliberately froze its
  `observed_at` so the calibration's frozen as-of stays answerable; an in-place fetch
  destroys the hand-recorded evidence and re-dates every row. A new file keeps both
  documents citable, and the operator (or a future engine boot) chooses which to load.
- **Freshness as a parallel `map[inst]time.Time` beside the filters map.** Two
  structures keyed the same way carrying halves of one fact: the update that touches
  one and not the other is exactly the relocated-premise drift the carrier type
  prevents. The date belongs on the row it dates.
- **Freshness inside `conformsToFilters`.** Shared by the Planner, which has no clock;
  forcing one in would wire a time seam into a use case whose long-running caller does
  not exist, to enforce a bound its current caller re-checks seconds later. The
  executor's preflight is the admission seam §60g names, and it is the last read
  before anything irreversible.
- **A live venue read on the decision path when the row is stale.** §40d forbids it
  twice over: the decision path reads revisions (parity — the backtest cannot call a
  venue), and a fetch-on-stale converts a declared refusal into a hidden dependency on
  venue availability at the worst moment.
- **A TTL re-fetch loop inside this slice.** The loop's owner is the engine binary
  (which would also own reload); building it now means building a daemon nothing runs.
  The slice delivers the fetch as a CLI invocable by hand, cron, or the future engine.

## §69d — evidence plan (collapse checklist BEFORE mutations)

Fixtures are trimmed live bodies (2026-08-05 probes, this session — the same probes
that pinned `lotSizeFilter` as the Bybit parent key), and the checklist is run on them
before any mutation:

- **Selection must split the picking rules** (fapi's client-side symbol select): the
  fapi fixture carries ≥3 symbols with pairwise-DIFFERENT filter values, BTCUSDT not
  first and not last, so first/last/wrong-symbol each disagree with the right answer.
- **The wrong-filter trap must be discriminating:** the Binance spot fixture carries
  both `LOT_SIZE` and `MARKET_LOT_SIZE` with DIFFERENT stepSize (the live values —
  MARKET's is zero, which `LotRules.Validate` refuses, so consulting the wrong filter
  is a named failure, not a wrong number). The fapi fixture: `MIN_NOTIONAL` beside
  filters that carry other spellings' fields, so reading the wrong spelling fails.
- **Cross-category confusion:** the Bybit spot decoder fed the linear fixture (and
  vice versa) must refuse by absence of its fields, and the assertion is on the
  EXPECTED error, positively — not `err != nil`.
- **Boundary values, both sides and the boundary:** freshness at `age < bound`,
  `age == bound` (passes — inclusive tolerance), `age > bound`, and `age < 0` (the
  future row, its own sentinel). The `<=`/`<` collapse is the checklist's own named
  trap.
- **The one-clock-read claim needs a clock that would tell (review round 1, major
  5):** on a fixed fake clock, `Now()` inside the leg loop and `Now()` before it are
  observationally identical, so the mutation aimed at the one-instant claim would
  survive. The discriminating fixture: two equally-dated legs and a STEPPING clock
  whose first read is inside the bound and whose second would be outside it — the
  package must admit (one read), and the per-leg mutant then refuses leg 2.
- **Failure matrix for the writer:** an nth-fetch transport error, a non-200 status
  (with a schema-valid body — the discriminating part), a wrong-symbol body, a
  suspended-status body, a wrong-quote body, and an existing `-out` target must each
  produce NO output file, asserted positively (the path does not exist afterward).
- **Cardinality:** a two-instrument fetch where the two rows differ in every field, so
  per-instrument claims cannot pass vacuously off row one; a two-leg package where leg
  2's filters are stale and leg 1's are not — the package refuses whole.
- **Golden:** the written document is pinned complete (a preservation claim needs a
  golden): fees/hurdle byte-identical to input, lots rows exactly the fetched values.
- **Positive controls:** every live fixture decodes to the venue-profiles §5 values —
  the same numbers the committed reference document carries, which is the
  two-independent-sources agreement this repo treats as the real oracle.

Mutation campaign (aims, per claim): the symbol-selection comparison; each filter-name
string; the `MARKET_LOT_SIZE` avoidance; the freshness comparison's direction and its
bound; the one-instant-per-package claim; the `class: "fetched"` stamp; the `source`
stamp; `ObservedAt` propagation in `exercise/filters.go`; `Params` ordering in the
rendered URL; the ReadLimit boundary on the ~1 MB fapi body; the in-place-overwrite
refusal. `NOBUILD!!` on the rule-2 assertions reads per the CLAUDE.md rule.

## §69e — spec edits owed by this slice

- Core §2.4 conformance bullet: filters resolve under a declared max row age **at boot
  and again at package admission** (the executor's preflight; boot-only was #182's
  one-shot scope, stated there as such).
- §40d: the lot-filter faces of the reference document may be written by the unsigned
  public fetch (the endpoints are venue-published definitions); fee rows keep the
  authenticated account-real precondition.
- venue-profiles §5: add the dated 2026-08-05 note — `lotSizeFilter`/`priceFilter`
  parent keys confirmed live on both Bybit categories; all four cells' values
  re-confirmed unchanged.
- adr-history: dated block for #183; HANDOFF; doc set version bump.
