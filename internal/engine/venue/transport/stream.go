package transport

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// DisruptionCause is why a socket stopped carrying data. It is transport's own
// vocabulary rather than capture.Cause, because this package is shared with the
// future live path and capture is one consumer of it (§32b).
//
// The mapping to a capture.Cause happens in the composition root, as a switch,
// so exhaustiveswitch reports a member added here and not handled there. That is
// the gate doing the work an import would otherwise do badly.
type DisruptionCause string

const (
	// DisruptionUnspecified is the zero value and names nothing.
	DisruptionUnspecified DisruptionCause = ""

	// DisruptionDial is a socket that would not open.
	DisruptionDial DisruptionCause = "dial"

	// DisruptionSubscribe is a subscription the venue refused, or an ack that
	// never came. Distinct from a dial failure because the host was reachable,
	// which is the difference between "the venue is down" and "our config is
	// wrong" — and those need different humans.
	DisruptionSubscribe DisruptionCause = "subscribe"

	// DisruptionRead is an established socket that failed or went silent past
	// the read timeout. The ordinary one: this is what a venue restart looks
	// like from here.
	DisruptionRead DisruptionCause = "read"

	// DisruptionNoFirstData is §32k's first-data assertion failing — subscribed,
	// acked, and nothing arrived. The one that catches a decommissioned path
	// that accepts connections and sends nothing forever.
	DisruptionNoFirstData DisruptionCause = "no_first_data"
)

// Validate reports whether c is a declared cause, excluding the zero value.
func (c DisruptionCause) Validate() error {
	switch c {
	case DisruptionUnspecified:
		return fmt.Errorf("disruption cause unset: %w", ErrProtocolUnknown)
	case DisruptionDial, DisruptionSubscribe, DisruptionRead, DisruptionNoFirstData:
		return nil
	default:
		return fmt.Errorf("disruption cause %q: %w", c, ErrProtocolUnknown)
	}
}

func (c DisruptionCause) String() string { return string(c) }

// Disruption is one loss of continuity, reported to the sink before any
// reconnect is attempted.
type Disruption struct {
	Cause DisruptionCause

	// Detail is for the operator and is never branched on. It carries the
	// venue's own words when the venue supplied any — a refused Bybit
	// subscription says why, and that sentence is worth more six weeks later
	// than any classification this package could invent for it.
	Detail string
}

// Sink receives everything a Stream observes. Every method is called from the
// Stream's own goroutine, in order, and none of them may block: the sink's job
// is to stamp and hand off to §32f's bounded queue, and a sink that blocks turns
// a bounded queue into TCP back-pressure — the rejected alternative, arrived at
// by accident.
type Sink interface {
	// Data is a venue data frame, verbatim.
	//
	// The sink MAY retain the slice. That is a guarantee this package makes
	// rather than one it passes through from the websocket library: today the
	// library allocates per message, and if that ever changes the copy goes
	// here, where the guarantee is written, and not in every sink.
	Data(payload []byte)

	// Subscribed is called once per successful connection, after the ack. Both
	// arguments are nil under the path discipline, which has neither — and that
	// nil is information: it is why §32k's assertion is the ONLY check that
	// stream is subscribed to anything.
	Subscribed(request, ack []byte)

	// Disrupted is called for each loss of continuity, before the backoff.
	Disrupted(d Disruption)
}

// StreamConfig is one socket. Every field is required except ReadLimit.
//
// No defaults here, for WriterConfig's reason one package over: §32j puts
// defaults in platform/config where a value has a name and a restart-required
// tag. A default applied silently at the constructor is a value with no name,
// and the two that would hurt most are a zero FirstDataTimeout — which disables
// the only check that catches a dead feed — and a zero ReadTimeout, which makes
// every read wait forever on a half-open socket.
type StreamConfig struct {
	// Endpoint and Topic are §35b's split. See Endpoint.
	Endpoint Endpoint
	Topic    string

	// Protocol is the venue dialect.
	Protocol ProtocolID

	// Sink receives the frames.
	Sink Sink

	// DialTimeout bounds the handshake.
	DialTimeout time.Duration

	// SubscribeTimeout bounds the wait for the ack, under the message
	// discipline. Unused under the path discipline, which has no ack.
	SubscribeTimeout time.Duration

	// FirstDataTimeout is §32k's assertion: how long after subscribing a stream
	// may produce nothing before we call it dead rather than quiet.
	//
	// It is per source because the honest value is a property of the feed. A
	// mark-price stream at 1 s and a level-1 book that repeats every 3 s do not
	// share a threshold with a thin spot book, and a single global value would
	// be set to the slowest feed — which is the same as not having the check on
	// the six faster ones.
	FirstDataTimeout time.Duration

	// ReadTimeout bounds each read on an established socket. Exceeding it is a
	// DisruptionRead and a reconnect: a silent socket and a dead socket are the
	// same thing to us, and reconnecting is how we find out which it was.
	ReadTimeout time.Duration

	// Backoff is the reconnect schedule, walked by attempt number and holding
	// at the last entry. A slice rather than a formula with jitter, because the
	// schedule is then exactly what the config says, a test asserts it without
	// a random source, and seven sources reconnecting in lockstep is nowhere
	// near Bybit's documented ceiling of 500 connections per 5 minutes.
	Backoff []time.Duration

	// ReadLimit caps one message, in bytes. Zero takes the library default.
	// The composition root passes capture's MaxRecord, so a frame the archive
	// could not hold is refused at the socket rather than at the writer, where
	// it would already have been counted as captured.
	ReadLimit int64
}

// Validate reports whether the config could open a stream.
func (c StreamConfig) Validate() error {
	if err := c.Endpoint.Validate(); err != nil {
		return err
	}
	if err := validateTopic(c.Topic); err != nil {
		return err
	}
	if err := c.Protocol.Validate(); err != nil {
		return err
	}
	if c.Sink == nil {
		return ErrSinkUnset
	}
	for name, d := range map[string]time.Duration{
		"dial timeout":       c.DialTimeout,
		"subscribe timeout":  c.SubscribeTimeout,
		"first-data timeout": c.FirstDataTimeout,
		"read timeout":       c.ReadTimeout,
	} {
		if d <= 0 {
			return fmt.Errorf("%s %s: %w", name, d, ErrTimeoutRange)
		}
	}
	if len(c.Backoff) == 0 {
		return ErrBackoffUnset
	}
	for i, d := range c.Backoff {
		if d <= 0 {
			return fmt.Errorf("backoff[%d] %s: %w", i, d, ErrTimeoutRange)
		}
	}
	return nil
}

// StreamStats is what a Stream knows about itself, for §32j's observability.
//
// ConsecutiveFailures is the one to alert on, and it is here because the failure
// this package cannot distinguish on its own is a permanent misconfiguration: a
// mistyped host retries forever at the backoff ceiling with every other number
// looking calm. A rising consecutive count with zero data frames is what that
// looks like, and it is only visible if somebody counted.
type StreamStats struct {
	Connects            uint64
	Subscribes          uint64
	DataFrames          uint64
	AckFrames           uint64
	PongFrames          uint64
	Disruptions         uint64
	ConsecutiveFailures uint64
	LastDetail          string
}

// Stream is one socket, supervised: connect, subscribe, assert first data, read,
// reconnect. One goroutine runs it; Stats may be read from another.
type Stream struct {
	cfg   StreamConfig
	proto Protocol
	url   string
	sub   []byte

	mu    sync.Mutex
	stats StreamStats
}

// NewStream resolves the dialect and builds the dial URL once, so a
// misconfiguration is a startup refusal naming the field rather than a reconnect
// loop that never carries data (see errors.go).
func NewStream(cfg StreamConfig) (*Stream, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	proto, err := ProtocolFor(cfg.Protocol)
	if err != nil {
		return nil, err
	}
	url, err := proto.DialURL(cfg.Endpoint, cfg.Topic)
	if err != nil {
		return nil, err
	}
	return &Stream{cfg: cfg, proto: proto, url: url, sub: proto.Subscribe(cfg.Topic)}, nil
}

// URL is the resolved dial URL, for the Open record's provenance (§32g). A file
// that cannot say which endpoint produced it is a file nobody can defend.
func (s *Stream) URL() string { return s.url }

// SubscribeFrame is the request bytes, or nil under the path discipline.
func (s *Stream) SubscribeFrame() []byte { return s.sub }

// Stats is a snapshot.
func (s *Stream) Stats() StreamStats {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stats
}

// Run supervises the socket until ctx is cancelled, reconnecting on every
// disruption. It returns nil on a clean cancellation and an error only when the
// stream cannot be run at all.
//
// There is no error return for a venue that is down, and that is the design: a
// socket dropping is normal operation for this tool, and a Run that gave up
// would turn a five-minute venue restart into a permanently missing day. What
// makes that safe rather than negligent is that every drop is REPORTED to the
// sink first, so the archive records the hole even while the reconnect succeeds.
func (s *Stream) Run(ctx context.Context) error {
	for attempt := 0; ; attempt++ {
		if ctx.Err() != nil {
			return nil
		}

		d, carried := s.session(ctx)

		// Checked BEFORE recording, so a clean shutdown does not write a gap.
		// The socket does end mid-read on the way out, and reporting that as a
		// disruption would put a reconnect gap in the archive for every orderly
		// stop — teaching whoever reads it later to discount the record that
		// matters most.
		if ctx.Err() != nil {
			return nil
		}

		// A session that carried data resets the schedule: the next disruption
		// is a fresh incident, not the continuation of an old one, and starting
		// it at the ceiling would add minutes of blindness to a socket that was
		// working a second ago.
		if carried {
			attempt = 0
		}
		s.record(d)

		if !s.wait(ctx, s.backoff(attempt)) {
			return nil
		}
	}
}

// backoff walks the schedule by attempt and holds at the last entry.
func (s *Stream) backoff(attempt int) time.Duration {
	if attempt >= len(s.cfg.Backoff) {
		return s.cfg.Backoff[len(s.cfg.Backoff)-1]
	}
	return s.cfg.Backoff[attempt]
}

// wait sleeps for d or until ctx is cancelled, reporting whether it slept.
//
// # This is notimenow's escape, and §32i predicted this exact line
//
// Rule 5's escape covers transport DEADLINES, not stamps, and a reconnect
// backoff is a deadline in the strict sense: it must elapse in real time even
// when the dispatcher is wedged, it never runs under replay, and nothing it
// produces is written to a record. Routing it through Clock.SetTimer would put
// socket liveness on the dispatch timer queue — worse architecture, not stricter
// discipline.
//
// The escape's own doc says that if engine/venue ships and no escape is ever
// used, the prediction was wrong and the mechanism should be deleted. This line
// and the ticker in keepAlive are the two uses; the prediction survives. Note
// what did NOT need one: every socket deadline goes through context.WithTimeout,
// which reads no clock in our code at all.
func (s *Stream) wait(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d) //notimenow:allow reconnect backoff is a transport deadline (§32i); a delay, never a stamp
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// record folds a disruption into the stats and hands it to the sink.
func (s *Stream) record(d Disruption) {
	s.mu.Lock()
	s.stats.Disruptions++
	s.stats.ConsecutiveFailures++
	s.stats.LastDetail = d.Detail
	s.mu.Unlock()
	s.cfg.Sink.Disrupted(d)
}

// session runs one socket from dial to failure. It reports the disruption that
// ended it, and whether the session ever carried a data frame — which is what
// Run uses to decide whether the backoff schedule restarts.
func (s *Stream) session(ctx context.Context) (Disruption, bool) {
	dialCtx, cancelDial := context.WithTimeout(ctx, s.cfg.DialTimeout)
	conn, _, err := websocket.Dial(dialCtx, s.url, nil)
	cancelDial()
	if err != nil {
		return Disruption{Cause: DisruptionDial, Detail: err.Error()}, false
	}
	// CloseNow rather than Close: a close handshake on a socket we are
	// abandoning can itself block, and this path runs when things are already
	// wrong.
	defer func() { _ = conn.CloseNow() }()

	if s.cfg.ReadLimit > 0 {
		conn.SetReadLimit(s.cfg.ReadLimit)
	}
	s.bump(func(st *StreamStats) { st.Connects++ })

	ack, d, ok := s.subscribe(ctx, conn)
	if !ok {
		return d, false
	}
	s.cfg.Sink.Subscribed(s.sub, ack)
	s.bump(func(st *StreamStats) { st.Subscribes++; st.ConsecutiveFailures = 0 })

	stop := s.keepAlive(ctx, conn)
	defer stop()

	return s.pump(ctx, conn)
}

// subscribe sends the request, if the dialect has one, and waits for the ack.
//
// Data frames arriving BEFORE the ack are delivered rather than discarded. Both
// venues ack first, so this costs nothing today — but the alternative is a
// package that silently drops market data whenever a venue reorders its own
// handshake, and "the venue did something we did not expect" is the case the
// archive exists for.
func (s *Stream) subscribe(ctx context.Context, conn *websocket.Conn) ([]byte, Disruption, bool) {
	if s.sub == nil {
		return nil, Disruption{}, true // path discipline: nothing to send or await
	}

	subCtx, cancel := context.WithTimeout(ctx, s.cfg.SubscribeTimeout)
	defer cancel()

	if err := conn.Write(subCtx, websocket.MessageText, s.sub); err != nil {
		return nil, Disruption{Cause: DisruptionSubscribe, Detail: "write: " + err.Error()}, false
	}

	for {
		_, frame, err := conn.Read(subCtx)
		if err != nil {
			return nil, Disruption{
				Cause:  DisruptionSubscribe,
				Detail: s.readDetail(ctx, subCtx, err, "awaiting subscribe ack"),
			}, false
		}
		switch class := s.proto.Classify(frame); class {
		case ClassAck:
			s.bump(func(st *StreamStats) { st.AckFrames++ })
			okAck, detail := s.proto.AckResult(frame)
			if !okAck {
				return nil, Disruption{Cause: DisruptionSubscribe, Detail: detail}, false
			}
			return frame, Disruption{}, true
		case ClassData:
			s.deliver(frame)
		case ClassPong:
			s.bump(func(st *StreamStats) { st.PongFrames++ })
		default:
			// Classify's contract is that it returns a declared class, and the
			// default direction is Data. Treating an undeclared one as Data
			// keeps this switch's behaviour identical to the contract rather
			// than inventing a second policy here.
			s.deliver(frame)
		}
	}
}

// pump is the read loop: first-data assertion, then steady reads.
//
// The two phases differ ONLY in the timeout, and they are one loop for that
// reason — two loops would be two places for the delivery path to drift, and the
// realistic drift is a first-data phase that counts a frame the steady phase
// would have delivered.
func (s *Stream) pump(ctx context.Context, conn *websocket.Conn) (Disruption, bool) {
	// The first-data deadline is ABSOLUTE from subscription, created once here
	// rather than per read, and that is the whole assertion.
	//
	// A per-read deadline restarts on every frame that is not data — so a Bybit
	// pong every 20 s under a 30 s first-data timeout renews the window forever
	// and the check never fires. That is §31g rule 4's failure wearing a
	// disguise: not "a pong counted as data", which Classify already refuses,
	// but "a pong resetting the clock that is watching for data", which costs
	// nothing to write and is invisible in a test whose server is silent.
	firstCtx, cancelFirst := context.WithTimeout(ctx, s.cfg.FirstDataTimeout)
	defer cancelFirst()

	gotData := false
	for {
		readCtx, cancel := s.readDeadline(ctx, firstCtx, gotData)
		_, frame, err := conn.Read(readCtx)
		cancel()

		if err != nil {
			cause := DisruptionRead
			what := "reading"
			if !gotData {
				// Nothing has arrived since we subscribed. Whether the socket
				// errored or merely went quiet, the operational fact is the
				// same and it is the one §32k names: we are subscribed to
				// something that is not sending.
				cause = DisruptionNoFirstData
				what = "awaiting first data"
			}
			return Disruption{Cause: cause, Detail: s.readDetail(ctx, readCtx, err, what)}, gotData
		}

		switch class := s.proto.Classify(frame); class {
		case ClassData:
			gotData = true
			s.deliver(frame)
		case ClassPong:
			s.bump(func(st *StreamStats) { st.PongFrames++ })
		case ClassAck:
			// A second ack on an established socket. Counted, not delivered:
			// it is not market data and it must not satisfy the assertion
			// above.
			s.bump(func(st *StreamStats) { st.AckFrames++ })
		default:
			gotData = true
			s.deliver(frame)
		}
	}
}

// readDeadline picks which deadline governs the next read.
//
// It is a named function rather than three inline lines because the choice it
// makes IS §32k's assertion, and a claim that cannot be aimed at cannot be
// shown to bite: written inline, the mutation that breaks it also makes firstCtx
// unused, so the harness reports NOBUILD!! — the verdict that means "re-aim" —
// and the claim goes unchecked while looking covered.
//
// Before first data the governing deadline is firstCtx, created ONCE at
// subscription and shared by every read in the phase, so it expires at a fixed
// instant however many frames arrive. Returning a fresh context.WithTimeout here
// instead compiles, passes every test whose server is silent, and renews the
// window on each pong — the assertion satisfied by our own keepalive, which is
// §31g rule 4 with one extra step.
//
// After first data each read gets its own ReadTimeout, which is the ordinary
// stalled-socket detector and has no such subtlety.
func (s *Stream) readDeadline(ctx, firstCtx context.Context, gotData bool) (context.Context, context.CancelFunc) {
	if gotData {
		return context.WithTimeout(ctx, s.cfg.ReadTimeout)
	}
	return firstCtx, func() {}
}

// deliver hands a frame to the sink and counts it. One place, so "counted" and
// "delivered" cannot come apart — a data frame the stats missed would make
// §32k's own coverage numbers wrong.
func (s *Stream) deliver(frame []byte) {
	s.bump(func(st *StreamStats) { st.DataFrames++ })
	s.cfg.Sink.Data(frame)
}

// readDetail explains a failed read, distinguishing OUR deadline from the
// venue's behaviour.
//
// Without this the archive records "context deadline exceeded" for a stream that
// went quiet and for a shutdown mid-read, and six weeks later those are the same
// sentence. They are not the same event: one is a gap in the data and one is the
// operator stopping the tool.
func (s *Stream) readDetail(ctx, opCtx context.Context, err error, what string) string {
	switch {
	case ctx.Err() != nil:
		return what + ": stopped"
	case errors.Is(opCtx.Err(), context.DeadlineExceeded):
		return what + ": timed out"
	default:
		return what + ": " + err.Error()
	}
}

// keepAlive starts the dialect's heartbeat, returning a stop function. A dialect
// with no heartbeat gets a no-op, so the caller has no branch.
func (s *Stream) keepAlive(ctx context.Context, conn *websocket.Conn) func() {
	frame, every := s.proto.KeepAlive()
	if frame == nil {
		return func() {}
	}

	done := make(chan struct{})
	var once sync.Once
	stop := func() { once.Do(func() { close(done) }) }

	go func() {
		tick := time.NewTicker(every) //notimenow:allow the venue's documented heartbeat is a transport deadline (§32i); it schedules a write and stamps nothing
		defer tick.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-tick.C:
				writeCtx, cancel := context.WithTimeout(ctx, every)
				// A failed ping is not reported as a disruption: the read loop
				// is the authority on whether this socket is alive, and a
				// second reporter would double-count one incident into two gap
				// records. The read timeout catches what a failed ping predicts.
				_ = conn.Write(writeCtx, websocket.MessageText, frame)
				cancel()
			}
		}
	}()
	return stop
}

// bump applies a stats mutation under the lock. Concurrent with Stats, which a
// metrics scrape calls from its own goroutine.
func (s *Stream) bump(f func(*StreamStats)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f(&s.stats)
}
