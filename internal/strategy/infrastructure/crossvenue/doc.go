// Package crossvenue is strategy #2's archive read-path (design record §45).
//
// It reads the §42c capture — the shortlist's four streams per symbol plus
// BTC's eight — through engine/capture's decode API and engine/venue's wire
// decoders, and turns bytes into the gate's evidence. It sits under
// infrastructure/ deliberately: a top-level strategy/crossvenue would
// classify as no layer at all under §3.6 rule 1's analyzer, and a package the
// gate is silent about is a package the gate is not policing (§45c).
//
// Slice A's smoke check (check.go) walks one UTC day of archive files,
// routes every stream to the decoder that owns its wire, and reports counts
// and refusals per stream — with every undecodable stream SKIPPED BY NAME,
// never silently. The §32 verify proves the bytes are intact; this proves
// they are the thing we think they are (§45d).
//
// Slice C is the measurement walk and the research sim: the manifest-gated
// scan with the gate's own guards (walk.go), the §43b post-cutoff seal in
// code (seal.go), the settled-funding REST store (store.go), decision-point
// sampling of the two venues' live estimates (sample.go), the Bybit book
// accumulator on the measured u-contiguity fact (accumulator.go), the
// book-cost walkers (books.go), and the labeled exploration assembly
// (sim.go) that scores the pre-declared portfolio at the bootstrap LCB.
//
// This package is a READ path. It imports the capture and venue engines
// (legal here, and only here in this context — §3.6 rule 1), touches no
// write machinery, and holds no gate parameters: those are pinned at the
// procedure commit (§45a), never in a reader.
package crossvenue
