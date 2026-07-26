package observability

import (
	"fmt"
	"strings"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// Kind names the shape of a metric.
//
// Integer enum, same rule 8 exception and same reasoning as Unit: what crosses
// to an exporter is the name and the sample, not this.
type Kind uint8

const (
	// KindUnspecified is the zero value and names no shape. Unusable, and there
	// is deliberately no default: a gauge accepted as a counter produces a
	// series that rate() reads as resetting on every decrease, and a counter
	// accepted as a gauge loses the reset semantics that make a restart
	// visible. Neither error is detectable from the data afterwards.
	KindUnspecified Kind = iota

	// KindCounter is monotonically non-decreasing and resets to zero on
	// process restart.
	KindCounter

	// KindGauge is a value that moves in both directions: open positions, a
	// backlog depth, an unrealized PnL, a liq distance.
	KindGauge

	// KindHistogram is a distribution, recorded into declared cumulative
	// buckets. §9's per-fault RTO budgets are quantile questions, so the
	// latencies they bound belong here rather than in a gauge holding the last
	// observation.
	KindHistogram
)

const maxKind = KindHistogram

// Validate reports whether k is a declared kind, excluding the sentinel.
func (k Kind) Validate() error {
	if k == KindUnspecified || k > maxKind {
		return fmt.Errorf("kind %d: %w", uint8(k), ErrKindInvalid)
	}
	return nil
}

// String implements fmt.Stringer.
func (k Kind) String() string {
	switch k {
	case KindUnspecified:
		return "KindUnspecified"
	case KindCounter:
		return "KindCounter"
	case KindGauge:
		return "KindGauge"
	case KindHistogram:
		return "KindHistogram"
	}
	return fmt.Sprintf("Kind(%d)", uint8(k))
}

// MaxLabelValues bounds a single label's declared value set.
//
// A bound on the escape-free path, not just on the escape. Declaring a set is
// the discipline; declaring ten thousand members satisfies its letter and none
// of its intent, and the product across labels is what actually sizes the
// registry — two labels of 100 values each is 10,000 series from a descriptor
// that looks modest.
//
// The number is not derived from anything. It is comfortably above every real
// bounded dimension in this system (2 venues, 2 product families, ~8 apperror
// classes, ~10 order states, a handful of symbols) and comfortably below the
// point where a value set is really an unbounded one somebody enumerated.
const MaxLabelValues = 64

// MaxLabelNameLen and MaxNameLen bound the two identifiers.
const (
	MaxNameLen      = 128
	MaxLabelNameLen = 64
)

// Label declares one dimension of a metric, and the values it may take.
//
// # Declaring the values is the point
//
// [SPEC EDIT LANDED — v0.9.6] §10 required metrics cut per venue and per symbol
// and said nothing about label cardinality, and §3.4 states the observability lane's
// isolation as "back-pressure here never blocks trading" — a guarantee about a
// stalled CONSUMER. Neither covers the way a metrics layer actually takes a
// trading process down, which is unbounded label cardinality: a metric labelled
// by ids.ClientOrderID allocates one time series per order, forever, inside the
// trading process's own heap. Nothing stalls. Every scrape succeeds, the lane
// reports zero drops, and §3.4's guarantee holds perfectly for the entire
// duration of the outage it fails to prevent. So the spec requires labelled
// metrics and relies, unstated, on somebody bounding their domains — the same
// shape as §3.2's unstated outbox ORDER guarantee. This package makes the bound
// explicit and default-deny, and scopes the cost: a label whose domain is
// genuinely open still works, it just has to say so in writing.
//
// So a label states its values up front, and Values is checked at record time.
// The check is what makes the declaration load-bearing rather than
// documentation.
//
// # And the escape carries a reason
//
// Some dimensions genuinely are open — a venue error code, a symbol on a venue
// whose listings change. Unbounded takes those, and it is a non-empty string
// because it is a reason, not a flag: the same shape as //nofloat:allow <reason>
// and config:"hot", and for the same purpose, which is that the person adding
// the risk writes down why they are entitled to.
type Label struct {
	// Name is the label key.
	Name string

	// Values is the closed set this label may take. Order is not significant.
	Values []string

	// Unbounded, when non-empty, is the written reason this label declares no
	// value set. Mutually exclusive with Values: declaring both is a
	// descriptor somebody edited halfway, and the safe reading of it is not
	// obvious enough to guess at.
	Unbounded string
}

// bounded reports whether this label enforces a value set.
func (l Label) bounded() bool { return l.Unbounded == "" }

// Validate reports whether the label is a usable declaration.
func (l Label) Validate() error {
	if l.Name == "" {
		return fmt.Errorf("label name empty: %w", ErrLabelNameInvalid)
	}
	if len(l.Name) > MaxLabelNameLen {
		return fmt.Errorf("label %q is %d bytes, limit %d: %w",
			l.Name, len(l.Name), MaxLabelNameLen, ErrLabelNameInvalid)
	}
	if i := indexNotIdentifier(l.Name); i >= 0 {
		return fmt.Errorf("label %q holds %q at byte %d: %w",
			l.Name, l.Name[i], i, ErrLabelNameInvalid)
	}

	switch {
	case l.Unbounded != "" && len(l.Values) > 0:
		return fmt.Errorf("label %q declares both a value set and an unbounded reason: %w",
			l.Name, ErrLabelUnbounded)
	case l.Unbounded != "":
		return nil
	case l.Values == nil:
		return fmt.Errorf("label %q: %w", l.Name, ErrLabelUnbounded)
	case len(l.Values) == 0:
		// Distinct from the nil case above. `Values: []string{}` is a caller
		// who meant to enumerate and enumerated nothing — a filtered slice that
		// came back empty, most often — and reading it as unbounded would hand
		// out the escape without the reason.
		return fmt.Errorf("label %q: %w", l.Name, ErrLabelValuesEmpty)
	case len(l.Values) > MaxLabelValues:
		return fmt.Errorf("label %q declares %d values, limit %d: %w",
			l.Name, len(l.Values), MaxLabelValues, ErrLabelCardinality)
	}

	seen := make(map[string]struct{}, len(l.Values))
	for _, v := range l.Values {
		if v == "" {
			return fmt.Errorf("label %q declares an empty value: %w", l.Name, ErrLabelNameInvalid)
		}
		if _, dup := seen[v]; dup {
			return fmt.Errorf("label %q lists %q twice: %w", l.Name, v, ErrLabelValueDuplicate)
		}
		seen[v] = struct{}{}
	}
	return nil
}

// permits reports whether v is admissible for this label.
func (l Label) permits(v string) bool {
	if !l.bounded() {
		return v != ""
	}
	for _, want := range l.Values {
		if v == want {
			return true
		}
	}
	return false
}

// Spec is a metric's descriptor: everything about it that is fixed at
// construction.
//
// The zero value is unusable, per the kernel-wide rule. An unpopulated
// descriptor would otherwise register a nameless, helpless, dimensionless
// counter — which does not fail, and reports zero forever.
type Spec struct {
	// Name is the metric's identifier, and must carry the suffix its Kind and
	// Unit require (see Validate).
	Name string

	// Help is the one-line explanation an operator reads on a panel. Required.
	Help string

	// Kind and Unit fix the shape and the dimension.
	Kind Kind
	Unit Unit

	// Asset denominates a UnitMoney metric. Required for one, refused on
	// anything else.
	Asset fixedpoint.Asset

	// Labels declares the metric's dimensions, in the order record calls pass
	// their values. Order is significant and part of the descriptor: it is what
	// makes ErrLabelArity able to catch a label inserted upstream.
	Labels []Label

	// Buckets are a histogram's cumulative upper bounds, strictly increasing,
	// as Samples in the metric's own unit. Required for a histogram, refused on
	// anything else.
	//
	// Samples rather than raw integers so that a bucket boundary goes through
	// exactly the same unit and asset check an observation does. A histogram
	// whose observations are in seconds and whose buckets were written in
	// milliseconds is a distribution entirely in the overflow bucket, which
	// looks like a system that is uniformly slow.
	Buckets []Sample
}

// Validate reports whether the descriptor is usable.
//
// Checked in the order a reader would debug: what it is called, what it means,
// what shape it has, what dimension it measures, how it is cut, and how it is
// bucketed.
func (s Spec) Validate() error {
	if err := s.validateName(); err != nil {
		return err
	}
	if s.Help == "" {
		return fmt.Errorf("metric %q: %w", s.Name, ErrHelpUnset)
	}
	if err := s.Kind.Validate(); err != nil {
		return fmt.Errorf("metric %q: %w", s.Name, err)
	}
	if err := s.Unit.Validate(); err != nil {
		return fmt.Errorf("metric %q: %w", s.Name, err)
	}
	if err := s.validateAsset(); err != nil {
		return err
	}
	if err := s.validateSuffix(); err != nil {
		return err
	}
	if err := s.validateLabels(); err != nil {
		return err
	}
	return s.validateBuckets()
}

func (s Spec) validateName() error {
	if s.Name == "" {
		return ErrNameUnset
	}
	if len(s.Name) > MaxNameLen {
		return fmt.Errorf("metric %q is %d bytes, limit %d: %w",
			s.Name, len(s.Name), MaxNameLen, ErrNameInvalid)
	}
	if i := indexNotIdentifier(s.Name); i >= 0 {
		return fmt.Errorf("metric %q holds %q at byte %d: %w",
			s.Name, s.Name[i], i, ErrNameInvalid)
	}
	return nil
}

func (s Spec) validateAsset() error {
	switch {
	case s.Unit == UnitMoney && s.Asset == "":
		return fmt.Errorf("metric %q: %w", s.Name, ErrAssetUnset)
	case s.Unit != UnitMoney && s.Asset != "":
		return fmt.Errorf("metric %q is %s but names asset %q: %w",
			s.Name, s.Unit, s.Asset, ErrAssetPresent)
	}
	return nil
}

// requiredSuffix is what Name must end with, given Kind and Unit.
//
// The two compose in the documented order — unit first, then kind — so a
// counter of seconds is `..._seconds_total`, which is the convention every
// Prometheus exposition and every naming guide uses. Composing them the other
// way round would also be self-consistent and would break every dashboard
// convention a reader brings with them.
func (s Spec) requiredSuffix() string {
	suffix := s.Unit.suffix()
	if s.Unit == UnitMoney {
		suffix = "_" + strings.ToLower(string(s.Asset))
	}
	if s.Kind == KindCounter {
		suffix += "_total"
	}
	return suffix
}

func (s Spec) validateSuffix() error {
	want := s.requiredSuffix()
	if want == "" {
		return nil
	}
	if !strings.HasSuffix(s.Name, want) {
		return fmt.Errorf("metric %q is a %s of %s and must end %q: %w",
			s.Name, s.Kind, s.Unit, want, ErrNameSuffix)
	}
	// A name that is only its suffix has no subject: `_seconds_total` names
	// nothing, passes the check above, and is a real typo shape (an empty
	// namespace variable joined to a suffix).
	if s.Name == want {
		return fmt.Errorf("metric %q is nothing but its suffix: %w", s.Name, ErrNameInvalid)
	}
	return nil
}

func (s Spec) validateLabels() error {
	seen := make(map[string]struct{}, len(s.Labels))
	for _, l := range s.Labels {
		if err := l.Validate(); err != nil {
			return fmt.Errorf("metric %q: %w", s.Name, err)
		}
		if _, dup := seen[l.Name]; dup {
			return fmt.Errorf("metric %q declares label %q twice: %w",
				s.Name, l.Name, ErrLabelDuplicate)
		}
		seen[l.Name] = struct{}{}
	}
	return nil
}

func (s Spec) validateBuckets() error {
	if s.Kind != KindHistogram {
		if len(s.Buckets) > 0 {
			return fmt.Errorf("metric %q is a %s: %w", s.Name, s.Kind, ErrBucketsPresent)
		}
		return nil
	}
	if len(s.Buckets) == 0 {
		return fmt.Errorf("metric %q: %w", s.Name, ErrBucketsUnset)
	}
	for i, b := range s.Buckets {
		if err := b.matches(s.Unit, s.Asset); err != nil {
			return fmt.Errorf("metric %q bucket %d: %w", s.Name, i, err)
		}
		if i > 0 && b.Mantissa() <= s.Buckets[i-1].Mantissa() {
			return fmt.Errorf("metric %q bucket %d (%s) does not exceed bucket %d (%s): %w",
				s.Name, i, b, i-1, s.Buckets[i-1], ErrBucketsUnordered)
		}
	}
	return nil
}

// equal reports whether two descriptors are the same declaration. Used by the
// registry to distinguish re-registering the same metric — normal — from two
// packages disagreeing about what a name means.
func (s Spec) equal(o Spec) bool {
	if s.Name != o.Name || s.Help != o.Help ||
		s.Kind != o.Kind || s.Unit != o.Unit || s.Asset != o.Asset {
		return false
	}
	if len(s.Labels) != len(o.Labels) || len(s.Buckets) != len(o.Buckets) {
		return false
	}
	for i := range s.Labels {
		if s.Labels[i].Name != o.Labels[i].Name ||
			s.Labels[i].Unbounded != o.Labels[i].Unbounded ||
			!equalStrings(s.Labels[i].Values, o.Labels[i].Values) {
			return false
		}
	}
	for i := range s.Buckets {
		if s.Buckets[i] != o.Buckets[i] {
			return false
		}
	}
	return true
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// indexNotIdentifier returns the byte index of the first character outside the
// permitted set, or -1.
//
// An allow-list, matching outbox's, and the direction is the point: a deny-list
// of "whitespace and control characters" admits every character nobody thought
// to name, and these strings are written into an exposition format that is
// newline-delimited.
//
// Narrower than outbox's by two characters. `.` and `:` are excluded because a
// metric name is not a topic: `:` is reserved by convention for recording-rule
// output, so a metric emitting one is claiming to be something it is not, and
// `.` is not permitted in the Prometheus exposition format at all.
func indexNotIdentifier(s string) int {
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z',
			c >= 'A' && c <= 'Z',
			c >= '0' && c <= '9',
			c == '_':
			continue
		}
		return i
	}
	return -1
}
