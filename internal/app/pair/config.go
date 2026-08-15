package pair

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/giiorg/arb-lab/internal/app/reconcile"
	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/config"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// Config is the cross-venue root's document: TWO §5.2 OMS blocks, and the one
// pair they jointly execute.
//
// # Why two blocks rather than one with two venues
//
// reconcile.Config's own comment settles it: "Venue is §5.2's per-venue block.
// ONE, singular … oms.Deps takes one venue client, so a second block here would
// have nowhere to go. A deployment trading two venues runs two of these,
// against two journals." So a cross-venue package is not a configuration of the
// existing binary (record §95 §3); it is this root, holding two of them.
//
// The schema is reconcile's, unchanged and mounted under a name — exercise's
// rule and its reason: two schemas for one block is the copy that drifts, and
// the drift would surface as a run that reconciles differently from the
// recovery run over the same journal.
//
// [SPEC EDIT LANDED — v0.9.71] §12 item 7 described strategy #2's execution
// half as "`strategy/crossvenue` live wiring over the milestone-3 OMS +
// milestone-4 controls". "Live wiring" reads as connection work and said
// neither that the composition root is single-venue by design nor that the
// hedge group cannot hold two venues. §5.2's OMS-deployment-block bullet now
// states that a cross-venue package is TWO blocks under one root, with the two
// refusals below as its rules; §12 item 7 carries the scope correction. The
// normative re-scope of §5.1/§6.7 is still task #241's and is not this.
type Config struct {
	// Sides are the two deployments, exactly two, on two venues, over two
	// journals. See ErrSideVenueDuplicate and ErrSideJournalShared: neither
	// rule is tidiness, and both are load-bearing for the shared trade id.
	Sides []SideConfig `json:"sides"`

	// Pair is the package these two sides jointly execute.
	Pair PairConfig `json:"pair"`

	// Reference names the committed reference revision both sides resolve their
	// §2.4 trading filters from, and how old a row may be. ONE revision for
	// both venues, deliberately: the two legs' grids are compared against each
	// other implicitly every time a fill on one is sized as an order on the
	// other, and two revisions of different ages would make that comparison
	// span two observation instants.
	Reference ReferenceConfig `json:"reference"`
}

// SideConfig is one deployment: §5.2's block exactly as `arblab reconcile run`
// reads it.
type SideConfig struct {
	OMS reconcile.Config `json:"oms"`
}

// ReferenceConfig locates the reference revision and bounds its age.
type ReferenceConfig struct {
	Path   string          `json:"path"`
	MaxAge config.Duration `json:"max_age"`
}

// Mode selects the pair's execution surface.
type Mode string

const (
	// ModeSequential opens leg A, establishes its exposure, and only then opens
	// leg B — record §95 §7 alternative (ii).
	ModeSequential Mode = "sequential"

	// ModePaired dispatches both legs concurrently and composes the two
	// readings at the fan-in.
	ModePaired Mode = "paired"
)

// PairConfig is one cross-venue package.
type PairConfig struct {
	// ID is the ids.TradeID BOTH legs plan under, and the sharing is the
	// design (D2). ClientOrderID encodes (Trade, Leg, Attempt) and carries no
	// venue, so both legs mint the same id — which is safe because no store
	// spans venues, and which is what makes "this leg has a partner elsewhere"
	// durable in both journals and on both venues' order pages with no change
	// to the kernel. The two refusals that hold it up are
	// ErrSideVenueDuplicate and ErrSideJournalShared.
	ID string `json:"id"`

	// Attempt distinguishes a replan of this pair. It is bounded BELOW the id
	// format's ceiling because the unwind mints Attempt+1 (D7).
	Attempt int `json:"attempt"`

	// Mode is required because sequential and paired execution have different
	// failure surfaces. A default would make that choice on an absent field.
	Mode Mode `json:"mode"`

	// Settle and SettleAttempts bound the exposure reading. After a leg's IOC
	// this root re-joins that leg to venue truth until the order is terminal
	// for state, waiting Settle between attempts and giving up after
	// SettleAttempts — at which point the leg is INDETERMINATE and the run
	// stops for a human.
	//
	// Both are required and neither may be zero. A zero here would make the
	// first read the only read, and the first read after an IOC is exactly when
	// a venue has not yet published the trade rows — which is the reading that
	// reports a filled leg as unfilled.
	Settle         config.Duration `json:"settle"`
	SettleAttempts int             `json:"settle_attempts"`

	// Legs are the pair's two orders, one per side. The FIRST is the leg this
	// root opens first and the only one it will unwind (D8); order in this
	// slice is therefore a decision, not a presentation.
	Legs []LegConfig `json:"legs"`
}

// LegConfig is one leg, bound to its side by venue.
type LegConfig struct {
	// Venue selects the side. There is no separate side name: the OMS block
	// already carries exactly one venue and the two must differ, so the venue
	// IS the side's identity and a leg cannot be bound to the wrong block by a
	// label going stale.
	Venue    ids.Venue           `json:"venue"`
	Product  ids.ProductCategory `json:"product"`
	Contract ids.ContractType    `json:"contract,omitempty"`
	Symbol   string              `json:"symbol"`

	// Side is "buy" or "sell". The two legs must differ — a pair whose legs are
	// both buys is not a hedge, and every verdict would report it intact.
	Side string `json:"side"`

	// Quantity is the ordered size in the BASE asset for the FIRST leg opened.
	// The second leg is sized to the first's ESTABLISHED FILL (D5), so its
	// quantity here is the operator's statement of intent and the ceiling the
	// document was validated against — never what is actually sent.
	Quantity string `json:"quantity"`

	// LimitPrice is §6 step 6's marketable limit for the opening order.
	LimitPrice string `json:"limit_price"`

	// UnwindLimitPrice is the marketable limit for the reduce-only close of
	// this leg. REQUIRED on every leg, and the requirement is fail-closed: an
	// operator who cannot state the price at which they would close a leg is
	// not ready to open it, and a defaulted unwind price is a compensating
	// order priced by whoever wrote the default.
	UnwindLimitPrice string `json:"unwind_limit_price"`
}

// maxPairAttempt is ids' own attempt ceiling less the one the unwind mints.
//
// The document is refused HERE rather than at the moment the unwind is planned,
// which is after leg A is live at a venue: an id that cannot be minted then is
// an exposed leg this root has no way to close.
const maxPairAttempt = 99

// errUnwindQuantityDeferred reports an unwind intent asked for before any fill
// exists — Validate's own call, which checks the leg's PRICE and side without a
// quantity in hand. It never reaches an operator.
var errUnwindQuantityDeferred = errors.New("pair: the unwind's quantity is not known yet")

// Validate reports whether the document describes a runnable pair.
//
// Sides are checked before the pair, because a leg cannot be resolved against a
// side list that does not hold.
func (c Config) Validate() error {
	if len(c.Sides) != 2 {
		return fmt.Errorf("sides: %d declared: %w", len(c.Sides), ErrSideCount)
	}
	for i := range c.Sides {
		if err := c.Sides[i].OMS.Validate(); err != nil {
			return fmt.Errorf("sides[%d].oms: %w", i, err)
		}
	}
	if c.Sides[0].OMS.Venue.Venue == c.Sides[1].OMS.Venue.Venue {
		return fmt.Errorf("sides both name venue %s: %w", c.Sides[0].OMS.Venue.Venue, ErrSideVenueDuplicate)
	}
	shared, err := sameJournal(c.Sides[0].OMS.JournalPath, c.Sides[1].OMS.JournalPath)
	if err != nil {
		return fmt.Errorf("resolve side journal identity: %w", err)
	}
	if shared {
		return fmt.Errorf("sides name one physical journal (%s, %s): %w",
			c.Sides[0].OMS.JournalPath, c.Sides[1].OMS.JournalPath, ErrSideJournalShared)
	}

	if c.Reference.Path == "" {
		return fmt.Errorf("reference.path: %w", ErrReferenceFieldUnset)
	}
	if c.Reference.MaxAge.Duration() <= 0 {
		return fmt.Errorf("reference.max_age: %w", ErrReferenceFieldUnset)
	}

	if c.Pair.ID == "" {
		return fmt.Errorf("pair.id: %w", ErrPairFieldUnset)
	}
	if _, err := ids.ParseTradeID(c.Pair.ID); err != nil {
		return fmt.Errorf("pair.id: %v: %w", err, ErrPairFieldInvalid)
	}
	if c.Pair.Attempt < 0 || c.Pair.Attempt >= maxPairAttempt {
		return fmt.Errorf("pair.attempt %d (0..%d; the unwind mints attempt+1): %w",
			c.Pair.Attempt, maxPairAttempt-1, ErrPairFieldInvalid)
	}
	if c.Pair.Mode == "" {
		return fmt.Errorf("pair.mode: %w", ErrPairFieldUnset)
	}
	if c.Pair.Mode != ModeSequential && c.Pair.Mode != ModePaired {
		return fmt.Errorf("pair.mode %q: %w", c.Pair.Mode, ErrPairFieldInvalid)
	}
	if c.Pair.Settle.Duration() <= 0 {
		return fmt.Errorf("pair.settle: %w", ErrPairFieldUnset)
	}
	if c.Pair.SettleAttempts <= 0 {
		return fmt.Errorf("pair.settle_attempts: %w", ErrPairFieldUnset)
	}
	if len(c.Pair.Legs) != 2 {
		return fmt.Errorf("pair.legs: %d declared: %w", len(c.Pair.Legs), ErrLegCount)
	}
	if c.Pair.Legs[0].Venue == c.Pair.Legs[1].Venue {
		return fmt.Errorf("pair.legs both name venue %s: %w", c.Pair.Legs[0].Venue, ErrLegCount)
	}
	for i := range c.Pair.Legs {
		if c.Pair.Legs[i].UnwindLimitPrice == "" {
			return fmt.Errorf("pair.legs[%d].unwind_limit_price: %w", i, ErrPairFieldUnset)
		}
		if _, err := c.legIntent(i); err != nil {
			return err
		}
		// The unwind is checked at load for the same reason its price is
		// required at load: every field it needs but the quantity exists now,
		// and the moment it is discovered missing is otherwise the moment a leg
		// is live at a venue with no way to close it.
		if _, err := c.unwindIntent(i, fixedpoint.Quantity{}); err != nil && !errors.Is(err, errUnwindQuantityDeferred) {
			return err
		}
	}
	a, err := c.legIntent(0)
	if err != nil {
		return err
	}
	b, err := c.legIntent(1)
	if err != nil {
		return err
	}
	if a.Side == b.Side {
		return fmt.Errorf("pair.legs are both %q: %w", c.Pair.Legs[0].Side, ErrLegSideSame)
	}
	if a.Quantity.Asset() != b.Quantity.Asset() {
		return fmt.Errorf("pair.legs bases %s and %s: %w",
			a.Quantity.Asset(), b.Quantity.Asset(), ErrLegBaseMismatch)
	}
	return nil
}

// sameJournal reports whether two configured paths name one physical journal.
//
// It compares IDENTITY rather than spelling, because §5.1's journal is a
// single-writer store and two oms.Modules over one file is two writers holding
// the two legs' identical ClientOrderIDs. String equality was the shallower
// check and it misses three real cases: a relative and an absolute spelling of
// one path, a symlinked parent, and two names already hard-linked to one inode.
func sameJournal(a, b string) (bool, error) {
	pa, ia, err := journalIdentity(a)
	if err != nil {
		return false, err
	}
	pb, ib, err := journalIdentity(b)
	if err != nil {
		return false, err
	}
	if ia != nil && ib != nil && os.SameFile(ia, ib) {
		return true, nil
	}
	return pa == pb, nil
}

// journalIdentity resolves the PARENT directory and rejoins the base name.
//
// # Resolving the whole path is the version that cannot fire
//
// filepath.EvalSymlinks lstats every component INCLUDING the last, so on a
// database that does not exist yet it returns an IsNotExist error and no
// resolved path — measured, not assumed. A version that swallowed that error
// and kept the unresolved path would leave `<dir>/link/a.db` and
// `<dir>/real/a.db` comparing unequal, so the symlink refusal would never fire
// on a CLEAN FIRST BOOT, which is the only boot where the journal is absent and
// therefore the ordinary case. The gate would pass its own fixture only if that
// fixture pre-created the databases — the test would be green and the rule dead.
//
// The parent must exist, and a missing parent is returned as an error rather
// than absorbed: a journal directory nobody created is a document to fix, and
// treating it as "a path unlike any other path" is how two sides come to share
// one journal because neither could be resolved.
func journalIdentity(path string) (string, os.FileInfo, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", nil, err
	}
	dir, base := filepath.Split(abs)
	resolvedDir, err := filepath.EvalSymlinks(filepath.Clean(dir))
	if err != nil {
		return "", nil, fmt.Errorf("resolve journal directory %s: %w", dir, err)
	}
	final := filepath.Join(resolvedDir, base)
	info, err := os.Stat(final)
	if os.IsNotExist(err) {
		return final, nil, nil // a first boot creates it; that is not an alias
	}
	if err != nil {
		return "", nil, err
	}
	return final, info, nil
}

// sideFor finds the side index a venue names.
func (c Config) sideFor(v ids.Venue) (int, bool) {
	for i := range c.Sides {
		if c.Sides[i].OMS.Venue.Venue == v {
			return i, true
		}
	}
	return 0, false
}

// legIntent resolves one leg against ITS OWN SIDE's instrument table.
//
// It is the shared half of Validate, openPlan and unwindPlan — exercise's rule,
// which is what makes a document that validated unable to fail at planning for
// a reason validation could have named.
func (c Config) legIntent(i int) (application.LegIntent, error) {
	leg := c.Pair.Legs[i]
	side, ok := c.sideFor(leg.Venue)
	if !ok {
		return application.LegIntent{}, fmt.Errorf("pair.legs[%d] names venue %s: %w",
			i, leg.Venue, ErrLegVenueUnknown)
	}
	instrument := ids.VenueInstrument{
		Venue: leg.Venue, Product: leg.Product, Contract: leg.Contract, Symbol: leg.Symbol,
	}
	row, ok := c.instrumentRow(side, instrument)
	if !ok {
		return application.LegIntent{}, fmt.Errorf("pair.legs[%d] names %s: %w",
			i, instrument, ErrLegInstrumentUnknown)
	}
	s, err := parseSide(i, leg.Side)
	if err != nil {
		return application.LegIntent{}, err
	}
	quantity, err := fixedpoint.ParseQuantity(row.Base, leg.Quantity)
	if err != nil {
		return application.LegIntent{}, fmt.Errorf("pair.legs[%d].quantity: %v: %w", i, err, ErrPairFieldInvalid)
	}
	limit, err := fixedpoint.ParsePrice(row.Base, row.Quote, leg.LimitPrice)
	if err != nil {
		return application.LegIntent{}, fmt.Errorf("pair.legs[%d].limit_price: %v: %w", i, err, ErrPairFieldInvalid)
	}
	return application.LegIntent{
		Instrument: instrument, Side: s, Quantity: quantity, LimitPrice: limit,
	}, nil
}

// unwindIntent is legIntent with the side inverted, ReduceOnly set, the price
// taken from unwind_limit_price and the quantity taken from the ESTABLISHED
// fill (D7).
//
// A zero Quantity means "not known yet" and yields errUnwindQuantityDeferred
// after every other field has been resolved, so Validate can check the price
// and the inversion at load without a fill in hand.
func (c Config) unwindIntent(i int, filled fixedpoint.Quantity) (application.LegIntent, error) {
	intent, err := c.legIntent(i)
	if err != nil {
		return application.LegIntent{}, err
	}
	leg := c.Pair.Legs[i]
	side, _ := c.sideFor(leg.Venue) // legIntent already refused an unknown venue
	row, _ := c.instrumentRow(side, intent.Instrument)

	limit, err := fixedpoint.ParsePrice(row.Base, row.Quote, leg.UnwindLimitPrice)
	if err != nil {
		return application.LegIntent{}, fmt.Errorf("pair.legs[%d].unwind_limit_price: %v: %w",
			i, err, ErrPairFieldInvalid)
	}
	intent.LimitPrice = limit
	intent.ReduceOnly = true
	switch intent.Side {
	case domain.SideBuy:
		intent.Side = domain.SideSell
	case domain.SideSell:
		intent.Side = domain.SideBuy
	}
	if filled.Validate() != nil {
		return application.LegIntent{}, errUnwindQuantityDeferred
	}
	intent.Quantity = filled
	return intent, nil
}

// instrumentRow is one side's declared row for an instrument.
func (c Config) instrumentRow(side int, instrument ids.VenueInstrument) (reconcile.InstrumentConfig, bool) {
	for j := range c.Sides[side].OMS.Instruments {
		if c.Sides[side].OMS.Instruments[j].Instrument() == instrument {
			return c.Sides[side].OMS.Instruments[j], true
		}
	}
	return reconcile.InstrumentConfig{}, false
}

// parseSide is the document's side word, refused rather than defaulted.
func parseSide(i int, word string) (domain.Side, error) {
	switch word {
	case "buy":
		return domain.SideBuy, nil
	case "sell":
		return domain.SideSell, nil
	}
	return "", fmt.Errorf("pair.legs[%d].side %q (want \"buy\" or \"sell\"): %w",
		i, word, ErrPairFieldInvalid)
}

// openPlan is the TradePlan for one leg's OPENING order, sized by the caller.
//
// The quantity is a PARAMETER and not the document's, and that is D5: leg B is
// sized to leg A's established fill, so a plan reading the document would be
// right for the first leg and would hedge a partial fill at full size on the
// second.
func (c Config) openPlan(i int, quantity fixedpoint.Quantity) (application.TradePlan, error) {
	trade, err := ids.ParseTradeID(c.Pair.ID)
	if err != nil {
		return application.TradePlan{}, fmt.Errorf("pair.id: %w", err)
	}
	intent, err := c.legIntent(i)
	if err != nil {
		return application.TradePlan{}, err
	}
	intent.Quantity = quantity
	return application.TradePlan{Trade: trade, Attempt: c.Pair.Attempt, Legs: []application.LegIntent{intent}}, nil
}

// unwindPlan is the TradePlan for the reduce-only close of leg i (D7): same
// trade, same leg index, attempt+1, opposite side, sized to the ESTABLISHED
// fill, at the leg's declared unwind limit.
//
// The attempt advance is §5.3's requirement and it is also the fail-safe: a
// second run of the same document mints ids the journal already holds, and the
// planner refuses the whole plan rather than sending a duplicate.
func (c Config) unwindPlan(i int, filled fixedpoint.Quantity) (application.TradePlan, error) {
	trade, err := ids.ParseTradeID(c.Pair.ID)
	if err != nil {
		return application.TradePlan{}, fmt.Errorf("pair.id: %w", err)
	}
	intent, err := c.unwindIntent(i, filled)
	if err != nil {
		return application.TradePlan{}, err
	}
	return application.TradePlan{Trade: trade, Attempt: c.Pair.Attempt + 1, Legs: []application.LegIntent{intent}}, nil
}
