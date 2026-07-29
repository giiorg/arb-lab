package capture

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

// §32f's single-writer invariant, and it is worth restating what it defends
// against, because the failure is silent:
//
//	Two capture processes over one root interleave CaptureSeq into something
//	that looks exactly like a healthy dense log.
//
// Every check passes. The sequence has no holes, the CRCs verify, the digests
// match. What is wrong is that half the frames belong to a different socket, and
// nothing in the file says so. And this is not hypothetical — the per-stream
// layout makes "run a second process for the new source" an attractive way to
// add a source without restarting the others.

// holder is what a lock file contains: enough to name the process holding it in
// the refusal message, and nothing that has to stay true.
//
// It is advisory information, not state. A stale holder record from a process
// that was SIGKILLed is harmless, because the kernel released the lock when the
// file descriptor closed — which is exactly why the lock is an flock and not
// this file's existence. A lock made of a file's existence needs a liveness
// check, a stale-lock timeout, and a policy for what to do when the timeout is
// wrong; an flock has none of those questions because the kernel answers them.
type holder struct {
	PID   int    `json:"pid"`
	Host  string `json:"host"`
	Since string `json:"since"`
}

func (h holder) String() string {
	if h.PID == 0 {
		return "an unidentified process"
	}
	host := h.Host
	if host == "" {
		host = "an unnamed host"
	}
	if h.Since == "" {
		return fmt.Sprintf("pid %d on %s", h.PID, host)
	}
	return fmt.Sprintf("pid %d on %s since %s", h.PID, host, h.Since)
}

// streamLock is a held exclusive lock on one stream's directory.
type streamLock struct {
	f    *os.File
	path string
}

// acquireLock takes the exclusive lock for a stream, or reports who holds it.
//
// The identity is written AFTER the lock is taken, which leaves a window where a
// refused writer reads an empty file and can only say "an unidentified process".
// That is the right way round: writing first would let a process that then FAILS
// to take the lock leave its own name behind as the apparent holder, which is a
// message that actively misleads. An honest "somebody, and I could not tell you
// who" beats a confident wrong name.
func acquireLock(path string, now time.Time) (*streamLock, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, fmt.Errorf("capture: open lock file: %w", err)
	}
	if err := flockExclusive(f); err != nil {
		h := readHolder(f)
		_ = f.Close()
		if errors.Is(err, ErrLockHeld) {
			return nil, fmt.Errorf("capture: %s is held by %s: %w", path, h, ErrLockHeld)
		}
		return nil, fmt.Errorf("capture: lock %s: %w", path, err)
	}

	host, err := os.Hostname()
	if err != nil {
		// Not fatal. A hostname is for the refusal message, and refusing to
		// capture because a host could not name itself would trade a real
		// archive for a cosmetic field.
		host = ""
	}
	body, err := json.Marshal(holder{PID: os.Getpid(), Host: host, Since: now.UTC().Format(time.RFC3339Nano)})
	if err != nil {
		_ = releaseLock(f)
		return nil, fmt.Errorf("capture: encode lock holder: %w", err)
	}
	if err := writeWhole(f, body); err != nil {
		_ = releaseLock(f)
		return nil, fmt.Errorf("capture: write lock holder: %w", err)
	}
	return &streamLock{f: f, path: path}, nil
}

// Release drops the lock.
//
// The lock FILE is deliberately left behind. Deleting it is the classic mistake:
// a second process may already have the file open and be blocked on the flock,
// and unlinking the path it holds leaves two processes locking two different
// inodes under one name — both succeed, and the invariant this whole file exists
// to enforce is gone with every check still passing. An orphan dotfile per
// stream is the cheap half of that trade.
func (l *streamLock) Release() error {
	if l == nil || l.f == nil {
		return nil
	}
	f := l.f
	l.f = nil
	return releaseLock(f)
}

func releaseLock(f *os.File) error {
	unlockErr := funlockFile(f)
	closeErr := f.Close()
	return errors.Join(unlockErr, closeErr)
}

// writeWhole replaces the file's contents with body.
func writeWhole(f *os.File, body []byte) error {
	if err := f.Truncate(0); err != nil {
		return err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if _, err := f.Write(body); err != nil {
		return err
	}
	return f.Sync()
}

// readHolder reads whatever identity the current holder left, tolerating every
// way it can be absent. It is called on the failure path only, where the answer
// is a message rather than a decision, so no error it could return would change
// what the caller does.
func readHolder(f *os.File) holder {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return holder{}
	}
	body, err := io.ReadAll(io.LimitReader(f, 4<<10))
	if err != nil {
		return holder{}
	}
	var h holder
	if err := json.Unmarshal(body, &h); err != nil {
		return holder{}
	}
	return h
}
