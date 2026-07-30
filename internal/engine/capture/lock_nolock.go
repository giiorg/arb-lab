//go:build !(darwin || dragonfly || freebsd || linux || netbsd || openbsd)

package capture

import "os"

// A platform with no advisory file locking REFUSES to capture, rather than
// capturing unprotected.
//
// This is the house rule about defaults with two directions: pick the one that
// is right when somebody forgets. §32f's argument is that two writers produce an
// archive that passes every check while being wrong, so "run anyway and hope" is
// a choice to accept a silent corruption, and a silent corruption is exactly
// what the archive cannot recover from. A refusal names the platform at start-up,
// where somebody can decide; the alternative names nothing, ever.
func flockExclusive(*os.File) error { return ErrLockUnsupported }

// flockExclusiveBlocking refuses for the same reason: an unserialized manifest
// read-modify-write loses entries silently, which is the manifest lying about
// the archive — worse than the refusal that names the platform.
func flockExclusiveBlocking(*os.File) error { return ErrLockUnsupported }

func funlockFile(*os.File) error { return nil }
