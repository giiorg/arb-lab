package exercise

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/app/reconcile"
	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/config"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// Config is the exercise binary's document: §5.2's OMS block, unchanged and
// with its one owner, plus the trade this run drives.
//
// # The OMS block is a NAMED field, not an embed, and platform/config decided
//
// reconcile.Config is §5.2's schema and this package must not hold a second
// statement of it — two schemas for one block is the copy that drifts, and the
// drift would surface as an exercise config that reconciles differently from
// the recovery run over the same journal.
//
// An anonymous embed was the first shape and the config kernel refused it:
// auditSchema defers embedded fields because the path naming and hot-tag
// semantics of a PROMOTED field are undefined there — "refused rather than
// half-supported" — and defining them is kernel work with its own collision
// rules (this repo has already paid for encoding/json's silent-drop collision
// behaviour once). So the block mounts under "oms": the schema is still owned
// once, and where a binary mounts it is that binary's document shape, not a
// second schema. §59i (vii) records the embed as the rejected alternative and
// the kernel's deferral now has a named prospective consumer.
type Config struct {
	// OMS is §5.2's per-OMS block, exactly as `arblab reconcile run` reads it.
	OMS reconcile.Config `json:"oms"`

	// Trade is the package this run plans and executes: §9's synchronized
	// spot↔perp cycle, stated as data.
	Trade TradeConfig `json:"trade"`

	// Reference names the committed reference revision the run resolves its
	// instruments' §2.4 trading filters from, and how old a row may be. It is
	// REQUIRED here and absent from reconcile.Config, deliberately: the
	// requirement belongs to the binary that plans, and §7.5's boot root is
	// fenced off from the strategy context the document belongs to — see
	// tradingFilters for the whole argument.
	Reference ReferenceConfig `json:"reference"`
}

// ReferenceConfig locates the reference revision and bounds its age.
type ReferenceConfig struct {
	// Path is the committed document (configs/reference.btcusdt.json in this
	// repo's deployments).
	Path string `json:"path"`

	// MaxAge bounds now − observed_at per resolved row. See
	// ErrFilterRowStale for what a stale row silently re-opens.
	MaxAge config.Duration `json:"max_age"`
}

// TradeConfig is one planned package.
type TradeConfig struct {
	// ID is the ids.TradeID this cycle plans under. It is the operator's
	// because §5.3 forbids ClientOrderID reuse within the recoverable window:
	// a re-run over a journal that already holds this trade's ids is refused
	// by the planner, and a FRESH id per cycle is what §9's campaign supplies.
	ID string `json:"id"`

	// Attempt distinguishes a replan of this trade — §6 step 5's whole-plan
	// advance, threaded through to every minted id.
	Attempt int `json:"attempt"`

	// Legs are the package's orders. Two for a cash-and-carry entry; the
	// executor enforces §6 step 6's identical base quantity across however
	// many are stated.
	Legs []LegConfig `json:"legs"`
}

// LegConfig is one leg, named against the document's own instrument table.
type LegConfig struct {
	// Product, Contract and Symbol select the instrument from Instruments —
	// the venue is the document's one venue. The row is where the base and
	// quote assets come from, so a leg cannot state a quantity in an asset the
	// document never declared.
	Product  ids.ProductCategory `json:"product"`
	Contract ids.ContractType    `json:"contract,omitempty"`
	Symbol   string              `json:"symbol"`

	// Side is "buy" or "sell".
	Side string `json:"side"`

	// Quantity is the ordered size in the BASE asset, and LimitPrice is §6
	// step 6's marketable limit in the quote.
	Quantity   string `json:"quantity"`
	LimitPrice string `json:"limit_price"`

	// ReduceOnly is §5.3's flag, the caller's decision exactly as it is on
	// application.LegIntent.
	ReduceOnly bool `json:"reduce_only"`
}

// Validate reports whether the document describes a runnable exercise.
func (c Config) Validate() error {
	if err := c.OMS.Validate(); err != nil {
		return err
	}
	if c.Reference.Path == "" {
		return fmt.Errorf("reference.path: %w", ErrReferenceFieldUnset)
	}
	if c.Reference.MaxAge.Duration() <= 0 {
		return fmt.Errorf("reference.max_age: %w", ErrReferenceFieldUnset)
	}
	if c.Trade.ID == "" {
		return fmt.Errorf("trade.id: %w", ErrTradeFieldUnset)
	}
	if _, err := ids.ParseTradeID(c.Trade.ID); err != nil {
		return fmt.Errorf("trade.id: %v: %w", err, ErrTradeFieldInvalid)
	}
	if c.Trade.Attempt < 0 {
		return fmt.Errorf("trade.attempt: %w", ErrTradeFieldInvalid)
	}
	if len(c.Trade.Legs) == 0 {
		return fmt.Errorf("trade.legs: %w", ErrTradeFieldUnset)
	}
	for i := range c.Trade.Legs {
		if _, err := c.legIntent(i); err != nil {
			return err
		}
	}
	return nil
}

// legIntent resolves one leg against the instrument table.
//
// It is the shared half of Validate and Plan — the same function answers both,
// so a document that validated cannot fail to plan for a reason validation
// could have named.
func (c Config) legIntent(i int) (application.LegIntent, error) {
	leg := c.Trade.Legs[i]
	instrument := ids.VenueInstrument{
		Venue:    c.OMS.Venue.Venue,
		Product:  leg.Product,
		Contract: leg.Contract,
		Symbol:   leg.Symbol,
	}

	var row *reconcile.InstrumentConfig
	for j := range c.OMS.Instruments {
		if c.OMS.Instruments[j].Instrument() == instrument {
			row = &c.OMS.Instruments[j]
			break
		}
	}
	if row == nil {
		return application.LegIntent{}, fmt.Errorf("trade.legs[%d] names %s: %w",
			i, instrument, ErrLegInstrumentUnknown)
	}

	var side domain.Side
	switch leg.Side {
	case "buy":
		side = domain.SideBuy
	case "sell":
		side = domain.SideSell
	default:
		return application.LegIntent{}, fmt.Errorf("trade.legs[%d].side %q (want \"buy\" or \"sell\"): %w",
			i, leg.Side, ErrTradeFieldInvalid)
	}

	quantity, err := fixedpoint.ParseQuantity(row.Base, leg.Quantity)
	if err != nil {
		return application.LegIntent{}, fmt.Errorf("trade.legs[%d].quantity: %v: %w", i, err, ErrTradeFieldInvalid)
	}
	limit, err := fixedpoint.ParsePrice(row.Base, row.Quote, leg.LimitPrice)
	if err != nil {
		return application.LegIntent{}, fmt.Errorf("trade.legs[%d].limit_price: %v: %w", i, err, ErrTradeFieldInvalid)
	}

	return application.LegIntent{
		Instrument: instrument,
		Side:       side,
		Quantity:   quantity,
		LimitPrice: limit,
		ReduceOnly: leg.ReduceOnly,
	}, nil
}

// plan is the TradePlan this document states. Validate has already vouched for
// every leg, so an error here is a wiring defect rather than an operator's.
func (c Config) plan() (application.TradePlan, error) {
	trade, err := ids.ParseTradeID(c.Trade.ID)
	if err != nil {
		return application.TradePlan{}, fmt.Errorf("trade.id: %w", err)
	}
	legs := make([]application.LegIntent, len(c.Trade.Legs))
	for i := range c.Trade.Legs {
		intent, err := c.legIntent(i)
		if err != nil {
			return application.TradePlan{}, err
		}
		legs[i] = intent
	}
	return application.TradePlan{Trade: trade, Attempt: c.Trade.Attempt, Legs: legs}, nil
}
