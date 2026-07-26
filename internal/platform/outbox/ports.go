package outbox

import (
	"context"
	"time"
)

// Store is the durable half of the outbox — the half this package deliberately
// does not implement (see the package doc).
//
// # Append is absent on purpose
//
// The producer side is not on this interface. Appending is the operation that
// must join the caller's transaction, so its signature has to name a transaction
// handle, and the transaction it joins is the OMS write-ahead journal's — which
// does not exist yet. An Append(ctx, Message) here would be strictly WORSE than
// no Append at all: it would compile, it would look like the outbox contract,
// and it would silently be a second transaction. A lost race between the two
// commits is exactly the split-brain the pattern exists to prevent.
//
// So Message is defined (the value crossing the boundary is knowable now) and
// the append verb is not (its shape is not).
type Store interface {
	// Claim returns up to batch records that are due for delivery at now,
	// ordered by Seq ascending, and marks them in-flight for this relay pass.
	//
	// `now` is passed in rather than read by the implementation, and that is
	// load-bearing rather than stylistic. Retry eligibility is a comparison
	// against RetryAt; an implementation reading its own clock would put two
	// clocks either side of that comparison — §3.6 rule 5's defect — and a
	// replay would claim a different set than the live run did.
	//
	// The returned records' Payload may be RETAINED by the caller for the
	// duration of the pass, so an implementation must not hand back a buffer it
	// reuses across calls.
	//
	// An empty result is not an error: an idle outbox is the normal state.
	Claim(ctx context.Context, now time.Time, batch int) ([]Record, error)

	// Ack durably removes the given records: they were published.
	//
	// This is a DELETE, and it is the one operation in the package that can
	// lose an audit event. It is called only after Publish returned nil.
	//
	// Batched because the alternative is one transaction — and on SQLite with
	// synchronous=FULL, one fsync — per record.
	Ack(ctx context.Context, seqs []uint64) error

	// Fail reschedules records that could not be published, incrementing each
	// one's Attempts and setting its RetryAt.
	//
	// Batched to mirror Ack. The mirror matters beyond symmetry: a per-record
	// Fail would make a pass that fails N records cost N transactions at
	// exactly the moment the system is already degraded.
	//
	// A Fail that does not commit leaves the record with its old, already-past
	// RetryAt, so the next pass reclaims it immediately. That is a busy loop
	// bounded only by the caller's cadence — which is why RunOnce surfaces the
	// error rather than swallowing it.
	Fail(ctx context.Context, failures []Failure) error
}

// Failure is one record's rescheduling instruction.
type Failure struct {
	Seq uint64

	// RetryAt is the earliest instant the record becomes claimable again,
	// computed from the injected clock.
	RetryAt time.Time
}

// Publisher delivers one record to its consumer.
//
// # One record, not a batch
//
// A batch signature cannot report WHICH records landed, and per-key FIFO needs
// exactly that: on a partial failure the relay must advance past the successes
// and stop at the first failure within each key. A batch publisher that returned
// one error for N records would force the relay to treat the whole batch as
// failed — correct under at-least-once, but it converts one consumer hiccup into
// a redelivery of everything, forever, if one record in the batch is poison.
//
// # What the error should carry
//
// A *apperror.Error whose Class is ClassRateLimit and whose RetryAfter is set
// will have that hint honoured (see Relay.RunOnce). Everything else is treated
// as a plain failure and backed off.
//
// The Effect axis is deliberately NOT consulted. Under at-least-once both
// EffectApplied and EffectNotApplied lead to the same action — redeliver — so
// branching on it would be a distinction the retry logic cannot act on. It still
// matters to the operator reading the log, where it is the difference between a
// duplicate that is certain and one that is merely possible.
type Publisher interface {
	Publish(ctx context.Context, r Record) error
}
