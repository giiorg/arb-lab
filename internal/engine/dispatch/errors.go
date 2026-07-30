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
