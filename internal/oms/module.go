// Package oms is §3.2's order-management bounded context, and this file is its
// composition seam: the one place that knows which adapter satisfies which
// port.
//
// # Why a seam here rather than wiring in app/
//
// §3.6 rule 7 gives every context a module.go and has the binary wire modules by
// hand. internal/marketdata/module.go states the argument for the split in full
// — app/ owns what varies BETWEEN binaries, this owns which adapter implements
// this context's ports — and it is not restated here.
//
// What is different here is that the split has a gate behind it. singlewriter
// polices application/ports and infrastructure/driven against every importer
// including app/, so "the context wires its own adapters" is no longer only a
// convention about where wiring reads best: for this context it is the only
// shape a root can express. §26e records the decision and its failure direction.
//
// # Scope: the write path is assembled, and the hole this file used to state is
// # closed
//
// Until slice B this file recorded an absence: driven.MemoryOrderWriteRepository
// was not constructed, because §6's write path is journal-then-apply and the
// apply half had no caller — ApplyPlanned belonged to §6 step 5's planning use
// case, which did not exist, and Dispatch took a domain.Order carrying no OCC
// version, so wiring a repository into it would have meant inventing the
// planning path to have produced a record for it to advance.
//
// That is what slice B built, and the two halves are now wired to ONE
// repository instance: application.Planner writes the record §6 step 5 journals,
// and application.Dispatcher reads its order back out and advances it. The
// shared instance is the point rather than an implementation detail — two would
// make Plan and Dispatch disagree about which orders exist, and Dispatch's
// refusal of an unplanned order would refuse everything.
//
// What is still NOT wired, stated so the absence stays visible: ApplyFill has no
// caller. §6's private order and execution stream handlers are slices C and D,
// and §5.3's one-transaction requirement for the fill path needs a transaction
// seam no port here declares.
//
// # What Module must NOT hand out, which is the decision this file exists for
//
// Exposing the repository — Module.Orders, typed ports.OrderWriteRepository —
// is the shape that would make the wiring look complete, and it is the one
// shape §3.6 rule 6 cannot survive. singlewriter reads DECLARED IMPORT EDGES,
// and Go's interfaces are structural: a root that receives such a field can
// hand it to any other context's constructor, and the receiving context needs
// no import at all to accept it. Every diagnostic the gate would have produced
// disappears, and what is left is a second writer to order state with a clean
// lint run — precisely the outcome §26e reasoned the gate's scope backwards
// from.
//
// So the write authority stays inside this package tree, and the rule that
// follows from it is worth stating in the general form: when the repository
// does get wired, it is wired INTO a use case here, and the use case is what
// Module exposes. A field of a policed type on this struct is never the answer.
package oms

import (
	"errors"
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/oms/infrastructure/baseline"
	"github.com/giiorg/arb-lab/internal/oms/infrastructure/driven"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// Deps is what a composition root supplies.
//
// # One port and one piece of configuration, which is the shape the gate forced
//
// This struct used to hold two ports, and its own doc predicted the change that
// has now happened: "Journal's adapter is this context's own … it is a Deps
// field because it has not been written yet, not because a root is the right
// owner of it. When it lands, singlewriter FORCES the correction rather than
// merely inviting it."
//
// Slice A landed driven.OpenSQLiteJournal inside the policed package, and the
// prediction went live immediately: with the adapter unnameable from outside
// internal/oms, the ONLY way a root could still have filled a
// `Journal ports.Journal` field was by writing its own journal implementation —
// a second write path to order state, which is the outcome §3.6 rule 6 exists to
// prevent. A field whose sole remaining legal value is a violation is not a
// seam; so the field is configuration now and this package constructs the
// adapter. §26e point 3's "move the type rather than widen the gate", arriving
// as scheduled.
//
// The second field went the same way one slice later, and this doc used to say
// it would not. It read: "Placer's adapter is a venue client. Venue clients live
// outside this context entirely, which is marketdata.Deps.Resnapshotter's
// situation exactly, so that field is permanent." The premise was right and the
// conclusion did not follow — a venue CLIENT does live outside, but what the
// field asked for was not a venue client. It asked for something implementing
// §4's Place over an oms/domain.Order, which is a venue client wrapped in this
// context's own ACL adapter, and §4 assigns that adapter to the context. So the
// field now holds the venue client it always described, and the adapter is built
// below. See Deps.VenueOrders.
//
// # Neither field is refused HERE
//
// A root passing a nil venue client gets driven.ErrVenueOrderPlacerMissing from
// the adapter that owns the wire; an unset path gets driven.ErrJournalPathUnset
// from the adapter that owns the file — see New.
type Deps struct {
	// JournalPath is where §5.1's durable, append-only, hash-chained
	// write-ahead journal lives on this host.
	//
	// # A path rather than a ports.Journal, and what the root gave up
	//
	// What a root can no longer do is supply an implementation — including a
	// fast one that batches, which is what the port's durability contract
	// forbids in prose and could not enforce. The failure that shape invited is
	// the one nothing reports: no test fails, and the defect surfaces once,
	// during the crash the fsync existed for, when recovery finds no record of
	// an order the venue is holding. Configuration cannot express it.
	//
	// What a root retains is the decision that is genuinely deployment's:
	// WHERE. §11 makes this journal host-local and lost on host death, and §13
	// adds a precondition this constructor does not yet check — local
	// filesystem only, never NFS/network FS — which is scheduled to §7.5's boot
	// posture list (task #168). Until then, the value supplied here is the only
	// place that requirement is met or missed.
	JournalPath string

	// VenueOrders is the venue's order surface — a *venue.OrderClient, in
	// practice.
	//
	// # It used to be a ports.OrderPlacer, and the field could not stay that way
	//
	// The old shape asked a root for something implementing §4's Place over a
	// domain.Order, and there were exactly two ways to satisfy it. Either
	// engine/venue imports oms/domain — inverting the layering §4 sets up, so
	// that a shared venue client depends on one context's model — or the root
	// writes this context's ACL adapter itself, which puts the translation §4
	// assigns to the context in a composition root instead, where a second
	// binary would write it again and the two copies would drift.
	//
	// So the field became the VENUE client, and driven.OrderGateway is
	// constructed here, exactly as JournalPath's conversion went in slice B and
	// for the same reason: what a root legitimately owns is which venue it
	// trades on, not how this context talks to it.
	//
	// The submit-once rule still binds whatever is supplied. §5.3 forbids
	// transport retries, and an implementation that retries inside Place breaks
	// that as thoroughly as a caller that calls it twice — invisibly, because
	// the caller cannot see it happen. A root wiring a client with retries
	// configured has disabled this context's central safety property without
	// touching this context.
	VenueOrders driven.VenueOrderPlacer

	// VenueOrderReads is the venue's order READ surface — the same
	// *venue.OrderClient, in practice, and a second field rather than a widened
	// first one.
	//
	// The split is the write/read separation §5.3 requires, carried up to the
	// wiring: application.Resolver must not be able to place an order, so the
	// two adapters take two capabilities and a root that supplies one client
	// twice is expressing that the client can do both while each use case still
	// cannot.
	VenueOrderReads driven.VenueOrderReader

	// VenuePositions is the venue's POSITION surface — the same
	// *venue.OrderClient again, and a third field for VenueOrderReads' reason
	// one step further.
	//
	// §7.5's net-delta invariant is asserted against it, and the use case that
	// asserts it must not be able to place an order or to re-resolve one: an
	// unexplained position that a use case could go looking for explanations for
	// is one it can explain away. Three capabilities, three adapters, one client.
	VenuePositions driven.VenuePositionReader

	// PositionScopes are the enumerations §7.5's net-delta invariant is asserted
	// over: a venue, a derivative family and a settlement asset each.
	//
	// A root's business, and NOT derivable from InstrumentAssets, which is the
	// distinction worth stating because the two look like the same list. The
	// enumeration exists to find exposure on instruments this system has never
	// heard of, so a scope computed from the instruments it trades could not
	// widen past them — the foreign position would be outside every scope, and
	// the invariant would assert cleanly over the account that holds it.
	//
	// An empty list is legal and asserts nothing. It is not defaulted to "every
	// family in InstrumentAssets" for the reason above, and the omission is
	// visible rather than silent: application.Join reports every instrument it
	// could not assert against a position surface.
	PositionScopes []application.PositionScope

	// InstrumentAssets is the base/quote decomposition of every instrument this
	// deployment resolves orders for.
	//
	// It is a root's business because it is a statement about which instruments
	// this system trades. See driven.NewOrderResolver for why it cannot be
	// derived from the symbol, and for the construct (§5.1's SymbolSpec) that
	// will own it.
	InstrumentAssets map[ids.VenueInstrument]venue.InstrumentAssets

	// SpotBalances is the venue's spot balance surface — the same client, a
	// fourth capability, for VenuePositions' reason one surface over: §7.5's
	// spot assertion compares a wallet against a recorded baseline, and the
	// use case doing it must not be able to re-resolve an order or
	// re-enumerate positions to explain a delta away (task #179, record §70).
	//
	// Required exactly when SpotBaselines is non-empty; nil otherwise, and a
	// deployment with spot instruments and no baselines boots to a HOLDING
	// gate that names UnassertedBaselineUnrecorded rather than to a wiring
	// error — the first-record flow depends on that ordering (§70g).
	SpotBalances driven.SpotBalanceReader

	// SpotBaselines is the recorded not-ours portion of the venue's spot
	// balance, per base asset, handed here as already-verified DATA — the
	// fixture seam, for tests that pin a baseline without a document. The
	// module re-validates shape (validateSpotLeg); it cannot re-verify
	// provenance and does not pretend to. Mutually exclusive with
	// SpotBaselineDocument.
	SpotBaselines []application.SpotBaseline

	// SpotBaselineDocument is §5.2's `spot_baseline` pin plus the bindings
	// only the root knows: the transport credential's fingerprint and the
	// venue dialect. When set, New runs the verifying loader over the pinned
	// document — digest, chain mark against the journal it just opened,
	// fingerprint, venue/account, rows-vs-prefix, recomputation — and a
	// document that will not load FAILS construction: a pinned document that
	// cannot govern is a wiring defect, not a gate refusal (record §70g).
	//
	// The load happens HERE rather than in the root because the chain mark is
	// verified against the journal, and §3.6 rule 6 keeps the journal adapter
	// out of every root's reach.
	SpotBaselineDocument *SpotBaselineDocument

	// RecoveryReadPriority is the §7.6 shed class the order-read surface
	// (Resolve and the executions sweep) carries — a ROOT's statement since
	// task #179, where driven.OrderResolver pinned Safety on the argument that
	// recovery was its only caller. The reconcile boot states Safety (§5.3
	// names the resolving queries Safety traffic in as many words); `arblab
	// baseline record` states Routine, because a maintenance sweep must be
	// shed before the emergency reserve rather than out of it. The zero value
	// is refused by the adapter's constructor, naming this field.
	RecoveryReadPriority ratelimit.Priority

	// SettleWait and SpotSettle are the spot second look's two halves (record
	// §70c): the blocking wait the root supplies — time.Sleep in a binary, a
	// recording fake in a test, because this layer owns no ambient time — and
	// §5.2's `spot_settle` bound. Both-or-neither, refused half-wired; a
	// deployment that leaves them unwired and then meets a spot delta FAILS
	// the gate by name rather than latching on one look.
	SettleWait func(time.Duration)
	SpotSettle time.Duration

	// InstrumentFilters is each tradable instrument's published trading
	// filters — the lot grid, the tick grid and the two floors §2.4's
	// verification half refuses against, resolved by the root from the dated
	// reference revision.
	//
	// The value type is oms/domain's, which is the one vocabulary a root may
	// name (§3.6 rule 6 polices ports and driven; domain is deliberately out —
	// §5.1's pattern). A nil map is legal and is a deployment that PLANS
	// nothing: the planner refuses every leg with ErrFiltersUnknown. That is
	// InstrumentAssets' refuse-don't-guess rule pointed at the write path,
	// and it is why the reconcile-only root supplies nothing here and loses
	// nothing — while a root that forgets the field has built a binary whose
	// first plan says so, rather than one whose first plan reaches a venue
	// unchecked.
	InstrumentFilters map[ids.VenueInstrument]domain.TradingFilters

	// FilterMaxAge is the deployment's declared tolerance for the age of a
	// filters row at package admission (§69) — the same tolerance the
	// planning binary's boot check judges against its reference file, handed
	// here so the executor re-judges it at every package, because a
	// long-running process's boot-fresh map stays green while the venue moves.
	//
	// Required to be positive exactly when InstrumentFilters is non-empty, and
	// zero exactly when it is empty — the pair is one policy, and
	// application.ErrFilterMaxAgeInvalid names a root that supplies half of
	// it. The reconcile-only root supplies neither and loses nothing.
	FilterMaxAge time.Duration

	// Clock is the time seam §11 requires and §5.3's stable-not-found rule
	// depends on: the dispatcher stamps the authorization anchor in the venue's
	// frame, and the resolver measures both of the rule's time bounds from it.
	//
	// A root supplies it because live-vs-backtest interchangeability is what the
	// seam is FOR (§3.5) — this context has no basis for choosing which clock a
	// binary runs on, and an ambient one here would be the parity mechanism
	// quietly opted out of.
	Clock clock.Clock

	// Account is the venue subaccount this OMS instance trades — §5.1's first
	// hedge-group component, and the one an order record cannot carry.
	//
	// It is a root's business for InstrumentAssets' reason: it is a statement
	// about what this deployment is, not about how this context works. §5.1's
	// Order field list has no account in it, so nothing in the write path can
	// supply it and nothing in the record set can be checked against it — see
	// driven.MemoryOrderWriteRepository's own field, where the single-account
	// premise is asserted rather than assumed.
	//
	// Empty is refused by the repository. Both plausible defaults fail in the
	// concluding direction: an empty account makes every subaccount's orders one
	// hedge group, and a placeholder hides the mismatch until a second account
	// exists.
	Account string

	// Resolution is the local policy §5.3's rule needs and no venue publishes:
	// the propagation bound, the sweep's strictness margin, and the assumed
	// retention for a surface whose venue states none.
	//
	// application.ResolutionPolicy documents each field's normative source, and
	// SurfaceKey documents why the third cannot be defaulted in either
	// direction.
	Resolution application.ResolutionPolicy
}

// Module is the context's driving surface.
//
// # One field, and it is a use case rather than a driving adapter
//
// marketdata exposes driving.FundingDispatch and driving.BookDispatch, because
// its inbound edge is an events.Envelope that something has to translate. This
// context has no infrastructure/driving/ at all: §6's inbound edges are the
// private order and execution streams, which are a later slice, and driven/
// doc.go already records why an empty driving/ package is not created to
// satisfy a literal reading of §3.6 rule 3.
//
// So Dispatch is exported as what it is. Interposing an adapter that only
// forwards would be the same empty shape one directory over, and it would put a
// translation layer between a caller and the one method in this system whose
// signature is load-bearing — Dispatch's (Outcome, error) split exists so that
// `if err != nil { return }` cannot silently drop a live order, and a
// forwarding wrapper is one more place for that to be re-derived wrongly.
//
// The struct rather than a bare *application.Dispatcher return is the extension
// point: marketdata's Module went from one field to two when its book path
// landed, and slice B is that happening here.
//
// The two fields are ORDERED in the sense that matters: nothing Dispatch will
// accept exists until Plan has produced it, because Dispatch loads what it sends
// from the repository Plan fills. That is §6 steps 5 and 6 expressed as a
// dependency rather than as a convention.
type Module struct {
	// Plan is §6 step 5: deterministic ClientOrderID minting, and intents
	// fsynced to the write-ahead journal before any submission.
	Plan *application.Planner

	// Dispatch is §6 step 6's crash boundary: the one place in this system
	// where an order becomes a request at a venue.
	Dispatch *application.Dispatcher

	// Execute is §6 step 6's package half: a planned package's legs submitted
	// in parallel, each exactly once, refused whole or sent whole.
	//
	// It is a second path to the same crash boundary — its per-leg dispatchers
	// and Dispatch share ONE DispatchAuthority, constructed in New, so the
	// in-process double-dispatch backstop is per process rather than per path.
	// An order sent through either path and re-presented to the other is
	// refused even when the projection is the thing that broke (§59b).
	//
	// It takes the root's boot.Checklist per call, and the parameter is §6
	// step 7 living where forgetting is inexpressible: a sent leg whose apply
	// failed files OutcomeProjectionStale at the fan-in, and the admission
	// read refuses the next Opening package over the latch while a Safety
	// package still passes. The root's own verdict gate runs once per run;
	// this one runs per package.
	Execute *application.Executor

	// Resolve is §5.3's venue-specific stable-not-found rule: the only way an
	// order that reached the crash boundary is ever answered for, and — by
	// holding no placer — the reason "never resubmitted" is not expressible.
	Resolve *application.Resolver

	// Classify is §6.7, "the single gate for every compensating trade": the
	// determined-vs-ambiguous verdict over a hedge group's residual, and the
	// owner of §6.7's outstanding_reserved_reductions ledger.
	//
	// It is exposed because its clients are outside this context — §6 step 7's
	// one-leg-failure handler, §6 step 8's forced-close path, §7.4's kill-switch
	// and takeover — and every one of them is required to route through it. What
	// it can hand back is a verdict and, in the deadline-exceeded branch, one
	// authorized interval-floor magnitude; the order that executes either is
	// planned and dispatched through the two fields above, which is what keeps
	// the gate a gate rather than a second write path.
	Classify *application.Classifier

	// Recover is §7.5's reconcile-only boot, first half: the current-state
	// projection rebuilt from the write-ahead journal before any venue call.
	//
	// It is exposed because its caller is a composition root's reconcile phase
	// (boot.KindReconcile), which is outside this context — and because the
	// ORDER between it and the three fields above is the one a root cannot be
	// allowed to get wrong. Plan, Dispatch, Resolve and Classify all read the
	// projection this rebuilds; run before it, each of them reads an empty store
	// and answers confidently: Plan mints an id the journal already holds,
	// Dispatch refuses a live order as unknown, and Classify reports a hedge
	// group flat. That last one is the reason this is a field rather than
	// something New could do for itself — see New.
	Recover *application.Replayer

	// Join is §7.5's reconcile-only boot, second half: the replayed journal
	// joined to venue truth, and the net-delta invariant asserted before trading
	// resumes.
	//
	// It runs AFTER Recover and the ordering is not a preference. Recover
	// rebuilds state without quantity — no fill is journaled — so between the two
	// calls every order carries a zero cumulative, which Classify reads as a flat
	// group and a flat group is where compensating trades are permitted. Join is
	// what turns the venue's own record into that quantity, and boot.Checklist's
	// two separate gates are what hold the interval: GateJournalReplayed and
	// GateNetDeltaAsserted, reduced to the weakest posture over both.
	Join *application.Joiner

	// Delever is SM §7's `PackageDelever` ladder — core §7.1's escalation when
	// isolated-margin liq distance breaches and the top-up path fails.
	//
	// It is exposed with NO trigger behind it yet, and the asymmetry is stated
	// rather than left to be discovered: §7.1's liq-distance escalation is
	// milestone 4's other workstream, so nothing in this build calls Open. What
	// this field's existence buys today is the RECOVERY half — Reconcile
	// reinstates a generation a previous process left live, and it does that
	// through this value.
	//
	// A root that drives the ladder must call Reconcile FIRST, and the ordering
	// is the same one Recover states for the projection with one extra edge: the
	// ledger entry of a generation the log left live is re-Reserved by the boot,
	// so an Open before the boot would find §6.7's entry gate looking at a ledger
	// that is short exactly one entry — and pass.
	Delever *application.Deleverer

	// Reconcile is §7.5's reconcile-only boot as ONE call, and it is what a
	// composition root's boot.KindReconcile phase invokes.
	//
	// # Why it exists beside Recover, Join and Resolve rather than instead of them
	//
	// The three phases are separately meaningful — §7.5 gives each its own gate,
	// and a root that only wants to replay a journal for inspection should be able
	// to — but they are correct in exactly ONE order, and the order is not
	// recoverable from their signatures. §5.3's absence resolution runs BETWEEN
	// the join and the net-delta assertion, which no one of them can express and
	// which the fused Join-that-also-asserted had wrong. So the ordering is a use
	// case rather than a paragraph in a root's comment, where a second binary
	// would re-derive it and the two would differ.
	//
	// It reports into the root's checklist and does not own one — §7.5's roster
	// spans contexts and this one supplies three of its gates. See
	// application.Reconciler.
	Reconcile *application.Reconciler

	// journal is held only so Close can release it, and it is unexported for
	// the same reason ports.OrderWriteRepository never becomes a field here: a
	// *driven.SQLiteJournal handed to a root is a write path to order state
	// that singlewriter cannot see, because the root needs no import to use a
	// value it was given.
	journal *driven.SQLiteJournal

	// The record protocol's capabilities (task #179, record §70b): the
	// projection the document rows come from, the order-read adapter the
	// settle re-read rides, the balance client its Routine adapter is built
	// over, and the deployment identity the document binds. Unexported for
	// journal's reason — RecordSpotBaseline is the one consumer, and a root
	// drives the protocol rather than the parts.
	orders    *driven.MemoryOrderWriteRepository
	reader    *driven.OrderResolver
	balances  driven.SpotBalanceReader
	venue     ids.Venue
	account   string
	roster    []application.SpotInstrument
	baselines []application.SpotBaseline
}

// New assembles the context, refusing an incomplete one.
//
// It adds no refusals of its own, on marketdata.New's argument and for its
// reason: the constructors below each name what they are missing, phrased in
// terms of the wire it is about, and a second statement of the same rule here is
// the copy that drifts — it is always the one written where the value is merely
// passing through. What this function contributes is ORDER, not validation.
//
// The consequence a caller can rely on: an error from New is the constructor's
// own sentinel, so a root can branch on application.ErrPlacerMissing or
// driven.ErrJournalPathUnset directly rather than on a wrapped string.
//
// # This constructor now ACQUIRES something, which it never did before
//
// # It CONSTRUCTS the replayer and does not RUN it, and the split is §7.5's
//
// Rebuilding the projection here would make a wired module a recovered one,
// which reads as the safer shape and is the wrong one twice over. First,
// ordering: §3.2 puts reconcile-only boot last, after observability, so that a
// process dying in the journal join dies with a diagnosis rather than an exit
// code — and a root constructs this context wherever it likes. Second, and
// worse: New reports failure by returning an error, and boot.Run turns a failed
// phase into an unwind and a dead process. A journal that will not replay must
// leave a process that is ALIVE, halted, latched and audible to §7.4's
// watchdog, because the alternative — exiting — presents to that watchdog
// exactly as a dead host and escalates to an actuator takeover. The verdict is
// a posture, and a constructor has no way to report one.
//
// Opening the journal is a resource acquisition, and every refusal after it must
// release it or leak a file handle and a SQLite connection for the lifetime of
// the process. The refusals after it are real — a root with a good path and no
// placer is exactly the half-configured case — so each failure path joins
// journal.Close() into its error rather than discarding it. Joining rather than
// ignoring is deliberate: a Close that fails during a failed New is the one
// report nobody would otherwise get, since there is no Module to call Close on.
func New(deps Deps) (*Module, error) {
	journal, err := driven.OpenSQLiteJournal(deps.JournalPath)
	if err != nil {
		return nil, fmt.Errorf("oms: journal: %w", err)
	}

	// One repository, shared. See the package comment: two would make Plan and
	// Dispatch disagree about which orders exist — and, since slice E, would
	// make Classify compute a residual over a different set of orders from the
	// one Dispatch is adding to.
	orders, err := driven.NewMemoryOrderWriteRepository(deps.Account)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("oms: write repository: %w", err), journal.Close())
	}

	planner, err := application.NewPlanner(journal, orders, deps.InstrumentFilters)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("oms: planner: %w", err), journal.Close())
	}
	// §4's thin ACL adapter, constructed HERE rather than by a root — see
	// Deps.VenueOrders. It is the layer that refuses what §6 step 6 forbids
	// sending and translates the rest into engine/venue's vocabulary.
	placer, err := driven.NewOrderGateway(deps.VenueOrders)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("oms: order gateway: %w", err), journal.Close())
	}

	// ONE authority AND one filter policy for the process's every dispatch path
	// — the module's own dispatcher, the executor's per-leg dispatchers, and
	// SM §7's ladder. Two authorities would be two backstops each blind to the
	// other's sends (§59b); two filter policies would be two answers to "does
	// this order conform to the venue's grid" held by the two use cases that
	// admit orders at send time (#184). Both constructors make the second one
	// inexpressible: the executor and the deleverer are built FROM this
	// dispatcher and inherit both, so this seam cannot cross the wires however
	// it is edited.
	dispatcher, err := application.NewDispatcher(journal, placer, orders, deps.Clock,
		application.NewDispatchAuthority(), deps.InstrumentFilters, deps.FilterMaxAge)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("oms: dispatcher: %w", err), journal.Close())
	}

	// The same gateway is narrowed into placement and reservation capabilities;
	// reserve-here/place-there cannot be wired by the root.
	executor, err := application.NewExecutor(dispatcher, placer)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("oms: executor: %w", err), journal.Close())
	}

	// The read half's ACL, beside the write half's and never merged with it.
	reader, err := driven.NewOrderResolver(deps.VenueOrderReads, deps.InstrumentAssets, deps.RecoveryReadPriority)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("oms: order resolver gateway: %w", err), journal.Close())
	}
	// The same repository, handed over a second time as the read capability
	// gate 3 rules sweep rows out against — the journal's two-halves pattern,
	// on the store this time.
	resolver, err := application.NewResolver(journal, orders, reader, orders, deps.Clock, deps.Resolution)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("oms: resolver: %w", err), journal.Close())
	}

	// The classifier takes the SAME repository, through a different port. It is
	// the one use case here that must never write, and ports.HedgeGroupOrders is
	// what makes that structural rather than reviewed — see that port's doc.
	classifier, err := application.NewClassifier(orders)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("oms: classifier: %w", err), journal.Close())
	}

	// The journal's read half, into the same repository every other use case
	// writes through. The same *SQLiteJournal satisfies both ports and is handed
	// over as two different capabilities: the planner and the dispatcher cannot
	// replay it, and the replayer cannot append to it.
	replayer, err := application.NewReplayer(journal, orders)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("oms: replayer: %w", err), journal.Close())
	}

	// The position surface's ACL, and the third capability taken off the same
	// venue client. The joiner holds the read gateway too — the SAME adapter the
	// resolver holds, because the join reads orders and executions through
	// exactly the port §5.3 already narrowed to the two methods recovery may use.
	positions, err := driven.NewVenuePositions(deps.VenuePositions, deps.InstrumentAssets)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("oms: venue positions gateway: %w", err), journal.Close())
	}
	// The spot leg's roster is derived from the instrument table — the
	// validated authority on base assets — while the SCOPES stay a root's
	// explicit business. The asymmetry is deliberate: a position scope exists
	// to find exposure on instruments nothing here knows about, so deriving it
	// would cap it at what we know; the spot roster exists to make every
	// instrument we DO know answered for, which is exactly a derivation.
	// The venue is the instrument table's — NewVenuePositions above has
	// already refused a table naming two, so the first key's venue is a
	// well-defined function of validated input, not a map-order sample.
	var moduleVenue ids.Venue
	for instrument := range deps.InstrumentAssets {
		moduleVenue = instrument.Venue
		break
	}
	baselines := deps.SpotBaselines
	if deps.SpotBaselineDocument != nil {
		if len(deps.SpotBaselines) > 0 {
			// Two authorities for one slot: the data seam is for fixtures, the
			// pin is for deployments, and a caller supplying both has two
			// answers to "which history is this baseline about".
			return nil, errors.Join(ErrSpotBaselineDoubleSourced, journal.Close())
		}
		loaded, err := baseline.Load(deps.SpotBaselineDocument.Pin, journal, baseline.Bindings{
			Venue:       moduleVenue,
			Account:     deps.Account,
			Fingerprint: deps.SpotBaselineDocument.Fingerprint,
			Dialect:     deps.SpotBaselineDocument.Dialect,
		})
		if err != nil {
			// A pinned document that will not load is a wiring defect, not a
			// gate refusal (record §70g): the boot ERRORS rather than holding.
			return nil, errors.Join(fmt.Errorf("oms: spot baseline document: %w", err), journal.Close())
		}
		baselines = loaded
	}
	spot := application.SpotLeg{Baselines: baselines}
	for instrument, assets := range deps.InstrumentAssets {
		if instrument.Product != ids.ProductSpot {
			continue
		}
		spot.Roster = append(spot.Roster, application.SpotInstrument{
			Instrument: instrument,
			Base:       assets.Base,
			Quote:      assets.Quote,
		})
	}
	if deps.SpotBalances != nil {
		// The boot side rides Safety: §7.6 names §7.5's reconcile in the
		// Safety row, and an assertion shed under load is an assertion that
		// does not happen. The record command constructs its OWN adapter at
		// Routine — the two rungs are the reason the priority is a constructor
		// argument at all.
		balances, err := driven.NewSpotBalances(deps.SpotBalances, moduleVenue, ratelimit.PrioritySafety)
		if err != nil {
			return nil, errors.Join(fmt.Errorf("oms: spot balances gateway: %w", err), journal.Close())
		}
		spot.Balances = balances
	}
	joiner, err := application.NewJoiner(reader, positions, orders, deps.Clock, deps.Account, spot, deps.PositionScopes...)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("oms: joiner: %w", err), journal.Close())
	}

	// SM §7's `PackageDelever` ladder, and the FIRST composition root to
	// construct one (task #204). Nothing here TRIGGERS it — core §7.1's
	// liq-distance escalation is milestone 4's other workstream and does not call
	// Open — so what this wiring buys is the recovery half: the boot below can
	// reinstate a generation a previous process left live, and the gate that
	// refuses when it cannot is now reachable rather than standing unused.
	//
	// It takes the SAME classifier and the SAME dispatcher the fields above
	// expose, on the reconciler's rule one type over: a ladder holding a private
	// classifier would amend a private ledger, which is FM-5's failure with the
	// bookkeeping done correctly.
	deleverer, err := application.NewDeleverer(dispatcher, classifier, journal)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("oms: deleverer: %w", err), journal.Close())
	}

	// §7.5's generation-scope rebuild. It reaches the ladder through the
	// interface and the projection through the READ port — see application's
	// GenerationLadder for why the capability handed over is three whole acts of
	// the ladder rather than a Classifier.
	reinstater, err := application.NewReinstater(deleverer, orders)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("oms: reinstater: %w", err), journal.Close())
	}

	// §7.5's phases in the one order they are correct in. It holds the same use
	// cases the fields above expose, not copies: a reconcile that ran against a
	// second replayer would rebuild a second projection and assert the invariant
	// over neither of them.
	reconciler, err := application.NewReconciler(replayer, joiner, resolver, reinstater,
		application.SecondLook{Wait: deps.SettleWait, Settle: deps.SpotSettle})
	if err != nil {
		return nil, errors.Join(fmt.Errorf("oms: reconciler: %w", err), journal.Close())
	}

	return &Module{
		Plan:      planner,
		Dispatch:  dispatcher,
		Execute:   executor,
		Resolve:   resolver,
		Classify:  classifier,
		Recover:   replayer,
		Join:      joiner,
		Delever:   deleverer,
		Reconcile: reconciler,
		journal:   journal,
		orders:    orders,
		reader:    reader,
		balances:  deps.SpotBalances,
		venue:     moduleVenue,
		account:   deps.Account,
		roster:    spot.Roster,
		baselines: baselines,
	}, nil
}

// Close releases what New acquired.
//
// The journal is the only closeable thing this context owns; the repository is a
// map. It is a method on Module rather than a returned func because the caller
// that must not forget it is a composition root holding the Module — and a
// cleanup function is a value a root can drop, while a missing Close on a type
// it stores is a shape review notices.
//
// It does NOT stop in-flight work, and the distinction matters for a root
// writing a shutdown sequence: closing the journal while a Dispatch is between
// its authorization fsync and its Place produces an order sent with no result
// record. Ordering the shutdown so that nothing is dispatching belongs to the
// root, and §7.5's boot is what makes the surviving journal enough.
func (m *Module) Close() error { return m.journal.Close() }
