// Package main is a granted root OUTSIDE internal/, which is the fixture that
// forces the module root to be resolved from the IMPORTED path rather than from
// the analyzed one. Resolved the other way, every importer outside internal/
// would be unpoliced — and cmd/ is where a binary lives.
package main

import "github.com/giiorg/arb-lab/internal/oms/application"

var _ application.Executor

func main() {}
