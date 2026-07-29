//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package capture

import (
	"errors"
	"os"
	"syscall"
)

// The build tag names the platforms rather than saying `unix`, and that is not
// pedantry: `unix` also covers aix, solaris and js/wasm-adjacent targets where
// syscall.Flock is not declared, so the broader tag would fail to COMPILE there
// instead of falling through to the refusal in lock_nolock.go. A missing
// mechanism should reach its own refusal, not a build error in an unrelated file.

// flockExclusive takes a non-blocking exclusive advisory lock.
//
// Non-blocking on purpose. Blocking would make a second capture process hang
// silently at start-up, which on a laptop looks identical to a process that is
// running and capturing nothing — §31g rule 4's failure, arriving through the
// mechanism built to prevent it.
func flockExclusive(f *os.File) error {
	err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, syscall.EWOULDBLOCK):
		return ErrLockHeld
	default:
		return err
	}
}

func funlockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}
