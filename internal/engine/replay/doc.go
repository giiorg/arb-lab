// Package replay is the sim's ingestion seam and its two pumps (§39p).
//
// It reconstructs raw arrival from a capture archive — the merge rule §39p-4
// fixes — stamps §3.5's AcceptOrdinal at the one place the process mints it,
// decodes venue frames through engine/venue, and drives the dispatcher two
// ways: Generate lets the arbitration decide and freezes the outcome in the
// dispatch log; Rerun follows an existing log entry by entry and never
// arbitrates, which is §3.5's replay ("not regenerating dispatch order").
//
// # What this package is NOT
//
// It is not a verifier — capture verify owns archive integrity, and a second
// verdict here would be a second authority. It is not a context — nothing
// under marketdata/ or any other bounded context is imported; the state
// machines behind the dispatcher reach it only through dispatch.Sink, composed
// by the app root. And it is not the live pipeline — the live pump (§39l-3)
// remains step 3's question; nothing here answers it.
package replay
