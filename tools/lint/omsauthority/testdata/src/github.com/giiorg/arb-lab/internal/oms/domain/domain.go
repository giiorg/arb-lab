// Package domain is deliberately NOT policed, inheriting rule 6's decision:
// §5.1 makes audit and reconcile projections over the journal, and denying a
// read model the order-state enum would force a second copy of SM §1's state
// set. It is also what separates the internal/oms entry's EXACT match from a
// tree match — under a tree match this import would be reported.
package domain

// OrderState is SM §1's twelve-state set.
type OrderState string
