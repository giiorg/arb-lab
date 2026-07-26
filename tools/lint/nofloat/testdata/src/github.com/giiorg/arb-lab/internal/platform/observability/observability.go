// Package observability is the escape hatch working as intended, and it is the
// exception §3 anticipated by name: Prometheus takes float64 at the wire and
// nothing else, so the conversion has to happen somewhere. It happens here, at
// the seam, which is the point — the alternative considered and rejected was
// zero-exception denial, which would have forced this shim outside internal/
// where no gate can see it at all.
//
// Every line below is exempt, so this package must produce NO diagnostic. It is
// the positive control for the hatch: without it, an escape mechanism that never
// actually opened would look identical to one that works.
package observability

import "time"

var latency float64 //nofloat:allow prometheus gauges are float64 at the wire

// A float in a signature is still a float, and it is still annotated. The
// annotation is per-line rather than per-function on purpose: a function-level
// escape would cover code written into it years later.
func Observe(d time.Duration) float64 { //nofloat:allow the metric seam converts here
	latency = d.Seconds() //nofloat:allow duration to seconds, for the histogram only
	return latency        //nofloat:allow the caller is the exporter
}
