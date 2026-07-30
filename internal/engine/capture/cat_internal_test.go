package capture

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// §38d's render tool, tested against the same archiveBuilder the verify suite
// trusts: the builder writes what it is told, so the fixture can hold every
// record kind the writer can emit in ONE file, in a known order, with known
// header fields — and the assertions read cat's actual stdout bytes, because
// cat's one product IS those bytes.
//
// Fixture-collapse notes, applied at design time rather than found later:
//   - the JSON data payload carries `<` and `&`, because a payload without
//     them renders identically under SetEscapeHTML(true) and (false) — the
//     escaping claim would be vacuously covered;
//   - the verbatim claim is asserted by RAW substring over the output line,
//     not by re-parsing, because a parser accepts the escaped form too and
//     would collapse "verbatim" into "equivalent";
//   - the binary payload is non-empty AND invalid JSON — empty would collapse
//     into the alive row (neither field), valid JSON into the verbatim row;
//   - data seqs are 7 and 9, distinct from their line indexes and from each
//     other, so "echoed the record's seq" cannot collapse into "counted";
//   - the corrupt frame has a VALID record after it, because a corrupt LAST
//     record cannot distinguish "stopped at the fault" from "reached the end".

// catLines runs Cat, requires it to succeed, and splits its NDJSON output.
// Each returned element is one complete line, newline stripped.
func catLines(t *testing.T, path string) []string {
	t.Helper()
	var out bytes.Buffer
	if err := Cat(&out, path); err != nil {
		t.Fatalf("Cat(%s): %v", path, err)
	}
	raw := out.String()
	if raw == "" {
		return nil
	}
	if !strings.HasSuffix(raw, "\n") {
		t.Fatalf("output does not end in a newline: %q", raw)
	}
	return strings.Split(strings.TrimSuffix(raw, "\n"), "\n")
}

// parseLine decodes one output line into raw fields, so a test can assert a
// field is ABSENT — unmarshalling into catLine would resurrect absent fields
// as zero values and collapse "omitted" into "empty".
func parseLine(t *testing.T, line string) map[string]json.RawMessage {
	t.Helper()
	m := map[string]json.RawMessage{}
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		t.Fatalf("line is not valid JSON: %v\n%s", err, line)
	}
	return m
}

func writeFixture(t *testing.T, buf []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture.arbcap")
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

// TestCatRendersEveryKind is §38d's roster in one file: every kind the writer
// can emit, each with the rendering decision that kind exists to exercise.
func TestCatRendersEveryKind(t *testing.T) {
	window := fixtureWindow(t, 1)
	b := newArchive(t, t.TempDir(), perpKey(), window)

	const jsonPayload = `{"px":"31.5","note":"a<b&c"}`
	binaryPayload := []byte{0x00, 0x01, 0xfe, 0xff}
	const restBody = `[{"symbol":"BTCUSDT","adjustedFundingRateCap":"0.02"}]`
	const restURL = "https://fapi.binance.com/fapi/v1/fundingInfo?a=1&b=2"

	b.open(0) // line 0
	b.lastSeq = 7
	b.emit(RecordKindData, 7, []byte(jsonPayload)) // line 1: JSON, verbatim
	b.lastSeq = 9
	b.emit(RecordKindData, 9, binaryPayload)                     // line 2: not JSON, base64
	b.alive()                                                    // line 3: no payload at all
	b.gap(Gap{Cause: CauseOverflow, From: 10, To: 11, Count: 2}) // line 4
	restOK, err := EncodeRestPayload(RestPayload{Meta: RestMeta{
		Method: "GET", URL: restURL, Status: 200, AskedAt: window,
	}, Body: []byte(restBody)})
	if err != nil {
		t.Fatalf("encode rest payload: %v", err)
	}
	b.emit(RecordKindRest, b.lastSeq, restOK) // line 5: meta+body split
	restFail, err := EncodeRestPayload(RestPayload{Meta: RestMeta{
		Method: "GET", URL: restURL,
		Error: "dial tcp: connection refused", AskedAt: window,
	}})
	if err != nil {
		t.Fatalf("encode failed-ask payload: %v", err)
	}
	b.emit(RecordKindRest, b.lastSeq, restFail) // line 6: status 0, NO body
	b.emit(RecordKindClockStep, b.lastSeq, b.marshal(clockStepPayload{
		BackstepCount: 1, BackstepTotalNs: 250,
		ClampedNow: window.Format(time.RFC3339Nano),
	})) // line 7
	b.seal() // line 8: close

	lines := catLines(t, writeFixture(t, b.buf))
	if got, want := uint64(len(lines)), b.records; got != want {
		t.Fatalf("%d lines, want %d (one per record)", got, want)
	}

	// Every line echoes its record's header: kind token, seq, and the
	// builder's minute-grid clock — RecvTime from the wall seam, Mono from
	// the monotonic one. The grid makes each line's pair unique, so an echo
	// swapped between records cannot pass.
	wantKinds := []string{"open", "data", "data", "alive", "gap", "rest", "rest", "clock_step", "close"}
	wantSeqs := []uint64{0, 7, 9, 9, 9, 9, 9, 9, 9}
	for i, line := range lines {
		m := parseLine(t, line)
		var kind string
		var seq uint64
		var recv string
		var mono int64
		mustField(t, m, "kind", &kind)
		mustField(t, m, "seq", &seq)
		mustField(t, m, "recv_time", &recv)
		mustField(t, m, "mono_ns", &mono)
		if kind != wantKinds[i] {
			t.Errorf("line %d kind = %q, want %q", i, kind, wantKinds[i])
		}
		if seq != wantSeqs[i] {
			t.Errorf("line %d seq = %d, want %d", i, seq, wantSeqs[i])
		}
		wantRecv := window.Add(time.Duration(i+1) * time.Minute).Format(time.RFC3339Nano)
		if recv != wantRecv {
			t.Errorf("line %d recv_time = %q, want %q", i, recv, wantRecv)
		}
		if want := int64(i) * int64(time.Minute); mono != want {
			t.Errorf("line %d mono_ns = %d, want %d", i, mono, want)
		}
	}

	// Line 1: valid JSON rides VERBATIM — the raw substring check is the
	// whole point, and it also proves SetEscapeHTML(false), because the
	// payload's `<` and `&` would otherwise print as < and &.
	if !strings.Contains(lines[1], jsonPayload) {
		t.Errorf("JSON payload not embedded verbatim:\n%s", lines[1])
	}
	if _, ok := parseLine(t, lines[1])["payload_b64"]; ok {
		t.Errorf("JSON payload also rendered as base64:\n%s", lines[1])
	}

	// Line 2: not JSON, so base64 under a name that says so — and it decodes
	// back to the archived bytes exactly.
	m2 := parseLine(t, lines[2])
	if _, ok := m2["payload"]; ok {
		t.Errorf("binary payload rendered as JSON:\n%s", lines[2])
	}
	var b64 string
	mustField(t, m2, "payload_b64", &b64)
	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("payload_b64 does not decode: %v", err)
	}
	if !bytes.Equal(decoded, binaryPayload) {
		t.Errorf("payload_b64 round-trip = %x, want %x", decoded, binaryPayload)
	}

	// Line 3: an alive record has no body, so NEITHER field appears —
	// "empty" and "omitted" must not be conflated on the way out.
	m3 := parseLine(t, lines[3])
	if _, ok := m3["payload"]; ok {
		t.Errorf("alive line carries a payload field:\n%s", lines[3])
	}
	if _, ok := m3["payload_b64"]; ok {
		t.Errorf("alive line carries a payload_b64 field:\n%s", lines[3])
	}

	// Line 4: a gap payload is our own JSON and renders like any other.
	if !strings.Contains(lines[4], `"cause":"overflow"`) {
		t.Errorf("gap payload not rendered verbatim:\n%s", lines[4])
	}

	// Line 5: the §37a frame is DECODED — meta and body as separate fields,
	// the body verbatim, the meta's URL unescaped.
	m5 := parseLine(t, lines[5])
	var meta5 RestMeta
	mustField(t, m5, "meta", &meta5)
	if meta5.Status != 200 || meta5.URL != restURL {
		t.Errorf("meta = %+v, want status 200 and the asked URL", meta5)
	}
	if !strings.Contains(lines[5], restBody) {
		t.Errorf("rest body not embedded verbatim:\n%s", lines[5])
	}
	if !strings.Contains(lines[5], restURL) {
		t.Errorf("meta URL escaped or lost:\n%s", lines[5])
	}
	if _, ok := m5["body_b64"]; ok {
		t.Errorf("JSON rest body also rendered as base64:\n%s", lines[5])
	}
	if _, ok := m5["payload"]; ok {
		t.Errorf("rest line leaks the undecoded payload field:\n%s", lines[5])
	}

	// Line 6: a transport failure has meta.error, no status, and NO body of
	// either spelling — there was no response to render.
	m6 := parseLine(t, lines[6])
	var meta6 RestMeta
	mustField(t, m6, "meta", &meta6)
	if meta6.Error != "dial tcp: connection refused" || meta6.Status != 0 {
		t.Errorf("failed-ask meta = %+v, want the transport's words and no status", meta6)
	}
	if _, ok := m6["body"]; ok {
		t.Errorf("failed ask carries a body field:\n%s", lines[6])
	}
	if _, ok := m6["body_b64"]; ok {
		t.Errorf("failed ask carries a body_b64 field:\n%s", lines[6])
	}

	// Line 8: the close payload renders verbatim like any of ours, and its
	// digest claim is visible to a grep — the six-week-old shell pipeline is
	// the stated consumer.
	if !strings.Contains(lines[8], `"sha256":"`) {
		t.Errorf("close payload not rendered:\n%s", lines[8])
	}
}

// mustField requires key to be present and to unmarshal into out.
func mustField(t *testing.T, m map[string]json.RawMessage, key string, out any) {
	t.Helper()
	raw, ok := m[key]
	if !ok {
		t.Fatalf("field %q absent", key)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		t.Fatalf("field %q: %v", key, err)
	}
}

// TestCatStopsAtTheFirstCorruptFrame is the §38d refusal: everything before
// the fault is on stdout as complete lines, then an error naming path and
// byte offset — and nothing AFTER the fault, which is why the fixture puts a
// valid record behind the corrupt one. The intact run over the same bytes is
// the positive control; without it, "stopped at the fault" is
// indistinguishable from a tool that always stops early.
func TestCatStopsAtTheFirstCorruptFrame(t *testing.T) {
	b := newArchive(t, t.TempDir(), perpKey(), fixtureWindow(t, 1))
	b.open(0)
	b.data(1)
	b.data(2)
	corruptAt := len(b.buf) // the 4th record starts here
	b.data(3)
	corruptEnd := len(b.buf)
	b.data(4) // valid, and must NOT be rendered

	intact := writeFixture(t, b.buf)
	if got := len(catLines(t, intact)); got != 5 {
		t.Fatalf("positive control: %d lines from the intact file, want 5", got)
	}

	corrupted := append([]byte(nil), b.buf...)
	corrupted[corruptEnd-1] ^= 0xff // last byte of the 4th record's frame
	path := writeFixture(t, corrupted)

	var out bytes.Buffer
	err := Cat(&out, path)
	if err == nil {
		t.Fatalf("Cat rendered a corrupt frame:\n%s", out.String())
	}
	if want := fmt.Sprintf("%s+%d", path, corruptAt); !strings.Contains(err.Error(), want) {
		t.Errorf("error %q does not name %q", err, want)
	}

	raw := out.String()
	if !strings.HasSuffix(raw, "\n") {
		t.Fatalf("output before the fault is not complete lines: %q", raw)
	}
	lines := strings.Split(strings.TrimSuffix(raw, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("%d lines before the fault, want 3 (open, data 1, data 2)", len(lines))
	}
	for _, line := range lines {
		parseLine(t, line) // each flushed line is complete JSON, no torn tail
	}
	if !strings.Contains(lines[2], `{"seq":2}`) {
		t.Errorf("last complete line is not data 2:\n%s", lines[2])
	}
}

// TestCatRefusesANonArchiveFile: not our magic, no output, ErrMagic — the
// wrap the cmd layer's exit code and the operator's eye both key on.
func TestCatRefusesANonArchiveFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(path, []byte("not an arbcap archive\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	var out bytes.Buffer
	err := Cat(&out, path)
	if !errors.Is(err, ErrMagic) {
		t.Fatalf("Cat(non-archive) = %v, want ErrMagic", err)
	}
	if out.Len() != 0 {
		t.Errorf("refused file still produced output: %q", out.String())
	}
}
