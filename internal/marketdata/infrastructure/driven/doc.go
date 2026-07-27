// Package driven holds marketdata's driven adapters: the implementations of
// the ports application/ declares.
//
// # Which way the arrows run, and why this package is where they turn around
//
// application/ports/ declares Budgets and FundingPublisher as interfaces it
// needs. Nothing in application/ or domain/ knows this package exists, and
// §3.6 rule 1 (hexboundary) makes that a build failure rather than a
// convention. What lives here is the other half of each of those interfaces —
// the concrete thing that reads a configuration store, the concrete thing that
// writes to an events lane — and the kernel packages they are written against
// (platform/config, platform/events) are named ONLY here.
//
// The test of whether the split is real: swapping the freshness budgets from a
// config file to a database touches this package and nothing else, and the
// backtest substitutes both ports without linking either kernel package.
//
// # These adapters hold no state of their own, and that is a decision
//
// Both are a pointer to something else plus a translation. Neither caches, and
// ConfigBudgets in particular re-reads the store on every call rather than
// keeping the snapshot it saw at construction — which is the whole reason
// Budgets is a port rather than a map field. An adapter that cached would be
// the staleness bug one layer above the staleness this context measures.
package driven
