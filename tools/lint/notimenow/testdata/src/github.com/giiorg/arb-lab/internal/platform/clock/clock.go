// Package clock is the fixture that matters most, and it must produce NO
// diagnostic: it is the adapter whose entire job is to read the wall clock, so a
// rule that policed it would make itself impossible to satisfy.
//
// This is why the scope is structural rather than an exclusion list. There is no
// annotation here and no configured exemption — platform/clock is not in a
// policed layer, so the seam is expressed by where the code lives. §3.5's Clock
// is the port; this is the driven adapter behind it, and the offset and
// uncertainty band it applies are what make exchange server time authoritative
// rather than this machine's.
package clock

import "time"

type Venue string

type Wall struct {
	offsets     map[Venue]time.Duration
	uncertainty map[Venue]time.Duration
}

func (w Wall) Now() time.Time { return time.Now() }

// ServerNow is the call domain and application code makes instead of time.Now,
// and the uncertainty band is the reason it is worth the indirection: a funding
// boundary decided at the edge of the band is a decision that has to know it is
// at the edge.
func (w Wall) ServerNow(v Venue) (time.Time, time.Duration) {
	return time.Now().Add(w.offsets[v]), w.uncertainty[v]
}

func (w Wall) Sleep(d time.Duration) { time.Sleep(d) }
