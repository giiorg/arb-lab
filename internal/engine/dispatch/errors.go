package dispatch

import "errors"

// Sentinel errors. Callers branch on these with errors.Is. Everything here is a
// wiring or producer defect rather than a runtime condition — the dispatcher's
// refusals are loud on purpose, because each one names a failure that would
// otherwise corrupt the replay key silently.
var (
	// ErrControlLaneNil, ErrMarketLaneNil, ErrTimerSourceNil and ErrSinkNil
	// report an unpopulated Config — the zero value is unusable (§11). Each
	// input is named separately so the refusal says which wire is missing
	// rather than that one of four is.
	ErrControlLaneNil = errors.New("dispatch: control lane is nil")
	ErrMarketLaneNil  = errors.New("dispatch: market-data lane is nil")
	ErrTimerSourceNil = errors.New("dispatch: timer source is nil")
	ErrSinkNil        = errors.New("dispatch: sink is nil")

	// ErrIngressSeqMinted reports an envelope that arrived already carrying an
	// IngressSeq. §3.5: the sequence is assigned ONLY at dispatch, and "a
	// producer that minted one would be a second authority over the replay
	// key". Accepting it — or worse, honouring it — would let a producer decide
	// an order that is exactly this package's to decide.
	ErrIngressSeqMinted = errors.New("dispatch: envelope arrived with IngressSeq already stamped")

	// ErrPrioInconstant reports a source whose envelope disagrees with the Prio
	// its first dispatched envelope fixed. §3.5 makes constancy the producer's
	// obligation; this is the checked half (§39f-5). The failure it prevents is
	// silent by construction: a producer that escalates priority on a WORSE
	// reading reorders that source inside its own mailbox drain, so a consumer
	// watching a degrading feed sees it recover.
	ErrPrioInconstant = errors.New("dispatch: source changed its Prio")
)

// The dispatch log's refusals (§39n). The container's own refusals — checksum,
// torn tail, undeclared kind, zero-filled region — arrive wrapped from
// engine/capture and keep their sentinels; these are the rules the LOG adds on
// top of the frame.
var (
	// ErrLogClosed reports a call on a LogWriter after Close. One order was
	// sealed; entries arriving after it belong to no log.
	ErrLogClosed = errors.New("dispatch: log writer is closed")

	// ErrLogConsumerNil reports a replay with nobody to re-drive — the zero
	// value is unusable (§11), and a nil consumer would make ReplayLog a file
	// checker that silently discards the order it exists to deliver.
	ErrLogConsumerNil = errors.New("dispatch: log consumer is nil")

	// ErrLogSeqNotDense reports a sequence position that is not exactly
	// last+1. On the writer it is the density ASSERTION (§39n-5): the
	// dispatcher guarantees density and the writer verifies it with one
	// comparison rather than trusting it — which also refuses a LogWriter
	// joined to a dispatcher that has already dispatched, a log that would lie
	// about where the order began. On the reader it is corruption: this log is
	// written in-process, so a hole is never a drop (capture's reading), it is
	// a wrong file.
	ErrLogSeqNotDense = errors.New("dispatch: log sequence is not dense")

	// ErrLogTimeRange reports a SourceTime outside what Unix nanoseconds can
	// hold. UnixNano wraps silently outside ~1678..2262, and a wrapped stamp
	// would archive a plausible instant in the wrong century — the codec's own
	// RecvTime rule, applied to the one time field the payload carries.
	ErrLogTimeRange = errors.New("dispatch: log time is not representable in Unix nanoseconds")

	// ErrLogKind reports a frame kind outside the log's roster {Data, Close}.
	// The container declares eight kinds; this file format uses two, and a
	// valid capture kind appearing here means the file is not a dispatch log —
	// most plausibly an archive file handed to the wrong reader, which is
	// exactly the mistake one shared magic makes possible and this refusal
	// makes loud.
	ErrLogKind = errors.New("dispatch: record kind does not belong in a dispatch log")

	// ErrLogEntryInvalid and ErrLogEntryUnknown report a Data payload that is
	// not a sequence record: bytes that do not parse (or decode to an invalid
	// SourceID — the reader re-derives validity, §39n-6), versus a `k` token
	// this build does not declare. Split on the standing two-mistakes
	// reasoning: the first is corruption or a broken writer, the second is an
	// archive from a newer build — the reader's version refusal, since the
	// token IS the payload's version surface (§39n-3).
	ErrLogEntryInvalid = errors.New("dispatch: log entry payload does not decode")
	ErrLogEntryUnknown = errors.New("dispatch: log entry token is not a declared one")

	// ErrLogSealMismatch reports a Close seal that disagrees with the file it
	// seals — wrong final seq, wrong entry count, or a seal over zero entries,
	// which the writer never produces. A seal that asserts nothing seals
	// nothing (§39n-4), so a disagreeing one is refused rather than trusted in
	// either direction.
	ErrLogSealMismatch = errors.New("dispatch: Close seal disagrees with the entries it seals")

	// ErrLogRecordAfterSeal reports bytes after the Close seal. The seal's
	// whole meaning is "this order ended here"; a record beyond it is a second
	// ending, and a log with two endings has none.
	ErrLogRecordAfterSeal = errors.New("dispatch: record after the Close seal")
)
