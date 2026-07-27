// Package infrastructure belongs to somebody else's module and is not this
// context's adapter edge.
//
// It is the fixture that makes classify's module-root guard bite. Its sibling
// example.com/thirdparty/engine does not: "engine" is matched POSITIONALLY
// (internal/engine/...), so a third-party path containing it never reaches the
// segment scan at all, and deleting the guard changes nothing there. Only a
// third-party path carrying one of the SEGMENT-matched layer names —
// domain, application, infrastructure — can tell the guard from its absence,
// and of those only infrastructure is a forbidden target. So this is the one
// shape in the whole fixture set that separates the two, and it exists because
// the mutation aimed at that guard would otherwise have survived.
package infrastructure

type Store struct{}
