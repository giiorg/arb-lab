// Package domain stands in for oms's domain layer, which is deliberately NOT
// policed: §5.1 makes audit and reconcile projections over the journal, and a
// projection naming an order state is that pattern working.
package domain

// State is the value a read model legitimately needs to name.
type State string
