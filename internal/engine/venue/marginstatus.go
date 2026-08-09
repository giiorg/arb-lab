package venue

import (
	"context"
	"fmt"

	"github.com/giiorg/arb-lab/internal/engine/venue/transport"
	"github.com/giiorg/arb-lab/internal/platform/apperror"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// This file is §4's `AccountSource.MarginStatus` surface, venue-side: the
// account-side input to §7.1's liq-distance trigger, and the REST failover the
// stale-mark ladder names in as many words.
//
// # It is a SECOND codec over one venue answer, not a widening of the position
// # codec
//
// On Binance both this and §7.5's enumeration read `/fapi/v3/positionRisk`, and
// the tempting move is to un-narrow binancePositionWire. Record §78 §5 rules it
// out: §4 declares this as its own method, and binancePositionWire's narrowing
// comment ("every other field is a number this system would then have two
// sources for") is right and stays. §7.5 asserts a net-delta invariant and must
// not acquire a second source for the position; this port asks a different
// question and projects a different set of fields off the same rows. Two codecs
// over one answer, each narrowed to its own consumer — the shape task #207 built
// for the mark, arriving here through an endpoint rather than through a frame.
//
// The corollary is the one that would be got wrong: the mark price is on these
// rows too (Binance `markPrice`, Bybit `markPrice`) and is NOT decoded here. The
// mark is §3.4's own data class with its own budget, its own ingest and its own
// read port (record §78 §1), and a second producer inside the account surface
// would give §7.1's ratio a numerator and a denominator from two different
// clocks — with nothing anywhere comparing them.
//
// # "Available margin" is a quantity NEITHER venue publishes per position, and
// # this is the ruling rather than an absorption
//
// §4 line 331 reads "available margin, maintenance, live liq price". Maintenance
// and the liq price are published per position by both venues. The first is not,
// and the two things it could mean come apart exactly where it matters:
//
//	the position's POSTED margin      Binance isolatedWallet · Bybit positionIM
//	the position's MARGIN BALANCE     posted + unrealized PnL — derived, not published
//	the account's FREE balance        Binance availableBalance — ACCOUNT-level, no symbol
//
// The third is settled by the signature: this method is keyed on a
// VenueInstrument and a free wallet balance is not a function of one. It is also
// already owned elsewhere — §7.2: "`treasury` owns balances, the concentration
// cap, inventory-drift monitor, and the rescue-margin float" — and Binance's own
// spec-generated connector describes `availableBalance` at the top level of
// `GET /fapi/v3/account`, beside the positions array rather than inside it.
//
// The first two are settled by what a venue states versus what this system would
// invent. Both venues publish the POSTED figure and neither publishes the margin
// balance, so the second reading is a subtraction this decoder would be
// performing — against the standing rule that the venue's own computation is the
// definition for anything its liquidation engine acts on. So PostedMargin is the
// posted figure, and it is NAMED for what it is.
//
// That naming is load-bearing, because the difference is unrealized PnL and the
// trigger's whole subject is a rally moving unrealized PnL against a short. A
// caller MUST NOT read `PostedMargin - Maintenance` as headroom: it overstates
// headroom by exactly the adverse PnL, which is largest at the moment the
// trigger fires. The headroom statement is the venue's own LiqPrice, against the
// mark, which is why §7.1's threshold is a price distance and not a margin ratio.
//
// Both of these were deviations from §4's declared line and are now IN it
// (v0.9.60): the method yields the POSTED figure, and its declared return gains
// the presence bool, because `(MarginStatus, error)` cannot express a flat
// instrument without a zero-valued status whose every field computes —
// MarkRiskReader's argument that absence is not an error, one port over.
//
// # The two venues spell "there is no liquidation price" DIFFERENTLY, and both
// # spellings are values a liq distance can be computed from
//
// Bybit, verbatim: liqPrice "keeps `\"\"` when liqPrice <= minPrice or
// liqPrice >= maxPrice" — a documented empty string for a position whose
// liquidation lies outside the symbol's price band, which is an ordinary healthy
// state and arrives on a well-margined position. Binance publishes a decimal
// string with no documented absence spelling at all.
//
// Read as a zero, either one is catastrophic in the same direction. §7.1's
// distance is taken against a SHORT, whose liquidation price sits ABOVE the
// mark; a liq price of zero sits far below it, and every formula that could be
// written over that pair reports a position at maximum headroom. So absence is a
// first-class answer here (LiqPriceStated), the rule that decides it is shared by
// both codecs (marginLiqPrice), and its failure direction is the conservative
// one: a consumer that cannot compute a distance must escalate on the stale-input
// rung, never conclude "far from liquidation".
//
// # SPOT HAS NO MARGIN STATUS, for the position surface's reason
//
// Both venues' margin figures live on their derivatives position surfaces —
// Binance's is /fapi, Bybit's rejects category=spot — so a spot instrument gets
// ErrPositionSurfaceAbsent rather than an empty status.
//
// # THE POSITION QUANTITY AND THE ENTRY PRICE ARE DECODED HERE, and the fence
// # that permits it binds in ONE direction only (record §85 §1, task #219)
//
// Until v0.9.60 the position amount on this row was read as a PRESENCE test and
// never carried out of the codec. §7.1's two remedies both take a quantity —
// the top-up's `Δ = (L*−L)·q·(1+MMR)` and the delever's target
// `q* = m/((1+MMR)·L* − e)` — and nothing in this system computes one. Record
// §85 rules that it comes from this row, on three independent arguments, of
// which the third is the one that decides it: §7.1 requires the escalation to
// re-query "margin + position ... target-state, not delta", and margin and
// quantity must therefore describe ONE instant. They are on one row. Taking the
// margin from here and the quantity from §7.5's enumeration is two calls at two
// instants, which is v0.9.60's own objection — "a ratio whose numerator and
// denominator came from two different producers is one nothing compares" —
// pointed at this slice.
//
// Widening the margin consumer's OWN codec is v0.9.60's rule working rather than
// breached: "the two consumers get two narrowed codecs over one venue answer,
// each projecting its own fields". What must be stated, because it is now the
// load-bearing half of that rule, is the prohibition in the OTHER direction:
//
//	SizingQuantity MUST NOT be fed to §7.5's net-delta invariant, and no future
//	consumer may substitute it for ports.VenuePositions on cost or convenience
//	grounds.
//
// The invariant's entire value is that it joins two authorities that CAN
// disagree — a local projection against a venue enumeration. A second decode of
// one venue field is not a second authority, so substituting it would leave the
// assertion comparing the venue with itself and passing forever. The field is
// named for the use it is permitted rather than for the number it holds, so a
// call site doing this is visibly doing it.
//
// The corollary above about the mark still stands and is NOT weakened by this:
// the mark has its own producer, its own §5.2 budget and its own silence timer,
// so a second decode here would create a rival. The position quantity had no
// producer on this path at all. That asymmetry is the whole ruling.
//
// # A ZERO ENTRY PRICE IS REFUSED, and the tempting cross-check is refused with
// # it for a sharper reason
//
// Neither venue documents an absence spelling for the entry price on a LIVE
// position, and no consumer can proceed without one, so it is decoded like a
// margin figure rather than like the liq price: empty or non-positive is an
// ERROR, not a flag. There is no `EntryPriceStated`, because a false on such a
// flag would have no defined behaviour anywhere — an unreachable sentinel that
// reads as handled.
//
// What is deliberately NOT enforced is the relation a reader reaches for next.
// EntryPrice and LiqPrice are both Price in the same pair, so a transposed
// assignment produces a status that validates, and the obvious guard is that a
// SHORT liquidates above its entry. Solving §85's relation shows why that guard
// must not be written: `L = (m/q + e)/(1 + MMR)` gives `L > e` exactly when
// `m/q > e·MMR` — margin per unit above the maintenance requirement per unit,
// which is the condition "this position is not currently liquidatable". A
// validator enforcing it would refuse precisely the readings §7.1's ladder
// exists to act on, at the moment it needs them. The transposition is caught by
// tests asserting the values instead, which is the same trade v0.9.60 already
// took for the two money fields.

// MarginStatus is one perpetual position's margin state at the venue,
// normalized: what the venue would liquidate it at, what it requires to keep it,
// and what is posted against it.
type MarginStatus struct {
	// Instrument is which market. Carried so a status passed through a layer
	// still names what it describes.
	Instrument ids.VenueInstrument

	// LiqPrice is the venue's own published liquidation price for this
	// position, in the instrument's quote asset. Meaningful only when
	// LiqPriceStated.
	//
	// It is the VENUE's number and never one this system reconstructs, for
	// Mark.Price's reason: §7.1 computes the distance the liquidation engine
	// will act on, so the engine's own figure is the definition.
	LiqPrice fixedpoint.Price

	// LiqPriceStated is whether the venue published one at all.
	//
	// Beside the value rather than a pointer, so the zero value of this struct
	// is a status that states nothing rather than one asserting a liq price of
	// zero — and Validate refuses a cleared flag beside a populated price, so
	// the two cannot disagree. See the file comment for the two spellings and
	// why a false here must never be read as distance.
	LiqPriceStated bool

	// Maintenance is the maintenance margin the venue requires to keep this
	// position open — Binance `maintMargin` ("maintenance margin required"),
	// Bybit `positionMM` ("Maintenance margin"). In the margin asset.
	Maintenance fixedpoint.Money

	// PostedMargin is the isolated margin posted against this position —
	// Binance `isolatedWallet` ("isolated wallet (if isolated position)" on
	// positionRisk; the same connector calls it "Isolated wallet balance" on
	// `GET /fapi/v3/account`, which is the wording that makes it legible as a
	// BALANCE rather than a requirement), Bybit `positionIM` ("Initial
	// margin"). In the margin asset.
	//
	// It is NOT headroom and NOT a margin balance: it excludes unrealized PnL on
	// both venues. See the file comment — the subtraction a reader reaches for
	// is wrong by the one quantity the trigger is about.
	//
	// The pairing is the one §7.1 already fixed for the other direction: a
	// margin top-up is resolved by "the `positionIM` delta vs the journaled
	// before-state" on Bybit and by "the position-risk margin delta" on Binance.
	// isolatedWallet is the Binance half of that pair rather than isolatedMargin,
	// because a top-up is the only thing that moves it — isolatedMargin moves
	// with price as well, so evidence taken from it is confounded by exactly the
	// market move that provoked the op.
	PostedMargin fixedpoint.Money

	// SizingQuantity is the venue's SIGNED position quantity on this row —
	// negative for the short §7.1 exists to watch — in the instrument's base
	// asset. Binance signs `positionAmt`; Bybit states an always-positive `size`
	// beside a `side`, and this codec applies the same rule §7.5's does.
	//
	// It is named for the use it is PERMITTED rather than for the number it
	// holds, and the name is the carrier of a prohibition (record §85 §1): it is
	// the input to §7.1's two remedy sizings — `Δ = (L*−L)·q·(1+MMR)` and
	// `q* = m/((1+MMR)·L* − e)` — and it MUST NOT be fed to §7.5's net-delta
	// invariant, which keeps ports.VenuePositions as its single source. See the
	// file comment: substituting it there leaves the invariant comparing the
	// venue with itself.
	//
	// A status this codec reports as held always carries a nonzero one, because
	// zero is what the presence test reads as flat.
	SizingQuantity fixedpoint.Quantity

	// EntryPrice is the position's average entry — Binance `entryPrice`, Bybit
	// `avgPrice` — in the instrument's quote asset.
	//
	// It is `e` in §85 §3's delever target `q* = m/((1+MMR)·L* − e)`, which is
	// the one place it does NOT cancel: the top-up's inverse solve differences
	// two states of the same position and loses it, and that is exactly why the
	// delever target needs it read rather than derived.
	//
	// Unlike the liq price this has no absence flag — see the file comment. Both
	// venues publish it for every live position, no absence spelling is
	// documented, and no consumer can size anything without it, so an empty or
	// non-positive value is refused at the decode.
	EntryPrice fixedpoint.Price
}

// Validate reports whether the status is one a caller may reason from.
//
// # The asset agreement is checked, and it is checked against the PRICE
//
// Both money figures are the venue's margin, and the quote asset is what the liq
// price is denominated in. On a linear perpetual — the only family this surface
// routes; inverse is unrouted and spot has no surface at all — the margin asset
// and the quote asset are the same asset, so a decoder that typed a margin in the
// BASE asset (the one accident available: a codec reaching for InstrumentAssets.
// Base out of habit, as every quantity here does) produces figures that validate
// individually and are silently in the wrong currency. fixedpoint's runtime tag
// check catches it at the first arithmetic, which is inside the consumer and far
// from the decoder that caused it.
func (m MarginStatus) Validate() error {
	if err := m.Instrument.Validate(); err != nil {
		return fmt.Errorf("margin status instrument: %w", err)
	}
	if err := m.Maintenance.Validate(); err != nil {
		return fmt.Errorf("margin status %s maintenance: %w", m.Instrument.Symbol, err)
	}
	if err := m.PostedMargin.Validate(); err != nil {
		return fmt.Errorf("margin status %s posted margin: %w", m.Instrument.Symbol, err)
	}
	if m.Maintenance.Asset() != m.PostedMargin.Asset() {
		return fmt.Errorf("margin status %s: maintenance in %s, posted margin in %s: %w",
			m.Instrument.Symbol, m.Maintenance.Asset(), m.PostedMargin.Asset(), ErrMarginAssetsDisagree)
	}
	if err := m.requireNonNegative("maintenance", m.Maintenance); err != nil {
		return err
	}
	if err := m.requireNonNegative("posted margin", m.PostedMargin); err != nil {
		return err
	}
	if err := m.validateSizing(); err != nil {
		return err
	}

	if !m.LiqPriceStated {
		// A price beside a cleared flag is a reading whose two halves say
		// different things, and the half a consumer branches on is the flag —
		// so the number would sit there unread until something copied the
		// struct field by field and lost the flag. Refused rather than zeroed,
		// because zeroing it here would make this function silently repair a
		// decoder defect.
		if m.LiqPrice != (fixedpoint.Price{}) {
			return fmt.Errorf("margin status %s states no liq price but carries %s: %w",
				m.Instrument.Symbol, m.LiqPrice, ErrMarginLiqPriceContradicts)
		}
		return nil
	}
	if err := m.LiqPrice.Validate(); err != nil {
		return fmt.Errorf("margin status %s liq price: %w", m.Instrument.Symbol, err)
	}
	sign, err := m.LiqPrice.Sign()
	if err != nil {
		return fmt.Errorf("margin status %s liq price: %w", m.Instrument.Symbol, err)
	}
	if sign <= 0 {
		// Mark.Validate's rule, one field over and for the same consumer: a
		// non-positive liq price is the value marginLiqPrice already declines to
		// state, so reaching here means something built the status directly.
		return fmt.Errorf("margin status %s liq price %s: %w",
			m.Instrument.Symbol, m.LiqPrice, ErrMarginLiqPriceNotPositive)
	}
	if m.LiqPrice.Quote() != m.Maintenance.Asset() {
		return fmt.Errorf("margin status %s: liq price quoted in %s, margin in %s: %w",
			m.Instrument.Symbol, m.LiqPrice.Quote(), m.Maintenance.Asset(), ErrMarginAssetsDisagree)
	}
	return nil
}

// validateSizing checks the two fields §7.1's remedies size from.
//
// # A zero quantity is REFUSED, and that is not the same check the codecs make
//
// Both codecs read the quantity as their presence test and report a zero one as
// "no position", so a decoded status never carries zero here. This function is
// reached by a status built DIRECTLY — the ports mirror restates it for exactly
// this reason, since a test double satisfies the port without going near a
// codec. A held position of size zero sizes a top-up of zero and a delever
// toward nothing, both of which are remedies that run and change nothing while
// the ladder reads them as attempted.
//
// # The asset checks are the BASE/QUOTE pair, which is the substitution
// # available at every call site
//
// The quantity is in the base and the entry price is quoted in the margin
// asset. Every other quantity on this client is typed in the base, so a codec
// reaching for InstrumentAssets.Base out of habit types the entry price in BTC
// — a Price whose own Validate passes and whose figures are silently in the
// wrong currency until they meet a USDT number inside the consumer. Checked
// against Maintenance.Asset() rather than against LiqPrice, because the liq
// price is the field that may legitimately be absent.
func (m MarginStatus) validateSizing() error {
	if err := m.SizingQuantity.Validate(); err != nil {
		return fmt.Errorf("margin status %s sizing quantity: %w", m.Instrument.Symbol, err)
	}
	sign, err := m.SizingQuantity.Sign()
	if err != nil {
		return fmt.Errorf("margin status %s sizing quantity: %w", m.Instrument.Symbol, err)
	}
	if sign == 0 {
		return fmt.Errorf("margin status %s: %w", m.Instrument.Symbol, ErrMarginQuantityZero)
	}
	if err := m.EntryPrice.Validate(); err != nil {
		return fmt.Errorf("margin status %s entry price: %w", m.Instrument.Symbol, err)
	}
	entrySign, err := m.EntryPrice.Sign()
	if err != nil {
		return fmt.Errorf("margin status %s entry price: %w", m.Instrument.Symbol, err)
	}
	if entrySign <= 0 {
		return fmt.Errorf("margin status %s entry price %s: %w",
			m.Instrument.Symbol, m.EntryPrice, ErrMarginEntryPriceNotPositive)
	}
	if m.EntryPrice.Quote() != m.Maintenance.Asset() {
		return fmt.Errorf("margin status %s: entry price quoted in %s, margin in %s: %w",
			m.Instrument.Symbol, m.EntryPrice.Quote(), m.Maintenance.Asset(), ErrMarginAssetsDisagree)
	}
	if m.SizingQuantity.Asset() != m.EntryPrice.Base() {
		return fmt.Errorf("margin status %s: sizing quantity in %s, entry price based in %s: %w",
			m.Instrument.Symbol, m.SizingQuantity.Asset(), m.EntryPrice.Base(), ErrMarginAssetsDisagree)
	}
	return nil
}

// requireNonNegative refuses a margin figure below zero.
//
// Neither venue publishes a negative maintenance requirement or a negative
// posted margin, so a negative one is a parse that read a sign off the wrong
// field — and the direction it fails in is the permissive one everywhere it is
// consumed: a negative maintenance makes any position look over-margined, and a
// negative posted margin makes a top-up look larger than it was.
func (m MarginStatus) requireNonNegative(what string, v fixedpoint.Money) error {
	sign, err := v.Sign()
	if err != nil {
		return fmt.Errorf("margin status %s %s: %w", m.Instrument.Symbol, what, err)
	}
	if sign < 0 {
		return fmt.Errorf("margin status %s %s is %s: %w",
			m.Instrument.Symbol, what, v, ErrMarginFigureNegative)
	}
	return nil
}

// MarginStatusCodec is one venue's translation of the margin-status surface.
//
// Embedded into OrderCodec rather than segregated, on PositionCodec's argument:
// a venue whose margin status cannot be read is a venue §7.1's trigger cannot
// run on, so it is not one this system may hold a perpetual short at — and a
// separate interface would model a combination that must not exist.
type MarginStatusCodec interface {
	// EncodeMarginStatus renders the per-instrument read.
	//
	// It returns ErrPositionSurfaceAbsent for a family with no position surface
	// — spot on both venues.
	EncodeMarginStatus(instrument ids.VenueInstrument) (transport.Call, error)

	// DecodeMarginStatus reads the venue's answer for one instrument.
	//
	// The bool reports whether the venue holds a position on this instrument at
	// all. It is not an error and not a zero-valued status: a flat instrument is
	// the ordinary answer for anything this system has closed, and §7.1 has
	// nothing to compute for one.
	//
	// An implementation MUST refuse a row that is not one-way, for
	// DecodePositions' reason: a hedge-mode account answers with two rows whose
	// margin figures belong to two different legs.
	DecodeMarginStatus(instrument ids.VenueInstrument, assets InstrumentAssets, resp transport.Response) (MarginStatus, bool, error)
}

// marginStatusReadRef is the placeholder client order id this surface's routing
// check carries. A margin read is not about one order, and route's validation is
// per-order — executionsSweepRef's argument, unchanged.
const marginStatusReadRef ids.ClientOrderID = "margin-status-read"

// MarginStatus reads one perpetual position's margin state.
//
// # The urgency is the caller's, and the two live classes are far apart
//
// §7.1 makes this a Safety read during an incident: it is the REST failover the
// stale-mark ladder polls "from the reserved rate-limit allocation", and it is
// what the escalation "re-queries margin + position first" before delevering. A
// periodic risk sweep on a healthy process is Routine. Positions' argument,
// unchanged, and here the spread between the two classes is the widest on any
// read surface — which is the reason to keep it an argument rather than pin it.
//
// # The bool is the venue's answer, not this method's judgement
//
// False means the venue reports no position on this instrument. Distinguishing
// it from a zero-valued status matters more here than anywhere else on this
// client, because every field of that zero status is a number §7.1 would
// happily compute with: a maintenance of zero reads as a position requiring
// nothing, and a posted margin of zero reads as one needing an immediate top-up.
func (c *OrderClient) MarginStatus(ctx context.Context, instrument ids.VenueInstrument, assets InstrumentAssets, urgency ratelimit.Priority) (MarginStatus, bool, error) {
	op := string(c.codec.Venue()) + ".MarginStatus"

	if err := assets.Validate(); err != nil {
		return MarginStatus{}, false, apperror.Precondition(op, "assets", err)
	}
	client, err := c.route(op, OrderRef{Instrument: instrument, ClientOrderID: marginStatusReadRef}, urgency)
	if err != nil {
		return MarginStatus{}, false, err
	}

	call, err := c.codec.EncodeMarginStatus(instrument)
	if err != nil {
		return MarginStatus{}, false, apperror.Precondition(op, "encode", err)
	}
	call.Priority = urgency

	resp, err := client.Do(ctx, call)
	if err != nil {
		// Unwrapped, on the read path's standing rule: the transport already
		// classified the effect, and a margin read that was not answered must
		// not be re-derived into a flat instrument — which is the answer that
		// takes §7.1's trigger off this position entirely.
		return MarginStatus{}, false, err
	}

	status, held, err := c.codec.DecodeMarginStatus(instrument, assets, resp)
	if err != nil {
		return MarginStatus{}, false, apperror.Precondition(op, "decode", err)
	}
	if !held {
		return MarginStatus{}, false, nil
	}
	if err := status.Validate(); err != nil {
		return MarginStatus{}, false, apperror.Precondition(op, "decode", err)
	}
	return status, true, nil
}

// marginLiqPrice applies the shared rule for reading a venue's liquidation
// price, and reports whether the venue stated one.
//
// # Shared by both codecs because the rule is the CONSUMER's, not a venue's
//
// sortPositionReport's argument. The two venues have different absence
// spellings — Bybit a documented empty string, Binance nothing documented at all
// — but the question "may §7.1 divide by this" has one answer, and a rule stated
// once cannot drift into two readings of it.
//
// Three arms, and the middle one is the one that would be got wrong:
//
//   - the empty string is Bybit's documented absence, and it parses as nothing;
//   - a value that parses and is NOT strictly positive is treated as absence
//     too, conservatively. Binance documents no absence spelling, so if it
//     signals one it can only be with a degenerate number; treating "0" as a
//     price puts a short's liquidation far BELOW its mark, which every distance
//     formula reads as maximum headroom. Treating it as absence costs one
//     escalation rung on a venue that never sends it, and the codebase has
//     already taken this trade once — Bybit's assumed empty-list absence is
//     written so "the unverified branch fails closed";
//   - anything else that will not parse is an ERROR, because an unreadable
//     answer is not an absence. That is §5.3's rule, and it is what keeps the
//     conservative arm above from swallowing a venue that changed its format.
func marginLiqPrice(base, quote fixedpoint.Asset, raw string) (fixedpoint.Price, bool, error) {
	if raw == "" {
		return fixedpoint.Price{}, false, nil
	}
	p, err := fixedpoint.ParsePrice(base, quote, raw)
	if err != nil {
		return fixedpoint.Price{}, false, fmt.Errorf("liq price %q: %w", raw, err)
	}
	sign, err := p.Sign()
	if err != nil {
		return fixedpoint.Price{}, false, fmt.Errorf("liq price %q: %w", raw, err)
	}
	if sign <= 0 {
		return fixedpoint.Price{}, false, nil
	}
	return p, true, nil
}

// marginEntryPrice applies the shared rule for reading a live position's entry
// price — Binance `entryPrice`, Bybit `avgPrice`.
//
// Shared by both codecs for marginLiqPrice's reason: the question "may §7.1's
// delever target be sized from this" has one answer, and a rule stated once
// cannot drift into two readings. What differs from marginLiqPrice is the
// ARITY, and deliberately so — that one has three arms because absence is a
// documented value on one venue and a legal state for the consumer, and this
// one has two, because there is no such thing as a live position with no entry
// price and no consumer that could proceed with one. Empty and non-positive are
// therefore both refusals rather than an absence flag.
//
// The direction that decides it: absorbed as zero, the delever target
// `q* = m/((1+MMR)·L* − e)` gains a LARGER denominator, so it computes a
// smaller target and delevers harder than asked, on a number the venue never
// stated. A refusal instead puts the position on §7.1's stale-input rung, which
// is where an unreadable margin answer already belongs.
func marginEntryPrice(base, quote fixedpoint.Asset, raw string) (fixedpoint.Price, error) {
	if raw == "" {
		return fixedpoint.Price{}, fmt.Errorf("entry price: %w", ErrMarginEntryPriceUnstated)
	}
	p, err := fixedpoint.ParsePrice(base, quote, raw)
	if err != nil {
		return fixedpoint.Price{}, fmt.Errorf("entry price %q: %w", raw, err)
	}
	sign, err := p.Sign()
	if err != nil {
		return fixedpoint.Price{}, fmt.Errorf("entry price %q: %w", raw, err)
	}
	if sign <= 0 {
		return fixedpoint.Price{}, fmt.Errorf("entry price %q: %w", raw, ErrMarginEntryPriceNotPositive)
	}
	return p, nil
}

// marginFigure parses one of the two margin amounts.
//
// The asset is the caller's declared QUOTE and never the base, which is the one
// substitution available here and the one MarginStatus.Validate exists to catch
// from the other side. Both are asset tags in scope at every call site, every
// other quantity on this client is typed in the base, and a margin typed in BTC
// validates perfectly until it meets a USDT figure inside the consumer.
func marginFigure(asset fixedpoint.Asset, what, raw string) (fixedpoint.Money, error) {
	v, err := fixedpoint.ParseMoney(asset, raw)
	if err != nil {
		return fixedpoint.Money{}, fmt.Errorf("%s %q: %w", what, raw, err)
	}
	return v, nil
}
