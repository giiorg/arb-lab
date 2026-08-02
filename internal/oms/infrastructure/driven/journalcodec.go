package driven

import (
	"crypto/sha256"
	"encoding/binary"

	"github.com/giiorg/arb-lab/internal/oms/domain"
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
// # There is no timestamp in a record, and the absence is deliberate
//
// §5.1 orders the journal by its sequence, not by a clock, and §7.5 replays it
// in that order. A wall-clock field would need platform/clock injected into an
// adapter that otherwise has no time in it, and it would add a value the chain
// covers but nothing reads — an attractive nuisance for a later reader who
// mistakes it for the ordering authority. When an event time is genuinely
// needed it is the venue's, and it arrives on a fill.

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
// two record shapes must not be able to collide even if their remaining fields
// serialise identically. They cannot today — a planned record has strictly more
// fields — but "cannot today" is a property of the current field lists, not of
// the encoding, and the field lists are what a later slice edits.
const (
	kindTagPlanned uint8 = 1
	kindTagState   uint8 = 2
)

// Kind names, as they appear in the kind column.
//
// Text rather than the integer tags above, for §3.6 rule 8's reason applied to
// a stored value: an integer encoding renumbers every existing row when a kind
// is inserted, and the corruption reads as a planned record that became a state
// record rather than as a crash.
const (
	kindPlanned = "planned"
	kindState   = "state"
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
