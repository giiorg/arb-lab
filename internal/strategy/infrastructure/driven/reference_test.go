package driven_test

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/config"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/strategy/infrastructure/driven"
)

// docJSON is a document shaped like the committed milestone-2 revision: four
// fee cells, four lot rows, one hurdle. The NUMBERS here are test fixtures,
// not venue facts — the committed configs/ document is the one that cites
// Profiles.
const docJSON = `{
  "fees": [
    {"venue":"binance","product":"spot","contract":"","maker":"0.001","taker":"0.001","tier":"VIP0","class":"hand_recorded","observed_at":"2026-07-31T00:00:00Z","source":"venue-profiles.md test row"},
    {"venue":"binance","product":"linear_perp","contract":"linear","maker":"0.0002","taker":"0.0005","tier":"VIP0","class":"hand_recorded","observed_at":"2026-07-31T00:00:00Z","source":"venue-profiles.md test row"},
    {"venue":"bybit","product":"spot","contract":"","maker":"0.0015","taker":"0.0015","tier":"non-VIP","class":"hand_recorded","observed_at":"2026-07-31T00:00:00Z","source":"venue-profiles.md test row"},
    {"venue":"bybit","product":"linear_perp","contract":"linear","maker":"0.0002","taker":"0.00055","tier":"non-VIP","class":"hand_recorded","observed_at":"2026-07-31T00:00:00Z","source":"venue-profiles.md test row"}
  ],
  "lots": [
    {"venue":"binance","product":"spot","contract":"","symbol":"BTCUSDT","base":"BTC","step":"0.00001","min_qty":"0.00001","quote":"USDT","min_notional":"5","class":"hand_recorded","observed_at":"2026-07-31T00:00:00Z","source":"exchangeInfo test row"},
    {"venue":"binance","product":"linear_perp","contract":"linear","symbol":"BTCUSDT","base":"BTC","step":"0.001","min_qty":"0.001","quote":"USDT","min_notional":"100","class":"hand_recorded","observed_at":"2026-07-31T00:00:00Z","source":"exchangeInfo test row"},
    {"venue":"bybit","product":"spot","contract":"","symbol":"BTCUSDT","base":"BTC","step":"0.000001","min_qty":"0.000048","quote":"USDT","min_notional":"1","class":"hand_recorded","observed_at":"2026-07-31T00:00:00Z","source":"instruments-info test row"},
    {"venue":"bybit","product":"linear_perp","contract":"linear","symbol":"BTCUSDT","base":"BTC","step":"0.001","min_qty":"0.001","quote":"USDT","min_notional":"0","class":"hand_recorded","observed_at":"2026-07-31T00:00:00Z","source":"instruments-info test row"}
  ],
  "hurdle": {"annual_rate":"0.046","access_cost":"0.001","liquidity_haircut":"0.002","counterparty_haircut":"0.003","as_of":"2026-07-30T00:00:00Z","class":"hand_recorded","observed_at":"2026-07-31T00:00:00Z","source":"benchmark test row"}
}`

var (
	loadAt   = time.Date(2026, 7, 31, 5, 0, 0, 0, time.UTC)
	asOfOK   = time.Date(2026, 7, 31, 6, 0, 0, 0, time.UTC)
	spotInst = ids.VenueInstrument{Venue: "binance", Product: ids.ProductSpot, Symbol: "BTCUSDT"}
	perpInst = ids.VenueInstrument{
		Venue: "binance", Product: ids.ProductLinearPerp,
		Symbol: "BTCUSDT", Contract: ids.ContractLinear,
	}
)

func newStore(t *testing.T, doc string) *driven.ReferenceStore {
	t.Helper()
	store, err := config.New[driven.Document]("configs/reference.test.json", loadAt, []byte(doc))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	ref, err := driven.NewReferenceStore(store)
	if err != nil {
		t.Fatal(err)
	}
	return ref
}

func TestFeeCellsAreDistinctPerLeg(t *testing.T) {
	ref := newStore(t, docJSON)

	spot, err := ref.Fees(spotInst, asOfOK)
	if err != nil {
		t.Fatal(err)
	}
	perp, err := ref.Fees(perpInst, asOfOK)
	if err != nil {
		t.Fatal(err)
	}
	// §2.4's explicit trap: reusing the perp figure for the spot leg. The
	// two cells must resolve to their own numbers.
	if spot.Taker == perp.Taker {
		t.Fatalf("spot and perp takers identical (%s) — the per-leg cells collapsed", spot.Taker)
	}
	// Maker/taker ORIENTATION, pinned on the row whose two rates differ —
	// the spot row's are equal, so a swap there changes nothing (the
	// fixture-collapse checklist's point), and this row is where the swap
	// is visible.
	if perp.Maker.Decimal() != "0.0002" || perp.Taker.Decimal() != "0.0005" {
		t.Fatalf("perp maker/taker %s/%s — orientation swapped?", perp.Maker.Decimal(), perp.Taker.Decimal())
	}
	if spot.Instrument != spotInst || perp.Instrument != perpInst {
		t.Fatalf("instruments %s / %s", spot.Instrument, perp.Instrument)
	}
	// Provenance rides the STORE's revision, not a row's claim.
	if spot.Provenance.Rev.Source != "configs/reference.test.json" || spot.Provenance.Rev.Num != 1 {
		t.Fatalf("provenance rev %+v", spot.Provenance.Rev)
	}
	if spot.Tier != "VIP0" {
		t.Fatalf("tier %q", spot.Tier)
	}
	if spot.Provenance.Class != "hand_recorded" {
		t.Fatalf("class %q", spot.Provenance.Class)
	}
}

func TestLotAndHurdleFacesAnswer(t *testing.T) {
	ref := newStore(t, docJSON)

	lots, err := ref.LotRules(perpInst, asOfOK)
	if err != nil {
		t.Fatal(err)
	}
	if lots.Instrument != perpInst {
		t.Fatalf("lots for %s", lots.Instrument)
	}
	if lots.Step.Decimal() != "0.001" {
		t.Fatalf("perp step %s", lots.Step.Decimal())
	}

	// Step/min-qty ORIENTATION, pinned on the row whose two quantities
	// differ — the Binance perp row's are both 0.001, where a swap changes
	// nothing.
	bybitSpot := ids.VenueInstrument{Venue: "bybit", Product: ids.ProductSpot, Symbol: "BTCUSDT"}
	bl, err := ref.LotRules(bybitSpot, asOfOK)
	if err != nil {
		t.Fatal(err)
	}
	if bl.Step.Decimal() != "0.000001" || bl.MinQty.Decimal() != "0.000048" {
		t.Fatalf("bybit spot step/min %s/%s — orientation swapped?", bl.Step.Decimal(), bl.MinQty.Decimal())
	}
	if bl.MinNotional.Decimal() != "1" {
		t.Fatalf("bybit spot min notional %s", bl.MinNotional.Decimal())
	}

	h, err := ref.Hurdle(asOfOK)
	if err != nil {
		t.Fatal(err)
	}
	// All four rate members pinned individually: the three haircuts are
	// deliberately distinct in the fixture so any pairwise swap is visible.
	if h.AnnualRate.Decimal() != "0.046" ||
		h.AccessCost.Decimal() != "0.001" ||
		h.LiquidityHaircut.Decimal() != "0.002" ||
		h.CounterpartyHaircut.Decimal() != "0.003" {
		t.Fatalf("hurdle members %s/%s/%s/%s",
			h.AnnualRate.Decimal(), h.AccessCost.Decimal(),
			h.LiquidityHaircut.Decimal(), h.CounterpartyHaircut.Decimal())
	}
}

func TestAsOfBeforeObservationIsARefusalNamingTheInput(t *testing.T) {
	ref := newStore(t, docJSON)
	early := time.Date(2026, 7, 30, 23, 0, 0, 0, time.UTC) // before observed_at

	for name, call := range map[string]func() error{
		"fees":   func() error { _, err := ref.Fees(spotInst, early); return err },
		"lots":   func() error { _, err := ref.LotRules(spotInst, early); return err },
		"hurdle": func() error { _, err := ref.Hurdle(early); return err },
	} {
		err := call()
		if !errors.Is(err, driven.ErrNotPeriodCorrect) {
			t.Fatalf("%s: want ErrNotPeriodCorrect, got %v", name, err)
		}
	}

	// The boundary itself: asOf exactly AT the observation instant is
	// served — the revision is in force from the instant it was observed.
	at := time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)
	if _, err := ref.Fees(spotInst, at); err != nil {
		t.Fatalf("asOf == observedAt refused: %v", err)
	}
}

func TestAsOfZeroAndMissingCells(t *testing.T) {
	ref := newStore(t, docJSON)

	if _, err := ref.Fees(spotInst, time.Time{}); !errors.Is(err, driven.ErrAsOfUnset) {
		t.Fatalf("zero asOf: want ErrAsOfUnset, got %v", err)
	}

	eth := spotInst
	eth.Symbol = "ETHUSDT"
	// Fee cells are per (venue, product): ETHUSDT spot still answers.
	if _, err := ref.Fees(eth, asOfOK); err != nil {
		t.Fatalf("fee cell is per (venue,product); ETH lookup failed: %v", err)
	}
	// Lot rows are per SYMBOL: ETHUSDT has none.
	if _, err := ref.LotRules(eth, asOfOK); !errors.Is(err, driven.ErrCellAbsent) {
		t.Fatalf("lots for unlisted symbol: want ErrCellAbsent, got %v", err)
	}
	// A venue the document does not hold.
	okx := perpInst
	okx.Venue = "okx"
	if _, err := ref.Fees(okx, asOfOK); !errors.Is(err, driven.ErrCellAbsent) {
		t.Fatalf("fees for unlisted venue: want ErrCellAbsent, got %v", err)
	}
}

func TestDocumentRefusals(t *testing.T) {
	load := func(doc string) error {
		_, err := config.New[driven.Document]("test", loadAt, []byte(doc))
		return err
	}

	// The valid document is the positive control for every refusal below.
	if err := load(docJSON); err != nil {
		t.Fatalf("positive control: %v", err)
	}

	for name, tc := range map[string]struct {
		mutate func(string) string
		want   error
	}{
		"duplicate fee cell": {
			mutate: func(s string) string {
				row := `{"venue":"binance","product":"spot","contract":"","maker":"0.002","taker":"0.002","tier":"VIP0","class":"hand_recorded","observed_at":"2026-07-31T00:00:00Z","source":"dup"},`
				return strings.Replace(s, `{"venue":"binance","product":"spot"`, row+`{"venue":"binance","product":"spot"`, 1)
			},
			want: driven.ErrCellDuplicate,
		},
		"unparseable rate": {
			mutate: func(s string) string { return strings.Replace(s, `"maker":"0.001"`, `"maker":"0.1%"`, 1) },
			want:   driven.ErrRowInvalid,
		},
		"missing source": {
			mutate: func(s string) string {
				return strings.Replace(s, `"source":"benchmark test row"`, `"source":""`, 1)
			},
			want: driven.ErrRowInvalid,
		},
		"unknown provenance class": {
			mutate: func(s string) string {
				return strings.Replace(s, `"class":"hand_recorded","observed_at":"2026-07-31T00:00:00Z","source":"venue-profiles.md test row"},`,
					`"class":"copied_from_blog","observed_at":"2026-07-31T00:00:00Z","source":"venue-profiles.md test row"},`, 1)
			},
		},
		"empty fee table": {
			mutate: func(s string) string {
				start := strings.Index(s, `"fees": [`)
				end := strings.Index(s, `"lots"`)
				return s[:start] + `"fees": [],` + "\n  " + s[end:]
			},
			want: driven.ErrRowInvalid,
		},
	} {
		t.Run(name, func(t *testing.T) {
			doc := tc.mutate(docJSON)
			if doc == docJSON {
				t.Fatal("mutation did not apply")
			}
			err := load(doc)
			if err == nil {
				t.Fatal("mutated document loaded")
			}
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatalf("want %v, got %v", tc.want, err)
			}
		})
	}
}

// TestCommittedMilestone2RevisionLoads guards the real committed document —
// configs/reference.btcusdt.json, the §40d milestone-2 revision citing the
// Profiles rows — not a fixture. Every cell of the BTC universe must answer,
// and the two takers must differ across products at both venues (the spot
// figure being materially higher is §2.4's own premise; a document where
// they collapsed would be a mis-transcription).
func TestCommittedMilestone2RevisionLoads(t *testing.T) {
	data, err := os.ReadFile("../../../../configs/reference.btcusdt.json")
	if err != nil {
		t.Fatalf("read committed revision: %v", err)
	}
	store, err := config.New[driven.Document]("configs/reference.btcusdt.json", loadAt, data)
	if err != nil {
		t.Fatalf("committed revision does not load: %v", err)
	}
	ref, err := driven.NewReferenceStore(store)
	if err != nil {
		t.Fatal(err)
	}

	asOf := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	for _, venue := range []ids.Venue{"binance", "bybit"} {
		spot := ids.VenueInstrument{Venue: venue, Product: ids.ProductSpot, Symbol: "BTCUSDT"}
		perp := ids.VenueInstrument{Venue: venue, Product: ids.ProductLinearPerp, Symbol: "BTCUSDT", Contract: ids.ContractLinear}

		sf, err := ref.Fees(spot, asOf)
		if err != nil {
			t.Fatalf("%s spot fees: %v", venue, err)
		}
		pf, err := ref.Fees(perp, asOf)
		if err != nil {
			t.Fatalf("%s perp fees: %v", venue, err)
		}
		c, err := sf.Taker.Cmp(pf.Taker)
		if err != nil {
			t.Fatal(err)
		}
		if c <= 0 {
			t.Fatalf("%s: spot taker %s not above perp taker %s", venue, sf.Taker, pf.Taker)
		}
		if sf.Provenance.Class != "hand_recorded" {
			t.Fatalf("%s spot fee class %q — a committed row may never claim fetched", venue, sf.Provenance.Class)
		}
		if _, err := ref.LotRules(spot, asOf); err != nil {
			t.Fatalf("%s spot lots: %v", venue, err)
		}
		if _, err := ref.LotRules(perp, asOf); err != nil {
			t.Fatalf("%s perp lots: %v", venue, err)
		}
	}
	if _, err := ref.Hurdle(asOf); err != nil {
		t.Fatalf("hurdle: %v", err)
	}
}
