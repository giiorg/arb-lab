// Package crossvenue is strategy #2's archive read-path (design record §45).
//
// It reads the §42c capture — the shortlist's four streams per symbol plus
// BTC's eight — through engine/capture's decode API and engine/venue's wire
// decoders, and turns bytes into the gate's evidence. It sits under
// infrastructure/ deliberately: a top-level strategy/crossvenue would
// classify as no layer at all under §3.6 rule 1's analyzer, and a package the
// gate is silent about is a package the gate is not policing (§45c).
//
// What exists today is slice A's smoke check: Check walks one UTC day of
// archive files, routes every stream to the decoder that owns its wire, and
// reports counts and refusals per stream — with every undecodable stream
// SKIPPED BY NAME, never silently. The §32 verify proves the bytes are
// intact; this proves they are the thing we think they are, and it exists
// because the shortlist's 32 streams started recording 2026-08-01T11:13Z and
// nothing had ever parsed an alt's bytes (§45d). The walker's measurement
// stages (matched funding series, book sampling) are slice C.
//
// This package is a READ path. It imports the capture and venue engines
// (legal here, and only here in this context — §3.6 rule 1), touches no
// write machinery, and holds no gate parameters: those are pinned at the
// procedure commit (§45a), never in a reader.
package crossvenue
