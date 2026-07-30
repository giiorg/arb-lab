package capture

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"time"
)

// This file is §32h's payload framing for RecordKindRest — the half slice B
// left open (Writer.Rest: "the payload framing is slice E's"). The frame:
//
//	payload := u32 metaLen ‖ meta ‖ body
//	meta    := JSON RestMeta
//	body    := the venue's response bytes, VERBATIM
//
// The meta is JSON and the body is not, and the line between them is §31g
// rule 1's: nothing of the VENUE's is re-encoded on the way in. The meta is
// ours — the request we made, the status we got, the instant we asked — so it
// gets the same treatment as the Open record's payload: JSON, because a human
// reading a hexdump of a corrupt file should be able to see what was asked.
//
// The length prefix is big-endian like every other integer in this format,
// and for the same stated reason: the one time anybody reads these bytes as
// hex is when something is wrong.

// RestMeta is the request/response half of a Rest record: what was asked,
// when, and how the venue answered. Exactly one of Status and Error is set —
// a venue that answered has a status and no transport error; a transport
// failure has the transport's words and no status. Both present and both
// absent are refused, because each is a record claiming two incompatible
// outcomes for one ask.
type RestMeta struct {
	// Method and URL are the request, spelled fully — scheme, host and path —
	// because the archive path names the stream, not the wire, and a file
	// that has been `mv`ed still has to say what was asked (§32g's argument
	// for openPayload, one record kind over).
	Method string `json:"method"`
	URL    string `json:"url"`

	// Status is the HTTP status the venue answered with, or 0 when the
	// transport failed before any status existed. A non-2xx is a RESULT and
	// is recorded like any other: §32h's fact is "we asked and this came
	// back", and a 429 is a fact worth six weeks of hindsight.
	Status int `json:"status,omitempty"`

	// Error is the transport's own words when the ask itself failed. Never
	// branched on, same rule as Gap.Detail.
	Error string `json:"error,omitempty"`

	// AskedAt is the instant the request was issued, from the INJECTED clock.
	// Distinct from the record header's RecvTime — the writer stamps that at
	// write time, and the two differ by up to the request timeout. "We asked
	// at T" is the fact §32h exists to record, so it is stated, not inferred.
	AskedAt time.Time `json:"asked_at"`
}

// Validate reports whether the meta describes one completed ask. The zero
// value fails, per the platform rule: an unpopulated meta reads as a
// successful ask about nothing.
func (m RestMeta) Validate() error {
	if m.Method == "" {
		return ErrRestMethodUnset
	}
	if m.URL == "" {
		return ErrRestURLUnset
	}
	if m.AskedAt.IsZero() {
		return ErrRestAskedAtUnset
	}
	if m.Status != 0 && (m.Status < 100 || m.Status > 599) {
		return fmt.Errorf("status %d: %w", m.Status, ErrRestStatusRange)
	}
	if (m.Status == 0) == (m.Error == "") {
		return fmt.Errorf("status %d, error %q: %w", m.Status, m.Error, ErrRestOutcomeConflict)
	}
	return nil
}

// RestPayload is one decoded Rest record payload: the meta and the venue's
// bytes. Body may be empty — an empty JSON array IS the §32h fact for
// fundingInfo, and a transport failure has no body at all.
type RestPayload struct {
	Meta RestMeta
	Body []byte
}

// restMetaLenSize is the u32 length prefix.
const restMetaLenSize = 4

// EncodeRestPayload frames a Rest payload for Writer.Rest. It refuses a meta
// that fails Validate: the framing is the last point where a malformed ask
// can be refused with a name, rather than archived and discovered at read
// time — the writer's own default-deny, one layer in.
func EncodeRestPayload(p RestPayload) ([]byte, error) {
	if err := p.Meta.Validate(); err != nil {
		return nil, err
	}
	meta, err := json.Marshal(p.Meta)
	if err != nil {
		return nil, fmt.Errorf("capture: encode rest meta: %w", err)
	}
	out := make([]byte, restMetaLenSize, restMetaLenSize+len(meta)+len(p.Body))
	binary.BigEndian.PutUint32(out, uint32(len(meta)))
	out = append(out, meta...)
	out = append(out, p.Body...)
	return out, nil
}

// DecodeRestPayload reverses EncodeRestPayload. It validates the decoded meta
// with the same rule the encoder enforced, because the bytes may not have
// come from this encoder — a truncated or hand-damaged payload must be
// refused here, not returned as a plausible-looking ask.
//
// The returned Body aliases payload rather than copying it, matching
// DecodeRecord's contract: the caller owns the bytes it handed in.
func DecodeRestPayload(payload []byte) (RestPayload, error) {
	if len(payload) < restMetaLenSize {
		return RestPayload{}, fmt.Errorf("%d bytes: %w", len(payload), ErrRestTruncated)
	}
	metaLen := binary.BigEndian.Uint32(payload)
	rest := payload[restMetaLenSize:]
	if uint64(metaLen) > uint64(len(rest)) {
		return RestPayload{}, fmt.Errorf("meta length %d exceeds %d remaining bytes: %w",
			metaLen, len(rest), ErrRestTruncated)
	}
	var meta RestMeta
	if err := json.Unmarshal(rest[:metaLen], &meta); err != nil {
		return RestPayload{}, fmt.Errorf("capture: decode rest meta: %w: %v", ErrRestMetaInvalid, err)
	}
	if err := meta.Validate(); err != nil {
		return RestPayload{}, err
	}
	return RestPayload{Meta: meta, Body: rest[metaLen:]}, nil
}
