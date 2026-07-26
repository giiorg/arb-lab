// Package aliased is domain code reached through a subdirectory, importing time
// under a different name and again with no name at all. It proves three things:
//
//   - Nesting does not help. internal/oms/domain/aliased is domain code, so a
//     rule that only matched the leaf segment would be escapable by mkdir.
//   - Aliasing does not help. The check resolves each identifier to its
//     types.Object, so the local name of the import is irrelevant — and a
//     textual "time.Now" match would find nothing in this whole file.
//   - Nor does a dot import, where the call site names no package at all. That
//     is the case a check written around SelectorExpr would miss, and it is why
//     this analyzer walks identifiers instead.
package aliased

import (
	. "time"
	wall "time"
)

func WallStamp() wall.Time {
	return wall.Now() // want `time\.Now reads the wall clock, which domain code must not do`
}

func Elapsed(since Time) Duration {
	return Since(since) // want `time\.Since reads the wall clock`
}
