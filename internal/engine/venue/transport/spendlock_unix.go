//go:build darwin || linux

package transport

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

type spendLockHandle struct {
	file *os.File
}

func acquireSpendLockHandle(path string, domain SpendDomain) (*spendLockHandle, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		closeErr := f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, errors.Join(fmt.Errorf("domain %s path %q: %w", domain, path, ErrSpendLockHeld), closeErr)
		}
		return nil, errors.Join(err, closeErr)
	}
	return &spendLockHandle{file: f}, nil
}

func (h *spendLockHandle) close() error {
	err := syscall.Flock(int(h.file.Fd()), syscall.LOCK_UN)
	return errors.Join(err, h.file.Close())
}
