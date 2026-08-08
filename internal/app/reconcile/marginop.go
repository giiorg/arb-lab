package reconcile

import (
	"github.com/giiorg/arb-lab/internal/oms"
	"github.com/giiorg/arb-lab/internal/oms/domain"
)

// ReconcileMarginOp is `arblab margin reconcile`'s composition: core §7.1's
// "explicit accounting / operator action" against an Ambiguous margin op,
// recorded where core §7.3 says a control action is recorded (task #213,
// record §82).
//
// # It is the SHORTEST composition in this package, and that is the design
//
// RecordBaseline takes the same deployment config and then wires a signed
// client, a rate-limit governor, a balance adapter and a whole Module, because
// minting a baseline reads the venue. This act reads nothing: two fields of the
// config — the journal's path and §5.1's subaccount id — plus what the operator
// typed. Everything else `Config` holds describes venues, credentials, budgets
// and policies that no part of this act consults.
//
// Taking the config anyway rather than a bare path is deliberate: the journal
// this act must write to is THE journal this deployment boots against, and a
// path flag would let an operator clear an incident in a file the engine never
// reads. The config is the one place that binding is already stated.
//
// # It requires no credentials, and that is a property worth keeping
//
// A §9 accounting incident is exactly when a venue may be unreachable, a key may
// have been rotated, or the one-spender lock may be held by something an
// operator is trying to understand. An act that could not be taken under those
// conditions would be unavailable precisely when it is needed.
func ReconcileMarginOp(cfg Config, act domain.MarginOpReconciliation) (oms.MarginOpClearance, error) {
	var zero oms.MarginOpClearance
	if err := cfg.Validate(); err != nil {
		return zero, err
	}
	return oms.ReconcileMarginOp(oms.ClearanceDeps{
		JournalPath: cfg.JournalPath,
		Account:     cfg.Account,
	}, act)
}
