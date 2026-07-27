package driven

import (
	"errors"
	"testing"

	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// snapshot's two error returns are unreachable through the exported API, and
// this file is why they are checked anyway.
//
// The invariant that makes them unreachable is that ApplyPlanned writes the
// record and registers the order in the ledger under one lock, so no id can be
// in one store and not the other. That invariant lives eight lines apart from
// this check and is one edit away from being false — a delete path, a §7.5
// journal replay that rebuilds records without re-registering fills, a second
// constructor. The check is what turns that edit into a refusal naming the
// order instead of a snapshot reporting a cumulative quantity of zero for an
// order that has filled.
//
// It is asserted from IN-PACKAGE, on §24g's precedent (domain.validatePrice's
// fallthrough, same problem one context over): a test that can only go through
// the constructor cannot build the state the check is about, so a mutation
// deleting the check survives every external test that could be written.
func TestSnapshotRefusesARecordTheLedgerDoesNotKnow(t *testing.T) {
	t.Parallel()

	id := ids.ClientOrderID("ab-0123456789ABCDEFGHJKMNPQRS-00-00")

	// The state ApplyPlanned cannot produce: a record with no ledger
	// registration behind it.
	repo := NewMemoryOrderWriteRepository()
	repo.records[id] = &record{
		order:   domain.Order{ClientOrderID: id},
		state:   domain.StatePlanned,
		version: firstVersion,
	}

	_, err := repo.Load(id)
	if !errors.Is(err, domain.ErrFillUnknownOrder) {
		t.Fatalf("Load of a record the ledger does not know = %v, want %v", err, domain.ErrFillUnknownOrder)
	}
}

// The positive control the check above needs, and it is not ceremony: without
// it, a snapshot that failed for EVERY record would satisfy the test above
// perfectly. That is the same universally-objecting predicate this repo
// requires every harness to rule out.
func TestSnapshotSucceedsOnAConsistentlyBuiltRecord(t *testing.T) {
	t.Parallel()

	id := ids.ClientOrderID("ab-0123456789ABCDEFGHJKMNPQRS-00-00")
	quantity, err := fixedpoint.ParseQuantity("BTC", "10")
	if err != nil {
		t.Fatalf("ParseQuantity: %v", err)
	}
	instrument := ids.VenueInstrument{
		Venue: "binance", Product: ids.ProductLinearPerp,
		Symbol: "BTCUSDT", Contract: ids.ContractLinear,
	}

	repo := NewMemoryOrderWriteRepository()
	if err := repo.ledger.Register(id, instrument, quantity); err != nil {
		t.Fatalf("Register: %v", err)
	}
	repo.records[id] = &record{
		order:   domain.Order{ClientOrderID: id, Instrument: instrument, Quantity: quantity},
		state:   domain.StatePlanned,
		version: firstVersion,
	}

	rec, err := repo.Load(id)
	if err != nil {
		t.Fatalf("Load of a consistent record: %v", err)
	}
	if rec.Version != firstVersion {
		t.Errorf("Version = %d, want %d", rec.Version, firstVersion)
	}
}
