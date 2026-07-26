// Package badescape covers the three ways the escape hatch can be misused. Each
// is a way a default-deny rule quietly stops denying anything, so each has to be
// a diagnostic rather than a convention.
//
// Note the block-comment form of the directive throughout this file. Go allows
// one // comment per line, so putting the line form of the directive ahead of an
// expectation comment yields a SINGLE comment whose reason text swallows the
// expectation — the escape would succeed and the fixture would prove nothing.
// The block form is what makes the rule's own test expressible in the rule's own
// syntax. (This paragraph is worded around the expectation keyword on purpose:
// analysistest scans every comment in the file for it, prose included.)
package badescape

// A bare directive with no reason is not an exception, it is an unexplained
// hole. So it does not exempt: the float is reported as well.
var noReason float64 /*nofloat:allow*/ // want `float64 value under internal/` `needs a reason`

// A misspelled directive is simply not a directive, and needs no diagnostic of
// its own — the float it failed to suppress is what surfaces the typo. This is
// the case that would read as an escape with the reason "ance" without the
// separator check in escapeReason.
var typo float64 /*nofloat:allowance*/ // want `float64 value under internal/`

// An escape left behind after the float it was written for was removed. Reported
// because otherwise it sits there pre-authorising whatever float lands on this
// line next, which is the failure mode of every unused-suppression in every
// linter that does not check for them.
var Count int64 /*nofloat:allow leftover from an earlier draft*/ // want `unused //nofloat:allow`
