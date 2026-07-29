package capture

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"pgregory.net/rapid"

	"github.com/giiorg/arb-lab/internal/platform/events"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// The fixture-collapse checklist, run BEFORE the mutations per the standing rule
// (nine prior occurrences, so it is a checklist item and not a war story). What
// it changed in this file:
//
//   - Every fixture used a clean symbol like "BTCUSDT", which collapses "encodes
//     the topic" and "encodes every segment" onto the same answer — §32e's
//     wording only requires the topic. A symbol containing a separator is the
//     case that distinguishes them, and it is not hypothetical: ids validates a
//     venue symbol against WHITESPACE and nothing else, on the stated ground
//     that folding or trimming would be a claim about venues.
//   - Every fixture was a perpetual, so the optional contract segment was always
//     present and "parse branches on the product" was never exercised against
//     the branch not taken. Spot instruments carry no contract.
//   - Every rotation period was an hour, collapsing the three file-name
//     precisions onto one.
//   - The percent sign sat mid-string in every malformed fixture, which is
//     literally this repo's `i >= 0` versus `i > 0` trap (outbox, session ~24).
//     First, middle and last positions now appear separately.

func perpKey() StreamKey {
	return StreamKey{
		Source: events.SourceID{
			Instrument: ids.VenueInstrument{
				Venue:    ids.Venue("binance"),
				Product:  ids.ProductLinearPerp,
				Symbol:   "BTCUSDT",
				Contract: ids.ContractLinear,
			},
			Stream: events.StreamFunding,
		},
		Topic: "btcusdt@markPrice@1s",
	}
}

func spotKey() StreamKey {
	return StreamKey{
		Source: events.SourceID{
			Instrument: ids.VenueInstrument{
				Venue:   ids.Venue("bybit"),
				Product: ids.ProductSpot,
				Symbol:  "BTCUSDT",
			},
			Stream: events.StreamBook,
		},
		Topic: "orderbook.50.BTCUSDT",
	}
}

func TestDirEncodesEverySegmentNotOnlyTheTopic(t *testing.T) {
	k := perpKey()
	// A separator in the SYMBOL, which ids permits: it rejects whitespace and
	// nothing else. Unencoded this becomes two directories and the round trip
	// silently reports a different stream.
	k.Source.Instrument.Symbol = "BTC/USD"
	k.Source.Instrument.Venue = ids.Venue("ven ue")
	k.Topic = "a@b.c"

	dir := k.Dir("/root")
	for _, want := range []string{"BTC%2FUSD", "ven%20ue", "a%40b.c"} {
		if !strings.Contains(dir, want) {
			t.Errorf("Dir = %q, want a segment %q", dir, want)
		}
	}
	// The negative control for the assertion above: an encoder that escaped
	// everything would also contain those substrings, so check that the
	// unreserved characters were left alone.
	if !strings.Contains(dir, "funding") {
		t.Errorf("Dir = %q, want the unreserved stream segment %q verbatim", dir, "funding")
	}
}

func TestFileNameGrowsPrecisionWithThePeriod(t *testing.T) {
	window := time.Date(2026, 7, 29, 14, 30, 15, 0, time.UTC)
	for _, tc := range []struct {
		period time.Duration
		want   string
	}{
		{time.Hour, "2026-07-29T14"},
		{2 * time.Hour, "2026-07-29T14"},
		{5 * time.Minute, "2026-07-29T14.30"},
		{30 * time.Second, "2026-07-29T14.30.15"},
	} {
		if got := fileName(window, tc.period); got != tc.want {
			t.Errorf("fileName(%s) = %q, want %q", tc.period, got, tc.want)
		}
	}
}

// TestRoundTripsEveryShape is the property §32e asks for, and it covers the
// contract branch by generating both product families.
func TestRoundTripsEveryShape(t *testing.T) {
	// The generators are constrained so NOTHING is skipped, and that is the
	// point rather than convenience. A property that draws freely and skips the
	// invalid draws is one bad generator away from skipping every case, at which
	// point it passes against any implementation at all — this repo's recurring
	// shape, and the reason `spot`/`perp` are counted below.
	//
	// `[!-~]` excludes the space, which ids refuses in a symbol; the topic class
	// additionally excludes `/`, which StreamKey refuses. Both classes still
	// include `%`, `.`, `-` and `_`, so the escape-the-escape case and the
	// leave-unreserved-alone case are both generated.
	venueGen := rapid.StringMatching(`[ -~]{1,12}`)
	symbolGen := rapid.StringMatching(`[!-~]{1,12}`)
	topicGen := rapid.StringMatching(`[!-.0-~]{1,12}`)

	var spotSeen, perpSeen int
	rapid.Check(t, func(t *rapid.T) {
		spot := rapid.Bool().Draw(t, "spot")
		vi := ids.VenueInstrument{
			Venue:   ids.Venue(venueGen.Draw(t, "venue")),
			Product: ids.ProductSpot,
			Symbol:  symbolGen.Draw(t, "symbol"),
		}
		if spot {
			spotSeen++
		} else {
			perpSeen++
			vi.Product = ids.ProductLinearPerp
			vi.Contract = ids.ContractLinear
		}
		key := StreamKey{
			Source: events.SourceID{Instrument: vi, Stream: events.StreamBook},
			Topic:  topicGen.Draw(t, "topic"),
		}
		if err := key.Validate(); err != nil {
			t.Fatalf("the generators produced an invalid key %#v: %v", key, err)
		}
		period := rapid.SampledFrom([]time.Duration{
			time.Hour, 4 * time.Hour, 15 * time.Minute, 10 * time.Second,
		}).Draw(t, "period")
		instant := time.Unix(rapid.Int64Range(0, 4_000_000_000).Draw(t, "unix"), 0).UTC()
		window := Window(instant, period)

		const root = "/archive"
		p := key.FilePath(root, window, period)
		gotKey, gotWindow, err := ParseFilePath(root, p)
		if err != nil {
			t.Fatalf("ParseFilePath(%q) = %v", p, err)
		}
		if gotKey != key {
			t.Errorf("key round-tripped to %#v, want %#v (path %q)", gotKey, key, p)
		}
		if !gotWindow.Equal(window) {
			t.Errorf("window round-tripped to %s, want %s (path %q)", gotWindow, window, p)
		}
	})

	// The non-vacuity check. The contract segment is present for one product
	// family and absent for the other, and ParseFilePath BRANCHES on that — a
	// run that generated only perpetuals would leave the branch not taken
	// completely unexercised while reporting a passing property.
	if spotSeen == 0 || perpSeen == 0 {
		t.Errorf("the property ran %d spot and %d perpetual cases; both shapes are "+
			"needed because the contract path segment is present in exactly one of them",
			spotSeen, perpSeen)
	}
}

func TestParseFilePathRejections(t *testing.T) {
	const root = "/archive"
	key, period := perpKey(), time.Hour
	window := time.Date(2026, 7, 29, 14, 0, 0, 0, time.UTC)
	good := key.FilePath(root, window, period)

	// The positive control. Without it a ParseFilePath that refused everything
	// would score perfectly on the table below — this repo's recurring shape.
	if _, _, err := ParseFilePath(root, good); err != nil {
		t.Fatalf("the well-formed path %q was refused: %v", good, err)
	}

	for _, tc := range []struct {
		name string
		path string
		want error
	}{
		{"outside the root", "/elsewhere/v1/a/b/c/d/e/2026/07/29/2026-07-29T14.arbcap", ErrPathShape},
		{"too few segments", filepath.Join(root, "v1", "a", "2026-07-29T14.arbcap"), ErrPathShape},
		{"wrong tree version", strings.Replace(good, "/v1/", "/v9/", 1), ErrFormatVersion},
		{"not an archive file", strings.TrimSuffix(good, archiveExt) + ".txt", ErrPathShape},
		{"name is not a window", strings.Replace(good, "2026-07-29T14.arbcap", "latest.arbcap", 1), ErrPathShape},
		{
			// The redundancy between the date directories and the file name is
			// only a CHECK if disagreement is refused. A sync script that moved
			// one file otherwise makes a coverage report double-count one day
			// and find another empty.
			"date directories disagree with the name",
			strings.Replace(good, "2026/07/29/", "2026/07/28/", 1),
			ErrPathShape,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := ParseFilePath(root, tc.path); !errorIs(err, tc.want) {
				t.Errorf("ParseFilePath(%q) = %v, want %v", tc.path, err, tc.want)
			}
		})
	}
}

func TestSegmentEncodingIsCanonical(t *testing.T) {
	// Round trip, including the escape of the escape character itself: an
	// encoder that left % alone maps "a%2Fb" and "a/b" onto one directory.
	// "." and ".." are the cases the round-trip property found, and they are the
	// reason a leading dot is escaped: filepath.Join CLEANS them, so before the
	// fix a topic of "." produced another stream's path and a topic of ".."
	// climbed out of the archive tree, with encoder and decoder agreeing that
	// nothing had happened.
	for _, s := range []string{
		"", "plain", "a/b", "%", "a%b", "%2F", "üñî", "\x00\xff", "a b",
		".", "..", "...", ".hidden", ".writer.lock", "a.b", "trailing.",
	} {
		got, err := decodeSegment(encodeSegment(s))
		if err != nil {
			t.Fatalf("decodeSegment(encodeSegment(%q)) = %v", s, err)
		}
		if got != s {
			t.Errorf("round trip of %q gave %q", s, got)
		}
	}

	// The percent sign at the FIRST, MIDDLE and LAST position separately —
	// where an `i > 0` predicate and an `i >= 0` one give the same answer if
	// every fixture puts it in the middle.
	for _, tc := range []string{"%", "%2", "a%", "ab%", "%zz", "a%zzb", "ab%2", "%2f", "a%2fb", "ab%2f"} {
		if _, err := decodeSegment(tc); !errorIs(err, ErrPathEncoding) {
			t.Errorf("decodeSegment(%q) = %v, want ErrPathEncoding", tc, err)
		}
	}

	// A redundant escape is refused, because accepting it gives two path
	// spellings for one stream and the whole point of the pair is that there is
	// one. "%41" is "A", which needs no escape.
	//
	// "AB%2E" and "%2E" are the pair that separates the carve-out from a blanket
	// exemption: `%2E` is legal at position 0 and illegal everywhere else,
	// exactly mirroring where the encoder emits it. A fixture that only ever put
	// the escape mid-string would pass against an implementation that allowed it
	// anywhere — the same collapse, one rule further in.
	for _, tc := range []string{"%41", "%41BC", "AB%41", "AB%2E", "a%2Eb", "%5F", "%2D"} {
		if _, err := decodeSegment(tc); !errorIs(err, ErrPathEncoding) {
			t.Errorf("decodeSegment(%q) = %v, want a refusal of the redundant escape", tc, err)
		}
	}

	// A literal leading dot is refused in the other direction: a path segment of
	// ".." reaching the decoder is a directory this package did not write, and
	// decoding it would report a stream key derived from outside the archive.
	for _, tc := range []string{".", "..", ".hidden", "./x"} {
		if _, err := decodeSegment(tc); !errorIs(err, ErrPathEncoding) {
			t.Errorf("decodeSegment(%q) = %v, want a refusal of the literal leading dot", tc, err)
		}
	}
	// And the carve-out is accepted, which is the positive control for the two
	// refusals above: without it the encoder's own output would be rejected by
	// its inverse and the round trip would fail closed instead of open.
	// Note the spelling: only the FIRST dot is escaped, so ".." is "%2E." and
	// not "%2E%2E" — which is also why the mid-string "%2E" above is refused.
	if got, err := decodeSegment("%2E."); err != nil || got != ".." {
		t.Errorf(`decodeSegment("%%2E.") = %q, %v; want "..", nil`, got, err)
	}
}

func TestNextTickIsStrictlyAfterABoundary(t *testing.T) {
	const interval = 7 * time.Minute
	// The boundary is DERIVED rather than assumed, and that is the finding this
	// test produced: time.Truncate rounds down to a multiple of d since the ZERO
	// time (year 1), not since the Unix epoch. A fixture that built a boundary
	// by adding N intervals to the epoch is only on a boundary when the interval
	// divides the year-1-to-epoch gap — true for 5 and 15 minutes, false for 7.
	onBoundary := time.Date(2026, 7, 29, 14, 3, 17, 0, time.UTC).Truncate(interval)

	// Exactly ON a boundary is the fixture that separates `>` from `>=`: a
	// nextTick that returned the boundary itself would let Tick write a second
	// Alive record for an interval it had already ticked.
	if got := nextTick(onBoundary, interval); !got.Equal(onBoundary.Add(interval)) {
		t.Errorf("nextTick(on a boundary) = %s, want %s", got, onBoundary.Add(interval))
	}
	if got := nextTick(onBoundary.Add(time.Nanosecond), interval); !got.Equal(onBoundary.Add(interval)) {
		t.Errorf("nextTick(boundary+1ns) = %s, want %s", got, onBoundary.Add(interval))
	}
	if got := nextTick(onBoundary.Add(interval-time.Nanosecond), interval); !got.Equal(onBoundary.Add(interval)) {
		t.Errorf("nextTick(just before the next boundary) = %s, want %s", got, onBoundary.Add(interval))
	}
}

// TestWindowLandsOnUTCHours pins §32e's actual requirement — "rotation is hourly
// on UTC RecvTime" — rather than trusting time.Truncate's anchor to happen to
// deliver it.
//
// It is the anchor question stated as the property that depends on it: the zero
// time is on an hour boundary, so hourly windows align to UTC hours; if the
// anchor ever changed, every archive file name would shift and this is the only
// test that would say so.
func TestWindowLandsOnUTCHours(t *testing.T) {
	for _, tc := range []time.Time{
		time.Date(2026, 7, 29, 14, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 29, 14, 59, 59, 999_999_999, time.UTC),
		time.Date(1970, 1, 1, 0, 30, 0, 0, time.UTC),
		time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC),
	} {
		w := Window(tc, time.Hour)
		if w.Minute() != 0 || w.Second() != 0 || w.Nanosecond() != 0 {
			t.Errorf("Window(%s, 1h) = %s, which is not a UTC hour boundary", tc, w)
		}
		if w.Hour() != tc.UTC().Hour() || w.Day() != tc.UTC().Day() {
			t.Errorf("Window(%s, 1h) = %s, want the same UTC hour", tc, w)
		}
	}

	// A non-UTC input must land in the same window as its UTC equivalent — a
	// local-time boundary would make file names depend on the writing host's
	// $TZ, and a DST transition would produce either a 25th file or a collision.
	east := time.FixedZone("UTC+9", 9*3600)
	local := time.Date(2026, 7, 29, 23, 30, 0, 0, east)
	if got, want := Window(local, time.Hour), Window(local.UTC(), time.Hour); !got.Equal(want) {
		t.Errorf("Window of a +09:00 instant = %s, want %s", got, want)
	}
}

func TestStreamKeyValidate(t *testing.T) {
	if err := perpKey().Validate(); err != nil {
		t.Fatalf("the well-formed key was refused: %v", err)
	}
	bad := perpKey()
	bad.Topic = ""
	if err := bad.Validate(); !errorIs(err, ErrTopicUnset) {
		t.Errorf("empty topic = %v, want ErrTopicUnset", err)
	}
	for _, topic := range []string{"a/b", "/leading", "trailing/", "with\nnewline", "with\x00nul"} {
		bad.Topic = topic
		if err := bad.Validate(); !errorIs(err, ErrTopicChar) {
			t.Errorf("topic %q = %v, want ErrTopicChar", topic, err)
		}
	}
}
