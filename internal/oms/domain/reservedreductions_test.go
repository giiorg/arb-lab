package domain_test

import (
	"errors"
	"testing"

	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// FM-5, as an executable claim: a new entry contributes NOTHING until its
// producer amends it.
//
// # The fixture is deliberately not an empty ledger
//
// On an empty ledger, "created at zero" and "there is nothing here" are the same
// sum, and every wrong constructor would pass. So the ledger already holds a
// nonzero entry from a different producer, and the assertion is that the total
// is UNCHANGED across the reservation — which is the property FM-5 is about: a
// generation opening cannot move the classifier's subtrahend, because at that
// instant it has committed nothing.
//
// The stronger half of the claim is not testable and does not need to be: there
// is no parameter on Reserve through which a target could be booked, so
// AmbiguousOrder.tla's RESERVE_AT_CREATION = TRUE — the control that IS FM-5 —
// has no Go spelling.
func TestReserveBooksNothingUntilItsProducerAmends(t *testing.T) {
	t.Parallel()

	ledger := domain.NewReservedReductions("BTC")
	mustReserve(t, ledger, "floor-1", domain.ProducerIntervalFloor)
	mustAmend(t, ledger, "floor-1", qty(t, "-0.6"))

	before := mustOutstanding(t, ledger)

	// A PackageDelever generation opens. SM §7 gives it a target; §6.7 says the
	// entry carries what it has COMMITTED, which at Planned is nothing.
	mustReserve(t, ledger, "pkg-1", domain.ProducerPackageDeleverSpot)

	after := mustOutstanding(t, ledger)
	if after.Decimal() != before.Decimal() {
		t.Fatalf("outstanding moved on reservation: %s -> %s; a generation that has committed nothing must not move the subtrahend", before, after)
	}
	if after.Decimal() != qty(t, "-0.6").Decimal() {
		t.Fatalf("outstanding = %s, want -0.6", after)
	}

	// And it moves the moment the producer states a commitment — otherwise the
	// assertion above would be equally satisfied by an Amend that does nothing.
	mustAmend(t, ledger, "pkg-1", qty(t, "0.25"))
	if got := mustOutstanding(t, ledger); got.Decimal() != qty(t, "-0.35").Decimal() {
		t.Fatalf("outstanding after amend = %s, want -0.35", got)
	}
}

// Amend SETS the outstanding amount; it does not add to it.
//
// The two values differ and neither is the other's double, so "set" (2), "add"
// (3) and "keep the first" (1) are three distinguishable answers. Amending to
// the same value twice — the fixture that reads naturally — cannot tell any of
// them apart.
func TestAmendReplacesTheOutstandingAmount(t *testing.T) {
	t.Parallel()

	ledger := domain.NewReservedReductions("BTC")
	mustReserve(t, ledger, "pkg-1", domain.ProducerPackageDeleverSpot)
	mustAmend(t, ledger, "pkg-1", qty(t, "1"))
	mustAmend(t, ledger, "pkg-1", qty(t, "2"))

	if got := mustOutstanding(t, ledger); got.Decimal() != qty(t, "2").Decimal() {
		t.Fatalf("outstanding = %s, want 2 (an additive Amend would give 3, a no-op 1)", got)
	}
}

// The sum is over SIGNED amounts, and the count is not derivable from it.
//
// The two entries cancel exactly, which is the fixture that separates the two
// questions: a caller answering "is a generation live" from the sum would read
// two live generations as none. §6.7's "never issued twice" and SM §7's "only
// after the prior generation reaches finality" are both claims about entries.
func TestOutstandingSumsSignedAmountsAndCountIsSeparate(t *testing.T) {
	t.Parallel()

	ledger := domain.NewReservedReductions("BTC")
	mustReserve(t, ledger, "floor-1", domain.ProducerIntervalFloor)
	mustAmend(t, ledger, "floor-1", qty(t, "1"))
	mustReserve(t, ledger, "exp-1", domain.ProducerExposureReduction)
	mustAmend(t, ledger, "exp-1", qty(t, "-1"))

	if got := mustOutstanding(t, ledger); got.Decimal() != qty(t, "0").Decimal() {
		t.Fatalf("outstanding = %s, want 0", got)
	}
	if got := ledger.Count(); got != 2 {
		t.Fatalf("Count = %d, want 2 — two live entries that happen to net to zero", got)
	}
}

// OutstandingFor decomposes the sum by producer — FM-9's seam.
//
// Three producers, three DIFFERENT amounts, and none of them equal to the total.
// With one producer, "this producer's contribution" and "the whole sum" are the
// same number; with two equal amounts, "the first entry" and "this producer's"
// coincide for one of them.
func TestOutstandingForSeparatesOneProducerFromTheSum(t *testing.T) {
	t.Parallel()

	ledger := domain.NewReservedReductions("BTC")
	mustReserve(t, ledger, "exp-1", domain.ProducerExposureReduction)
	mustAmend(t, ledger, "exp-1", qty(t, "0.5"))
	mustReserve(t, ledger, "floor-1", domain.ProducerIntervalFloor)
	mustAmend(t, ledger, "floor-1", qty(t, "0.25"))
	mustReserve(t, ledger, "pkg-1", domain.ProducerPackageDeleverSpot)
	mustAmend(t, ledger, "pkg-1", qty(t, "-0.125"))

	cases := map[domain.ReductionProducer]string{
		domain.ProducerExposureReduction:  "0.5",
		domain.ProducerIntervalFloor:      "0.25",
		domain.ProducerPackageDeleverSpot: "-0.125",
	}
	for producer, want := range cases {
		got, err := ledger.OutstandingFor(producer)
		if err != nil {
			t.Fatalf("OutstandingFor(%s): %v", producer, err)
		}
		if got.Decimal() != qty(t, want).Decimal() {
			t.Fatalf("OutstandingFor(%s) = %s, want %s", producer, got, want)
		}
	}
	if got := mustOutstanding(t, ledger); got.Decimal() != qty(t, "0.625").Decimal() {
		t.Fatalf("outstanding = %s, want 0.625", got)
	}
}

// Release reports the amount that stopped being subtracted, and removes only its
// own entry.
//
// The second entry is what makes the removal claim checkable: with one entry,
// "deleted this id" and "cleared the ledger" are the same end state. The
// returned amount is asserted against a known exact value rather than against
// "nonzero", because §6.7 row 4 requires the caller to re-enter exactly that
// exposure as an unresolved order — a caller handed the wrong magnitude loses
// the difference silently.
func TestReleaseReportsTheAmountAndRemovesOnlyThatEntry(t *testing.T) {
	t.Parallel()

	ledger := domain.NewReservedReductions("BTC")
	mustReserve(t, ledger, "floor-1", domain.ProducerIntervalFloor)
	mustAmend(t, ledger, "floor-1", qty(t, "0.75"))
	mustReserve(t, ledger, "pkg-1", domain.ProducerPackageDeleverSpot)
	mustAmend(t, ledger, "pkg-1", qty(t, "0.25"))

	released, err := ledger.Release("floor-1")
	if err != nil {
		t.Fatalf("Release: %v", err)
	}
	if released.Decimal() != qty(t, "0.75").Decimal() {
		t.Fatalf("Release returned %s, want 0.75", released)
	}
	if ledger.Holds("floor-1") {
		t.Fatal("Holds(floor-1) = true after Release")
	}
	if !ledger.Holds("pkg-1") {
		t.Fatal("Holds(pkg-1) = false — Release removed an entry it was not asked about")
	}
	if got := mustOutstanding(t, ledger); got.Decimal() != qty(t, "0.25").Decimal() {
		t.Fatalf("outstanding = %s, want 0.25", got)
	}
}

func TestReservedReductionsRefusals(t *testing.T) {
	t.Parallel()

	t.Run("a duplicate reservation id", func(t *testing.T) {
		t.Parallel()
		ledger := domain.NewReservedReductions("BTC")
		mustReserve(t, ledger, "pkg-1", domain.ProducerPackageDeleverSpot)
		err := ledger.Reserve("pkg-1", domain.ProducerIntervalFloor)
		if !errors.Is(err, domain.ErrDuplicateReservation) {
			t.Fatalf("Reserve = %v, want ErrDuplicateReservation", err)
		}
	})

	t.Run("an empty reservation id", func(t *testing.T) {
		t.Parallel()
		ledger := domain.NewReservedReductions("BTC")
		if err := ledger.Reserve("", domain.ProducerIntervalFloor); !errors.Is(err, domain.ErrReservationUnset) {
			t.Fatalf("Reserve = %v, want ErrReservationUnset", err)
		}
	})

	t.Run("an undeclared producer", func(t *testing.T) {
		t.Parallel()
		ledger := domain.NewReservedReductions("BTC")
		err := ledger.Reserve("x", domain.ReductionProducer("protective_order_leg"))
		if !errors.Is(err, domain.ErrReductionProducerInvalid) {
			t.Fatalf("Reserve = %v, want ErrReductionProducerInvalid", err)
		}
	})

	t.Run("amending an id the ledger does not hold", func(t *testing.T) {
		t.Parallel()
		ledger := domain.NewReservedReductions("BTC")
		err := ledger.Amend("pkg-1", qty(t, "1"))
		if !errors.Is(err, domain.ErrUnknownReservation) {
			t.Fatalf("Amend = %v, want ErrUnknownReservation", err)
		}
	})

	t.Run("releasing an id the ledger does not hold", func(t *testing.T) {
		t.Parallel()
		ledger := domain.NewReservedReductions("BTC")
		if _, err := ledger.Release("pkg-1"); !errors.Is(err, domain.ErrUnknownReservation) {
			t.Fatalf("Release = %v, want ErrUnknownReservation", err)
		}
	})

	// The asset check is Amend's own rather than fixedpoint's, because Amend
	// STORES and performs no arithmetic. Without it the disagreement surfaces
	// later, inside the fold, attributed to whichever entry the map visited
	// first.
	t.Run("amending with a different asset", func(t *testing.T) {
		t.Parallel()
		ledger := domain.NewReservedReductions("BTC")
		mustReserve(t, ledger, "pkg-1", domain.ProducerPackageDeleverSpot)
		eth, err := fixedpoint.ParseQuantity("ETH", "1")
		if err != nil {
			t.Fatalf("ParseQuantity: %v", err)
		}
		if err := ledger.Amend("pkg-1", eth); !errors.Is(err, domain.ErrReservationAssetMismatch) {
			t.Fatalf("Amend = %v, want ErrReservationAssetMismatch", err)
		}
	})

	// An unpopulated quantity is refused separately from a mismatched one: they
	// send a reader to different places, and the empty tag is what a
	// zero-valued struct produces.
	t.Run("amending with an unpopulated quantity", func(t *testing.T) {
		t.Parallel()
		ledger := domain.NewReservedReductions("BTC")
		mustReserve(t, ledger, "pkg-1", domain.ProducerPackageDeleverSpot)
		if err := ledger.Amend("pkg-1", fixedpoint.Quantity{}); !errors.Is(err, fixedpoint.ErrAssetUnset) {
			t.Fatalf("Amend = %v, want ErrAssetUnset", err)
		}
	})
}

// An empty ledger answers zero OF ITS ASSET rather than the untagged zero, which
// is what lets a group with no reservations be classified at all.
func TestEmptyLedgerAnswersZeroOfItsAsset(t *testing.T) {
	t.Parallel()

	got := mustOutstanding(t, domain.NewReservedReductions("BTC"))
	if err := got.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if got.Asset() != "BTC" {
		t.Fatalf("asset = %q, want BTC", got.Asset())
	}
}

func mustReserve(t *testing.T, l *domain.ReservedReductions, id domain.ReservationID, p domain.ReductionProducer) {
	t.Helper()
	if err := l.Reserve(id, p); err != nil {
		t.Fatalf("Reserve(%s, %s): %v", id, p, err)
	}
}

func mustAmend(t *testing.T, l *domain.ReservedReductions, id domain.ReservationID, q fixedpoint.Quantity) {
	t.Helper()
	if err := l.Amend(id, q); err != nil {
		t.Fatalf("Amend(%s, %s): %v", id, q, err)
	}
}

func mustOutstanding(t *testing.T, l *domain.ReservedReductions) fixedpoint.Quantity {
	t.Helper()
	total, err := l.Outstanding()
	if err != nil {
		t.Fatalf("Outstanding: %v", err)
	}
	return total
}
