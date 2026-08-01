// Package screening is the §42b screening harness: the milestone-2b step
// that turns both venues' settled-funding REST history into the ranked
// shortlist the §42c capture expansion reads, plus the §42b-1 dislocation
// census.
//
// It is read-only research machinery, deliberately outside the calibration
// fetcher (driving.Fetcher), because two of that fetcher's guarantees are
// WRONG here rather than merely unneeded: the H1 guard refuses rows on or
// after 2026-02-01 (screening's window runs to its own cutoff), and the H3
// closed-grid identity demands a grid declared up front (an alt's grid is
// measured from its own history — listing dates vary and intervals have
// changed over time, §42a). What it keeps from that lineage: the venue's
// words stored verbatim, pagination that refuses to stall silently, and a
// digest ledger over every file it writes.
//
// The one seal it does enforce: BTCUSDT is excluded from the fetch
// universe, because §42e makes the H1 holdout (post-2026-02-01 BTC)
// permanent and a screening read of BTC funding through the cutoff would
// cross it.
//
// Every number the screening uses is pinned in params.go at this commit
// (§41e's pattern); the scoping record (§42) deliberately names none of
// them.
package screening
