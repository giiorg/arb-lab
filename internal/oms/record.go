package oms

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/infrastructure/baseline"
	"github.com/giiorg/arb-lab/internal/oms/infrastructure/driven"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// This file is `arblab baseline record`'s six-step protocol (task #179,
// record §70b): the one way a spot exposure baseline comes into existence.
//
// # "Determined" is a PROTOCOL, not a predicate
//
// Terminal state does not close the fill ledger — §5.3 admits a post-cancel
// late fill indefinitely — and the join's order read, executions sweep and the
// balance read are three independent calls with no common snapshot. So the
// protocol brackets the whole determination: settle both surfaces, wait the
// bound, re-read BOTH, and refuse if either moved. It refuses rather than
// waits in every arm, and it does NOT require flatness — terminal-and-
// concluded gives `ours` exactly, which is all the subtraction needs, and a
// mid-carry re-record must not demand flattening a healthy hedge.
//
// # Recording IS absorption, and the guard is visibility plus the two-act rule
//
// `B − ours` folds any currently unexplained delta into the baseline by
// construction — that is what an operator who just deposited BTC needs. The
// command's caller prints the delta being absorbed when a prior baseline is
// configured, and the config digest edit is the second, diffable act. There is
// no `--absorb-delta` flag: diagnosis and forgiveness must not share a
// keystroke.

// SpotBaselineDocument is Deps' pin for the recorded baseline: §5.2's
// `spot_baseline` config field plus the two bindings only a root can state —
// the transport credential's fingerprint and the venue dialect the loader's
// balance derivation is selected by.
type SpotBaselineDocument struct {
	Pin         baseline.Pin
	Fingerprint string
	Dialect     baseline.Dialect
}

// ErrSpotBaselineDoubleSourced reports Deps carrying both the data seam and
// the document pin — two answers to which history the baseline is about.
var ErrSpotBaselineDoubleSourced = errors.New("oms: spot baselines supplied both as data and as a document pin")

// RecordBaselineDeps is what the composition root supplies beyond the module:
// the transport's credential fingerprint (a binding the document carries), the
// venue dialect (which balance derivation the loader will re-run), and the
// settle bracket.
type RecordBaselineDeps struct {
	Fingerprint string
	Dialect     baseline.Dialect

	// SettleWait and SpotSettle are §5.2's `spot_settle` bound with the
	// blocking wait the root owns — the same pair the boot's second look uses,
	// because the two brackets are one rule with two callers. REQUIRED here:
	// a record without the bracket would mint a permanent fact from one read
	// of two surfaces that can lag each other.
	SettleWait func(time.Duration)
	SpotSettle time.Duration

	// PriorPin is the currently-configured baseline document, if any, loaded
	// BEST-EFFORT for the absorbed-delta print. Best-effort is deliberate and
	// it is the one place the pin's strictness inverts: re-recording is the
	// REMEDY for a prior that no longer verifies (a reset journal, a rotated
	// credential), so a record command that refused to run over an unloadable
	// prior would refuse exactly the operator it exists to rescue. The boot
	// stays strict (oms.New errors); a failed prior load here lands in
	// RecordedBaseline.PriorError instead, for the command to print.
	PriorPin *baseline.Pin
}

// RecordedBaseline is what the protocol established.
type RecordedBaseline struct {
	// Path and SHA256 are the operator's config pin — the digest is of the
	// exact bytes on disk.
	Path   string
	SHA256 string

	// Document is what was written.
	Document baseline.Document

	// Baselines are the per-asset quantities the VERIFYING LOADER recomputed
	// from the written file — not this protocol's own arithmetic, so the
	// number the operator sees is the number every future boot will derive.
	Baselines []application.SpotBaseline

	// Absorbed is what this record folds into the baseline relative to the
	// PRIOR one, per asset — non-empty only when a prior baseline was
	// available. Recording IS absorption, and the guard is visibility: the
	// command prints these, and the config edit is the second act.
	Absorbed []AbsorbedDelta

	// PriorError is why the prior pin could not be loaded, when one was given
	// and failed — the absorbed print is then unavailable, stated rather than
	// silently empty.
	PriorError error
}

// AbsorbedDelta is one asset's baseline movement across a re-record.
type AbsorbedDelta struct {
	Asset fixedpoint.Asset
	Prior fixedpoint.Quantity
	New   fixedpoint.Quantity

	// Delta is New − Prior: the quantity this record just declared "not
	// ours" (positive) or reclaimed as ours (negative).
	Delta fixedpoint.Quantity
}

// RecordSpotBaseline runs the six-step protocol and writes the document at
// path (O_EXCL — a new dated file, never an overwrite).
//
// The gauntlet's refusals, each with its own sentinel because each names a
// different operator conversation: an open spot order (wait for terminal), an
// incomplete sweep (retry), an unattributed execution (the non-negotiable one
// — real exposure with no owner, and recording over it folds it into "not
// ours" permanently), a contradicted lifecycle reading (the venue and this
// system hold states SM §1 has no edge between, so it is not settled which
// record the baseline would be about), an unproved absence (§5.3's timers have
// not concluded), a stale projection (fix the store first), an unpinned
// balance row (a resting order or borrow — resolve it, then record), and
// either surface moving across the settle window (the world is still trading;
// run it again).
func (m *Module) RecordSpotBaseline(path string, deps RecordBaselineDeps) (RecordedBaseline, error) {
	var out RecordedBaseline
	if deps.Fingerprint == "" {
		return out, ErrRecordFingerprintUnset
	}
	if err := deps.Dialect.Validate(); err != nil {
		return out, err
	}
	if deps.SettleWait == nil || deps.SpotSettle <= 0 {
		return out, ErrRecordSettleUnwired
	}
	if m.balances == nil {
		return out, ErrRecordBalancesUnwired
	}
	if err := m.venue.Validate(); err != nil {
		return out, fmt.Errorf("record baseline venue: %w", err)
	}
	if len(m.roster) == 0 {
		// No configured spot instrument means no coverage set: a document
		// with no balance rows pins nothing and refuses at Validate anyway —
		// this refusal just names the cause instead of the symptom.
		return out, ErrRecordNoSpotInstruments
	}

	// The record protocol's reads ride ROUTINE: an operator maintenance sweep
	// must be shed before the emergency reserve, not out of it (§70f). The
	// module's own adapters are the BOOT's (Safety on balances; the order
	// reads carry the root's RecoveryReadPriority, which the record root sets
	// to Routine) — this one is the protocol's own.
	adapter, err := driven.NewSpotBalances(m.balances, m.venue, ratelimit.PriorityRoutine)
	if err != nil {
		return out, fmt.Errorf("record baseline balances adapter: %w", err)
	}

	// 1. Replay. Dispatched is exactly the MayExistAtVenue set, so a
	// Planned-only order is excluded here BY THE SAME PREDICATE the join
	// uses: it is non-terminal, has no venue side, contributes exact zero,
	// and the journal has no discard record kind — a crash between plan and
	// dispatch must not make recording permanently impossible (§70b step 1).
	rec, err := m.Recover.Rebuild()
	if err != nil {
		return out, fmt.Errorf("record baseline replay: %w", err)
	}

	// 2. Join: sweep concluded, cross-check agreed, lifecycle readings agreed.
	joined, err := m.Join.Join(rec.Dispatched)
	if err != nil {
		return out, fmt.Errorf("record baseline join: %w", err)
	}
	if len(joined.Unattributed) > 0 {
		return out, fmt.Errorf("orders %v: %w", joined.Unattributed, ErrRecordUnattributed)
	}
	if len(joined.Incomplete) > 0 {
		return out, fmt.Errorf("orders %v: %w", joined.Incomplete, ErrRecordSweepIncomplete)
	}
	// The third condition, and it is about STATE where the two above are about
	// quantity. A baseline is an attestation of `B − ours`, and `ours` is
	// summed over the spot orders' cumulatives — so recording over two
	// histories SM §1 has no edge between mints a permanent fact from a
	// projection it is not settled we can read. It refuses rather than waits,
	// like every other arm here, and unlike Incomplete it is not the kind of
	// refusal a retry lifts: the same account answers the same way.
	if len(joined.Contradicted) > 0 {
		return out, fmt.Errorf("orders %v: %w", joined.Contradicted, ErrRecordContradicted)
	}

	// 3. Every absence through §5.3's rule — a pending absence refuses, a
	// concluded one is the rule working.
	absentAtJoin := make(map[ids.ClientOrderID]struct{}, len(joined.Absent))
	for _, id := range joined.Absent {
		absentAtJoin[id] = struct{}{}
		res, err := m.Resolve.Resolve(id)
		if err != nil {
			return out, fmt.Errorf("record baseline resolve %q: %w", id, err)
		}
		if res.Outcome == application.OutcomeUnproved {
			return out, fmt.Errorf("order %q (%s): %w", id, res.Reason, ErrRecordAbsenceUnproved)
		}
		if res.Journaled && !res.Applied {
			return out, fmt.Errorf("order %q: %w", id, ErrRecordProjectionStale)
		}
	}

	// Terminal check and row collection, AFTER resolution so an absence that
	// §5.3 just concluded counts as what it became.
	bases := make(map[ids.VenueInstrument]fixedpoint.Asset, len(m.roster))
	for _, r := range m.roster {
		bases[r.Instrument] = r.Base
	}
	type spotRecord struct {
		rec ports.OrderRecord
		row baseline.OrderRow
	}
	var spots []spotRecord
	for _, id := range rec.Dispatched {
		r, err := m.orders.Load(id)
		if err != nil {
			return out, fmt.Errorf("record baseline order %q: %w", id, err)
		}
		if r.Order.Instrument.Product != ids.ProductSpot {
			// A live perp order is FINE: a mid-carry re-record holds a healthy
			// hedge open, and flat-only recording was rejected for exactly it.
			continue
		}
		terminal, err := r.State.IsTerminal()
		if err != nil {
			return out, fmt.Errorf("record baseline order %q at %s: %w", id, r.State, err)
		}
		if !terminal {
			return out, fmt.Errorf("order %q at %s: %w", id, r.State, ErrRecordSpotOrderOpen)
		}
		base, known := bases[r.Order.Instrument]
		if !known {
			// A journal-held instrument the config dropped: its fills moved
			// the same wallet, so its row is counted under the cumulative's
			// own asset tag — assertSpot's rule, at record time.
			base = r.Cumulative.Asset()
		}
		row := baseline.OrderRow{
			ClientOrderID: string(id),
			Side:          string(r.Order.Side),
			Base:          string(base),
			Cumulative:    r.Cumulative.Decimal(),
		}
		for _, fee := range r.Fees {
			row.Fees = append(row.Fees, baseline.Fee{Asset: string(fee.Asset()), Amount: fee.Decimal()})
		}
		spots = append(spots, spotRecord{rec: r, row: row})
	}
	sort.Slice(spots, func(a, b int) bool { return spots[a].row.ClientOrderID < spots[b].row.ClientOrderID })

	// The coverage set: every configured spot base (sorted, deduplicated —
	// two pairs sharing a base share the cell).
	baseSet := make(map[fixedpoint.Asset]struct{}, len(m.roster))
	for _, r := range m.roster {
		baseSet[r.Base] = struct{}{}
	}
	coverage := make([]fixedpoint.Asset, 0, len(baseSet))
	for asset := range baseSet {
		coverage = append(coverage, asset)
	}
	sort.Slice(coverage, func(a, b int) bool { return coverage[a] < coverage[b] })

	// 3b. Read B.
	first, err := m.recordBalanceEvidence(adapter, coverage)
	if err != nil {
		return out, err
	}

	// 4. Wait the settle bound — observed, never inferred from journal
	// instants (§70b step 4: authorization is days old on a long-lived order
	// canceled seconds ago, and state records carry no instant).
	deps.SettleWait(deps.SpotSettle)

	// 5. Re-read BOTH surfaces and require them unchanged. A late-reporting
	// execution either moved a cumulative across the window (refused here),
	// was counted by the cumulative but missing from the sweep (refused at
	// step 2 as Unattributed), or moved the balance (refused here). The
	// residual premise is NAMED, not argued away: a venue order surface stale
	// by MORE than the settle bound, consistently across both reads, is
	// undetectable — that premise is bounded, per-venue, operator-owned via
	// the config field.
	for _, s := range spots {
		status, err := m.reader.Resolve(s.rec.Order)
		if err != nil {
			return out, fmt.Errorf("record baseline re-read %q: %w", s.row.ClientOrderID, err)
		}
		if _, wasAbsent := absentAtJoin[s.rec.Order.ClientOrderID]; wasAbsent {
			if status.Presence != ports.VenueHasNoOrder {
				// The venue found an order it denied minutes ago: the surface
				// is still converging, and a cumulative read off it now
				// describes an instant the first look never saw.
				return out, fmt.Errorf("order %q reappeared at the venue: %w",
					s.row.ClientOrderID, ErrRecordCumulativeMoved)
			}
			continue
		}
		if status.Presence != ports.VenueHasOrder {
			return out, fmt.Errorf("order %q vanished from the venue across the settle window: %w",
				s.row.ClientOrderID, ErrRecordCumulativeMoved)
		}
		same, err := status.Executed.Cmp(s.rec.Cumulative)
		if err != nil {
			return out, fmt.Errorf("record baseline re-read %q: %w", s.row.ClientOrderID, err)
		}
		if same != 0 {
			return out, fmt.Errorf("order %q cumulative moved from %s to %s across the settle window: %w",
				s.row.ClientOrderID, s.rec.Cumulative, status.Executed, ErrRecordCumulativeMoved)
		}
	}
	second, err := m.recordBalanceEvidence(adapter, coverage)
	if err != nil {
		return out, err
	}
	for _, asset := range coverage {
		if !balanceEvidenceEqual(first.evidence[asset], second.evidence[asset]) {
			return out, fmt.Errorf("balance of %s moved across the settle window: %w",
				asset, ErrRecordBalanceMoved)
		}
	}

	// 6. Write the document, chain-marked at the journal's head as it stands
	// now — the resolution above may have appended state records, and the
	// mark must name the history that produced these rows.
	seq, hash, err := m.journal.Head()
	if err != nil {
		return out, fmt.Errorf("record baseline chain mark: %w", err)
	}
	if err := second.observedAt.Validate(); err != nil {
		return out, fmt.Errorf("record baseline observed instant: %w", err)
	}
	doc := baseline.Document{
		Kind:             baseline.Kind,
		Version:          baseline.Version,
		Venue:            string(m.venue),
		Account:          m.account,
		CredentialSHA256: deps.Fingerprint,
		ObservedAt:       second.observedAt.Instant().UTC().Format(time.RFC3339),
		ChainSeq:         seq,
	}
	if seq > 0 {
		doc.ChainHash = hex.EncodeToString(hash[:])
	}
	for _, asset := range coverage {
		evidence := second.evidence[asset]
		row := baseline.BalanceRow{Asset: string(asset), Absent: evidence.absent}
		for _, w := range evidence.words {
			row.Words = append(row.Words, baseline.Word{Name: w.Name, Value: w.Value})
		}
		doc.Balances = append(doc.Balances, row)
	}
	for _, s := range spots {
		doc.Orders = append(doc.Orders, s.row)
	}

	if err := baseline.Write(path, doc); err != nil {
		return out, err
	}
	body, err := baseline.Encode(doc)
	if err != nil {
		return out, err
	}
	digest := sha256.Sum256(body)
	pin := baseline.Pin{Path: path, SHA256: hex.EncodeToString(digest[:])}

	// The number the operator is told is the number every future boot will
	// derive: the written file goes through the SAME verifying loader the
	// boot uses, ladder and all, before anything is reported recorded.
	loaded, err := baseline.Load(pin, m.journal, baseline.Bindings{
		Venue:       m.venue,
		Account:     m.account,
		Fingerprint: deps.Fingerprint,
		Dialect:     deps.Dialect,
	})
	if err != nil {
		return out, fmt.Errorf("record baseline verify-back: %w", err)
	}
	prior := m.baselines
	var priorErr error
	if deps.PriorPin != nil {
		loadedPrior, err := baseline.Load(*deps.PriorPin, m.journal, baseline.Bindings{
			Venue:       m.venue,
			Account:     m.account,
			Fingerprint: deps.Fingerprint,
			Dialect:     deps.Dialect,
		})
		if err != nil {
			priorErr = err
		} else {
			prior = loadedPrior
		}
	}
	absorbed, err := absorbedDeltas(prior, loaded)
	if err != nil {
		return out, fmt.Errorf("record baseline absorbed delta: %w", err)
	}
	return RecordedBaseline{
		Path: pin.Path, SHA256: pin.SHA256, Document: doc,
		Baselines: loaded, Absorbed: absorbed, PriorError: priorErr,
	}, nil
}

// absorbedDeltas compares the prior configured baselines against the newly
// recorded ones, per asset over the union — an asset leaving the coverage or
// entering it still shows its movement, against zero.
func absorbedDeltas(prior, next []application.SpotBaseline) ([]AbsorbedDelta, error) {
	if len(prior) == 0 {
		return nil, nil
	}
	assets := make(map[fixedpoint.Asset]struct{}, len(prior)+len(next))
	oldBy := make(map[fixedpoint.Asset]fixedpoint.Quantity, len(prior))
	newBy := make(map[fixedpoint.Asset]fixedpoint.Quantity, len(next))
	for _, b := range prior {
		oldBy[b.Asset] = b.Amount
		assets[b.Asset] = struct{}{}
	}
	for _, b := range next {
		newBy[b.Asset] = b.Amount
		assets[b.Asset] = struct{}{}
	}
	sorted := make([]fixedpoint.Asset, 0, len(assets))
	for asset := range assets {
		sorted = append(sorted, asset)
	}
	sort.Slice(sorted, func(a, b int) bool { return sorted[a] < sorted[b] })

	out := make([]AbsorbedDelta, 0, len(sorted))
	for _, asset := range sorted {
		old, has := oldBy[asset]
		if !has {
			old = fixedpoint.ZeroQuantity(asset)
		}
		next, has := newBy[asset]
		if !has {
			next = fixedpoint.ZeroQuantity(asset)
		}
		delta, err := next.Sub(old)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", asset, err)
		}
		out = append(out, AbsorbedDelta{Asset: asset, Prior: old, New: next, Delta: delta})
	}
	return out, nil
}

// recordEvidence is one asset's balance reading: the venue's words, or its
// silence, held for the unchanged-across-the-window comparison and the
// document rows.
type recordEvidence struct {
	absent bool
	words  []ports.VerbatimField
}

// recordEvidenceView is one balance read's answer for the whole coverage set,
// with the read's venue instant (one read, one instant).
type recordEvidenceView struct {
	evidence   map[fixedpoint.Asset]recordEvidence
	observedAt clock.ServerTime
}

// recordBalanceEvidence reads the wallet ONCE and answers for every coverage
// asset: words, absence, or the refusal an unpinned row forces — a baseline
// is an attestation, and attesting an ambiguous B stores the ambiguity as
// fact (§70d).
func (m *Module) recordBalanceEvidence(adapter *driven.SpotBalances, coverage []fixedpoint.Asset) (recordEvidenceView, error) {
	report, err := adapter.Balances(ports.SpotBalanceScope{Venue: m.venue})
	if err != nil {
		return recordEvidenceView{}, fmt.Errorf("record baseline balance read: %w", err)
	}
	if err := report.Absence.Validate(); err != nil {
		return recordEvidenceView{}, fmt.Errorf("record baseline balance read: %w", err)
	}
	view := recordEvidenceView{
		evidence:   make(map[fixedpoint.Asset]recordEvidence, len(coverage)),
		observedAt: report.ObservedAt,
	}
	for _, asset := range coverage {
		unpinned := false
		var reason ports.UnpinnedReason
		for _, u := range report.Unpinned {
			if u.Asset == string(asset) {
				unpinned, reason = true, u.Reason
				break
			}
		}
		if unpinned {
			return recordEvidenceView{}, fmt.Errorf("balance of %s is unpinned (%s): %w",
				asset, reason, ErrRecordBalanceUnpinned)
		}
		found := false
		for _, row := range report.Rows {
			if row.Asset != asset {
				continue
			}
			view.evidence[asset] = recordEvidence{words: row.Words}
			found = true
			break
		}
		if !found {
			if report.Absence != ports.AbsenceIsZero {
				// Only a report whose silence is the venue's own zero may have
				// that silence stored as evidence. Under AbsenceInconclusive
				// the documented shape contains every asset, so a missing
				// coverage asset is an anomaly — and a baseline is an
				// attestation, so attesting a quantity the venue never stated
				// would store missing evidence as fact (§70i).
				return recordEvidenceView{}, fmt.Errorf("balance of %s is absent from a read whose silence concludes nothing: %w",
					asset, ErrRecordBalanceAbsent)
			}
			view.evidence[asset] = recordEvidence{absent: true}
		}
	}
	return view, nil
}

// balanceEvidenceEqual reports whether two reads said the same thing about
// one asset — same absence, same words, verbatim and in order.
func balanceEvidenceEqual(a, b recordEvidence) bool {
	if a.absent != b.absent || len(a.words) != len(b.words) {
		return false
	}
	for i := range a.words {
		if a.words[i] != b.words[i] {
			return false
		}
	}
	return true
}

// The record gauntlet's refusals. Each is its own sentinel because §70h item
// 8's claim is behavioural: distinct refusals on distinct fixtures, or the
// gauntlet collapses into "something refused". Item 8 enumerated four; the
// roster has grown past them and the rule is what carries, not the count.
var (
	ErrRecordFingerprintUnset  = errors.New("oms: record baseline needs the transport credential fingerprint")
	ErrRecordSettleUnwired     = errors.New("oms: record baseline needs the settle bracket (spot_settle and a wait)")
	ErrRecordBalancesUnwired   = errors.New("oms: record baseline needs the spot balance client")
	ErrRecordNoSpotInstruments = errors.New("oms: record baseline over a deployment with no spot instruments")
	ErrRecordSpotOrderOpen     = errors.New("oms: record baseline refused — a spot order is not terminal")
	ErrRecordSweepIncomplete   = errors.New("oms: record baseline refused — an executions sweep did not conclude")
	ErrRecordUnattributed      = errors.New("oms: record baseline refused — an execution the cross-check could not attribute is real exposure with no owner")
	ErrRecordContradicted      = errors.New("oms: record baseline refused — the venue and this system report lifecycle states SM §1 has no edge between, so which record the baseline describes is not settled")
	ErrRecordAbsenceUnproved   = errors.New("oms: record baseline refused — an absent order §5.3 has not concluded about")
	ErrRecordProjectionStale   = errors.New("oms: record baseline refused — a resolved state did not reach the projection")
	ErrRecordBalanceUnpinned   = errors.New("oms: record baseline refused — a coverage asset's balance could not be pinned this read")
	ErrRecordBalanceAbsent     = errors.New("oms: record baseline refused — a coverage asset is absent from a read whose silence concludes nothing")
	ErrRecordCumulativeMoved   = errors.New("oms: record baseline refused — an order surface moved across the settle window")
	ErrRecordBalanceMoved      = errors.New("oms: record baseline refused — the balance moved across the settle window")
)
