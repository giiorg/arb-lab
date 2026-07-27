package driven

import "errors"

// The driven adapters' refusals. Sentinels for branching, wrapped at the site
// with the values that failed — application/errors.go's rule and domain's, for
// the same reason: one sentinel cannot say which check fired.
var (
	// ErrStoreNil and ErrLaneNil report an adapter constructed without the
	// thing it adapts. Separate sentinels because they name different wires,
	// and refused at construction rather than at first use: a ConfigBudgets
	// over a nil store would report every venue unconfigured, which reads
	// downstream as a deliberate configuration decision rather than as the
	// wiring mistake it is.
	ErrStoreNil = errors.New("marketdata: freshness config store not supplied")
	ErrLaneNil  = errors.New("marketdata: market-data lane not supplied")

	// ErrNoVenues reports a funding-freshness section with no venues in it.
	//
	// Refused rather than accepted as "deny everything", on the same argument
	// application.ErrNoBudget makes one layer in: a process that boots with an
	// empty section refuses every observation for the rest of its life, which
	// presents to an operator as a total market-data outage and sends them to
	// the exchange status page. It is also §11's zero-value rule applied to a
	// section rather than to a field — an unpopulated FreshnessConfig must not
	// be a usable one.
	ErrNoVenues = errors.New("marketdata: no venue has a funding freshness budget configured")
)
