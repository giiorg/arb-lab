package capture

import "errors"

// Sentinel errors. Every one names a configuration defect an operator can fix
// before the process does anything, which is why they exist as sentinels at
// all: a boot refusal that can be tested with errors.Is is a boot refusal whose
// message can be improved without breaking the test.
var (
	// ErrRootRequired reports a configuration with no archive root. An empty
	// root would make the writer refuse at open with its own sentinel; refusing
	// here names the config field rather than the writer's internals.
	ErrRootRequired = errors.New("capture: archive root unset")

	// ErrDurationRequired reports a zero or negative interval or timeout. The
	// wrapped message names the field. Zero is refused because every one of
	// these is a cadence or a deadline, and §32j's two named hazards — a
	// rotation period of zero and an alive interval of zero — both fail as an
	// archive that is enormous and looks healthy.
	ErrDurationRequired = errors.New("capture: duration must be positive")

	// ErrAliveIntervalRange reports an Alive interval longer than the rotation
	// period. The expected tick count per window would be zero, and a coverage
	// threshold over an expectation of zero passes every window vacuously.
	ErrAliveIntervalRange = errors.New("capture: alive_interval exceeds rotation_period")

	// ErrQueueDepthRange reports a queue depth that could not buffer anything.
	ErrQueueDepthRange = errors.New("capture: queue_depth must be positive")

	// ErrMaxRecordRange reports a max_record outside 1..capture.MaxPayload. The
	// upper bound is the archive format's own payload cap: a socket allowed to
	// accept a larger frame would capture bytes the writer must then refuse,
	// after they were counted as captured.
	ErrMaxRecordRange = errors.New("capture: max_record outside the archive's payload bounds")

	// ErrAlivePercentRange reports a coverage threshold outside 1..100.
	ErrAlivePercentRange = errors.New("capture: min_alive_percent outside 1..100")

	// ErrBackoffRequired reports an empty reconnect schedule.
	ErrBackoffRequired = errors.New("capture: backoff schedule empty")

	// ErrSourcesRequired reports a configuration with nothing to capture. A
	// capture run over zero sources exits clean having recorded nothing, which
	// is indistinguishable from one that worked.
	ErrSourcesRequired = errors.New("capture: no sources configured")

	// ErrSourceDuplicate reports two sources with one (SourceID, Topic) key —
	// two writers over one archive tree. The flock refuses the second at
	// runtime; this refuses it at boot, naming both rows.
	ErrSourceDuplicate = errors.New("capture: duplicate capture key")

	// ErrTimeoutRequired reports a source with no first-data timeout — which
	// would disable the only check that catches a dead feed (§32k).
	ErrTimeoutRequired = errors.New("capture: first_data_timeout must be positive")

	// ErrRestTimeoutRange reports an overlay request timeout longer than the
	// Alive interval. The poll loop owns its writer single-threaded (§37f),
	// so a request in flight delays Tick by at most the timeout — bounded
	// here so the worst a slow venue may cost is one late Alive tick, never
	// a coverage finding about our own patience.
	ErrRestTimeoutRange = errors.New("capture: request_timeout exceeds alive_interval")
)
