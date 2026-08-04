//go:build !darwin && !linux

package transport

type spendLockHandle struct{}

func acquireSpendLockHandle(string, SpendDomain) (*spendLockHandle, error) {
	return nil, ErrSpendLockUnsupported
}

func (*spendLockHandle) close() error { return nil }
