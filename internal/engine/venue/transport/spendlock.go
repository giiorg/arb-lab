package transport

import (
	"fmt"
	"path/filepath"
	"sync"
)

// SpendLock is the process-lifetime proof that this host has one budget spender
// for a closed exchange domain, bound to the credential the signer carries. Its
// file is <dir>/<domain>.lock, contains no secret, and is never deleted: unlinking
// a file another process may already be flocking creates a second inode and a
// classic split-lock race. The persistent file is only a rendezvous point; the
// flock on its open handle is the lock.
type SpendLock struct {
	mu          sync.Mutex
	domain      SpendDomain
	fingerprint string
	path        string
	handle      *spendLockHandle
}

// Domain returns the compiled budget namespace held by the token.
func (l *SpendLock) Domain() SpendDomain { return l.domain }

// Fingerprint returns the credential identity bound to the token.
func (l *SpendLock) Fingerprint() string { return l.fingerprint }

// Close releases the process-lifetime token. Idempotence lets the root join a
// cleanup error on every ownership boundary without risking a double-release.
func (l *SpendLock) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.handle == nil {
		return nil
	}
	err := l.handle.close()
	l.handle = nil
	return err
}

// AcquireSpendLock takes an exclusive, non-blocking lock for a spend domain.
// Domain and credential identities are derived inside this constructor so a
// caller cannot make two spenders appear different by supplying metadata.
func AcquireSpendLock(dir string, domain SpendDomain, cred Credential) (*SpendLock, error) {
	if err := domain.Validate(); err != nil {
		return nil, err
	}
	if err := cred.Validate(); err != nil {
		return nil, err
	}
	if dir == "" {
		return nil, ErrSpendLockDirUnset
	}
	path := filepath.Join(dir, string(domain)+".lock")
	handle, err := acquireSpendLockHandle(path, domain)
	if err != nil {
		return nil, fmt.Errorf("acquire spend lock for %s at %q: %w", domain, path, err)
	}
	return &SpendLock{
		domain: domain, fingerprint: cred.Fingerprint(), path: path, handle: handle,
	}, nil
}
