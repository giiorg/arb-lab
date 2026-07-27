// Package domainservice is the substring trap on the IMPORTER side: its name
// merely starts with "domain", so a substring match would police it and report
// the import below.
//
// The mirror of engineering.go, and it is here rather than assumed because the
// two ends of the edge are classified by one function but reached by two
// different call sites — a fix applied on one side gets applied to the other.
package domainservice

import "github.com/giiorg/arb-lab/internal/engine/state"

type Service struct {
	MV state.MarketView
}
