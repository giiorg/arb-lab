// Package observability is build-spec §10's metrics and structured logs, on
// §3.4's isolated, lossy lane.
//
// # Three defaults, each pointed the way that is right when somebody forgets
//
// A label declares its permitted values, or writes down why it cannot
// (Label.Unbounded). A descriptor's zero value fails Validate, so an
// unpopulated metric registers nothing rather than reporting zero forever. And
// a recorded value carries the unit it was converted into, so the conversion is
// checked rather than trusted. Each of these is the same rule the rest of the
// kernel runs on — nofloat denies float64 until a reason is written, config
// denies hot-reload until a tag says otherwise — pointed at the three ways a
// metrics layer lies quietly.
//
// # The hazard that is specific to this package
//
// §3.4 makes this lane lossy so that back-pressure here never blocks trading,
// and that guarantee is about a stalled consumer. It says nothing about the
// other way observability kills a trading process, which is cardinality: a
// metric labelled by ids.ClientOrderID allocates one time series per order,
// forever, in the trading process's own heap. Nothing stalls. Every scrape
// succeeds, the lane reports zero drops, and the lossy-lane guarantee holds
// perfectly for the entire duration of the outage it fails to prevent. Hence
// Label, and hence MaxLabelValues bounding even the declared sets.
//
// # No float64 above the export seam
//
// Every wire metric format carries a double, so a float64 exists somewhere. It
// exists in exactly one file (export.go), and the ports above it are typed in
// int64 mantissas at fixedpoint's canonical scale. Had Counter.Add mirrored
// prometheus.Counter.Add and taken a float64, §3.6 rule 4's single sanctioned
// metric seam would have become a //nofloat:allow at every instrumented line in
// the system — each written by somebody thinking about a dashboard rather than
// about rounding. `grep -c nofloat:allow` over this package is the check that it
// stayed where it was put.
//
// # What is deferred, and why that is a decision rather than a gap
//
//   - The Prometheus adapter. §11 names prometheus/client_golang and §3.6 rule 9
//     scopes kernelpurity to in-module imports specifically so this package may
//     take it. It is still deferred: nothing under internal/ emits a metric yet,
//     so a registered exporter would be wiring no test can exercise end to end,
//     and WHICH binary exposes /metrics on WHICH interface is a composition-root
//     decision — §11 gives arblab-actuator no HTTP surface at all. The
//     conversion an adapter needs (export.go) is NOT deferred, because it
//     belongs to metrics rather than to any one client library.
//   - Tracing. §10 lists it alongside metrics and logs. A span is a claim about
//     a causal chain, and this system's causal chain is §3.5's DispatchSeq —
//     which does not exist until the dispatcher does. A tracing API built now
//     would model goroutine call trees, which is the wrong spine, and it would
//     be load-bearing by the time the right one arrived.
//
// # Nothing here reads the wall clock
//
// Logger takes a clock.Clock and builds its slog.Record from it, because
// slog.Logger reads time.Now() internally where no Handler can reach. §3.6 rule
// 5's notimenow gate polices domain/, application/ and engine/ — not platform/,
// which is where platform/clock has to live — so this is precisely the region
// where an ambient clock read would be invisible to the gate. See Logger.
package observability
