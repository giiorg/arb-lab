// Package ratelimit is build-spec §7.6's rate-limit discipline: the
// header-aware governors that decide whether a venue request may be issued, and
// the reserved allocation §6.7, §7.1 and §7.5 spend when the system is
// recovering rather than trading.
//
// # Machinery, not a venue table
//
// There are no venue numbers here. §7.6 names Binance's weight/order budgets and
// Bybit's ~600/5s gateway ban, but a Profile is data supplied by the caller —
// the same split platform/config took for §5.2's schema (a kernel holding risk
// caps makes every context's parameters a kernel edit) and platform/apperror
// took for the venue-code → Class table (venue knowledge belongs beside the
// client that produces the codes, where venue-profiles.md can be cited next to
// it).
//
// Here the argument is stronger than precedent, because venue-profiles.md
// records the rate-limit HEADER NAMES as verified facts and does not record the
// numeric budgets. A table in this package would therefore have to be populated
// with numbers nobody verified, in a document set whose standing rule is that
// nothing goes in unverified.
//
// # The ladder, and why capacity is a ceiling for every class
//
// [SPEC EDIT OWED] §7.6 says "back off at ~70% weight" without saying what backs
// off. §7.1 says the margin top-up gets "a reserved rate-limit allocation so it
// is not starved by trading traffic", and §6.7 says an ambiguous residual holds
// new trading and reserves budget for recovery — both without saying how much,
// or what happens when the reserve itself runs out. So the spec states a
// threshold in one section and a reservation in two others, and relies, unstated,
// on the priority ORDERING that makes them one mechanism.
//
// The ordering is load-bearing rather than cosmetic, and it is not the obvious
// one. Shedding market-data and account reads to protect trading traffic would
// make the rate limiter itself the cause of a stale mark — which is §7.1's own
// escalation trigger, so the governor would fire the de-risk it exists to help
// avoid. Monitoring therefore outranks new exposure. And the top of the ladder
// is a ceiling rather than a bypass, which the spec also never says: see below.
//
// Those are one mechanism seen three times: traffic has a priority, and each
// priority has its own ceiling on the same budget.
//
//	used ─────────────────────────────────────────►
//	         │ Opening  │ Routine │ Safety │ ✗ refused
//	         0         70%       90%     100%
//	                             └─ reserve ─┘
//
// Opening creates new exposure and is shed first — the same act §5.3 already
// calls `stop-opening`. Routine is the reads that keep the risk view live, and
// they outrank openings because cutting the monitoring to preserve the trading
// is backwards. Safety is recovery: §6.7 resolution, §7.1's top-up and REST mark
// failover, §7.5's reconcile, a flatten.
//
// Safety's ceiling is Capacity itself. It gets an EXCLUSIVE band, not a bypass,
// and that was a decision rather than a default. Past Capacity the venue is
// expected to refuse, so an issued flatten is not a flatten — and on Binance
// repeat violations escalate a 2-minute ban toward three days, so pushing
// lengthens the outage you are pushing against. The spec already has a designed
// answer for "cannot act at the venue": §7.4's pre-placed venue-native
// protection and the off-host actuator. A governor that spent past the budget
// would substitute wishful throughput for that ladder, and would do it silently.
//
// # Header-aware means the venue's count wins, in one direction
//
// A governor that only counted its own sends would be a guess. The venues report
// consumption back (`X-MBX-USED-WEIGHT-*`, `X-Bapi-Limit-*`), so Observe folds
// that reading in. It folds in only UPWARD, and the asymmetry is the point.
//
// A reading is always stale: it describes the server at the moment it processed
// some earlier request, and others were in flight behind it. So a reading BELOW
// the local count is exactly what staleness looks like, and adopting it would
// discard the record of every request sent after the one that carried the
// header. A reading ABOVE it is information we do not otherwise have — an
// endpoint whose weight we underestimated, or another consumer on the same IP.
//
// The two failure directions are not symmetric: ignoring a legitimate decrease
// costs throughput until the window rolls, and adopting an illegitimate decrease
// costs a ban. So the correction ratchets.
//
// It is applied as a synthetic charge at the current instant rather than as a
// separate floor, which keeps one mechanism instead of two and ages the
// correction out conservatively — the real usage expired earlier than the
// correction models it expiring.
//
// # Two mechanisms, because there are two failure modes
//
// A family's weight budget is headered, so local counting is an estimate that
// gets corrected. The venue-scoped IP envelope is NOT: Bybit's ~600/5s gateway
// ban (retCode 10018) arrives with no header at all, and Binance's IP-ban track
// is — in this spec's own words — "inferred, not documented". For those, local
// counting is the only defence there will ever be, which is why the envelope is
// a plain request counter at venue scope rather than a weight bucket at family
// scope. §7.6 asks for exactly this hedge: separate product weight/order
// counters, one shared IP-ban envelope per venue.
//
// # A sliding window, deliberately, even though the venues use fixed ones
//
// Binance's weight window is a fixed interval on the server, aligned to a
// boundary we cannot observe. Modelling it as fixed locally means guessing that
// alignment, and a wrong guess lets a burst land at the end of our window and
// again at the start of the next — twice the intended rate across the seam,
// which is the classic way a correct-looking limiter earns a ban. A sliding
// window cannot do that: budget is returned only once a charge has been held for
// the full interval. It is strictly more conservative than the venue's own rule,
// and it errs by refusing rather than by sending.
//
// # No goroutines, and no blocking
//
// Acquire is a decision, not a wait. Two reasons, and neither is style. §6.3
// makes rate-limit budget one of the things `risk.Gate` re-validates before a
// trade — that is a predicate, and a call that blocked until budget appeared
// would answer it by hanging. And a blocking governor needs a timer, which needs
// a goroutine and an ambient clock; platform/clock deliberately has neither, and
// outbox.Relay deliberately has neither, for the same reason: the instant a
// retry becomes eligible has to be reproducible in a replay.
//
// So a refusal carries NotBefore — the earliest instant a retry could succeed —
// and the caller arms a clock timer for it. Eviction happens lazily on the next
// call rather than on a ticker.
//
// # What is deliberately NOT here
//
// No header parsing. `X-MBX-USED-WEIGHT-1M` → which Limit is a venue fact, and
// the adapter that already parses the venue's error codes is where it belongs;
// Observe takes a limit name and a number.
//
// No alert delivery. §7.6 requires 418 and an IP ban to raise an immediate
// operator alert; this package classifies the penalty and records it in the
// Snapshot, and the caller routes it — the same division apperror draws, where
// the kernel says what a thing IS and the composition root decides where it
// goes.
//
// No cost table. What an endpoint weighs is venue documentation, and Cost is
// supplied per call.
package ratelimit
