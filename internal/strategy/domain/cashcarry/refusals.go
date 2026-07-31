package cashcarry

import "fmt"

// Refusal rule names (§40f: "the refusing rule named"). These are journal
// vocabulary — a RefusedBy begins with one of these constants, optionally
// followed by ": <detail>" — so a reader of the decision journal can count
// refusals by rule without parsing prose, and a test can assert the rule
// without pinning the detail.
const (
	// RefuseMinFundingAPR is §5.2's min_funding_apr floor: the annualized
	// live funding estimate is below the policy floor, so the hold is not
	// evaluated further.
	RefuseMinFundingAPR = "cashcarry/min_funding_apr"

	// RefuseMinHold is §5.2's min_hold_settlements: the proposed plan holds
	// fewer settlements than policy admits entering for.
	RefuseMinHold = "cashcarry/min_hold_settlements"

	// RefuseForecastDraw reports forecasts that are not one coherent draw at
	// the policy's decision quantile — drawn at a different quantile than the
	// policy states, or refused by the joint rule (mixed quantiles, mixed
	// artifacts).
	RefuseForecastDraw = "cashcarry/forecast_draw"

	// RefuseLots reports leg lot grids with no usable intersection (§40g).
	RefuseLots = "cashcarry/lot_intersection"

	// RefuseResidual is §2.4's residual-delta rule: quantization left more of
	// the candidate un-sized than the declared tolerance admits.
	RefuseResidual = "cashcarry/residual_tolerance"

	// RefuseMinQty reports a quantized target below a leg's venue minimum.
	RefuseMinQty = "cashcarry/min_qty"

	// RefuseMinNotional reports a quantized target below a leg's minimum
	// order value, judged at the BID (the understated notional — if that
	// clears, the real order clears).
	RefuseMinNotional = "cashcarry/min_notional"

	// RefuseCapital reports a candidate the declared C_allocated
	// decomposition cannot fund: the spot leg's notional exceeds the spot
	// principal, or the perp leg's notional exceeds initial margin at the
	// pinned leverage. The candidate is explicit config (§40b); this gate is
	// what keeps it honest against the capital the verdict reports on.
	RefuseCapital = "cashcarry/capital_underfunded"
)

// Sentinel errors for the quantization path. TargetQty returns these; Enter
// maps each to its refusal rule above. They are exported because the service
// and the tests branch on them with errors.Is.
var (
	// ErrLotIncompatible reports two leg step sizes whose least common
	// multiple overflows — no order quantity satisfies both grids.
	ErrLotIncompatible = fmt.Errorf("cashcarry: leg lot grids have no usable intersection")

	// ErrResidualExceeded reports quantization dropping more than the
	// declared residual-delta tolerance.
	ErrResidualExceeded = fmt.Errorf("cashcarry: quantization residual exceeds tolerance")

	// ErrBelowMinQty reports a quantized target below a leg's minimum
	// quantity (a candidate smaller than one intersection step lands here
	// too: its quantized target is zero).
	ErrBelowMinQty = fmt.Errorf("cashcarry: quantized target below a leg's minimum quantity")

	// ErrBelowMinNotional reports a quantized target below a leg's minimum
	// order value at the bid.
	ErrBelowMinNotional = fmt.Errorf("cashcarry: quantized target below a leg's minimum notional")

	// ErrKindWrong reports an input whose Kind is not the one the called
	// function evaluates — a service bug, never a refusal.
	ErrKindWrong = fmt.Errorf("cashcarry: input kind does not match the evaluation")
)
