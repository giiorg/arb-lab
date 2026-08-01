package cashcarry

import (
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
)

// yearNanos is the annualization base: a 365-day year, the convention the
// whole doc set already computes with — tutorial §3.9's 5.68% in-position
// figure reproduces only under 365 (the pinned-ledger test would catch a
// drift to 365.25 or 360), and the hurdle benchmark's coupon-equivalent
// yield is quoted on the same basis.
const yearNanos = int64(365 * 24 * time.Hour)

// holdFraction is §2.4's holdFraction: a span as a fraction of a year,
// scaling the annualized hurdle (and the annualized margin and band) to the
// actual hold. The span is Terminal−At — planned for an entry, remaining for
// an exit; the two directions differ only in what At is.
//
// A Rate's mantissa is its value at scale 9, and a Duration's integer
// nanosecond count IS its second count at scale 9 — same integer, two
// readings — so the quotient of the two durations-as-Rates is exact fixture
// for Rate.Div with no unit constant in sight.
func holdFraction(at, terminal time.Time) (fixedpoint.Rate, error) {
	span := terminal.Sub(at)
	if span <= 0 {
		return fixedpoint.Rate{}, fmt.Errorf("hold %s to %s: %w", at, terminal, domain.ErrScheduleOverrun)
	}
	return fixedpoint.RateFromMantissa(int64(span)).
		Div(fixedpoint.RateFromMantissa(yearNanos), fixedpoint.RoundHalfEven)
}

// Annualized converts a per-settlement rate at the live interval to an APR:
// rate × (year / interval), on the 365-day base above. Used by the
// min_funding_apr floor — and exported for the step-4 calibrate derivations,
// which must emit EntryDrift and the exit band in exactly the unit
// scaledThreshold consumes: one implementation, both consumers, or the two
// annualization bases drift a few basis points apart and the artifact's
// numbers silently mean something else here.
func Annualized(perSettlement fixedpoint.Rate, interval time.Duration) (fixedpoint.Rate, error) {
	perYear, err := fixedpoint.RateFromMantissa(yearNanos).
		Div(fixedpoint.RateFromMantissa(int64(interval)), fixedpoint.RoundHalfEven)
	if err != nil {
		return fixedpoint.Rate{}, fmt.Errorf("annualize: %w", err)
	}
	return perSettlement.Mul(perYear, fixedpoint.RoundHalfEven)
}

// effectiveHurdle nets §2.1's recorded adjustments off the published rate:
// annual − access cost − liquidity haircut − counterparty haircut. Declared
// zeros leave the full paper yield — the harder gate, the committed
// reference revision's own stated direction.
func effectiveHurdle(h domain.HurdleBenchmark) (fixedpoint.Rate, error) {
	r, err := h.AnnualRate.Sub(h.AccessCost)
	if err == nil {
		r, err = r.Sub(h.LiquidityHaircut)
	}
	if err == nil {
		r, err = r.Sub(h.CounterpartyHaircut)
	}
	if err != nil {
		return fixedpoint.Rate{}, fmt.Errorf("effective hurdle: %w", err)
	}
	return r, nil
}

// scaledThreshold is annualRate · C_allocated · fraction — the shape shared
// by the scaled hurdle, the entry margin and the exit band. Both multiplies
// round away from zero: every caller sits on the right-hand side of a
// comparison the position must beat, so overstating the threshold is the
// conservative direction.
func scaledThreshold(capital fixedpoint.Money, annualRate, fraction fixedpoint.Rate) (fixedpoint.Money, error) {
	m, err := capital.Mul(annualRate, fixedpoint.RoundAwayFromZero)
	if err != nil {
		return fixedpoint.Money{}, err
	}
	return m.Mul(fraction, fixedpoint.RoundAwayFromZero)
}

// fundingMoney converts the aggregate adverse rate sum to Money against the
// candidate's perp notional. Income arithmetic: the notional is priced at the
// perp BID rounded toward zero, and the product rounds toward negative
// infinity — understated when funding pays, overstated when it charges.
func fundingMoney(adverseSum fixedpoint.Rate, perpBid fixedpoint.Price, qty fixedpoint.Quantity) (fixedpoint.Money, error) {
	notional, err := fixedpoint.Notional(perpBid, qty, fixedpoint.RoundTowardZero)
	if err != nil {
		return fixedpoint.Money{}, fmt.Errorf("funding notional: %w", err)
	}
	return notional.Mul(adverseSum, fixedpoint.RoundFloor)
}

// feeCell is one of the four fee lines: rate × the leg's ask-priced notional,
// both rounded away from zero — cost arithmetic, the slice B adapters' own
// direction (§40m).
func feeCell(rate fixedpoint.Rate, ask fixedpoint.Price, qty fixedpoint.Quantity) (fixedpoint.Money, error) {
	notional, err := fixedpoint.Notional(ask, qty, fixedpoint.RoundAwayFromZero)
	if err != nil {
		return fixedpoint.Money{}, fmt.Errorf("fee notional: %w", err)
	}
	return notional.Mul(rate, fixedpoint.RoundAwayFromZero)
}

// basisNet is the basis term's net contribution: declared favorable
// convergence minus the measured adverse move.
func basisNet(b domain.BasisTerm) (fixedpoint.Money, error) {
	return b.FavorableConvergence.Sub(b.AdverseMove.Amount)
}

// checkDraw enforces that every forecast was drawn at the policy's decision
// quantile — a draw at any other quantile is a scenario the policy never
// stated, and the joint rule's own checks (one quantile, one artifact,
// ascending) run inside Aggregate.
func checkDraw(fs []domain.SettlementForecast, decisionQuantile fixedpoint.Rate) error {
	for n, f := range fs {
		c, err := f.Quantile.Cmp(decisionQuantile)
		if err != nil {
			return fmt.Errorf("forecast %d quantile: %w", n, err)
		}
		if c != 0 {
			return fmt.Errorf("forecast %d drawn at %s, policy decides at %s: %w",
				n, f.Quantile, decisionQuantile, domain.ErrQuantileMixed)
		}
	}
	return nil
}

// refusal builds the journaled refusal Decision (§40f): the verdict that
// declines to score, naming its rule, carrying whatever input had been
// assembled.
func refusal(in domain.DecisionInput, rule string, detail string) domain.Decision {
	refusedBy := rule
	if detail != "" {
		refusedBy = rule + ": " + detail
	}
	return domain.Decision{
		Kind:      in.Kind,
		Verdict:   domain.VerdictRefuse,
		At:        in.At,
		Input:     in,
		RefusedBy: refusedBy,
	}
}
