package observability_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/observability"
)

// countSpec is a minimal valid descriptor the tests mutate one field at a time.
func countSpec() observability.Spec {
	return observability.Spec{
		Name: "arblab_fills_total",
		Help: "fills observed",
		Kind: observability.KindCounter,
		Unit: observability.UnitCount,
	}
}

func TestSpecZeroValueIsUnusable(t *testing.T) {
	t.Parallel()

	var s observability.Spec
	if err := s.Validate(); err == nil {
		t.Fatal("the zero Spec validated; an unpopulated descriptor must not register")
	}
}

func TestSpecValidateRejects(t *testing.T) {
	t.Parallel()

	usd := fixedpoint.Asset("USD")
	sec := func(n int64) observability.Sample {
		return observability.Seconds(time.Duration(n) * time.Second)
	}

	tests := []struct {
		name string
		spec observability.Spec
		want error
	}{
		{
			name: "name unset",
			spec: observability.Spec{Help: "h", Kind: observability.KindGauge, Unit: observability.UnitCount},
			want: observability.ErrNameUnset,
		},
		{
			// The bad byte is at index 0 deliberately. Every malformed fixture
			// in the outbox suite put its bad character mid-string, where
			// `i >= 0` and `i > 0` are the same predicate, and " audit"
			// validated. Both positions are covered here and below.
			name: "bad byte at index 0",
			spec: withName(countSpec(), " arblab_fills_total"),
			want: observability.ErrNameInvalid,
		},
		{
			name: "bad byte mid-string",
			spec: withName(countSpec(), "arblab fills_total"),
			want: observability.ErrNameInvalid,
		},
		{
			name: "dot is not permitted in an exposition name",
			spec: withName(countSpec(), "arblab.fills_total"),
			want: observability.ErrNameInvalid,
		},
		{
			name: "colon is reserved for recording rules",
			spec: withName(countSpec(), "arblab:fills_total"),
			want: observability.ErrNameInvalid,
		},
		{
			name: "name over the length bound",
			spec: withName(countSpec(), strings.Repeat("a", observability.MaxNameLen)+"_total"),
			want: observability.ErrNameInvalid,
		},
		{
			name: "help unset",
			spec: observability.Spec{Name: "arblab_fills_total", Kind: observability.KindCounter, Unit: observability.UnitCount},
			want: observability.ErrHelpUnset,
		},
		{
			name: "kind unspecified",
			spec: observability.Spec{Name: "arblab_x", Help: "h", Unit: observability.UnitCount},
			want: observability.ErrKindInvalid,
		},
		{
			name: "kind past the declared set",
			spec: observability.Spec{Name: "arblab_x", Help: "h", Kind: observability.KindHistogram + 1, Unit: observability.UnitCount},
			want: observability.ErrKindInvalid,
		},
		{
			name: "unit unspecified",
			spec: observability.Spec{Name: "arblab_x", Help: "h", Kind: observability.KindGauge},
			want: observability.ErrUnitInvalid,
		},
		{
			name: "unit past the declared set",
			spec: observability.Spec{Name: "arblab_x", Help: "h", Kind: observability.KindGauge, Unit: observability.UnitMoney + 1},
			want: observability.ErrUnitInvalid,
		},
		{
			name: "money without an asset",
			spec: observability.Spec{Name: "arblab_pnl_usd", Help: "h", Kind: observability.KindGauge, Unit: observability.UnitMoney},
			want: observability.ErrAssetUnset,
		},
		{
			name: "asset on a non-money metric",
			spec: observability.Spec{Name: "arblab_x", Help: "h", Kind: observability.KindGauge, Unit: observability.UnitCount, Asset: usd},
			want: observability.ErrAssetPresent,
		},
		{
			name: "counter without _total",
			spec: withName(countSpec(), "arblab_fills"),
			want: observability.ErrNameSuffix,
		},
		{
			name: "seconds without _seconds",
			spec: observability.Spec{Name: "arblab_latency", Help: "h", Kind: observability.KindGauge, Unit: observability.UnitSeconds},
			want: observability.ErrNameSuffix,
		},
		{
			name: "seconds counter needs both, in order",
			spec: observability.Spec{Name: "arblab_busy_total_seconds", Help: "h", Kind: observability.KindCounter, Unit: observability.UnitSeconds},
			want: observability.ErrNameSuffix,
		},
		{
			name: "money suffix must be the asset",
			spec: observability.Spec{Name: "arblab_pnl_eur", Help: "h", Kind: observability.KindGauge, Unit: observability.UnitMoney, Asset: usd},
			want: observability.ErrNameSuffix,
		},
		{
			name: "name that is only its suffix",
			spec: observability.Spec{Name: "_seconds", Help: "h", Kind: observability.KindGauge, Unit: observability.UnitSeconds},
			want: observability.ErrNameInvalid,
		},
		{
			name: "label declares neither values nor a reason",
			spec: withLabels(countSpec(), observability.Label{Name: "venue"}),
			want: observability.ErrLabelUnbounded,
		},
		{
			name: "label declares both",
			spec: withLabels(countSpec(), observability.Label{
				Name: "venue", Values: []string{"binance"}, Unbounded: "why",
			}),
			want: observability.ErrLabelUnbounded,
		},
		{
			// Distinct from the nil case: an explicitly empty slice is a caller
			// who meant to enumerate. Reading it as unbounded would hand out
			// the escape without the written reason.
			name: "label declares an empty value set",
			spec: withLabels(countSpec(), observability.Label{Name: "venue", Values: []string{}}),
			want: observability.ErrLabelValuesEmpty,
		},
		{
			// An empty member of a declared set is the escape arriving by the
			// back door: permits() would then admit "", which is what a caller
			// passes when the value was never populated, so every unlabelled
			// observation merges into one series.
			name: "label declares an empty value inside its set",
			spec: withLabels(countSpec(), observability.Label{
				Name: "venue", Values: []string{"binance", ""},
			}),
			want: observability.ErrLabelNameInvalid,
		},
		{
			name: "label lists a value twice",
			spec: withLabels(countSpec(), observability.Label{
				Name: "venue", Values: []string{"binance", "bybit", "binance"},
			}),
			want: observability.ErrLabelValueDuplicate,
		},
		{
			name: "label value set over the cardinality bound",
			spec: withLabels(countSpec(), observability.Label{
				Name: "order", Values: manyValues(observability.MaxLabelValues + 1),
			}),
			want: observability.ErrLabelCardinality,
		},
		{
			name: "label name malformed at index 0",
			spec: withLabels(countSpec(), observability.Label{Name: "-venue", Values: []string{"binance"}}),
			want: observability.ErrLabelNameInvalid,
		},
		{
			name: "label name malformed mid-string",
			spec: withLabels(countSpec(), observability.Label{Name: "ven ue", Values: []string{"binance"}}),
			want: observability.ErrLabelNameInvalid,
		},
		{
			name: "label declared twice",
			spec: withLabels(countSpec(),
				observability.Label{Name: "venue", Values: []string{"binance"}},
				observability.Label{Name: "venue", Values: []string{"bybit"}},
			),
			want: observability.ErrLabelDuplicate,
		},
		{
			name: "histogram without buckets",
			spec: observability.Spec{Name: "arblab_rtt_seconds", Help: "h", Kind: observability.KindHistogram, Unit: observability.UnitSeconds},
			want: observability.ErrBucketsUnset,
		},
		{
			name: "buckets on a gauge",
			spec: observability.Spec{
				Name: "arblab_rtt_seconds", Help: "h", Kind: observability.KindGauge,
				Unit: observability.UnitSeconds, Buckets: []observability.Sample{sec(1)},
			},
			want: observability.ErrBucketsPresent,
		},
		{
			name: "buckets not increasing",
			spec: observability.Spec{
				Name: "arblab_rtt_seconds", Help: "h", Kind: observability.KindHistogram,
				Unit: observability.UnitSeconds, Buckets: []observability.Sample{sec(1), sec(3), sec(3)},
			},
			want: observability.ErrBucketsUnordered,
		},
		{
			// The failure this catches is a histogram whose observations are in
			// seconds and whose bounds were written in some other dimension:
			// every observation lands in the overflow bucket, which reads as a
			// uniformly slow system rather than as a broken descriptor.
			name: "bucket in the wrong unit",
			spec: observability.Spec{
				Name: "arblab_rtt_seconds", Help: "h", Kind: observability.KindHistogram,
				Unit: observability.UnitSeconds, Buckets: []observability.Sample{observability.Count(1)},
			},
			want: observability.ErrUnitMismatch,
		},
		{
			name: "unconstructed bucket",
			spec: observability.Spec{
				Name: "arblab_rtt_seconds", Help: "h", Kind: observability.KindHistogram,
				Unit: observability.UnitSeconds, Buckets: []observability.Sample{{}},
			},
			want: observability.ErrSampleUnset,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.spec.Validate()
			if !errors.Is(err, tc.want) {
				t.Fatalf("Validate() = %v; want %v", err, tc.want)
			}
		})
	}
}

// TestSpecValidateAccepts is the positive control. Without it a Validate that
// objected to everything would score a perfect pass on the table above.
func TestSpecValidateAccepts(t *testing.T) {
	t.Parallel()

	usd := fixedpoint.Asset("USD")
	sec := func(n int64) observability.Sample {
		return observability.Seconds(time.Duration(n) * time.Second)
	}

	tests := []struct {
		name string
		spec observability.Spec
	}{
		{"plain counter", countSpec()},
		{"gauge of counts needs no suffix", observability.Spec{
			Name: "arblab_open_positions", Help: "h",
			Kind: observability.KindGauge, Unit: observability.UnitCount,
		}},
		{"seconds counter, unit before kind", observability.Spec{
			Name: "arblab_busy_seconds_total", Help: "h",
			Kind: observability.KindCounter, Unit: observability.UnitSeconds,
		}},
		{"money gauge suffixed by its asset, lowercased", observability.Spec{
			Name: "arblab_unrealized_pnl_usd", Help: "h",
			Kind: observability.KindGauge, Unit: observability.UnitMoney, Asset: usd,
		}},
		{"ratio gauge", observability.Spec{
			Name: "arblab_funding_ratio", Help: "h",
			Kind: observability.KindGauge, Unit: observability.UnitRatio,
		}},
		{"bytes gauge", observability.Spec{
			Name: "arblab_journal_bytes", Help: "h",
			Kind: observability.KindGauge, Unit: observability.UnitBytes,
		}},
		{"bounded labels", withLabels(countSpec(),
			observability.Label{Name: "venue", Values: []string{"binance", "bybit"}},
			observability.Label{Name: "side", Values: []string{"buy", "sell"}},
		)},
		{"unbounded label with a written reason", withLabels(countSpec(),
			observability.Label{Name: "venue_code", Unbounded: "venue error codes are an open set"},
		)},
		{"label value set exactly at the bound", withLabels(countSpec(),
			observability.Label{Name: "bucket", Values: manyValues(observability.MaxLabelValues)},
		)},
		{"histogram", observability.Spec{
			Name: "arblab_rtt_seconds", Help: "h", Kind: observability.KindHistogram,
			Unit: observability.UnitSeconds, Buckets: []observability.Sample{sec(1), sec(2), sec(5)},
		}},
		{"negative bucket bounds, increasing", observability.Spec{
			Name: "arblab_skew_seconds", Help: "h", Kind: observability.KindHistogram,
			Unit: observability.UnitSeconds, Buckets: []observability.Sample{sec(-2), sec(-1), sec(0), sec(1)},
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if err := tc.spec.Validate(); err != nil {
				t.Fatalf("Validate() = %v; want nil", err)
			}
		})
	}
}

// TestKindAndUnitStringersCoverTheDeclaredSet fails when a value is added
// without extending its String, which is the shape the deferred exhaustiveswitch
// gate exists to police.
func TestKindAndUnitStringersCoverTheDeclaredSet(t *testing.T) {
	t.Parallel()

	for k := observability.KindUnspecified; k <= observability.KindHistogram; k++ {
		if got := k.String(); strings.HasPrefix(got, "Kind(") {
			t.Errorf("Kind(%d).String() = %q; a declared kind fell through", uint8(k), got)
		}
	}
	for u := observability.UnitUnspecified; u <= observability.UnitMoney; u++ {
		if got := u.String(); strings.HasPrefix(got, "Unit(") {
			t.Errorf("Unit(%d).String() = %q; a declared unit fell through", uint8(u), got)
		}
	}

	// The negative control: an undeclared value must fall through, or the check
	// above proves nothing.
	if got := (observability.KindHistogram + 1).String(); !strings.HasPrefix(got, "Kind(") {
		t.Errorf("an undeclared Kind rendered as %q; the fallthrough is not reachable", got)
	}
	if got := (observability.UnitMoney + 1).String(); !strings.HasPrefix(got, "Unit(") {
		t.Errorf("an undeclared Unit rendered as %q; the fallthrough is not reachable", got)
	}
}

func withName(s observability.Spec, name string) observability.Spec {
	s.Name = name
	return s
}

func withLabels(s observability.Spec, labels ...observability.Label) observability.Spec {
	s.Labels = labels
	return s
}

func manyValues(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = "v" + string(rune('a'+i%26)) + string(rune('a'+i/26))
	}
	return out
}
