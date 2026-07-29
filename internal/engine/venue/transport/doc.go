// Package transport is the raw venue socket: it connects, subscribes, reads,
// reconnects, and hands out UNPARSED frames. §11 puts the raw Binance/Bybit
// clients in engine/venue/, and §32b makes this package the precise shape of
// capture's fence against §31e.
//
// [SPEC EDIT OWED] §11's backend roster names "nhooyr.io/websocket or
// gorilla/websocket". That module was RENAMED to github.com/coder/websocket in
// 2024 and the old path no longer resolves, so the spec's first-named option
// cannot be taken as written. This package takes it under its current name —
// the user's call, recorded in slice C's record — and §11 owes the rename.
//
// Recorded rather than absorbed because the two are not the same claim: "we
// chose differently from the spec" and "the spec names something that no longer
// exists" need different edits, and only the second is true here. It is also
// the engine module's FIRST runtime third-party dependency, which §11 permits
// for exactly this ("native venue REST/WS") and which is worth a reader knowing
// without reading go.mod: the module has zero transitive dependencies, so the
// graph gains one node.
//
// # What is shared with the live path, and what never is
//
// The loose form of §31e's rule — "keep capture separate" — would forbid the one
// sharing that makes capture worth doing. Capturing through a different socket
// stack than the engine will use makes the archive a record of a pipeline we do
// not run, and every latency and gap figure measured from it describes the wrong
// system.
//
//   - SHARED: the connection, the subscription, the reconnect, the framing —
//     the bytes and when they arrived.
//   - NEVER SHARED: arbitration. This package mints no CaptureSeq, no
//     AcceptOrdinal and no IngressSeq, and it makes no decision about the order
//     in which two different sources are consumed. One Stream knows about one
//     socket and nothing else exists to it.
//
// It follows that this package does not import engine/capture. Capture is an
// ops tool built on top of this; the live path will be a different consumer of
// the same seam. A Disruption is therefore transport's own vocabulary, and the
// mapping to a capture.Cause happens in the composition root — as a switch, so
// exhaustiveswitch catches a member added here and not handled there.
//
// # Parsing: the line is the ENVELOPE, and never the payload
//
// §32n says capture parses nothing, and that rule is about the archive. This
// package has to understand a little more, because it must SEND a subscribe
// frame and know whether the venue accepted it — a tool that cannot tell a
// subscription ack from market data cannot implement §32k's first-data
// assertion, which is the mechanism that catches a silently-dead feed.
//
// So: Classify reads the envelope and never the payload, and — this is the part
// that matters — its result never changes the bytes. A frame handed to the sink
// is the same slice that came off the socket, never re-encoded, whatever
// Classify decided about it.
//
// # Classification defaults to Data, and that direction is the whole design
//
// A frame is Data unless it is POSITIVELY RECOGNISED as one of the two things
// that are protocol noise: a subscription ack, or the venue's reply to our own
// keepalive ping. Everything else — including an envelope shape nobody here has
// seen — is Data, and lands in the archive verbatim.
//
// The default is chosen for what happens when somebody forgets. A venue adding a
// new envelope next year is then archived (recoverable, and possibly the very
// error message that explains a gap six weeks later) rather than silently
// discarded (unrecoverable, and invisible). The opposite default reads as
// tidier and quietly makes this package the thing that decides what history
// contains.
//
// The two exclusions are each load-bearing rather than tidiness. A pong counted
// as data would let §32k's first-data assertion be satisfied by OUR OWN
// keepalive — a liveness monitor answered by its own noise, which is the exact
// failure §31g rule 4 exists to prevent. An ack counted as data would do the
// same one message earlier, and it already has a home: §32g puts the subscribe
// request and its ack in the Open record's provenance.
package transport
