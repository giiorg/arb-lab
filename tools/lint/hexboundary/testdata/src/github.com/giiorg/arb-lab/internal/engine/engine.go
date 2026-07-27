// Package engine sits at EXACTLY internal/engine, not beneath it.
//
// It is the fixture for within()'s equality arm, which every other fixture
// leaves untested: they all import internal/engine/state, which the prefix arm
// alone matches. Delete `pkgPath == dir ||` and the whole suite still passed —
// found by mutation, not by review, and the identical gap exists in
// kernelpurity, which this helper was copied from.
package engine

type Spine struct{}
