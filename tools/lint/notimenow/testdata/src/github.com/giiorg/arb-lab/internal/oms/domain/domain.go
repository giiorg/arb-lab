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
