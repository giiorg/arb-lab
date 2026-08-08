package driven

import (
	"crypto/sha256"
	"encoding/binary"

	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// The journal's canonical encoding and its chain rule.
//
// This file holds what a record HASHES AS, which is a separate question from
// what it is STORED AS — journal.go owns the columns. Keeping them apart is the
// decision that makes the oracle in journalcodec_internal_test.go worth having:
// the stored form is a set of typed columns, so a replayer has to REBUILD the
// preimage from semantic values rather than re-hash a blob the writer handed
// it. An implementation checked against its own output is not checked.
//
// # Every variable-length field is length-prefixed, and that is the whole point
//
// A hash over concatenated fields is a hash over their concatenation, not over
// the fields. Venue "BTC" with symbol "USDT" and venue "BTCU" with symbol "SDT"
// produce identical bytes unprefixed, so two different orders share a chain
// entry and the tamper-evidence the chain exists for is gone for that pair.
//
// This repo has already paid for the lesson one layer out: observability's
// series-key collision fixture used ("ab","c") vs ("a","bc") and contained NO
// separator, so it collided only under the EMPTY separator and passed against
// every wrong choice a person would actually make. The fixture here therefore
// varies the split point across fields that are both non-empty on both sides.
//
// # There is no timestamp on a record, and there IS one inside exactly one
// # record kind — the distinction is the whole of what slice A got right and
// # what it got wrong
//
// Slice A refused a timestamp COLUMN and gave two reasons: §5.1 orders the
// journal by its sequence rather than by a clock, so a wall-clock field would
// be "a value the chain covers but nothing reads" and an attractive nuisance
// for a reader who mistook it for the ordering authority; and "when an event
// time is genuinely needed it is the venue's, and it arrives on a fill".
//
// The first reason still holds and is why there is still no per-record
// timestamp: nothing here records WHEN a record was written. The second expired
// one slice later, and expired in the way a rejection's reason does — quietly,
// with no clause anywhere for a grep to find. §5.3's stable-not-found rule
// measures both of its time bounds from the instant this system authorized the
// single Place, and the order it resolves that way is one with NO FILL and no
// venue record at all: there is no fill for the venue's time to arrive on. So
// the anchor is a field OF one record kind, in the venue's own clock frame,
// carried because the rule reads it — which is the test the first reason
// states, met rather than contradicted.
//
// See ports.OrderRecord.AuthorizedAt for the consequence of not having it: a
// crash-recovered order at StateDispatchAuthorized with no anchor cannot reach
// StateAbsentAfterFinality by any evidence, so it holds its hedge group
// ambiguous for ever with every light green.

const (
	// chainLabel domain-separates this hash chain from every other one in the
	// system. Without it, a digest computed over some other structure that
	// happens to serialise identically is a valid link here — and §5.1 puts a
	// hash chain on the audit projection too, so "some other structure" is not
	// hypothetical.
	//
	// The version suffix is load-bearing: a change to the encoding below must
	// change this string, so that records written by two encodings can never
	// verify as one chain. A chain that silently spans two formats reports
	// tampering at the boundary or, worse, does not.
	chainLabel = "arb-lab/oms/journal/chain/v1"

	// genesisLabel seeds the first record's prev_hash.
	//
	// A constant rather than 32 zero bytes, because zero is what an
	// uninitialised buffer holds: under a zero seed, a record whose prev_hash
	// column was never written verifies as the first record of a chain. The
	// seed is derived from chainLabel so the two cannot drift apart.
	genesisLabel = chainLabel + "/genesis"
)

// Record kind tags, as they appear in the hashed preimage.
//
// A tag inside the preimage rather than only in the kind column, because the
// record shapes must not be able to collide even if their remaining fields
// serialise identically. When there were two kinds that was a precaution about
// a future edit — a planned record has strictly more fields than a state one,
// so nothing could collide as the format then stood. The third kind is the edit
// the precaution was for: an authorization is a client id followed by sixteen
// fixed bytes, and a state record is a client id followed by a length-prefixed
// string, so a state whose text happened to encode as those sixteen bytes would
// hash identically to an authorization of the same order. The tag is what
// separates them, and it is now load-bearing rather than defensive.
// The generation kinds make it load-bearing four more times, and this is the
// first set where the collision is between two records of the SAME machine. A
// perp-leg submission and a spot-leg submission are both "a generation id
// followed by one length-prefixed client order id" — byte-identical shapes,
// distinguished by nothing but the tag — and the port declares them as separate
// methods precisely because the two are not interchangeable to any reader of
// this log. Sharing a tag would undo that at the only layer where it is checked.
// The superseded and phase records are the same shape again, so four of the six
// generation kinds are separated by the tag alone.
const (
	kindTagPlanned    uint8 = 1
	kindTagState      uint8 = 2
	kindTagAuthorized uint8 = 3
	kindTagAcked      uint8 = 4

	kindTagGenerationOpened        uint8 = 5
	kindTagGenerationPerpSubmitted uint8 = 6
	kindTagGenerationAuthorized    uint8 = 7
	kindTagGenerationSpotSubmitted uint8 = 8
	kindTagGenerationSuperseded    uint8 = 9
	kindTagGenerationPhase         uint8 = 10

	kindTagMarginOpDispatched uint8 = 11
	kindTagMarginOpResolved   uint8 = 12
)

// Kind names, as they appear in the kind column.
//
// Text rather than the integer tags above, for §3.6 rule 8's reason applied to
// a stored value: an integer encoding renumbers every existing row when a kind
// is inserted, and the corruption reads as a planned record that became a state
// record rather than as a crash.
//
// # The kind column is also the SUBJECT COLUMN'S NAMESPACE, and that is new
//
// Slice A's `client_id` column held one kind of thing, so the kind was purely
// descriptive. The generation records share the chain (see journal.go's schema
// comment) and their subject is a domain.GenerationID, which is a different id
// space from ids.ClientOrderID with no shared namespace — the two could spell
// the same string. So `subject` alone does not identify anything: a row is
// "generation G" only as (kind ∈ the generation kinds, subject = G).
//
// Nothing in this package looks a record up BY subject, and that is deliberate
// rather than incidental. A `WHERE subject = ?` with no kind predicate is the
// query that silently matches the other id space, and the way to make it
// unwritable is to have no method that takes a bare subject. The replay walks
// every record in sequence order and branches on the kind it already has.
const (
	kindPlanned    = "planned"
	kindState      = "state"
	kindAuthorized = "authorized"
	kindAcked      = "acked"

	kindGenerationOpened        = "generation_opened"
	kindGenerationPerpSubmitted = "generation_perp_submitted"
	kindGenerationAuthorized    = "generation_authorized"
	kindGenerationSpotSubmitted = "generation_spot_submitted"
	kindGenerationSuperseded    = "generation_superseded"
	kindGenerationPhase         = "generation_phase"

	kindMarginOpDispatched = "margin_op_dispatched"
	kindMarginOpResolved   = "margin_op_resolved"
)

// genesisHash is the prev_hash of sequence 1.
func genesisHash() [32]byte { return sha256.Sum256([]byte(genesisLabel)) }

// encodePlanned is the canonical preimage of an AppendPlanned record.
//
// Field order is domain.Order's declaration order, with the instrument expanded
// in ids.VenueInstrument's own field order. That is a decision worth stating
// because it is the kind of thing a later edit reorders for tidiness: the order
// is part of the format, and changing it changes every hash. The version suffix
// on chainLabel is what makes such a change safe to make loudly.
//
// A market order's LimitPrice is the zero Price — domain.Order.Validate refuses
// any other value on one — so its two asset tags encode as empty strings and
// its mantissa as 0. That is a canonical absent rather than a legal-looking
// zero: there is exactly one byte string an absent price can produce, because
// the type has exactly one spelling of absent.
func encodePlanned(o domain.Order) []byte {
	b := make([]byte, 0, 256)
	b = append(b, kindTagPlanned)
	b = appendString(b, string(o.ClientOrderID))
	b = appendString(b, string(o.Instrument.Venue))
	b = appendString(b, string(o.Instrument.Product))
	b = appendString(b, o.Instrument.Symbol)
	b = appendString(b, string(o.Instrument.Contract))
	b = appendString(b, string(o.Side))
	b = appendString(b, string(o.Type))
	b = appendString(b, string(o.TimeInForce))
	b = appendString(b, string(o.Quantity.Asset()))
	b = appendInt64(b, o.Quantity.Mantissa())
	b = appendString(b, string(o.LimitPrice.Base()))
	b = appendString(b, string(o.LimitPrice.Quote()))
	b = appendInt64(b, o.LimitPrice.Mantissa())
	b = appendBool(b, o.ReduceOnly)
	return b
}

// encodeState is the canonical preimage of an AppendState record.
func encodeState(id ids.ClientOrderID, state domain.OrderState) []byte {
	b := make([]byte, 0, 64)
	b = append(b, kindTagState)
	b = appendString(b, string(id))
	b = appendString(b, string(state))
	return b
}

// encodeAuthorized is the canonical preimage of an AppendAuthorized record.
//
// The state is NOT in it, and its absence is the same decision Journal made by
// giving the authorization its own method: there is exactly one state this
// record kind can mean, so encoding it would be a second spelling of the kind
// tag — two fields that must agree, with no rule for a replay that finds they
// do not.
//
// Both halves of the ServerTime are carried. The instant alone would be a
// reading whose uncertainty a replay would have to invent, and clock.ServerTime
// refuses to be built without one precisely because every boundary comparison
// §5.3 makes is three-valued: a band nobody recorded reads at replay as a
// perfectly synchronised venue, which answers Certainly to boundaries it should
// answer Uncertain to. That is the resolver refusing LATE — the one direction
// its whole design avoids.
func encodeAuthorized(id ids.ClientOrderID, at clock.ServerTime) []byte {
	b := make([]byte, 0, 64)
	b = append(b, kindTagAuthorized)
	b = appendString(b, string(id))
	b = appendInt64(b, at.Instant().UnixNano())
	b = appendInt64(b, int64(at.Uncertainty()))
	return b
}

// encodeAcked is the canonical preimage of an AppendAcked record.
//
// Structurally it is encodeState with a different tag — a client id followed
// by one length-prefixed string — which is exactly the collision the kind tag
// exists for: without it, an acked record whose venue id happened to spell a
// state name would hash identically to a state record of the same order. The
// tag was load-bearing before this kind existed (the authorized/state pair);
// this kind makes it load-bearing twice.
//
// Adding a kind changes no existing preimage, so chainLabel keeps its /v1:
// every record an older build wrote verifies unchanged, and an older build
// reading a journal WITH acked records refuses at the kind column
// (ErrJournalKindUnknown), which is the documented downgrade direction.
func encodeAcked(id ids.ClientOrderID, venueOrderID string) []byte {
	b := make([]byte, 0, 64)
	b = append(b, kindTagAcked)
	b = appendString(b, string(id))
	b = appendString(b, venueOrderID)
	return b
}

// encodeGenerationOpened is the canonical preimage of an
// AppendGenerationOpened record — SM §7's `Planned`.
//
// The hedge group is expanded into its three components rather than encoded
// through its String(), and the difference is the one appendString exists for.
// HedgeGroup.String() joins account, venue and symbol with '/', so an account
// named "a/b" on symbol "C" and an account named "a" on venue "b" would produce
// the same text — the concatenation collision, arriving through a formatter
// written for an operator's eyes. Three prefixed fields have one spelling each.
//
// The reservation is in the preimage because it is in the record, and it is in
// the record for the reason ports.GenerationJournal states: it is the handle the
// shared ledger is keyed by, so a recovered generation that could not name its
// own entry could not retire it, and an entry nobody retires subtracts from
// every later sizing for the life of the process.
func encodeGenerationOpened(
	id domain.GenerationID,
	group domain.HedgeGroup,
	reservation domain.ReservationID,
	target fixedpoint.Quantity,
) []byte {
	b := make([]byte, 0, 128)
	b = append(b, kindTagGenerationOpened)
	b = appendString(b, string(id))
	b = appendString(b, group.Account)
	b = appendString(b, string(group.Venue))
	b = appendString(b, group.Symbol)
	b = appendString(b, string(reservation))
	b = appendString(b, string(target.Asset()))
	b = appendInt64(b, target.Mantissa())
	return b
}

// encodeGenerationPerpSubmitted is the canonical preimage of the perp-leg
// submission — `PerpReduceInFlight` with the buy-to-reduce order's id.
//
// It is byte-identical in SHAPE to encodeGenerationSpotSubmitted below, and the
// tag is the only thing that separates them. That is not a latent hazard: the
// two carry the same TYPE of value about the same generation, so without the tag
// a record saying "the perp leg is order X" and one saying "the spot leg is
// order X" hash alike, and a replay could be handed either where the log stored
// the other. domain.Generation.OwnLegs is FM-9's netting quantity and
// GenerationPhase.ReservedLegLive decides whether the quiesce-first obligation
// binds — both read WHICH leg, so a swapped pair is not a cosmetic error.
func encodeGenerationPerpSubmitted(id domain.GenerationID, order ids.ClientOrderID) []byte {
	b := make([]byte, 0, 64)
	b = append(b, kindTagGenerationPerpSubmitted)
	b = appendString(b, string(id))
	b = appendString(b, string(order))
	return b
}

// encodeGenerationAuthorized is the canonical preimage of FLAG-3's record — the
// one SM §7 names in its own right, and the reason the generation port exists.
//
// The size is a quantity rather than a bare mantissa, so the asset tag rides
// along. fixedpoint's whole seam is that a mantissa without its asset is a
// number that means nothing, and here the consequence is sharper than usual: a
// replay that rebuilt the authorized size against the wrong asset would produce
// a spot cap in the wrong unit, and "no more spot than the CONFIRMED perp
// reduction" is the never-net-short invariant sized from exactly this value.
//
// A ZERO size encodes as a legal, ordinary record, and that is required rather
// than tolerated — a perp reduction that confirmed nothing authorizes no spot
// sale and the generation completes. What must not be possible is a DROPPED size
// producing the same bytes as a genuine zero, and that is why the port gives
// this its own method: absence is unrepresentable because there is no record.
func encodeGenerationAuthorized(id domain.GenerationID, size fixedpoint.Quantity) []byte {
	b := make([]byte, 0, 64)
	b = append(b, kindTagGenerationAuthorized)
	b = appendString(b, string(id))
	b = appendString(b, string(size.Asset()))
	b = appendInt64(b, size.Mantissa())
	return b
}

// encodeGenerationSpotSubmitted is the canonical preimage of the reserved spot
// reduction's submission. See encodeGenerationPerpSubmitted for why the two are
// separate kinds rather than one with a leg field.
func encodeGenerationSpotSubmitted(id domain.GenerationID, order ids.ClientOrderID) []byte {
	b := make([]byte, 0, 64)
	b = append(b, kindTagGenerationSpotSubmitted)
	b = appendString(b, string(id))
	b = appendString(b, string(order))
	return b
}

// encodeGenerationSuperseded is the canonical preimage of `Superseded` WITH the
// row that ended the generation.
//
// The path is in the preimage because it is the field whose loss is not
// symmetric. domain.SupersessionPath.ResumesLadder gives the three rows two
// different answers about whether core §7.1's ladder may open a later
// generation, and neither answer is re-derivable at boot: the external rows are
// arbitrations no order event mentions, and the premise-lost row fires on a
// quantity that leaves no trace at order scope. A path covered by the chain is a
// path a replay can trust; one merely stored beside it is not.
func encodeGenerationSuperseded(id domain.GenerationID, path domain.SupersessionPath) []byte {
	b := make([]byte, 0, 64)
	b = append(b, kindTagGenerationSuperseded)
	b = appendString(b, string(id))
	b = appendString(b, string(path))
	return b
}

// encodeGenerationPhase is the canonical preimage of the five phases that carry
// nothing but themselves.
//
// Structurally this is encodeState one machine over, and the same reasoning
// applies to what is NOT here: the source phase. The log is append-only and
// replayed in order, so the predecessor is already in it, and a replay finding
// two records disagree about where a generation came from has no rule for which
// wins.
//
// Adding these six kinds changes no existing preimage — encodePlanned and its
// three siblings hash DOMAIN VALUES, not the column those values happen to be
// stored in — so chainLabel keeps its /v1 across the subject-column migration
// journal.go performs. Every record an older build wrote verifies unchanged, and
// an older build reading a journal WITH generation records refuses at the kind
// column (ErrJournalKindUnknown), which is the documented downgrade direction.
func encodeGenerationPhase(id domain.GenerationID, phase domain.GenerationPhase) []byte {
	b := make([]byte, 0, 64)
	b = append(b, kindTagGenerationPhase)
	b = appendString(b, string(id))
	b = appendString(b, string(phase))
	return b
}

// encodeMarginOpDispatched is the canonical preimage of core §7.1's `MarginOp`.
//
// Every field of the record is in it, and for this kind that is not the usual
// completeness reflex: the delta, the before-state and the watermark are the
// three inputs of ONE comparison, and a chain covering two of them would leave
// the third alterable without the link breaking. The one whose alteration is
// silent is the before-state — a resolution against a shifted before-state does
// not fail, it CONCLUDES, and the conclusion it reaches is whichever verdict the
// shift was chosen to produce.
//
// The asset is encoded once, and the record carries it once, because
// domain.MarginOp.Validate requires the two amounts to share it. Two encoded
// copies would be two fields that must agree with no rule for a replay finding
// they do not — the shape encodeAuthorized declines for the same reason.
func encodeMarginOpDispatched(op domain.MarginOp) []byte {
	b := make([]byte, 0, 96)
	b = append(b, kindTagMarginOpDispatched)
	b = appendString(b, string(op.ID))
	b = appendString(b, string(op.Instrument.Venue))
	b = appendString(b, string(op.Instrument.Product))
	b = appendString(b, op.Instrument.Symbol)
	b = appendString(b, string(op.Instrument.Contract))
	b = appendString(b, string(op.Delta.Asset()))
	b = appendInt64(b, op.Delta.Mantissa())
	b = appendInt64(b, op.BeforePosted.Mantissa())
	b = appendInt64(b, op.Watermark.UnixNano())
	return b
}

// encodeMarginOpResolved is the canonical preimage of a margin op's verdict.
//
// Structurally encodeState again — a subject followed by one length-prefixed
// string — which is the collision the kind tag exists for, now for the fifth
// distinct pair. It matters more here than usual because the two id spaces are
// not the same: an order id and a margin-op id can spell the same string, so
// without the tag a state record for order X and a resolution for margin op X
// would hash alike.
//
// The verdict is inside the chain rather than merely stored beside it, and that
// is the property the stored-verdict ruling rests on (record §81 §2a). A verdict
// nothing covers is a verdict an edit can change, and both directions of that
// edit are silent: an `ambiguous` flipped to `applied` lifts a §9 incident, and
// an `applied` flipped to `not_applied` invites a top-up that already happened.
func encodeMarginOpResolved(id domain.MarginOpID, status domain.MarginOpStatus) []byte {
	b := make([]byte, 0, 64)
	b = append(b, kindTagMarginOpResolved)
	b = appendString(b, string(id))
	b = appendString(b, string(status))
	return b
}

// chainHash links a record to its predecessor.
//
// The sequence is inside the preimage, and it is the field a reader is most
// likely to think redundant — the row already has a seq column, and the chain
// already orders the records. It is here because two records can be
// byte-identical in every other field: the journal is a recorder, and §5.3's
// duplicate deliveries mean AppendState may legitimately record the same order
// reaching the same state twice. Without the sequence, those two records hash
// alike, and a chain segment containing both can be reordered or have one of
// them dropped with every remaining link still verifying.
//
// prev is what makes a tamper propagate. Editing record i without it
// invalidates record i alone, which is a checksum; with it, every record after
// i fails too, which is a chain. The distinction is only observable on a chain
// of at least three — see the collapse note in journal_internal_test.go.
func chainHash(seq uint64, prev [32]byte, body []byte) [32]byte {
	h := sha256.New()
	h.Write([]byte(chainLabel))
	var seqBytes [8]byte
	binary.BigEndian.PutUint64(seqBytes[:], seq)
	h.Write(seqBytes[:])
	h.Write(prev[:])
	h.Write(body)

	// body is last and every field before it is fixed-width, so no length
	// prefix is needed for it — the split is unambiguous by construction. Each
	// field WITHIN body carries its own prefix; see appendString.
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

// appendString writes a length-prefixed string.
//
// Big-endian for the reason capture's frame gives: the one time anybody reads
// these bytes by hand is when they disagree with a recomputation, and a
// big-endian length is legible in a hexdump.
//
// The length is u32 rather than a varint. A varint would save bytes on every
// field and buy an ambiguity this encoding must not have — two varint spellings
// of the same length are two preimages for one record, which is exactly the
// property a chain is supposed to exclude.
func appendString(dst []byte, s string) []byte {
	var n [4]byte
	binary.BigEndian.PutUint32(n[:], uint32(len(s)))
	dst = append(dst, n[:]...)
	return append(dst, s...)
}

// appendInt64 writes a mantissa in two's complement, big-endian.
//
// Fixed width rather than decimal text, so that a mantissa has one spelling.
// Decimal would admit "-0" and leading zeros, and fixedpoint's own persistence
// seam already establishes the int64 as the stored form (see its codec.go).
func appendInt64(dst []byte, v int64) []byte {
	var n [8]byte
	binary.BigEndian.PutUint64(n[:], uint64(v))
	return append(dst, n[:]...)
}

// appendBool writes one byte, 0 or 1.
func appendBool(dst []byte, v bool) []byte {
	if v {
		return append(dst, 1)
	}
	return append(dst, 0)
}
