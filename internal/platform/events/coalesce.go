package events

// MergeFunc decides how an arriving envelope combines with the newest one
// already sitting in a mailbox.
//
// It returns the envelope that should occupy the newest slot, and whether the
// two merged at all. Reporting false means "these do not combine" — the arriving
// envelope is appended alongside the queued one, and the mailbox's bounded depth
// governs from there.
//
// # It must be a pure function of its two arguments
//
// §3.5 requires coalescing to be "a deterministic rule keyed on (Source, field)
// — never a function of goroutine scheduling". This signature is what enforces
// the first half: a MergeFunc sees two envelopes and nothing else, so it cannot
// consult a clock, a counter or the mailbox's history.
//
// The second half is not, and cannot be, enforced here — and the distinction is
// worth being exact about, because overclaiming it would be worse than not
// claiming it. Whether two updates ever meet in a mailbox depends on whether the
// consumer drained between them, which IS scheduling. What §3.5 resolves that
// with is persistence, not purity: the live dispatcher freezes its arbitration
// into the DispatchSeq log, and replay re-applies the log rather than re-deriving
// the outcome. So the RULE is deterministic; the live interleaving is recorded.
type MergeFunc func(queued, next Envelope) (Envelope, bool)

// CoalescePolicy chooses the merge rule for a source. Returning nil means that
// source does not coalesce — its mailbox is a plain bounded ring with
// drop-oldest.
//
// Keyed on SourceID because that is as far as the kernel can see. §3.5's
// "(Source, field)" keying has a field half too, and that half lives inside the
// payload, which is domain data — so it is expressed by the MergeFunc the policy
// returns, not by this signature.
type CoalescePolicy func(SourceID) MergeFunc

// LatestWins is §3.4's ordinary market-data rule: a superseded update is worth
// nothing once a newer one exists, so the newer one replaces it outright.
//
// Correct for reconstructed book snapshots, tickers and funding metadata. NOT
// correct for the perp mark — see AdversePreserving, and note that the whole of
// [→ R2c-1] is about that exception.
func LatestWins(_, next Envelope) (Envelope, bool) { return next, true }

// NoCoalescing is the policy under which nothing merges: every mailbox is a
// bounded ring that drops its oldest entry on overflow and counts the drop.
//
// Correct where every update carries information the next one does not replace.
func NoCoalescing(SourceID) MergeFunc { return nil }

// AdversePreserving builds the coalescing rule §3.4 mandates for the perp mark:
// "the worst adverse mark since the OMS last consumed it is never dropped, so a
// brief adverse excursion that breaches a threshold cannot be hidden by a later
// recovery" `[→ R2c-1]`.
//
// # Why the mark cannot use latest-wins
//
// Liq distance is a function of mark. Mark moves continuously — Bybit ticker
// ~100 ms, Binance markPrice@1s — with no account mutation at all, so a pure
// rally walks a short position toward liquidation while nothing else in the
// process changes. If a spike and its recovery both land between two consumer
// turns, latest-wins hands over the recovery and the breach is gone: no event,
// no error, and a position closer to liquidation than anything in the process
// knows.
//
// # The merged envelope is always a real observation
//
// Whichever payload is more adverse wins, and it brings its OWN envelope — its
// sequence numbers, its RecvTime. The rejected alternative was to keep the
// newest envelope's metadata with the worst payload, which reads better on a
// freshness metric and is a fabrication: a price stamped at an instant it did not
// hold. Downstream that value is compared against thresholds and written to an
// audit trail, and a synthetic tick in either is worse than a slightly stale one.
//
// The cost is stated rather than hidden: while an unconsumed excursion sits in
// the mailbox, the source reads as marginally staler than it is. That direction
// is safe — it can only trigger a freshness check, never suppress one — and the
// window is bounded by how often the consumer drains.
//
// # What "adverse" means is the caller's to say
//
// It depends on which way the position faces, and the shared kernel may not know
// that (§3.6 rule 9). moreAdverse reports whether candidate is worse than
// incumbent; for the cash-and-carry short (§2.2) that is "candidate is higher".
//
// If either payload is not a T the merge declines. Declining keeps both
// envelopes, so a wiring mistake costs memory pressure and a visible drop-oldest
// count rather than silently discarding a mark the rule could not compare.
func AdversePreserving[T any](moreAdverse func(candidate, incumbent T) bool) MergeFunc {
	return func(queued, next Envelope) (Envelope, bool) {
		q, qok := queued.Payload.(T)
		n, nok := next.Payload.(T)
		if !qok || !nok {
			return Envelope{}, false
		}
		if moreAdverse(n, q) {
			return next, true
		}
		// The incumbent stays, WHOLE. This is the branch that makes a recovery
		// unable to erase a breach, and it is why the newest envelope is
		// discarded rather than merged: keeping its timestamp with the old
		// price is the fabrication described above.
		return queued, true
	}
}
