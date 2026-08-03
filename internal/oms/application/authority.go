package application

import (
	"sync"

	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// DispatchAuthority is the in-process record of every ClientOrderID this
// process has committed to Place — the double-dispatch backstop, extracted from
// the dispatcher so that it is one set per PROCESS rather than one per path.
//
// # Why it is not a field on Dispatcher any more
//
// While exactly one dispatch path existed, a private map on the dispatcher WAS
// the process-wide set, by accident of there being nothing else. The executor
// is a second path — §6 step 6's parallel legs run on per-leg dispatchers — and
// a private map per dispatcher would quietly scope the backstop's guarantee to
// "this path never double-Places". The case that abandons is the exact case the
// backstop exists for: with the projection broken (Outcome.Applied false) the
// record sits at StatePlanned, the repository re-admits the order, and the only
// thing left that knows the send happened is this set. An order sent through
// one path and re-presented to another would be checked against a map that
// never saw it. §59b records the finding.
//
// # Safe for concurrent use, and WHY that is not the mutex the dispatcher's
// # doc rejects
//
// Dispatcher refuses internal synchronisation because a mutex there would make
// two concurrent dispatches of one order pass the race detector while still
// double-Placing — the race it cannot fix is check-then-mark spanning the
// authorization fsync. This type does not move that boundary: dispatched and
// mark are still two calls on opposite sides of the fsync, serialisation per
// id is still the caller's (the executor guarantees it structurally — distinct
// ids per call, one call at a time), and what the lock here protects is only
// the map itself, which multiple goroutine-confined dispatchers now read and
// grow concurrently for DISTINCT ids. The set is monotone — entries are never
// removed — so there is no read-modify-write to get wrong.
//
// # A fresh process starts empty, and that is correct
//
// Durable enforcement of the single-Place rule is the journal: recovery finds
// an order at StateDispatchAuthorized and resolves it by query, never by
// resubmitting. This set covers the one in-process bug the journal cannot — a
// caller that treats a failed dispatch as retryable when the failure happened
// AFTER the send.
type DispatchAuthority struct {
	mu   sync.Mutex
	sent map[ids.ClientOrderID]struct{}
}

// NewDispatchAuthority constructs an empty authority.
//
// A composition seam constructs exactly ONE per process and threads it through
// every dispatcher — oms.New does this — because two authorities are two
// backstops each blind to the other's sends, which is the fragmentation this
// type exists to end.
func NewDispatchAuthority() *DispatchAuthority {
	return &DispatchAuthority{sent: make(map[ids.ClientOrderID]struct{})}
}

// dispatched reports whether this process has already committed to Place id.
func (a *DispatchAuthority) dispatched(id ids.ClientOrderID) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	_, ok := a.sent[id]
	return ok
}

// mark records the commitment. It is called after the authorization fsync and
// never before — marking on a refusal would burn the id, see Dispatch.
func (a *DispatchAuthority) mark(id ids.ClientOrderID) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.sent[id] = struct{}{}
}
