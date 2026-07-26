package observability

import "fmt"

// Meter vends metric handles from descriptors. It is the port an instrumented
// component holds.
//
// # Why the handle is requested by Kind rather than returned as a sum
//
// Counter, Gauge and Histogram have genuinely different contracts — a counter
// refuses a decrease, a histogram has buckets — and a single Metric interface
// carrying all three verbs would put "Observe on a counter" at runtime instead
// of at the type. Asking for the handle you intend to use also means the Kind in
// the descriptor is checked against the call site rather than trusted, which is
// what ErrKindMismatch reports.
//
// Every method returns an error, and construction is where the descriptor is
// validated. That is deliberately the expensive, fallible end of the package:
// recording is on the trading path and does no allocation-heavy work, and
// wiring is at boot where a failure should stop the process.
type Meter interface {
	// Counter returns a handle for a KindCounter descriptor.
	Counter(Spec) (Counter, error)

	// Gauge returns a handle for a KindGauge descriptor.
	Gauge(Spec) (Gauge, error)

	// Histogram returns a handle for a KindHistogram descriptor.
	Histogram(Spec) (Histogram, error)
}

// Counter is a monotonically non-decreasing series.
//
// # Add takes a Sample, and the sign check is not redundant
//
// It would be tempting to type the delta as a uint64 and let the compiler
// enforce monotonicity. It does not work here, because a counter's unit may be
// money and money is signed for a real reason: funding is received on some
// settlements and PAID on others, and §2.4's whole kill-gate arithmetic depends
// on that sign surviving. So the delta arrives as a Sample and the refusal is
// explicit — which is also the point at which somebody discovers that cumulative
// funding wants a gauge, or two counters, rather than one that goes backwards.
type Counter interface {
	// Add increases the series identified by labelValues, in the descriptor's
	// declared label order.
	//
	// It returns an error for a negative delta, a unit or asset mismatch, a
	// label arity mismatch, or an undeclared label value. The observation is
	// dropped in each case — never partially applied.
	Add(delta Sample, labelValues ...string) error

	// Inc adds one, and is defined only for UnitCount metrics. One second, one
	// dollar and one byte are not the same increment and this method cannot ask
	// which was meant, so it refuses rather than guessing.
	Inc(labelValues ...string) error
}

// Gauge is a value that moves in both directions.
type Gauge interface {
	// Set replaces the current value of the series identified by labelValues.
	Set(v Sample, labelValues ...string) error
}

// Histogram is a distribution recorded into the descriptor's declared buckets.
type Histogram interface {
	// Observe records one value into the series identified by labelValues.
	//
	// Negative observations are permitted: a cross-leg skew and a basis
	// divergence are both signed, and a histogram that refused them would push
	// every call site into recording an absolute value, which discards the
	// direction that says which leg is ahead.
	Observe(v Sample, labelValues ...string) error
}

// checkLabels validates positional label values against a descriptor and
// returns the series key.
//
// The key is the values joined by a byte that indexNotIdentifier excludes from
// every label value, so two different value tuples cannot collide on one key.
// Joining on a permitted character would let ("ab","c") and ("a","bc") name the
// same series — the classic delimiter collision, and here it would silently
// merge two venues' error counts.
func checkLabels(s Spec, values []string) (string, error) {
	if len(values) != len(s.Labels) {
		return "", newLabelArityError(s, values)
	}
	if len(values) == 0 {
		return "", nil
	}

	// Sized from the values so the common single-label case does one
	// allocation.
	n := len(values)
	for _, v := range values {
		n += len(v)
	}
	key := make([]byte, 0, n)
	for i, v := range values {
		if !s.Labels[i].permits(v) {
			return "", newLabelValueError(s, i, v)
		}
		if i > 0 {
			key = append(key, keySeparator)
		}
		key = append(key, v...)
	}
	return string(key), nil
}

// newLabelArityError names both counts and the labels the descriptor declares.
//
// The declared names are included because the arity mismatch this exists to
// catch is almost always an insertion upstream — somebody added a `venue` label
// to a shared descriptor — and "wanted 3, got 2" sends the reader to count
// arguments while "wanted [venue symbol side]" tells them which one is new.
func newLabelArityError(s Spec, values []string) error {
	names := make([]string, len(s.Labels))
	for i, l := range s.Labels {
		names[i] = l.Name
	}
	return fmt.Errorf("metric %q declares %d labels %v, got %d %v: %w",
		s.Name, len(s.Labels), names, len(values), values, ErrLabelArity)
}

// newLabelValueError names the offending value and the set it was checked
// against.
//
// The permitted set is printed because this error is the cardinality guard
// firing, and the two readings of it are opposite: either the call site passed
// something wrong, or the declared set is genuinely incomplete and needs
// extending. Only the set makes that distinguishable without opening the
// descriptor.
func newLabelValueError(s Spec, i int, v string) error {
	return fmt.Errorf("metric %q label %q got %q, declared %v: %w",
		s.Name, s.Labels[i].Name, v, s.Labels[i].Values, ErrLabelValueUndeclared)
}

// The four constructors below are shared by Registry and NopMeter so the two
// report a refusal identically. Divergent messages would be a quiet way for the
// vacuous-test failure NopMeter exists to prevent to come back: a test asserting
// on an error string would pass against the no-op meter and fail against the
// real one, or worse, the reverse.

func newKindMismatchError(s Spec, want Kind) error {
	return fmt.Errorf("metric %q is a %s, requested as %s: %w",
		s.Name, s.Kind, want, ErrKindMismatch)
}

func newCounterDeltaError(s Spec, delta Sample) error {
	return fmt.Errorf("metric %q offered %s: %w", s.Name, delta, ErrCounterDelta)
}

func newCounterUnitError(s Spec) error {
	return fmt.Errorf("metric %q is %s: %w", s.Name, s.Unit, ErrCounterUnit)
}

func newSampleError(s Spec, err error) error {
	return fmt.Errorf("metric %q: %w", s.Name, err)
}

// keySeparator joins label values into a series key. 0x1f (ASCII unit
// separator) is outside indexNotIdentifier's allow-list, so no label NAME can
// contain it; a label VALUE is only guaranteed free of it when the label is
// bounded, since an unbounded label's values are not character-checked.
//
// That gap is closed at the source rather than here: recordable values reaching
// an unbounded label come from identifiers this system mints, and a value
// carrying a control character would be a venue-adapter defect. Stated because a
// separator whose uniqueness is assumed rather than argued is how two series
// silently become one.
const keySeparator = 0x1f
