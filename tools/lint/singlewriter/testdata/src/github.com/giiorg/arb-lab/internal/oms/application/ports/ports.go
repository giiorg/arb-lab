// Package ports stands in for oms's driven ports: the write authority half of
// §3.6 rule 6's subject.
package ports

// OrderWriteRepository is the interface whose possession IS write authority —
// which is why the gate polices this package and not only the adapter.
type OrderWriteRepository interface {
	ApplyState(id string, expected int64, to string) error
}
