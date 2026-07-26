// Package venue is a deliberate limit of the rule as build-spec §3.6 rule 5
// states it, recorded here rather than left implicit.
//
// This is engine/ infrastructure, not a domain or application layer, so the rule
// does not reach it and the time.Now below produces NO diagnostic. That is the
// spec's scope, and widening it would be a spec change rather than an
// implementation choice — so the fixture pins the scope instead of quietly
// exceeding it.
//
// Whether it *should* be widened is a real question: a venue adapter stamping a
// received message with this machine's clock is how a wrong timestamp enters the
// system in the first place. Left as spec work, not decided here.
package venue

import "time"

type Tick struct {
	Received time.Time
	Symbol   string
}

func Receive(symbol string) Tick {
	return Tick{Received: time.Now(), Symbol: symbol}
}
