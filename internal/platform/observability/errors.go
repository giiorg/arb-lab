package observability

import "errors"

var (
	// ErrNameUnset and ErrNameInvalid report a metric that cannot be addressed.
	// Separate sentinels because the causes differ — nobody populated the
	// descriptor, versus somebody built the name wrong — and one sentinel
	// covering both cannot tell an operator which.
	ErrNameUnset   = errors.New("observability: metric name unset")
	ErrNameInvalid = errors.New("observability: metric name malformed")

	// ErrNameSuffix reports a name whose suffix does not match what its Kind
	// and Unit require: a counter that does not end _total, a duration that
	// does not end _seconds, a USD amount that does not end _usd.
	//
	// This is a naming rule with a measurement consequence. The suffix is the
	// only unit information that reaches a dashboard query or an alerting rule
	// — neither reads this descriptor — so a histogram named `..._latency`
	// holding seconds is indistinguishable, at the point of use, from one
	// holding milliseconds.
	ErrNameSuffix = errors.New("observability: metric name lacks its required unit/kind suffix")

	// ErrHelpUnset reports a descriptor with no help text.
	//
	// Required rather than optional, on the default-deny rule: help text is
	// written once, by the person who knows what the metric means, and read at
	// 03:00 by whoever is holding the pager. Optional documentation is absent
	// documentation.
	ErrHelpUnset = errors.New("observability: metric help unset")

	// ErrKindInvalid and ErrUnitInvalid report an undeclared Kind or Unit,
	// including the zero value. Both zero values are unusable sentinels: there
	// is no conservative default kind (a gauge silently accepted as a counter
	// would break rate() forever) and no dimensionless default unit.
	ErrKindInvalid = errors.New("observability: metric kind is not a declared kind")
	ErrUnitInvalid = errors.New("observability: metric unit is not a declared unit")

	// ErrAssetUnset reports a UnitMoney descriptor that named no asset, and
	// ErrAssetPresent an asset named on a descriptor whose unit is not money.
	// The second direction matters: an asset on a seconds histogram is a
	// descriptor somebody edited halfway, and accepting it would silently drop
	// the tag the money check depends on.
	ErrAssetUnset   = errors.New("observability: money metric names no asset")
	ErrAssetPresent = errors.New("observability: non-money metric names an asset")

	// ErrLabelNameInvalid reports a label name outside the permitted character
	// set, and ErrLabelDuplicate the same name declared twice — where one
	// declaration silently wins and the other's value set is never enforced.
	ErrLabelNameInvalid = errors.New("observability: label name malformed")
	ErrLabelDuplicate   = errors.New("observability: label declared twice")

	// ErrLabelUnbounded reports a label that declared neither a value set nor a
	// reason for having none.
	//
	// This is the package's central default-deny, and it is the same shape as
	// //nofloat:allow <reason> and config:"hot" — the unsafe direction is
	// available, it is spelled out, and it carries a written reason. An
	// undeclared label set is how a metrics system kills the process it was
	// added to observe: one time series per ids.ClientOrderID is unbounded
	// memory growth that no back-pressure signal reports, because §3.4's lossy
	// lane is working exactly as designed the whole time.
	ErrLabelUnbounded = errors.New("observability: label declares no value set and no unbounded reason")

	// ErrLabelValuesEmpty reports a declared-but-empty value set, and
	// ErrLabelValueDuplicate the same value listed twice. An empty set is
	// refused rather than read as "unbounded": that reading turns a typo into
	// the very escape this package makes people write a reason for.
	ErrLabelValuesEmpty    = errors.New("observability: label declares an empty value set")
	ErrLabelValueDuplicate = errors.New("observability: label value listed twice")

	// ErrLabelCardinality reports a declared value set larger than
	// MaxLabelValues. Declaring the set is the discipline; declaring ten
	// thousand of them satisfies the letter of it and none of the intent.
	ErrLabelCardinality = errors.New("observability: label value set exceeds the cardinality bound")

	// ErrLabelArity reports a record call whose value count does not match the
	// descriptor's label count. Positional values are how the wire format works
	// and how prometheus/client_golang works; the arity check is what stops a
	// label being inserted upstream and every call site silently shifting by
	// one.
	ErrLabelArity = errors.New("observability: label value count does not match the descriptor")

	// ErrLabelValueUndeclared reports a record call carrying a value outside
	// its label's declared set. The observation is refused and counted; it
	// never blocks and never widens the set.
	ErrLabelValueUndeclared = errors.New("observability: label value is not in the declared set")

	// ErrBucketsUnset reports a histogram with no buckets, and
	// ErrBucketsPresent buckets on a counter or a gauge. A bucketless histogram
	// records a count and a sum and answers no quantile question — which is the
	// only reason to have chosen a histogram.
	ErrBucketsUnset   = errors.New("observability: histogram declares no buckets")
	ErrBucketsPresent = errors.New("observability: non-histogram declares buckets")

	// ErrBucketsUnordered reports buckets that are not strictly increasing.
	// Cumulative buckets that repeat or go backwards produce a non-monotonic
	// cumulative count, which every histogram_quantile implementation reads as
	// corrupt data rather than as an error.
	ErrBucketsUnordered = errors.New("observability: histogram buckets are not strictly increasing")

	// ErrSampleUnset reports an unconstructed Sample — the zero value. It is
	// unusable for the same reason fixedpoint's zero values are: an
	// unpopulated latency reads as instantaneous and an unpopulated cost reads
	// as free.
	ErrSampleUnset = errors.New("observability: sample unset")

	// ErrUnitMismatch reports a Sample recorded against a metric of a different
	// unit, and ErrAssetMismatch a money Sample recorded against a metric
	// denominated in a different asset.
	//
	// The asset case is fixedpoint's runtime tag check extended one layer out.
	// Money is a distinct compile-time category, so a Quantity cannot reach a
	// money gauge — but a BTC-denominated Money and a USDT-denominated Money
	// are the same Go type, and the one that lands on the USD PnL panel is
	// wrong by a factor of the BTC price with nothing failing.
	ErrUnitMismatch  = errors.New("observability: sample unit does not match the metric")
	ErrAssetMismatch = errors.New("observability: sample asset does not match the metric")

	// ErrCounterDelta reports a negative delta offered to a counter.
	//
	// The type system carries none of this for us, because a counter's unit may
	// be money and funding genuinely goes negative — you pay it. A counter that
	// decreases is not merely wrong at that instant: every rate() over the
	// series afterwards reads the decrease as a process restart and silently
	// re-bases. Cumulative funding therefore belongs in a gauge, or in two
	// counters (received and paid), and this error is where that gets decided
	// rather than discovered on a dashboard.
	ErrCounterDelta = errors.New("observability: counter delta is negative")

	// ErrValueOverflow reports an accumulation that would not fit in int64.
	//
	// Refused rather than wrapped, and this is the outbox backoff's lesson
	// applied to its mirror: a wrapped int64 does not become large, it becomes
	// NEGATIVE. A counter that wraps therefore reports a decrease, every rate()
	// over it re-bases as though the process restarted, and the series goes on
	// looking healthy while carrying nothing.
	ErrValueOverflow = errors.New("observability: value would overflow int64")

	// ErrCounterUnit reports Inc called on a metric whose unit is not
	// UnitCount. "One" is only meaningful for a dimensionless count — one
	// second, one dollar and one byte are not the same increment, and the
	// convenience method has no way to ask which was meant.
	ErrCounterUnit = errors.New("observability: Inc is only defined for UnitCount metrics")

	// ErrMetricDuplicate reports a name registered twice with a different
	// descriptor. Registering the same descriptor twice is allowed and returns
	// the existing handle — two packages instrumenting the same thing is
	// normal; two packages disagreeing about what it means is not.
	ErrMetricDuplicate = errors.New("observability: metric already registered with a different descriptor")

	// ErrKindMismatch reports a handle requested at a kind the registered
	// descriptor does not have — Counter(spec) where spec.Kind is a gauge.
	ErrKindMismatch = errors.New("observability: metric kind does not match the requested handle")

	// ErrClockNil and ErrHandlerNil report a logger wired without one of its
	// two collaborators, named separately so the failure says which wire is
	// missing.
	ErrClockNil   = errors.New("observability: clock is nil")
	ErrHandlerNil = errors.New("observability: handler is nil")

	// ErrCapacityInvalid reports a non-positive buffer capacity. Same refusal
	// events.NewObservabilityLane makes, for the same reason: a zero-capacity
	// buffer drops everything while reporting itself as working.
	ErrCapacityInvalid = errors.New("observability: buffer capacity must be positive")
)
