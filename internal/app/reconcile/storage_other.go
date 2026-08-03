//go:build !linux && !darwin

package reconcile

// filesystemName has no implementation on this platform, and the refusal is the
// implementation.
//
// A build for a host whose filesystem this package cannot inspect must not report
// GateJournalDurable as satisfied, so the stub returns an error rather than a
// name that would classify as local. That is the direction the whole check exists
// for: an unverifiable claim about durability is not a verified one, and this file
// is the place a reader would most expect to find the harmless-looking `return
// "ext4", nil` that quietly turns the gate into a formality on every platform
// nobody tested.
func filesystemName(string) (string, error) {
	return "", ErrJournalStorageUnsupported
}
