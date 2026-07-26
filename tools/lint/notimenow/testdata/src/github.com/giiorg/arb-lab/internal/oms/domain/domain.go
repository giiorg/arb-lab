// Package domain is the offending fixture: a bounded context's domain layer
// reading the wall clock every way there is.
package domain

import "time"

type Order struct {
	Placed time.Time
	TTL    time.Duration
}

// The rule as build-spec §3.6 rule 5 names it.
func (o Order) Expired() bool {
	return time.Now().After(o.Placed.Add(o.TTL)) // want `time\.Now reads the wall clock, which domain code must not do`
}

// Since is Now with arithmetic attached, and it is what anyone reaching for
// elapsed time writes first. A gate that stopped at Now would be bypassed here
// by idiom rather than by evasion.
func (o Order) Age() time.Duration {
	return time.Since(o.Placed) // want `time\.Since reads the wall clock`
}

func (o Order) Remaining() time.Duration {
	return time.Until(o.Placed.Add(o.TTL)) // want `time\.Until reads the wall clock`
}

// The scheduling family. §3.5 assigns these to Clock.SetTimer so that the
// backtester can drive time instead of waiting for it — a domain that sleeps is
// a domain that cannot be replayed.
func (o Order) Await() {
	time.Sleep(o.TTL)                    // want `time\.Sleep reads the wall clock`
	<-time.After(o.TTL)                  // want `time\.After reads the wall clock`
	<-time.Tick(o.TTL)                   // want `time\.Tick reads the wall clock`
	_ = time.NewTimer(o.TTL)             // want `time\.NewTimer reads the wall clock`
	_ = time.NewTicker(o.TTL)            // want `time\.NewTicker reads the wall clock`
	_ = time.AfterFunc(o.TTL, func() {}) // want `time\.AfterFunc reads the wall clock`
}

// Taken as a value rather than called. The violation is the reference: whoever
// calls deadline() later reads this machine's clock, and a check shaped around
// call expressions would see nothing here at all.
var deadline = time.Now // want `time\.Now reads the wall clock`

// Two ambient reads on one line produce ONE diagnostic, not two. The reduction
// is not cosmetic: the escape hatch works per line, so diagnosing per reference
// while suppressing per line would let a single annotation silently answer
// several findings. The rule reports the earliest read on the line and stops.
func (o Order) Span(from time.Time) time.Duration {
	return time.Since(o.Placed) - time.Since(from) // want `time\.Since reads the wall clock`
}

// The escape hatch is engine-only, and this is the control that pins it. Rule 5
// admits an annotated exception for live-only transport; it admits none here,
// where a wall-clock read is a design error and injecting the Clock is always
// the fix. So the annotation suppresses nothing AND is called out — a directive
// that silently did nothing would read as a working suppression to whoever
// wrote it.
func (o Order) Escaped() bool {
	return time.Now().After(o.Placed) /*notimenow:allow domain code cannot escape this rule*/ // want `time\.Now reads the wall clock` `notimenow:allow is not available in domain code`
}
