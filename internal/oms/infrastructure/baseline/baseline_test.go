package baseline_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/oms/infrastructure/baseline"
	"github.com/giiorg/arb-lab/internal/oms/infrastructure/driven"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// This file checks the baseline document's loader ladder against a REAL
// journal, because the chain mark is a claim about a real chain (task #179,
// record §70b; §70h items 3, 7, 16, 19, 20).
//
// # The collapse checklist was run first
//
//   - B and ours are BOTH nonzero on the main fixture, so `B − baseline ==
//     ours` and `B == ours` are different predicates (§70h item 3 — the §56g
//     motivating case IS this fixture).
//   - A fee is charged in the BASE asset on a buy (drop the fee term and the
//     answer moves), a fee in a NON-base asset rides the same document (widen
//     the fee scope from asset-matched to all and the answer moves), and a
//     fee on a DIFFERENT pair debits the baselined asset (narrow the scope to
//     own-pair rows and the answer moves) — §70h items 1 and 2.
//   - The prefix fixture journals the excluded order AFTER the mark, so
//     "in the journal" and "in the prefix at the mark" are different sets.
//   - The credential-swap fixture matches on venue, account, chain mark and
//     digest, so the fingerprint comparison is the ONLY thing refusing.

const (
	testAccount     = "sub-1"
	testFingerprint = "fp-recorded"
)

func openJournal(t *testing.T) *driven.SQLiteJournal {
	t.Helper()
	j, err := driven.OpenSQLiteJournal(filepath.Join(t.TempDir(), "journal.db"))
	if err != nil {
		t.Fatalf("OpenSQLiteJournal: %v", err)
	}
	t.Cleanup(func() { _ = j.Close() })
	return j
}

func spotOrder(t *testing.T, id, symbol, base, quote string, side domain.Side) domain.Order {
	t.Helper()
	qty, err := fixedpoint.ParseQuantity(fixedpoint.Asset(base), "1")
	if err != nil {
		t.Fatalf("ParseQuantity: %v", err)
	}
	price, err := fixedpoint.ParsePrice(fixedpoint.Asset(base), fixedpoint.Asset(quote), "100")
	if err != nil {
		t.Fatalf("ParsePrice: %v", err)
	}
	return domain.Order{
		ClientOrderID: ids.ClientOrderID(id),
		Instrument:    ids.VenueInstrument{Venue: "binance", Product: ids.ProductSpot, Symbol: symbol},
		Side:          side,
		Type:          domain.OrderTypeLimit,
		TimeInForce:   domain.TimeInForceIOC,
		Quantity:      qty,
		LimitPrice:    price,
	}
}

// mark takes the journal's head as a document chain mark.
func mark(t *testing.T, j *driven.SQLiteJournal) (uint64, string) {
	t.Helper()
	seq, hash, err := j.Head()
	if err != nil {
		t.Fatalf("Head: %v", err)
	}
	if seq == 0 {
		return 0, ""
	}
	return seq, hex.EncodeToString(hash[:])
}

// pinned writes the document to disk and returns its honest pin.
func pinned(t *testing.T, doc baseline.Document) baseline.Pin {
	t.Helper()
	body, err := baseline.Encode(doc)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	path := filepath.Join(t.TempDir(), "baseline.json")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	digest := sha256.Sum256(body)
	return baseline.Pin{Path: path, SHA256: hex.EncodeToString(digest[:])}
}

func bindings() baseline.Bindings {
	return baseline.Bindings{
		Venue:       "binance",
		Account:     testAccount,
		Fingerprint: testFingerprint,
		Dialect:     baseline.DialectBinance,
	}
}

// mainDocument is the fixture the recomputation claims ride on: three spot
// orders on the journal, all counted, with the fee shapes §70h items 1 and 2
// name, and TWO baselined assets so the answer is a set rather than a scalar.
func mainDocument(t *testing.T, j *driven.SQLiteJournal) baseline.Document {
	t.Helper()
	for _, o := range []domain.Order{
		spotOrder(t, "ord-btc-buy", "BTCUSDT", "BTC", "USDT", domain.SideBuy),
		// Journaled as the SELL the document row states: the loader recomputes
		// the side from the prefix since §70i, so a fixture whose journal and
		// document disagreed — as this one silently did before the check
		// existed — refuses instead of computing from the document's word.
		spotOrder(t, "ord-btc-sell", "BTCUSDT", "BTC", "USDT", domain.SideSell),
		spotOrder(t, "ord-eth-buy", "ETHUSDT", "ETH", "USDT", domain.SideBuy),
	} {
		if err := j.AppendPlanned(o); err != nil {
			t.Fatalf("AppendPlanned(%s): %v", o.ClientOrderID, err)
		}
	}
	seq, hash := mark(t, j)
	return baseline.Document{
		Kind:             baseline.Kind,
		Version:          baseline.Version,
		Venue:            "binance",
		Account:          testAccount,
		CredentialSHA256: testFingerprint,
		ObservedAt:       "2026-08-05T12:00:00Z",
		ChainSeq:         seq,
		ChainHash:        hash,
		Balances: []baseline.BalanceRow{
			{Asset: "BTC", Words: []baseline.Word{
				{Name: "free", Value: "0.75000000"},
				{Name: "locked", Value: "0.25000000"},
			}},
			// B(ETH) is zero AS THE VENUE STATES IT: Binance returns every
			// asset with omitZeroBalances unset, so its zero is a row with
			// zero words, never an absence (§70i — the absent marker is the
			// Bybit dialect's, whose enumeration omits zero coins).
			{Asset: "ETH", Words: []baseline.Word{
				{Name: "free", Value: "0.00000000"},
				{Name: "locked", Value: "0.00000000"},
			}},
		},
		Orders: []baseline.OrderRow{
			// A buy with a fee in the BASE asset (item 1's first half)...
			{ClientOrderID: "ord-btc-buy", Side: "buy", Base: "BTC", Cumulative: "0.5",
				Fees: []baseline.Fee{{Asset: "BTC", Amount: "0.0005"}}},
			// ...a sell with a fee in a NON-base asset (item 1's second half:
			// the USDT fee must not move any baselined asset's arithmetic)...
			{ClientOrderID: "ord-btc-sell", Side: "sell", Base: "BTC", Cumulative: "0.2",
				Fees: []baseline.Fee{{Asset: "USDT", Amount: "1.2"}}},
			// ...and a DIFFERENT pair's order whose fee debits BTC (item 2,
			// the BNB shape): scoping fees to own-pair rows loses this one.
			{ClientOrderID: "ord-eth-buy", Side: "buy", Base: "ETH", Cumulative: "1",
				Fees: []baseline.Fee{{Asset: "BTC", Amount: "0.0001"}}},
		},
	}
}

func loadQuantity(t *testing.T, got []application.SpotBaseline, asset string) fixedpoint.Quantity {
	t.Helper()
	for _, b := range got {
		if string(b.Asset) == asset {
			return b.Amount
		}
	}
	t.Fatalf("no baseline for %s in %+v", asset, got)
	return fixedpoint.Quantity{}
}

func TestLoadRecomputesEverythingFromStoredInputs(t *testing.T) {
	t.Parallel()

	j := openJournal(t)
	doc := mainDocument(t, j)
	got, err := baseline.Load(pinned(t, doc), j, bindings())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("Load returned %d baselines, want 2", len(got))
	}

	// B(BTC) = 0.75 + 0.25 = 1.0 (the shared derivation over the words).
	// ours(BTC) = 0.5 − 0.2 − 0.0005 − 0.0001 = 0.2994.
	// baseline = 1.0 − 0.2994 = 0.7006 — B and ours both nonzero, fee terms
	// from a buy, from another pair, and never from the USDT fee.
	btc := loadQuantity(t, got, "BTC")
	want, err := fixedpoint.ParseQuantity("BTC", "0.7006")
	if err != nil {
		t.Fatalf("ParseQuantity: %v", err)
	}
	if eq, err := btc.Cmp(want); err != nil || eq != 0 {
		t.Errorf("baseline(BTC) = %s, want %s", btc, want)
	}

	// B(ETH) = 0 (stored absence); ours(ETH) = 1 → baseline = −1: the venue
	// holds less than the journal explains, and the number says so with its
	// sign rather than clamping.
	eth := loadQuantity(t, got, "ETH")
	wantETH, err := fixedpoint.ParseQuantity("ETH", "-1")
	if err != nil {
		t.Fatalf("ParseQuantity: %v", err)
	}
	if eq, err := eth.Cmp(wantETH); err != nil || eq != 0 {
		t.Errorf("baseline(ETH) = %s, want %s", eth, wantETH)
	}
}

func TestLoadRefusesBytesTheConfigDoesNotPin(t *testing.T) {
	t.Parallel()

	j := openJournal(t)
	doc := mainDocument(t, j)
	pin := pinned(t, doc)

	// Replace the document at the pinned path with a schema-valid alternative
	// — same shape, one word changed — WITHOUT the config edit. O_EXCL cannot
	// see this; the digest is what refuses it (§70h item 20).
	doc.Balances[0].Words[1].Value = "0.35000000"
	body, err := baseline.Encode(doc)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if err := os.WriteFile(pin.Path, body, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := baseline.Load(pin, j, bindings()); !errors.Is(err, baseline.ErrDigestMismatch) {
		t.Errorf("Load(replaced bytes) = %v, want ErrDigestMismatch", err)
	}
}

func TestLoadRefusesARowOutsideTheChainMarkedPrefix(t *testing.T) {
	t.Parallel()

	j := openJournal(t)
	doc := mainDocument(t, j)

	// Journal a FOURTH order after the mark was taken, and claim it in the
	// document. It is in the journal — only the prefix comparison can refuse
	// it, so "in the journal" and "in the prefix" must be different sets.
	if err := j.AppendPlanned(spotOrder(t, "ord-late", "BTCUSDT", "BTC", "USDT", domain.SideBuy)); err != nil {
		t.Fatalf("AppendPlanned: %v", err)
	}
	doc.Orders = append(doc.Orders, baseline.OrderRow{
		ClientOrderID: "ord-late", Side: "buy", Base: "BTC", Cumulative: "0.1",
	})
	if _, err := baseline.Load(pinned(t, doc), j, bindings()); !errors.Is(err, baseline.ErrOrderOutsidePrefix) {
		t.Errorf("Load(row past the mark) = %v, want ErrOrderOutsidePrefix", err)
	}
}

func TestLoadChainMarkRefusalsAreTwoDifferentRefusals(t *testing.T) {
	t.Parallel()

	// (a) The hash at the marked seq differs: a rebuilt or restored history.
	t.Run("hash differs at seq", func(t *testing.T) {
		t.Parallel()
		j := openJournal(t)
		doc := mainDocument(t, j)
		doc.ChainHash = "00000000000000000000000000000000" + doc.ChainHash[32:]
		if doc.ChainHash[:32] == mainHashPrefix(t, j) {
			t.Fatal("fixture failed to change the hash")
		}
		if _, err := baseline.Load(pinned(t, doc), j, bindings()); !errors.Is(err, baseline.ErrChainMarkMismatch) {
			t.Errorf("Load(wrong hash) = %v, want ErrChainMarkMismatch", err)
		}
	})

	// (b) The seq is past the head: the journal LOST records — a different
	// operator conversation, named by the journal's own sentinel.
	t.Run("seq beyond head", func(t *testing.T) {
		t.Parallel()
		j := openJournal(t)
		doc := mainDocument(t, j)
		doc.ChainSeq++
		if _, err := baseline.Load(pinned(t, doc), j, bindings()); !errors.Is(err, driven.ErrJournalMarkBeyondHead) {
			t.Errorf("Load(seq past head) = %v, want ErrJournalMarkBeyondHead", err)
		}
	})

	// (c) Seq 0 verifies vacuously: a baseline recorded over an EMPTY journal
	// attested nothing, and every journal extends the empty one.
	t.Run("seq zero is vacuous", func(t *testing.T) {
		t.Parallel()
		j := openJournal(t)
		doc := baseline.Document{
			Kind: baseline.Kind, Version: baseline.Version,
			Venue: "binance", Account: testAccount, CredentialSHA256: testFingerprint,
			ObservedAt: "2026-08-05T12:00:00Z",
			Balances: []baseline.BalanceRow{{Asset: "BTC", Words: []baseline.Word{
				{Name: "free", Value: "1.00000000"}, {Name: "locked", Value: "0"},
			}}},
		}
		got, err := baseline.Load(pinned(t, doc), j, bindings())
		if err != nil {
			t.Fatalf("Load(empty journal, seq 0): %v", err)
		}
		want, err := fixedpoint.ParseQuantity("BTC", "1")
		if err != nil {
			t.Fatalf("ParseQuantity: %v", err)
		}
		if eq, err := loadQuantity(t, got, "BTC").Cmp(want); err != nil || eq != 0 {
			t.Errorf("baseline(BTC) = %s, want %s (ours was zero by construction)", got[0].Amount, want)
		}
	})
}

func mainHashPrefix(t *testing.T, j *driven.SQLiteJournal) string {
	t.Helper()
	_, hash := mark(t, j)
	return hash[:32]
}

func TestLoadRefusesACredentialSwapEverythingElseMatching(t *testing.T) {
	t.Parallel()

	j := openJournal(t)
	doc := mainDocument(t, j)
	bind := bindings()
	bind.Fingerprint = "fp-rotated"

	// Venue, account, chain mark and digest all match; the fingerprint is the
	// ONLY discriminator (§70h item 19) — a credential swapped to another
	// subaccount under an unchanged label passes every other binding.
	if _, err := baseline.Load(pinned(t, doc), j, bind); !errors.Is(err, baseline.ErrCredentialMismatch) {
		t.Errorf("Load(rotated credential) = %v, want ErrCredentialMismatch", err)
	}
}

func TestLoadRefusesAForeignDeployment(t *testing.T) {
	t.Parallel()

	j := openJournal(t)
	doc := mainDocument(t, j)

	bind := bindings()
	bind.Venue = "bybit"
	bind.Dialect = baseline.DialectBybit
	if _, err := baseline.Load(pinned(t, doc), j, bind); !errors.Is(err, baseline.ErrVenueMismatch) {
		t.Errorf("Load(other venue) = %v, want ErrVenueMismatch", err)
	}

	bind = bindings()
	bind.Account = "sub-2"
	if _, err := baseline.Load(pinned(t, doc), j, bind); !errors.Is(err, baseline.ErrAccountMismatch) {
		t.Errorf("Load(other account) = %v, want ErrAccountMismatch", err)
	}
}

func TestLoadRefusesAStoredRowTheDerivationCannotPin(t *testing.T) {
	t.Parallel()

	// A Bybit document whose stored words carry locked != 0 — the record
	// command never writes one (it refuses unpinned rows), so this is a
	// hand-made document. The VALUE-swap shape: walletBalance and locked
	// exchanged is exactly this refusal, which is what makes the swap loud
	// (§70h item 16's word-mutation half for the venue where it can bite).
	j := openJournal(t)
	doc := baseline.Document{
		Kind: baseline.Kind, Version: baseline.Version,
		Venue: "bybit", Account: testAccount, CredentialSHA256: testFingerprint,
		ObservedAt: "2026-08-05T12:00:00Z",
		Balances: []baseline.BalanceRow{{Asset: "BTC", Words: []baseline.Word{
			{Name: "walletBalance", Value: "0"},
			{Name: "locked", Value: "1.25000000"},
		}}},
	}
	bind := baseline.Bindings{Venue: "bybit", Account: testAccount,
		Fingerprint: testFingerprint, Dialect: baseline.DialectBybit}
	if _, err := baseline.Load(pinned(t, doc), j, bind); !errors.Is(err, baseline.ErrStoredRowUnpinnable) {
		t.Errorf("Load(unpinnable words) = %v, want ErrStoredRowUnpinnable", err)
	}
}

func TestLoadAbsentRowIsTheBybitDialectsWord(t *testing.T) {
	t.Parallel()

	// Bybit's coin-less enumeration returns non-zero assets by the venue's
	// own sentence, so a stored absence re-derives as zero THERE — beside a
	// counted order, so the zero feeds a real subtraction rather than a
	// degenerate 0−0 (§70i).
	j := openJournal(t)
	order := domain.Order{
		ClientOrderID: ids.ClientOrderID("ord-by-buy"),
		Instrument:    ids.VenueInstrument{Venue: "bybit", Product: ids.ProductSpot, Symbol: "BTCUSDT"},
		Side:          domain.SideBuy,
		Type:          domain.OrderTypeLimit,
		TimeInForce:   domain.TimeInForceIOC,
	}
	var err error
	if order.Quantity, err = fixedpoint.ParseQuantity("BTC", "1"); err != nil {
		t.Fatalf("ParseQuantity: %v", err)
	}
	if order.LimitPrice, err = fixedpoint.ParsePrice("BTC", "USDT", "100"); err != nil {
		t.Fatalf("ParsePrice: %v", err)
	}
	if err := j.AppendPlanned(order); err != nil {
		t.Fatalf("AppendPlanned: %v", err)
	}
	seq, hash := mark(t, j)
	doc := baseline.Document{
		Kind: baseline.Kind, Version: baseline.Version,
		Venue: "bybit", Account: testAccount, CredentialSHA256: testFingerprint,
		ObservedAt: "2026-08-05T12:00:00Z",
		ChainSeq:   seq, ChainHash: hash,
		Balances: []baseline.BalanceRow{{Asset: "BTC", Absent: true}},
		Orders: []baseline.OrderRow{{ClientOrderID: "ord-by-buy", Side: "buy",
			Base: "BTC", Cumulative: "0.5"}},
	}
	bind := baseline.Bindings{Venue: "bybit", Account: testAccount,
		Fingerprint: testFingerprint, Dialect: baseline.DialectBybit}
	got, err := baseline.Load(pinned(t, doc), j, bind)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// baseline = B − ours = 0 − 0.5 = −0.5.
	want, err := fixedpoint.ParseQuantity("BTC", "-0.5")
	if err != nil {
		t.Fatalf("ParseQuantity: %v", err)
	}
	if cmp, err := loadQuantity(t, got, "BTC").Cmp(want); err != nil || cmp != 0 {
		t.Errorf("baseline(BTC) = %v, want -0.5 (cmp %d, err %v)", loadQuantity(t, got, "BTC"), cmp, err)
	}
}

func TestLoadRefusesAnAbsentRowUnderTheBinanceDialect(t *testing.T) {
	t.Parallel()

	// Binance returns every asset with omitZeroBalances unset, so a stored
	// absence is a silence that venue could not have answered with — the
	// record command never writes one, and re-deriving zero from it would
	// mint a quantity from missing evidence (§70i). The positive control is
	// TestLoadRecomputesEverythingFromStoredInputs, whose ETH zero is stated
	// in words.
	j := openJournal(t)
	doc := mainDocument(t, j)
	doc.Balances[1] = baseline.BalanceRow{Asset: "ETH", Absent: true}
	if _, err := baseline.Load(pinned(t, doc), j, bindings()); !errors.Is(err, baseline.ErrStoredAbsenceUninterpretable) {
		t.Errorf("Load(absent under binance) = %v, want ErrStoredAbsenceUninterpretable", err)
	}
}

func TestLoadRefusesARowWhoseSideContradictsThePrefix(t *testing.T) {
	t.Parallel()

	// The side is the one row field the journal prefix can recompute, and a
	// flip inverts the row's whole contribution to `ours` — checked in both
	// directions, because the two flips move the baseline opposite ways and
	// either would read as a clean load without the check (§70i).
	for name, flip := range map[string]func(*baseline.Document){
		"buy claimed as sell": func(d *baseline.Document) { d.Orders[0].Side = "sell" },
		"sell claimed as buy": func(d *baseline.Document) { d.Orders[1].Side = "buy" },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			j := openJournal(t)
			doc := mainDocument(t, j)
			flip(&doc)
			if _, err := baseline.Load(pinned(t, doc), j, bindings()); !errors.Is(err, baseline.ErrOrderRowSideDisagrees) {
				t.Errorf("Load(flipped side) = %v, want ErrOrderRowSideDisagrees", err)
			}
		})
	}
}

func TestLoadDerivesFromTheWordsNotFromAnyStoredConclusion(t *testing.T) {
	t.Parallel()

	// Two documents identical except ONE word value, each honestly pinned:
	// the recomputed baselines must differ by exactly that edit. There is no
	// stored total to fall back on — this is the stored-form rule made
	// observable (§70h item 16).
	j := openJournal(t)
	doc := mainDocument(t, j)
	first, err := baseline.Load(pinned(t, doc), j, bindings())
	if err != nil {
		t.Fatalf("Load(first): %v", err)
	}
	doc.Balances[0].Words[1].Value = "0.35000000"
	second, err := baseline.Load(pinned(t, doc), j, bindings())
	if err != nil {
		t.Fatalf("Load(second): %v", err)
	}
	a := loadQuantity(t, first, "BTC")
	b := loadQuantity(t, second, "BTC")
	diff, err := b.Sub(a)
	if err != nil {
		t.Fatalf("Sub: %v", err)
	}
	want, err := fixedpoint.ParseQuantity("BTC", "0.1")
	if err != nil {
		t.Fatalf("ParseQuantity: %v", err)
	}
	if eq, err := diff.Cmp(want); err != nil || eq != 0 {
		t.Errorf("edit moved the baseline by %s, want 0.1 — the loader is not consuming the words", diff)
	}
}

func TestWriteIsExclusiveAndRoundTrips(t *testing.T) {
	t.Parallel()

	j := openJournal(t)
	doc := mainDocument(t, j)
	path := filepath.Join(t.TempDir(), "2026-08-05-baseline.json")
	if err := baseline.Write(path, doc); err != nil {
		t.Fatalf("Write: %v", err)
	}
	// No overwrite path exists: the second act is a config edit, never a
	// rewrite of history.
	if err := baseline.Write(path, doc); err == nil {
		t.Error("Write over an existing document = nil, want the O_EXCL refusal")
	}

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	digest := sha256.Sum256(body)
	got, err := baseline.Load(baseline.Pin{Path: path, SHA256: hex.EncodeToString(digest[:])}, j, bindings())
	if err != nil {
		t.Fatalf("Load(written document): %v", err)
	}
	want, err := fixedpoint.ParseQuantity("BTC", "0.7006")
	if err != nil {
		t.Fatalf("ParseQuantity: %v", err)
	}
	if eq, err := loadQuantity(t, got, "BTC").Cmp(want); err != nil || eq != 0 {
		t.Errorf("round-tripped baseline(BTC) = %s, want %s", loadQuantity(t, got, "BTC"), want)
	}
}

func TestWriteCleansUpAPartialFile(t *testing.T) {
	t.Parallel()

	// An invalid document must leave NOTHING at the path: a partial file
	// would be a document that exists and cannot load, and O_EXCL would then
	// refuse the retry that could fix it.
	path := filepath.Join(t.TempDir(), "baseline.json")
	if err := baseline.Write(path, baseline.Document{Kind: "wrong"}); err == nil {
		t.Fatal("Write(invalid) = nil, want a refusal")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Stat after refused write = %v, want ErrNotExist", err)
	}
}

func TestEncodeIsPinnedToTheByte(t *testing.T) {
	t.Parallel()

	// The config pins documents BY DIGEST, so the encoding is part of the
	// deployment contract: a marshalling change that reordered or reformatted
	// fields would refuse every already-pinned document in the field. The
	// complete output is pinned — a substring test cannot state "and nothing
	// else changed" (a preservation claim needs a golden).
	doc := baseline.Document{
		Kind: baseline.Kind, Version: baseline.Version,
		Venue: "binance", Account: testAccount, CredentialSHA256: testFingerprint,
		ObservedAt: "2026-08-05T12:00:00Z",
		ChainSeq:   2,
		ChainHash:  "6c62272e07bb014262b821756295c58d6c62272e07bb014262b821756295c58d",
		Balances: []baseline.BalanceRow{
			{Asset: "BTC", Words: []baseline.Word{
				{Name: "free", Value: "0.75000000"}, {Name: "locked", Value: "0.25000000"},
			}},
			{Asset: "ETH", Absent: true},
		},
		Orders: []baseline.OrderRow{{ClientOrderID: "ord-1", Side: "buy", Base: "BTC",
			Cumulative: "0.5", Fees: []baseline.Fee{{Asset: "BTC", Amount: "0.0005"}}}},
	}
	body, err := baseline.Encode(doc)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	want := `{
  "kind": "arblab-spot-baseline",
  "version": 1,
  "venue": "binance",
  "account": "sub-1",
  "credential_sha256": "fp-recorded",
  "observed_at": "2026-08-05T12:00:00Z",
  "chain_seq": 2,
  "chain_hash": "6c62272e07bb014262b821756295c58d6c62272e07bb014262b821756295c58d",
  "balances": [
    {
      "asset": "BTC",
      "words": [
        {
          "name": "free",
          "value": "0.75000000"
        },
        {
          "name": "locked",
          "value": "0.25000000"
        }
      ]
    },
    {
      "asset": "ETH",
      "absent": true
    }
  ],
  "orders": [
    {
      "client_order_id": "ord-1",
      "side": "buy",
      "base": "BTC",
      "cumulative": "0.5",
      "fees": [
        {
          "asset": "BTC",
          "amount": "0.0005"
        }
      ]
    }
  ]
}
`
	if string(body) != want {
		t.Errorf("Encode drifted from the pinned bytes:\n got: %q\nwant: %q", body, want)
	}
}

func TestDocumentValidateRefusals(t *testing.T) {
	t.Parallel()

	valid := func() baseline.Document {
		return baseline.Document{
			Kind: baseline.Kind, Version: baseline.Version,
			Venue: "binance", Account: testAccount, CredentialSHA256: testFingerprint,
			ObservedAt: "2026-08-05T12:00:00Z",
			Balances: []baseline.BalanceRow{{Asset: "BTC", Words: []baseline.Word{
				{Name: "free", Value: "1"}, {Name: "locked", Value: "0"},
			}}},
			Orders: []baseline.OrderRow{{ClientOrderID: "ord-1", Side: "buy", Base: "BTC", Cumulative: "0.5",
				Fees: []baseline.Fee{{Asset: "BTC", Amount: "0.001"}}}},
		}
	}
	if err := valid().Validate(); err != nil {
		t.Fatalf("the fixture must validate, or every refusal below proves nothing: %v", err)
	}

	cases := []struct {
		name   string
		mutate func(*baseline.Document)
	}{
		{"foreign kind", func(d *baseline.Document) { d.Kind = "arblab-reference" }},
		{"foreign version", func(d *baseline.Document) { d.Version = 2 }},
		{"venue unset", func(d *baseline.Document) { d.Venue = "" }},
		{"account unset", func(d *baseline.Document) { d.Account = "" }},
		{"fingerprint unset", func(d *baseline.Document) { d.CredentialSHA256 = "" }},
		{"observed_at unparseable", func(d *baseline.Document) { d.ObservedAt = "yesterday" }},
		{"chain hash without seq", func(d *baseline.Document) { d.ChainHash = "ab" }},
		{"chain seq with a short hash", func(d *baseline.Document) { d.ChainSeq = 3; d.ChainHash = "abcd" }},
		{"no balance rows", func(d *baseline.Document) { d.Balances = nil }},
		{"balance asset twice", func(d *baseline.Document) {
			d.Balances = append(d.Balances, d.Balances[0])
		}},
		{"absent AND words", func(d *baseline.Document) { d.Balances[0].Absent = true }},
		{"neither absent nor words", func(d *baseline.Document) { d.Balances[0].Words = nil }},
		{"order id twice", func(d *baseline.Document) { d.Orders = append(d.Orders, d.Orders[0]) }},
		{"order side foreign", func(d *baseline.Document) { d.Orders[0].Side = "long" }},
		{"order base unset", func(d *baseline.Document) { d.Orders[0].Base = "" }},
		{"cumulative negative", func(d *baseline.Document) { d.Orders[0].Cumulative = "-0.5" }},
		{"cumulative unparseable", func(d *baseline.Document) { d.Orders[0].Cumulative = "much" }},
		{"fee asset twice", func(d *baseline.Document) {
			d.Orders[0].Fees = append(d.Orders[0].Fees, d.Orders[0].Fees[0])
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			doc := valid()
			tc.mutate(&doc)
			if err := doc.Validate(); !errors.Is(err, baseline.ErrDocumentMalformed) {
				t.Errorf("Validate(%s) = %v, want ErrDocumentMalformed", tc.name, err)
			}
		})
	}
}
