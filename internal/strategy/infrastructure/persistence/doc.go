// Package persistence reads research/persistence/episodes.json — the
// per-episode expansion the §99 persistence study produced — and recomputes
// its answers at coordinates the CALLER names.
//
// It exists because record §103 measured a structural absence: summary.json
// and episodes.json had zero Go readers, and four separate coordinate losses
// (F55, F60, F79, F83) live in that gap. The contrast is one directory over —
// screening/report.go declares Go types for its three artifacts and has none.
//
// # Why it recomputes rather than decoding summary.json
//
// summarise.py computes four blocks and records the coordinates of one. Its
// run_vs_breakeven block is pinned to entry level 0.0005 and cost 0.0021; its
// per_symbol and verdict blocks are pinned to level 0.001; none of the three
// writes the pin down. A reader that decoded that file could not supply what
// the file never wrote, so its refusals would be permanent — and serving the
// blocks anyway would mean hard-coding summarise.py's pins as Go constants,
// which is a constant asserting an external fact about a script that can
// change without the constant changing.
//
// So this package parses ONLY episodes.json, which carries `entry` on every
// element, and folds it here. Every result struct embeds the coordinate it was
// computed at, which makes the loss unrepresentable rather than annotated.
//
// The accepted consequence is that this fold is a second independent
// implementation of summarise.py. That is this repo's standing method — real
// defects here have been found by two implementations disagreeing, not by
// review — and a disagreement on the committed data is a finding, not a test
// to loosen.
//
// # Why there is no port
//
// strategy/application/ports declares "seven in all", which is a normative
// count. More decisively: the consumer that would own an eighth entry is a
// phase-3 dashboard context that does not exist, and a driven port with no
// caller is a check that cannot fail. Phase 3 declares the interface it needs;
// this package satisfies it.
package persistence
