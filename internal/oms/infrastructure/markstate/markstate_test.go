package markstate_test

import (
	"errors"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/state"
	mports "github.com/giiorg/arb-lab/internal/marketdata/application/ports"
	mdomain "github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/infrastructure/markstate"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// The three instants are pairwise distinct and their GAPS are distinct too.
//
// That is the collapse check this file most needs: the reader computes TWO ages
// from ONE entry, against two different instants. With recv == source, or with
// equal gaps, an implementation measuring both ages against the same clock —
// the §3.5 mistake the port's doc names — produces identical output to a
// correct one.
var (
	recvAt   = time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	sourceAt = recvAt.Add(-7 * time.Second)

	// engineNow is 3s past receipt; serverNow is 11s past the venue's stamp.
	// 3 != 11, and neither is a multiple of the other.
	engineNow = recvAt.Add(3 * time.Second)
	serverNow = sourceAt.Add(11 * time.Second)
)

func perp() ids.VenueInstrument {
	return ids.VenueInstrument{
		Venue:    "binance",
		Product:  ids.ProductLinearPerp,
		Symbol:   "BTCUSDT",
		Contract: ids.ContractLinear,
	}
}

func loadedCache(t *testing.T) *state.Cache {
	t.Helper()
	c := state.NewCache()
	price, err := fixedpoint.ParsePrice("BTC", "USDT", "64000.5")
	if err != nil {
		t.Fatalf("ParsePrice: %v", err)
	}
	err = c.PutMark(mports.MarkSnapshot{
		Observation: mdomain.MarkObservation{
			Instrument: perp(),
			Price:      price,
			Watermark: mdomain.Watermark{
				VenueSeq:   9,
				SourceTime: sourceAt,
				RecvTime:   recvAt,
			},
		},
		Transport: mdomain.FreshnessFresh,
		Data:      mdomain.FreshnessFresh,
		Trigger:   4,
	})
	if err != nil {
		t.Fatalf("PutMark: %v", err)
	}
	return c
}

func TestNewReaderRefusesANilCache(t *testing.T) {
	t.Parallel()

	if _, err := markstate.NewReader(nil); !errors.Is(err, markstate.ErrMarkCacheNil) {
		t.Errorf("err = %v; want ErrMarkCacheNil", err)
	}
	// Positive control: a real cache is accepted, so the refusal above is about
	// the nil and not about a constructor that refuses everything.
	if _, err := markstate.NewReader(state.NewCache()); err != nil {
		t.Errorf("NewReader over a real cache: %v", err)
	}
}

// The two ages are computed against DIFFERENT instants, and the fixture is
// built so that swapping them changes both numbers.
func TestMarkReturnsTwoAgesMeasuredAgainstTwoClocks(t *testing.T) {
	t.Parallel()

	r, err := markstate.NewReader(loadedCache(t))
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	got, ok, err := r.Mark(perp(), engineNow, serverNow)
	if err != nil {
		t.Fatalf("Mark: %v", err)
	}
	if !ok {
		t.Fatal("Mark reported nothing for an instrument the cache holds")
	}
	if want := 3 * time.Second; got.Age != want {
		t.Errorf("Age = %v; want %v — measured from RecvTime against engine time", got.Age, want)
	}
	if want := 11 * time.Second; got.DataAge != want {
		t.Errorf("DataAge = %v; want %v — measured from SourceTime against the VENUE's time",
			got.DataAge, want)
	}
	if got.Instrument != perp() {
		t.Errorf("Instrument = %v; want %v", got.Instrument, perp())
	}
}

// Absence is (zero, false, nil) — never a zero reading, because a zero
// MarkReading is a mark of zero and §7.1's liq distance divides by it.
func TestMarkAbsenceIsFalseAndNotAnError(t *testing.T) {
	t.Parallel()

	r, err := markstate.NewReader(state.NewCache())
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	got, ok, err := r.Mark(perp(), engineNow, serverNow)
	if err != nil {
		t.Errorf("err = %v; want none — an empty cache at boot is a state, not a fault", err)
	}
	if ok {
		t.Error("an empty cache reported a mark")
	}
	if got != (ports.MarkReading{}) {
		t.Errorf("absence returned %+v; want the zero reading", got)
	}
}

// A clock inversion in EITHER age refuses the whole reading. Both directions
// are asserted: an implementation checking only the transport age passes the
// first case and fails the second, and vice versa — so one case alone cannot
// tell the two apart.
func TestMarkRefusesAClockInversionInEitherAge(t *testing.T) {
	t.Parallel()

	r, err := markstate.NewReader(loadedCache(t))
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	cases := []struct {
		name     string
		now, srv time.Time
	}{
		{"engine clock precedes receipt", recvAt.Add(-time.Second), serverNow},
		{"venue clock precedes its own stamp", engineNow, sourceAt.Add(-time.Second)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok, err := r.Mark(perp(), tc.now, tc.srv)
			if !errors.Is(err, mdomain.ErrClockInversion) {
				t.Errorf("err = %v; want ErrClockInversion", err)
			}
			if ok {
				t.Error("an inverted clock still reported a usable reading")
			}
			if got != (ports.MarkReading{}) {
				t.Errorf("returned %+v; want the zero reading — a negative age compares "+
					"as fresher than everything", got)
			}
		})
	}

	// Positive control at the same fixture: with both clocks forward the
	// reading is returned, so the two refusals above are about the inversion.
	if _, ok, err := r.Mark(perp(), engineNow, serverNow); err != nil || !ok {
		t.Errorf("the uninverted read failed: ok = %t, err = %v", ok, err)
	}
}
