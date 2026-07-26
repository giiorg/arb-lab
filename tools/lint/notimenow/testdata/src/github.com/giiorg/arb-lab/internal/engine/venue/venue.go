// Package venue is engine/ transport under the widened rule 5, and it carries
// both halves of the widening: the read that must be caught, and the one that
// must be allowed to escape.
//
// Why the rule reaches engine/ at all. The hazard is not abstract parity
// purity — it is that platform/clock's monotonic clamp is only a guarantee if
// everyone reads the same clock. Wall absorbs backwards time (a stalled clock
// beats a re-evaluated settlement); time.Now does not. So a RecvTime stamped
// from the raw clock and an age computed from the injected one disagree across
// an NTP step backwards, and the subtraction yields a negative or inflated age
// in code where freshness gates execution (§3.4) and stale marks drive
// liq-distance. Two clocks meeting at a subtraction is the defect; the ambient
// call is only where it starts.
//
// Why the escape exists here and nowhere else. In domain/ and application/ a
// wall-clock read is a design error and injecting the Clock is always the fix.
// In transport it is not: a websocket read deadline has no venue-server-time
// semantics, and routing it through Clock.SetTimer would put transport liveness
// on the *dispatch* timer queue — coupling a keepalive to the state machine's
// scheduler, which is worse architecture rather than stricter discipline.
package venue

import "time"

type Tick struct {
	Received time.Time
	Symbol   string
}

// The violation the widening exists to catch. This is the raw clock, not the
// injected one, so this timestamp cannot be compared to a Clock-derived instant
// without the two disagreeing exactly when it matters most.
func Receive(symbol string) Tick {
	return Tick{Received: time.Now(), Symbol: symbol} // want `time\.Now reads the wall clock, which engine code must not do`
}

// The legitimate escape, annotated on the line it applies to. A read deadline
// is transport liveness: it has no venue-server-time meaning, it never runs
// under replay (§3.5 substitutes the whole adapter), and it must fire on real
// elapsed time even when the dispatcher is wedged — which is the case against
// routing it through the Clock.
func keepalive(pong <-chan struct{}) bool {
	deadline := time.NewTimer(30 * time.Second) //notimenow:allow live-only transport: a socket read deadline carries no venue-server-time semantics and must fire even when the dispatcher is wedged
	defer deadline.Stop()
	select {
	case <-pong:
		return true
	case <-deadline.C:
		return false
	}
}

// One annotation covers the whole line, both reads included. This is the other
// half of reducing findings per line: the annotation's claim is about the line,
// so it had better answer everything on it rather than the first thing only.
func elapsed(connected, lastPong time.Time) time.Duration {
	return time.Since(connected) - time.Since(lastPong) //notimenow:allow live-only transport: connection-liveness bookkeeping, never replayed and never compared against a Clock-derived instant
}

// An escape with no reason is not an exception, it is an unexplained hole — so
// it exempts nothing and says why. Block form because a line comment would
// swallow the assertion into the reason (see the nofloat package comment).
func unreasoned(start time.Time) time.Duration {
	return time.Since(start) /*notimenow:allow*/ // want `time\.Since reads the wall clock` `notimenow:allow needs a reason`
}

// An escape on a line with no ambient read is reported rather than ignored.
// Left alone it would outlive the call it was written for and silently
// pre-authorise whatever lands on that line next.
func stale(t Tick) string {
	return t.Symbol /*notimenow:allow the read this covered was deleted*/ // want `unused .*notimenow:allow`
}

// A misspelled directive is simply not a directive, and needs no diagnostic of
// its own: the read it failed to suppress is still reported, which is how the
// author finds the typo.
func misspelled(start time.Time) time.Duration {
	return time.Since(start) /*notimenow:allowance*/ // want `time\.Since reads the wall clock`
}
