// Package application is the second policed layer. It is a separate fixture
// rather than another file under domain/ because the layer name appears in the
// diagnostic, and a rule that matched only "domain" would still pass a test that
// never exercised "application".
package application

import "time"

// The legitimate uses, which must produce NO diagnostic. This is the important
// half of this fixture: the rule is about reading the clock, not about importing
// time. A gate that objected to time.Duration would be turned off within a week,
// and then rule 5 would not be enforced at all.
type Window struct {
	Opens time.Time
	Every time.Duration
}

// Constructing a specific instant reads no clock.
func FundingBoundary(hour int) time.Time {
	return time.Date(2026, time.July, 26, hour, 0, 0, 0, time.UTC)
}

// Neither does parsing one off a venue wire — which is how a timestamp is
// *supposed* to enter the system.
func ParseVenueStamp(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}

// Nor does arithmetic on instants the caller already had.
func (w Window) Closes() time.Time {
	return w.Opens.Add(w.Every)
}

// Methods on time.Time are not ambient either: the receiver is the clock read,
// and it happened somewhere with a Clock injected.
func (w Window) ContainsSameInstant(t time.Time) bool {
	return !t.Before(w.Opens) && t.Before(w.Closes())
}

// And the one that is not legitimate.
func (w Window) IsOpen() bool {
	return w.ContainsSameInstant(time.Now()) // want `time\.Now reads the wall clock, which application code must not do`
}
