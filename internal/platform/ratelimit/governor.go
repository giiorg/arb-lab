package ratelimit

import (
	"fmt"
	"sync"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// Request is one venue call, described well enough to charge it.
type Request struct {
	// Family is the API family the call goes to. It must be one the profile
	// declares — see ErrFamilyUnknown for why an undeclared one is refused
	// rather than treated as "no family budget applies".
	Family ids.ProductCategory

	// Priority is what the call is for, and therefore which ceiling it may
	// spend up to.
	Priority Priority

	// Cost is what the call charges.
	Cost Cost
}

// Validate reports whether the request is well enough formed to price.
//
// Family first, because a request naming no family cannot be charged at all,
// and the remaining two fields are policy questions about a call we could not
// route in the first place.
func (r Request) Validate() error {
	if err := r.Family.Validate(); err != nil {
		return err
	}
	if err := r.Priority.Validate(); err != nil {
		return err
	}
	return r.Cost.Validate()
}

// Governor is one venue's rate-limit state: its budgets, and any pause in force.
//
// It is safe for concurrent use. §3.2 puts execution on per-venue worker
// goroutines, and the recovery paths of §6.7 and §7.1 run concurrently with
// them by design — that concurrency is the reason the reserve exists, so a
// governor that assumed a single caller would be defending against the wrong
// thing.
//
// It owns no goroutine and no ticker. Windows are evicted lazily on the next
// call. See the package doc.
type Governor struct {
	mu sync.Mutex

	clk     clock.Clock
	profile Profile

	// windows holds one counter per limit; order is the profile's own ordering,
	// so Snapshot and the choice of which limit a refusal names are both
	// deterministic.
	windows map[limitKey]*window
	order   []limitKey

	// applicable is the pre-resolved set of windows a request from each family
	// must clear: the venue-scoped ones plus that family's own.
	applicable map[ids.ProductCategory][]*window

	// counted is, per family, the set of dimensions some applicable window
	// counts — what decide's cannot-count refusal reads. Precomputed beside
	// applicable because they must agree: a dimension in this set and absent
	// from that family's windows (or the reverse) would let a charge pass the
	// check and then land nowhere.
	counted map[ids.ProductCategory]map[Dimension]bool

	pausedUntil time.Time
	pauseKind   PenaltyKind
	bans        uint64
	throttles   uint64
}

// New builds a governor over a validated profile.
func New(c clock.Clock, p Profile) (*Governor, error) {
	if c == nil {
		return nil, ErrClockNil
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}

	g := &Governor{
		clk:        c,
		profile:    p,
		windows:    make(map[limitKey]*window, len(p.Limits)),
		order:      make([]limitKey, 0, len(p.Limits)),
		applicable: make(map[ids.ProductCategory][]*window, len(p.Families)),
		counted:    make(map[ids.ProductCategory]map[Dimension]bool, len(p.Families)),
	}
	for _, l := range p.Limits {
		k := l.key()
		g.windows[k] = newWindow(l)
		g.order = append(g.order, k)
	}
	for _, f := range p.Families {
		var ws []*window
		dims := make(map[Dimension]bool)
		for _, k := range g.order {
			w := g.windows[k]
			if w.limit.Scope == ScopeVenue || w.limit.Family == f {
				ws = append(ws, w)
				dims[w.limit.Dimension] = true
			}
		}
		g.applicable[f] = ws
		g.counted[f] = dims
	}
	return g, nil
}

// Check answers whether a request would be admitted, without charging it.
//
// This is §6.3's question: `risk.Gate` re-validates rate-limit budget among the
// preconditions of a trade, and a call that spent the budget in order to report
// on it would make the gate itself the thing that exhausts the venue.
//
// It is ADVISORY. Only Acquire is atomic with the charge, so a Check that says
// yes can still be followed by an Acquire that says no if another goroutine got
// there first. That is the correct weakness for a precondition check: the trade
// path calls Acquire before it sends, and that call is the one that decides.
func (g *Governor) Check(r Request) (Decision, error) {
	return g.decide(r, false)
}

// Acquire decides and, if admitted, charges every applicable budget.
//
// A refusal charges NOTHING. Partial charging would mean a request the order
// budget turned away had still spent the weight budget that admitted it, so a
// caller retrying in a loop would drain budgets on requests it never got to
// send — and would do it fastest exactly when the venue is most constrained.
//
// The charge is not released if the request is then abandoned or fails in
// transport. That is correct rather than convenient: the venue counted the call
// the moment it arrived, and a governor that gave budget back on a timeout
// would hand back budget the venue is still holding against us.
func (g *Governor) Acquire(r Request) (Decision, error) {
	return g.decide(r, true)
}

func (g *Governor) decide(r Request, charge bool) (Decision, error) {
	if err := r.Validate(); err != nil {
		return Decision{}, err
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	ws, ok := g.applicable[r.Family]
	if !ok {
		return Decision{}, fmt.Errorf("venue %q family %q: %w",
			g.profile.Venue, r.Family, ErrFamilyUnknown)
	}

	// §7.6's headline sentence, enforced on the actual cost: a governor MUST
	// refuse a cost it cannot count. Profile.Validate already made every
	// caller-charged dimension either budgeted or written down as Uncounted, so
	// this fires only under such a declaration — and the declaration was a
	// claim that no such cost arrives, which the cost in hand has just
	// falsified. The refusal is an error rather than a Decision because it is a
	// profile-versus-adapter disagreement, like ErrFamilyUnknown: no amount of
	// waiting changes it. DimensionRequests is exempt for the reason it is
	// everywhere else in this file: the venue envelope is unconditional at
	// construction, so its window cannot be absent here.
	for d := DimensionUnspecified + 1; d <= maxDimension; d++ {
		if d == DimensionRequests {
			continue
		}
		if r.Cost.units(d) > 0 && !g.counted[r.Family][d] {
			return Decision{}, fmt.Errorf("venue %q family %q: cost charges %s: %w",
				g.profile.Venue, r.Family, d, ErrCostUncounted)
		}
	}

	now := g.clk.Now()
	d := Decision{Venue: g.profile.Venue, At: now}

	// A pause outranks every budget, and it outranks PrioritySafety too. It is
	// not a policy refusal: the venue has already declined, so issuing the
	// request would not perform the action, and on Binance it would lengthen
	// the ban we are waiting out.
	if now.Before(g.pausedUntil) {
		d.Reason = ReasonPaused
		d.NotBefore = g.pausedUntil
		return d, nil
	}

	refused := false
	for _, w := range ws {
		units := r.Cost.units(w.limit.Dimension)
		ceiling := w.limit.Ceiling(r.Priority)
		if w.admits(now, units, ceiling) {
			continue
		}
		at := w.freeAt(now, units, ceiling)
		if !refused || worseThan(at, d.NotBefore) {
			d.NotBefore = at
			d.Limit = w.limit.Name
		}
		refused = true
	}
	if refused {
		d.Reason = ReasonCeiling
		return d, nil
	}

	d.Admitted = true
	d.Reason = ReasonAdmitted
	if charge {
		for _, w := range ws {
			w.charge(now, r.Cost.units(w.limit.Dimension))
		}
	}
	return d, nil
}

// Observe folds a venue-reported consumption into a budget: the header-aware
// half of §7.6.
//
// name is the limit's name in the profile, not the venue's header name — the
// mapping from `X-MBX-USED-WEIGHT-1M` is venue knowledge and belongs in the
// adapter. An empty family addresses a venue-scoped limit, which cannot collide
// with a real family because ids.ProductCategory refuses the empty string.
//
// The reading is folded in UPWARD ONLY; see the package doc for the argument.
//
// An unknown limit is an error rather than a silent no-op. A reading nobody can
// place is a header the adapter parses and the profile does not declare — a
// budget going entirely uncounted, which is the failure that looks exactly like
// everything working.
func (g *Governor) Observe(family ids.ProductCategory, name string, used uint64) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if family != "" {
		if _, ok := g.applicable[family]; !ok {
			return fmt.Errorf("venue %q family %q: %w", g.profile.Venue, family, ErrFamilyUnknown)
		}
	}
	w, ok := g.windows[limitKey{family: family, name: name}]
	if !ok {
		return fmt.Errorf("venue %q family %q limit %q: %w",
			g.profile.Venue, family, name, ErrLimitUnknown)
	}
	w.observe(g.clk.Now(), used)
	return nil
}

// Penalize records a venue refusal and holds the venue off.
//
// The pause is venue-wide, across every family, and that is §7.6's own hedge
// rather than a simplification: both named ban tracks are IP-level, and the
// spec states that IP-ban-track separation between Binance's spot and futures
// budgets is "inferred, not documented". Scoping a pause to the family that
// earned it would encode exactly the separation the spec declines to assume.
//
// The new pause never shortens one already in force, and a milder kind never
// downgrades a more severe one that is still running. Both are the same ratchet
// the header reading gets: releasing early costs a ban, and holding too long
// costs throughput.
func (g *Governor) Penalize(p Penalty) error {
	if err := p.Validate(); err != nil {
		return err
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	switch p.Kind {
	case PenaltyBan:
		g.bans++
	case PenaltyThrottle:
		g.throttles++
	case PenaltyUnspecified:
		// Counted as neither, and named rather than left to fall out (§3.6
		// rule 10). Unreachable: p.Validate above refuses the zero kind. A
		// penalty kind added later must decide which counter it moves, because
		// these two feed the operator's view of why a venue is paused.
	}

	hold := g.profile.ThrottlePause
	if p.Kind == PenaltyBan {
		hold = g.profile.BanPause
	}
	if p.RetryAfter > hold {
		hold = p.RetryAfter
	}

	// Whether a pause was already running is read BEFORE the new one is
	// applied, and it has to be. Read afterwards it is a constant: hold is
	// always positive, so pausedUntil is always in the future by the time the
	// question is asked — the test would be dead code, and its purpose would
	// silently move to the severity comparison, where a Ban that expired last
	// week would keep outranking the Throttle running now.
	now := g.clk.Now()
	wasPaused := now.Before(g.pausedUntil)

	if until := now.Add(hold); until.After(g.pausedUntil) {
		g.pausedUntil = until
	}
	if !wasPaused || p.Kind.moreSevereThan(g.pauseKind) {
		g.pauseKind = p.Kind
	}
	return nil
}

// LimitState is one budget as it stands.
type LimitState struct {
	Name      string
	Family    ids.ProductCategory
	Scope     Scope
	Dimension Dimension
	Interval  time.Duration

	// Used is the consumption inside the current window, including anything a
	// venue reading ratcheted in.
	Used uint64

	OpeningCeiling uint64
	RoutineCeiling uint64
	Capacity       uint64

	// Reserve is the band only PrioritySafety may spend — §7.1's reserved
	// allocation, carried here because it is what an operator sizes against a
	// real incident's query volume.
	Reserve uint64
}

// Snapshot is the governor's state for §10's metrics and for §7.6's alerts.
type Snapshot struct {
	Venue ids.Venue

	// Paused and PausedUntil describe a hold in force now. PauseKind is
	// PenaltyUnspecified when nothing is paused: it describes the current
	// pause, not the last one that ever happened.
	Paused      bool
	PausedUntil time.Time
	PauseKind   PenaltyKind

	// Bans and Throttles are never reset. Bans is the counter §7.6's
	// "surface 418 / IP ban as immediate operator alerts" is raised from — this
	// package classifies the penalty and the composition root delivers the
	// alert, the same division apperror draws.
	Bans      uint64
	Throttles uint64

	// Limits are in profile order, so a metric exporter walking them produces a
	// stable series order across scrapes.
	Limits []LimitState
}

// Snapshot reads the current state.
//
// It evicts expired charges as it goes, like every other entry point, so a
// scrape of an idle process reports a budget that has actually decayed rather
// than the last one anybody spent.
func (g *Governor) Snapshot() Snapshot {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := g.clk.Now()
	s := Snapshot{
		Venue:       g.profile.Venue,
		Paused:      now.Before(g.pausedUntil),
		PausedUntil: g.pausedUntil,
		Bans:        g.bans,
		Throttles:   g.throttles,
		Limits:      make([]LimitState, 0, len(g.order)),
	}
	if s.Paused {
		s.PauseKind = g.pauseKind
	}
	for _, k := range g.order {
		w := g.windows[k]
		s.Limits = append(s.Limits, LimitState{
			Name:           w.limit.Name,
			Family:         w.limit.Family,
			Scope:          w.limit.Scope,
			Dimension:      w.limit.Dimension,
			Interval:       w.limit.Interval,
			Used:           w.usedAt(now),
			OpeningCeiling: w.limit.OpeningCeiling,
			RoutineCeiling: w.limit.RoutineCeiling,
			Capacity:       w.limit.Capacity,
			Reserve:        w.limit.Reserve(),
		})
	}
	return s
}
