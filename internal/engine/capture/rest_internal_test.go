package capture

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"pgregory.net/rapid"
)

// The two outcomes §37a admits, drawn as one: a venue answer (status, no
// error) or a transport failure (no status, error words). The draw never
// produces the conflict shapes — those are TestRestMetaValidate's, where they
// are named individually rather than hoped for.
func drawRestMeta(rt *rapid.T) RestMeta {
	m := RestMeta{
		Method: rapid.StringMatching(`[A-Z]{3,7}`).Draw(rt, "method"),
		URL:    "https://" + rapid.StringMatching(`[a-z]{1,12}\.example`).Draw(rt, "host") + "/" + rapid.StringMatching(`[a-zA-Z0-9/._-]{0,40}`).Draw(rt, "path"),
		// Year stays within time.Time's JSON-marshalable range; zero excluded.
		AskedAt: time.Unix(0, rapid.Int64Range(1, 4_000_000_000_000_000_000).Draw(rt, "askedNs")).UTC(),
	}
	if rapid.Bool().Draw(rt, "failed") {
		m.Error = rapid.StringMatching(`[ -~]{1,60}`).Draw(rt, "error")
	} else {
		m.Status = rapid.IntRange(100, 599).Draw(rt, "status")
	}
	return m
}

func TestRestPayloadRoundTrip(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		in := RestPayload{
			Meta: drawRestMeta(rt),
			Body: rapid.SliceOfN(rapid.Byte(), 0, 1<<10).Draw(rt, "body"),
		}
		enc, err := EncodeRestPayload(in)
		if err != nil {
			t.Fatalf("encode: %v", err)
		}
		out, err := DecodeRestPayload(enc)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if out.Meta.Method != in.Meta.Method || out.Meta.URL != in.Meta.URL ||
			out.Meta.Status != in.Meta.Status || out.Meta.Error != in.Meta.Error {
			t.Fatalf("meta round-trip: got %+v, want %+v", out.Meta, in.Meta)
		}
		if !out.Meta.AskedAt.Equal(in.Meta.AskedAt) {
			t.Fatalf("asked_at round-trip: got %v, want %v", out.Meta.AskedAt, in.Meta.AskedAt)
		}
		if !bytes.Equal(out.Body, in.Body) {
			t.Fatalf("body round-trip: got %d bytes, want %d", len(out.Body), len(in.Body))
		}
	})
}

// The length prefix's byte order is pinned by a hand-assembled payload, not by
// the round trip — an encoder and decoder that agree on the WRONG endianness
// round-trip perfectly. The pin works because the meta is far shorter than
// 2^24: its big-endian prefix read little-endian is a length in the millions,
// which the truncation check refuses, so a wrong-endian decoder cannot pass.
func TestDecodeRestPayloadPrefixIsBigEndian(t *testing.T) {
	meta, err := json.Marshal(RestMeta{
		Method: "GET", URL: "https://fapi.binance.com/fapi/v1/fundingInfo",
		Status: 200, AskedAt: time.Unix(1_753_866_000, 0).UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	payload := binary.BigEndian.AppendUint32(nil, uint32(len(meta)))
	payload = append(payload, meta...)
	payload = append(payload, []byte(`[]`)...)

	got, err := DecodeRestPayload(payload)
	if err != nil {
		t.Fatalf("decode hand-assembled payload: %v", err)
	}
	if string(got.Body) != `[]` {
		t.Fatalf("body: got %q, want %q", got.Body, `[]`)
	}
}

func TestRestMetaValidate(t *testing.T) {
	asked := time.Unix(1_753_866_000, 0).UTC()
	ok := RestMeta{Method: "GET", URL: "https://h/x", Status: 200, AskedAt: asked}

	cases := []struct {
		name string
		mut  func(m *RestMeta)
		want error
	}{
		{"venue answer is valid", func(m *RestMeta) {}, nil},
		{"transport failure is valid", func(m *RestMeta) { m.Status = 0; m.Error = "dial tcp: refused" }, nil},
		{"status floor 100 is in range", func(m *RestMeta) { m.Status = 100 }, nil},
		{"status ceiling 599 is in range", func(m *RestMeta) { m.Status = 599 }, nil},
		{"zero value", func(m *RestMeta) { *m = RestMeta{} }, ErrRestMethodUnset},
		{"method unset", func(m *RestMeta) { m.Method = "" }, ErrRestMethodUnset},
		{"url unset", func(m *RestMeta) { m.URL = "" }, ErrRestURLUnset},
		{"asked_at unset", func(m *RestMeta) { m.AskedAt = time.Time{} }, ErrRestAskedAtUnset},
		{"status 99 is below range", func(m *RestMeta) { m.Status = 99 }, ErrRestStatusRange},
		{"status 600 is above range", func(m *RestMeta) { m.Status = 600 }, ErrRestStatusRange},
		{"status negative", func(m *RestMeta) { m.Status = -1 }, ErrRestStatusRange},
		{"both outcomes", func(m *RestMeta) { m.Error = "words" }, ErrRestOutcomeConflict},
		{"neither outcome", func(m *RestMeta) { m.Status = 0 }, ErrRestOutcomeConflict},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := ok
			tc.mut(&m)
			err := m.Validate()
			if tc.want == nil {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("Validate() = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestEncodeRestPayloadRefusesInvalidMeta(t *testing.T) {
	_, err := EncodeRestPayload(RestPayload{})
	if !errors.Is(err, ErrRestMethodUnset) {
		t.Fatalf("EncodeRestPayload(zero) = %v, want %v", err, ErrRestMethodUnset)
	}
}

func TestDecodeRestPayloadRefusesDamage(t *testing.T) {
	valid, err := EncodeRestPayload(RestPayload{
		Meta: RestMeta{Method: "GET", URL: "https://h/x", Status: 200,
			AskedAt: time.Unix(1_753_866_000, 0).UTC()},
		Body: []byte(`[]`),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("shorter than the prefix", func(t *testing.T) {
		for n := 0; n < restMetaLenSize; n++ {
			if _, err := DecodeRestPayload(valid[:n]); !errors.Is(err, ErrRestTruncated) {
				t.Fatalf("DecodeRestPayload(%d bytes) = %v, want %v", n, err, ErrRestTruncated)
			}
		}
	})

	t.Run("meta length one past the payload end", func(t *testing.T) {
		// The boundary where > and >= part ways: metaLen equal to the
		// remaining bytes is a LEGAL payload (its body is empty), so the
		// refusing fixture must overrun by exactly one.
		short := valid[:len(valid)-len(`[]`)] // strip the body: metaLen == len(rest)
		if _, err := DecodeRestPayload(short); err != nil {
			t.Fatalf("metaLen == remaining must decode (empty body), got %v", err)
		}
		over := append([]byte(nil), short...)
		binary.BigEndian.PutUint32(over, binary.BigEndian.Uint32(over)+1)
		if _, err := DecodeRestPayload(over); !errors.Is(err, ErrRestTruncated) {
			t.Fatalf("DecodeRestPayload(metaLen+1) = %v, want %v", err, ErrRestTruncated)
		}
	})

	t.Run("meta is not JSON", func(t *testing.T) {
		bad := binary.BigEndian.AppendUint32(nil, 3)
		bad = append(bad, []byte(`{"x`)...)
		if _, err := DecodeRestPayload(bad); !errors.Is(err, ErrRestMetaInvalid) {
			t.Fatalf("DecodeRestPayload(bad json) = %v, want %v", err, ErrRestMetaInvalid)
		}
	})

	t.Run("meta parses but fails validation", func(t *testing.T) {
		meta := []byte(`{"method":"GET","url":"https://h/x","status":42,"asked_at":"2026-07-30T09:00:00Z"}`)
		bad := binary.BigEndian.AppendUint32(nil, uint32(len(meta)))
		bad = append(bad, meta...)
		if _, err := DecodeRestPayload(bad); !errors.Is(err, ErrRestStatusRange) {
			t.Fatalf("DecodeRestPayload(status 42) = %v, want %v", err, ErrRestStatusRange)
		}
	})

	t.Run("decoded body aliases the input", func(t *testing.T) {
		got, err := DecodeRestPayload(valid)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(string(valid), string(got.Body)) {
			t.Fatalf("body %q is not the payload's tail", got.Body)
		}
	})
}
