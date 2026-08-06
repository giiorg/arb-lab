// Package domain holds the risk context's rules — §4.1's account posture
// first (task #198, record §73).
//
// # Why this context exists, and why the posture rules are HERE
//
// §4's port declaration assigns `AccountPostureSource` to *"risk — the
// safety-posture guard (§4.1)"*, and `oms/application/reconcile.go` already
// states the ownership in a comment: *"the §4.1 account posture is risk's and
// engine/venue's"*. So the check is not parked in `oms`: a context that owns a
// gate whose inputs it does not own is the shape §7.5's checklist comment
// argues against at length.
//
// # This package JUDGES; it does not read
//
// `engine/venue`'s posture surface reports the venue's own word and knows
// nothing about what the word should be. This package holds the other half —
// the requirement — and states it once, against an operator-declared pin, in
// this context's own vocabulary. The split is the point rather than layering
// etiquette: the moment a codec encodes *"ISOLATED is correct"*, the check that
// compares the venue against the requirement has both halves supplied by the
// same file and a wrong constant is self-consistent under every mutation
// (record §73b F3).
//
// The vocabulary here is therefore risk's own and is deliberately NOT
// `engine/venue`'s types — §3.6 rule 1 forbids the import outright, and the ACL
// that translates lives in `risk/infrastructure/driven`.
//
// # The direction every default points
//
// Silence never passes. A setting the venue did not mention, a value it
// answered blank, a configured symbol the read said nothing about, an
// unattested Portfolio-Margin claim, an account-kind value nobody enumerated —
// each is a REFUSAL that names itself, because every one of them is a claim
// with no evidence and the passing value of most of these settings is `false`.
// The failure direction that costs is not "refused a healthy account"; it is
// trading an account whose collateral model was never established.
package domain
