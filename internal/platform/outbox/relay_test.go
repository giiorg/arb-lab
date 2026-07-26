package outbox_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/apperror"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/outbox"
)

// --- fakes ------------------------------------------------------------------

type fakeStore struct {
	claimable []outbox.Record

	// Recorded calls.
	claimNow   []time.Time
	claimBatch []int
	acked      []uint64
	failed     []outbox.Failure

	claimErr, ackErr, failErr error
}

func (s *fakeStore) Claim(_ context.Context, now time.Time, batch int) ([]outbox.Record, error) {
	s.claimNow = append(s.claimNow, now)
	s.claimBatch = append(s.claimBatch, batch)
	if s.claimErr != nil {
		return nil, s.claimErr
	}
	return s.claimable, nil
}

func (s *fakeStore) Ack(_ context.Context, seqs []uint64) error {
	s.acked = append(s.acked, seqs...)
	return s.ackErr
}

func (s *fakeStore) Fail(_ context.Context, fs []outbox.Failure) error {
	s.failed = append(s.failed, fs...)
	return s.failErr
}

// failedSeq returns the recorded failure for seq, if any.
func (s *fakeStore) failedSeq(seq uint64) (outbox.Failure, bool) {
	for _, f := range s.failed {
		if f.Seq == seq {
			return f, true
		}
	}
	return outbox.Failure{}, false
}

type fakePublisher struct {
	// published is every record Publish was CALLED with, in call order —
	// including the ones it then rejected. Order is the thing most of these
	// tests assert on, so it must record attempts, not successes.
	published []outbox.Record

	// reject maps a Seq to the error Publish returns for it.
	reject map[uint64]error
}

func (p *fakePublisher) Publish(_ context.Context, r outbox.Record) error {
	p.published = append(p.published, r)
	return p.reject[r.Seq]
}

func (p *fakePublisher) seqs() []uint64 {
	out := make([]uint64, 0, len(p.published))
	for _, r := range p.published {
		out = append(out, r.Seq)
	}
	return out
}

// --- fixtures ---------------------------------------------------------------

var epoch = time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)

func rec(seq uint64, key outbox.Key) outbox.Record {
	return outbox.Record{
		Seq: seq,
		Message: outbox.Message{
			Topic:   "audit",
			Key:     key,
			Type:    "oms.FillApplied",
			Payload: []byte(`{}`),
		},
		EnqueuedAt: epoch,
	}
}

func testBackoff() outbox.Backoff {
	return outbox.Backoff{Base: time.Second, Max: time.Minute}
}

// newRelay wires a relay over the two fakes and a TestClock pinned at epoch.
func newRelay(t *testing.T, s *fakeStore, p *fakePublisher) *outbox.Relay {
	t.Helper()
	clk, err := clock.NewTestClock(epoch, nil)
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	r, err := outbox.NewRelay(outbox.RelayConfig{
		Store:     s,
		Publisher: p,
		Clock:     clk,
		Batch:     16,
		Backoff:   testBackoff(),
	})
	if err != nil {
		t.Fatalf("NewRelay: %v", err)
	}
	return r
}

func equalSeqs(a, b []uint64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// --- construction -----------------------------------------------------------

func TestNewRelayAcceptsTheFixture(t *testing.T) {
	// Positive control: without it, every rejection below would pass against a
	// constructor that refuses everything.
	newRelay(t, &fakeStore{}, &fakePublisher{})
}

func TestNewRelayRejects(t *testing.T) {
	clk, err := clock.NewTestClock(epoch, nil)
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	valid := outbox.RelayConfig{
		Store:     &fakeStore{},
		Publisher: &fakePublisher{},
		Clock:     clk,
		Batch:     16,
		Backoff:   testBackoff(),
	}

	tests := []struct {
		name string
		mut  func(*outbox.RelayConfig)
		want error
	}{
		{"nil store", func(c *outbox.RelayConfig) { c.Store = nil }, outbox.ErrStoreNil},
		{"nil publisher", func(c *outbox.RelayConfig) { c.Publisher = nil }, outbox.ErrPublisherNil},
		{"nil clock", func(c *outbox.RelayConfig) { c.Clock = nil }, outbox.ErrClockNil},
		{"zero batch", func(c *outbox.RelayConfig) { c.Batch = 0 }, outbox.ErrBatchInvalid},
		{"negative batch", func(c *outbox.RelayConfig) { c.Batch = -1 }, outbox.ErrBatchInvalid},
		{"zero backoff", func(c *outbox.RelayConfig) { c.Backoff = outbox.Backoff{} }, outbox.ErrBackoffInvalid},
		{"max below base", func(c *outbox.RelayConfig) {
			c.Backoff = outbox.Backoff{Base: time.Minute, Max: time.Second}
		}, outbox.ErrBackoffInvalid},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := valid
			tc.mut(&cfg)

			_, err := outbox.NewRelay(cfg)
			if !errors.Is(err, tc.want) {
				t.Fatalf("NewRelay() = %v, want %v", err, tc.want)
			}
		})
	}
}

// --- the happy path ---------------------------------------------------------

func TestRelayPublishesThenAcks(t *testing.T) {
	s := &fakeStore{claimable: []outbox.Record{rec(1, "a"), rec(2, "b")}}
	p := &fakePublisher{}

	res, err := newRelay(t, s, p).RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	if res.Claimed != 2 || res.Published != 2 || res.Failed != 0 || res.Blocked != 0 {
		t.Fatalf("Result = %+v, want 2 claimed / 2 published / 0 failed / 0 blocked", res)
	}
	if !equalSeqs(s.acked, []uint64{1, 2}) {
		t.Fatalf("acked = %v, want [1 2]", s.acked)
	}
	if len(s.failed) != 0 {
		t.Fatalf("failed = %v, want none", s.failed)
	}
}

func TestRelayEmptyClaimIsNotAnError(t *testing.T) {
	// An idle outbox is the normal state, not a fault. A relay that errored on
	// it would alert continuously in the case it is supposed to be silent.
	s := &fakeStore{}
	p := &fakePublisher{}

	res, err := newRelay(t, s, p).RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if res != (outbox.Result{}) {
		t.Fatalf("Result = %+v, want the zero result", res)
	}
	if len(s.acked) != 0 || len(s.failed) != 0 {
		t.Fatalf("an empty claim must settle nothing, acked=%v failed=%v", s.acked, s.failed)
	}
}

// Claim is handed the INJECTED clock's now, never the store's own reading.
// Two clocks either side of a retry-eligibility comparison is §3.6 rule 5's
// defect in a new place: a store reading time.Now() would claim a different set
// under replay than it did live.
func TestRelayClaimUsesTheInjectedClock(t *testing.T) {
	s := &fakeStore{}
	p := &fakePublisher{}

	if _, err := newRelay(t, s, p).RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	if len(s.claimNow) != 1 || !s.claimNow[0].Equal(epoch) {
		t.Fatalf("Claim now = %v, want the injected %v", s.claimNow, epoch)
	}
	if len(s.claimBatch) != 1 || s.claimBatch[0] != 16 {
		t.Fatalf("Claim batch = %v, want [16]", s.claimBatch)
	}
}

// --- per-key FIFO -----------------------------------------------------------

func TestRelayFailureBlocksTheRestOfItsKey(t *testing.T) {
	// Seq 1 and 3 share key "a". If 1 cannot be published, 3 must not be
	// either — publishing it would put a later event ahead of an earlier one in
	// a projection whose whole value is that it agrees with the journal.
	s := &fakeStore{claimable: []outbox.Record{rec(1, "a"), rec(3, "a")}}
	p := &fakePublisher{reject: map[uint64]error{1: errors.New("consumer down")}}

	res, err := newRelay(t, s, p).RunOnce(context.Background())
	if err == nil {
		t.Fatal("a publish failure must surface as an error")
	}

	if !equalSeqs(p.seqs(), []uint64{1}) {
		t.Fatalf("published = %v, want only [1] attempted", p.seqs())
	}
	if res.Blocked != 1 {
		t.Fatalf("Result.Blocked = %d, want 1", res.Blocked)
	}
	if len(s.acked) != 0 {
		t.Fatalf("acked = %v, want none", s.acked)
	}
}

// A blocked record is NOT recorded as a failure. It was never attempted, so
// counting it would inflate its attempt count and back it off for somebody
// else's failure — and its backoff is what decides how long the whole key
// stalls.
func TestRelayBlockedRecordIsNotFailed(t *testing.T) {
	s := &fakeStore{claimable: []outbox.Record{rec(1, "a"), rec(3, "a")}}
	p := &fakePublisher{reject: map[uint64]error{1: errors.New("consumer down")}}

	if _, err := newRelay(t, s, p).RunOnce(context.Background()); err == nil {
		t.Fatal("want an error")
	}

	if _, found := s.failedSeq(3); found {
		t.Fatalf("blocked record 3 must not be recorded as a failure, failed=%v", s.failed)
	}
	if _, found := s.failedSeq(1); !found {
		t.Fatalf("the record that actually failed must be recorded, failed=%v", s.failed)
	}
}

func TestRelayOneKeyFailingDoesNotBlockAnother(t *testing.T) {
	// The whole reason FIFO is per-key rather than global: an unready key never
	// stalls a ready one. Same rule §3.5 states for dispatch across SourceIDs.
	s := &fakeStore{claimable: []outbox.Record{rec(1, "a"), rec(2, "b"), rec(3, "a"), rec(4, "b")}}
	p := &fakePublisher{reject: map[uint64]error{1: errors.New("consumer down")}}

	res, err := newRelay(t, s, p).RunOnce(context.Background())
	if err == nil {
		t.Fatal("want an error")
	}

	if !equalSeqs(p.seqs(), []uint64{1, 2, 4}) {
		t.Fatalf("published = %v, want [1 2 4] — key b flows while key a is stuck", p.seqs())
	}
	if !equalSeqs(s.acked, []uint64{2, 4}) {
		t.Fatalf("acked = %v, want [2 4]", s.acked)
	}
	if res.Published != 2 || res.Failed != 1 || res.Blocked != 1 {
		t.Fatalf("Result = %+v, want 2 published / 1 failed / 1 blocked", res)
	}
}

// Per-key order is derived from Seq by the RELAY, not inherited from whatever
// order the store happened to return. A store whose ORDER BY is wrong (or
// absent) would otherwise reorder a tamper-evident projection with nothing
// failing.
func TestRelayPublishesInSeqOrderRegardlessOfClaimOrder(t *testing.T) {
	s := &fakeStore{claimable: []outbox.Record{rec(3, "a"), rec(1, "a"), rec(2, "a")}}
	p := &fakePublisher{}

	if _, err := newRelay(t, s, p).RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	if !equalSeqs(p.seqs(), []uint64{1, 2, 3}) {
		t.Fatalf("published = %v, want ascending [1 2 3]", p.seqs())
	}
}

// The mirror of the test above: the sort must not merely happen to hold when
// the store returns records already ordered. Two keys interleaved out of order
// exercise the same comparator against data that can falsify it.
func TestRelaySortIsBySeqNotByArrivalGrouping(t *testing.T) {
	s := &fakeStore{claimable: []outbox.Record{rec(4, "b"), rec(1, "a"), rec(3, "b"), rec(2, "a")}}
	p := &fakePublisher{}

	if _, err := newRelay(t, s, p).RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	if !equalSeqs(p.seqs(), []uint64{1, 2, 3, 4}) {
		t.Fatalf("published = %v, want ascending [1 2 3 4]", p.seqs())
	}
}

// The relay sorts a COPY. The slice a store hands across the port stays the
// store's, and reordering it in place would be a mutation of somebody else's
// value performed by a package they only know through an interface.
//
// The direction is the usual question — which way does it fail when somebody
// forgets? A store that returns a cached or reused slice is a perfectly
// reasonable implementation of Claim as documented, and the symptom of the
// relay destroying it is a silently reordered audit projection, which is the
// exact failure per-key FIFO exists to prevent.
func TestRelayDoesNotReorderTheStoresSlice(t *testing.T) {
	s := &fakeStore{claimable: []outbox.Record{rec(3, "a"), rec(1, "a"), rec(2, "a")}}
	p := &fakePublisher{}

	if _, err := newRelay(t, s, p).RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	got := make([]uint64, 0, len(s.claimable))
	for _, r := range s.claimable {
		got = append(got, r.Seq)
	}
	if !equalSeqs(got, []uint64{3, 1, 2}) {
		t.Fatalf("the store's slice is now %v, want it untouched at [3 1 2]", got)
	}
}

// --- never drop -------------------------------------------------------------

func TestRelayNeverAcksAnUnpublishedRecord(t *testing.T) {
	// The single claim this package cannot get wrong. An ack is a delete; an
	// ack without a publish is a silently lost audit event, and D2 assigns the
	// audit projection no other source.
	s := &fakeStore{claimable: []outbox.Record{rec(1, "a")}}
	p := &fakePublisher{reject: map[uint64]error{1: errors.New("consumer down")}}

	if _, err := newRelay(t, s, p).RunOnce(context.Background()); err == nil {
		t.Fatal("want an error")
	}

	if len(s.acked) != 0 {
		t.Fatalf("acked = %v, want none — an unpublished record must remain", s.acked)
	}
	f, found := s.failedSeq(1)
	if !found {
		t.Fatal("the record must be rescheduled, not forgotten")
	}
	if !f.RetryAt.After(epoch) {
		t.Fatalf("RetryAt = %v, want strictly after now (%v)", f.RetryAt, epoch)
	}
}

// An invalid record is a poison pill: it will never become publishable. It is
// still not dropped — it blocks its key and backs off, which is loud, where
// dropping is silent.
func TestRelayInvalidRecordIsNotPublishedAndNotDropped(t *testing.T) {
	bad := rec(1, "a")
	bad.Payload = nil
	s := &fakeStore{claimable: []outbox.Record{bad, rec(2, "a"), rec(3, "b")}}
	p := &fakePublisher{}

	res, err := newRelay(t, s, p).RunOnce(context.Background())
	if !errors.Is(err, outbox.ErrPayloadEmpty) {
		t.Fatalf("RunOnce err = %v, want it to wrap %v", err, outbox.ErrPayloadEmpty)
	}

	if !equalSeqs(p.seqs(), []uint64{3}) {
		t.Fatalf("published = %v, want only key b's record [3]", p.seqs())
	}
	if len(s.acked) != 1 || s.acked[0] != 3 {
		t.Fatalf("acked = %v, want [3]", s.acked)
	}
	if _, found := s.failedSeq(1); !found {
		t.Fatal("the invalid record must be rescheduled rather than dropped")
	}
	if res.Failed != 1 || res.Blocked != 1 {
		t.Fatalf("Result = %+v, want 1 failed / 1 blocked", res)
	}
}

// A failure on one key must not abort the pass: the error is informational, and
// the records that could be exported were.
func TestRelayReturnsAnErrorYetStillExports(t *testing.T) {
	s := &fakeStore{claimable: []outbox.Record{rec(1, "a"), rec(2, "b")}}
	p := &fakePublisher{reject: map[uint64]error{1: errors.New("consumer down")}}

	res, err := newRelay(t, s, p).RunOnce(context.Background())
	if err == nil {
		t.Fatal("want an error")
	}
	if res.Published != 1 {
		t.Fatalf("Result.Published = %d, want 1 — a non-nil error must not mean nothing shipped", res.Published)
	}
}

// --- backoff ----------------------------------------------------------------

func TestBackoffEscalatesWithAttempts(t *testing.T) {
	b := outbox.Backoff{Base: time.Second, Max: time.Hour}

	for _, tc := range []struct {
		attempts uint32
		want     time.Duration
	}{
		{0, time.Second},
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 8 * time.Second},
	} {
		if got := b.Delay(tc.attempts); got != tc.want {
			t.Fatalf("Delay(%d) = %v, want %v", tc.attempts, got, tc.want)
		}
	}
}

func TestBackoffIsCapped(t *testing.T) {
	b := outbox.Backoff{Base: time.Second, Max: 10 * time.Second}

	if got := b.Delay(20); got != 10*time.Second {
		t.Fatalf("Delay(20) = %v, want the ceiling 10s", got)
	}
}

// A shift wide enough to overflow must saturate at the ceiling rather than
// wrapping. time.Duration is an int64 of nanoseconds, so 1s << 64 is not a long
// wait — it is a NEGATIVE one, which schedules a retry in the past and turns the
// backoff into a hot loop against a consumer that is already failing.
func TestBackoffSaturatesRatherThanOverflowing(t *testing.T) {
	b := outbox.Backoff{Base: time.Second, Max: time.Hour}

	for _, attempts := range []uint32{62, 63, 64, 65, 1000, 1 << 31} {
		got := b.Delay(attempts)
		if got != time.Hour {
			t.Fatalf("Delay(%d) = %v, want the ceiling 1h (overflow must saturate)", attempts, got)
		}
	}
}

func TestBackoffValidate(t *testing.T) {
	if err := testBackoff().Validate(); err != nil {
		t.Fatalf("the valid fixture must validate, got %v", err)
	}
	for _, tc := range []struct {
		name string
		b    outbox.Backoff
	}{
		{"zero value", outbox.Backoff{}},
		{"zero base", outbox.Backoff{Base: 0, Max: time.Minute}},
		{"negative base", outbox.Backoff{Base: -time.Second, Max: time.Minute}},
		{"max below base", outbox.Backoff{Base: time.Minute, Max: time.Second}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.b.Validate(); !errors.Is(err, outbox.ErrBackoffInvalid) {
				t.Fatalf("Validate() = %v, want %v", err, outbox.ErrBackoffInvalid)
			}
		})
	}
}

// The record's own attempt count drives its backoff, not a counter the relay
// keeps. A restarted process must not reset a poison record to attempt zero and
// resume hammering a consumer that has been rejecting it for an hour.
func TestRelayBackoffUsesTheRecordsAttemptCount(t *testing.T) {
	r := rec(1, "a")
	r.Attempts = 3
	s := &fakeStore{claimable: []outbox.Record{r}}
	p := &fakePublisher{reject: map[uint64]error{1: errors.New("consumer down")}}

	if _, err := newRelay(t, s, p).RunOnce(context.Background()); err == nil {
		t.Fatal("want an error")
	}

	f, found := s.failedSeq(1)
	if !found {
		t.Fatal("want a recorded failure")
	}
	want := epoch.Add(8 * time.Second) // Base 1s << 3
	if !f.RetryAt.Equal(want) {
		t.Fatalf("RetryAt = %v, want %v (backoff from the record's 3 attempts)", f.RetryAt, want)
	}
}

// A venue that says "wait 30s" means it. Retrying sooner is guaranteed to fail
// and spends rate-limit budget §5.3 reserves for recovery.
func TestRelayHonoursRetryAfterWhenItExceedsTheBackoff(t *testing.T) {
	s := &fakeStore{claimable: []outbox.Record{rec(1, "a")}}
	p := &fakePublisher{reject: map[uint64]error{
		1: apperror.RateLimited("outbox.publish", "429", 30*time.Second, nil),
	}}

	if _, err := newRelay(t, s, p).RunOnce(context.Background()); err == nil {
		t.Fatal("want an error")
	}

	f, _ := s.failedSeq(1)
	if want := epoch.Add(30 * time.Second); !f.RetryAt.Equal(want) {
		t.Fatalf("RetryAt = %v, want %v (RetryAfter beats the 1s backoff)", f.RetryAt, want)
	}
}

// ...and the mirror. A short RetryAfter must not RESET an escalated backoff:
// the longer of the two wins in both directions, so a venue advertising 1s
// cannot undo an hour of accumulated failures.
func TestRelayKeepsTheLongerBackoffWhenRetryAfterIsShorter(t *testing.T) {
	r := rec(1, "a")
	r.Attempts = 5 // 1s << 5 = 32s
	s := &fakeStore{claimable: []outbox.Record{r}}
	p := &fakePublisher{reject: map[uint64]error{
		1: apperror.RateLimited("outbox.publish", "429", time.Second, nil),
	}}

	if _, err := newRelay(t, s, p).RunOnce(context.Background()); err == nil {
		t.Fatal("want an error")
	}

	f, _ := s.failedSeq(1)
	if want := epoch.Add(32 * time.Second); !f.RetryAt.Equal(want) {
		t.Fatalf("RetryAt = %v, want %v (the escalated backoff, not the 1s RetryAfter)", f.RetryAt, want)
	}
}

// --- cancellation -----------------------------------------------------------

// A shutdown mid-pass must lose nothing. The records not reached are left
// untouched — neither acked nor failed — so the next pass reclaims them at their
// existing RetryAt. Crucially they must not be recorded as FAILURES: a graceful
// shutdown would otherwise charge an attempt to every record it did not get to,
// and a restart would find the whole backlog backed off for a fault that never
// happened.
func TestRelayCancellationSettlesWhatItDidAndAbandonsTheRest(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	s := &fakeStore{claimable: []outbox.Record{rec(1, "a"), rec(2, "b"), rec(3, "c")}}
	p := &fakePublisher{}
	// Cancel the moment the first record is published, so the loop's ctx check
	// fires before the second.
	p.reject = map[uint64]error{}
	cancelAfterFirst := &cancellingPublisher{inner: p, cancel: cancel, after: 1}

	clk, err := clock.NewTestClock(epoch, nil)
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	r, err := outbox.NewRelay(outbox.RelayConfig{
		Store: s, Publisher: cancelAfterFirst, Clock: clk,
		Batch: 16, Backoff: testBackoff(),
	})
	if err != nil {
		t.Fatalf("NewRelay: %v", err)
	}

	res, err := r.RunOnce(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RunOnce err = %v, want it to wrap context.Canceled", err)
	}

	if !equalSeqs(p.seqs(), []uint64{1}) {
		t.Fatalf("published = %v, want only [1]", p.seqs())
	}
	if !equalSeqs(s.acked, []uint64{1}) {
		t.Fatalf("acked = %v, want [1] — the one that did publish is settled", s.acked)
	}
	if len(s.failed) != 0 {
		t.Fatalf("failed = %v, want none — unreached records are abandoned, not charged an attempt", s.failed)
	}
	if res.Claimed != 3 || res.Published != 1 || res.Failed != 0 || res.Blocked != 0 {
		t.Fatalf("Result = %+v, want 3 claimed / 1 published / 0 failed / 0 blocked", res)
	}
}

// cancellingPublisher cancels a context once it has published `after` records.
type cancellingPublisher struct {
	inner  *fakePublisher
	cancel context.CancelFunc
	after  int
	n      int
}

func (c *cancellingPublisher) Publish(ctx context.Context, r outbox.Record) error {
	err := c.inner.Publish(ctx, r)
	c.n++
	if c.n >= c.after {
		c.cancel()
	}
	return err
}

// --- store faults -----------------------------------------------------------

func TestRelaySurfacesAClaimError(t *testing.T) {
	boom := errors.New("store down")
	s := &fakeStore{claimErr: boom}
	p := &fakePublisher{}

	res, err := newRelay(t, s, p).RunOnce(context.Background())
	if !errors.Is(err, boom) {
		t.Fatalf("RunOnce err = %v, want it to wrap %v", err, boom)
	}
	if res != (outbox.Result{}) {
		t.Fatalf("Result = %+v, want the zero result", res)
	}
	if len(p.published) != 0 {
		t.Fatalf("nothing may be published when the claim failed, got %v", p.seqs())
	}
}

// An ack that does not commit means the record redelivers — which is safe, and
// is exactly the at-least-once contract. It is still an error, because a store
// that cannot ack will replay the same records forever.
func TestRelaySurfacesAnAckError(t *testing.T) {
	boom := errors.New("ack failed")
	s := &fakeStore{claimable: []outbox.Record{rec(1, "a")}, ackErr: boom}
	p := &fakePublisher{}

	res, err := newRelay(t, s, p).RunOnce(context.Background())
	if !errors.Is(err, boom) {
		t.Fatalf("RunOnce err = %v, want it to wrap %v", err, boom)
	}
	if res.Published != 1 {
		t.Fatalf("Result.Published = %d, want 1 — the publish DID happen", res.Published)
	}
}
