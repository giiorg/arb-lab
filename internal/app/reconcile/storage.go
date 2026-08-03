package reconcile

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// This file is task #168: §13's local-filesystem-only precondition, checked at
// §7.5's boot rather than in the journal adapter's constructor.
//
// # Why the boot list and not the constructor, which §11 decided and this obeys
//
// The adapter's constructor opens a file. This check is about the HOST that file
// is on, which is deployment's business and not the store's — and, decisively, a
// constructor reports by returning an error, which for a boot phase is an unwind
// and a dead process. §7.5 needs a verdict a live process can carry: the answer
// to "is this journal durable" is a posture, and only the checklist can hold one.
//
// # What it is actually protecting, which is not tidiness about mount tables
//
// §7.5 states the tamper-evidence's limit in as many words: a hash chain "proves
// nothing about records that were never seen", so truncation at the tail is
// undetectable without an independently durable head, which this milestone does
// not build. A network filesystem is precisely the condition under which records
// go missing with every remaining link still verifying — SQLite documents NFS and
// friends as corruption-prone, and its locking assumptions do not hold there. So
// the journal's verification would keep saying yes about a log that had lost its
// last records, which is the one failure the chain cannot report.
//
// # The gate is DURABILITY and §13 named only one of the two ways to lose it
//
// [SPEC EDIT LANDED — v0.9.39] §11/§13 say "local filesystem only — ext4/xfs with
// write barriers, never NFS/network FS", and read literally that admits tmpfs,
// which is local, fast, honours fsync, and loses the entire journal on the next
// reboot. A journal that does not survive a restart fails at exactly the moment
// §7.5 exists for: the process comes back, replays a log that is not there, finds
// no record of orders the venue is holding, and the venue's whole standing
// position reports as an unexplained delta. Locality is a proxy for durability
// and it is not the property, so this check refuses both classes and says which.

// filesystemClass is what one filesystem is, for the purpose of holding a
// write-ahead journal.
type filesystemClass uint8

const (
	// classUnknown is a filesystem no table below names. It is the DEFAULT and it
	// refuses — see ErrJournalStorageUnknown.
	classUnknown filesystemClass = iota

	// classLocal is a local, disk-backed filesystem that survives a reboot.
	classLocal

	// classRemote is a network filesystem: §13's forbidden case.
	classRemote

	// classVolatile is local, honours fsync, and does not survive a reboot.
	classVolatile
)

// localFilesystems, remoteFilesystems and volatileFilesystems are the three
// tables, and everything absent from all three is classUnknown.
//
// Membership is the direction that matters: a filesystem is admitted only by
// being NAMED, so a network filesystem arriving under a name nobody has seen — a
// new client, a FUSE driver, a vendor's own module — is refused rather than
// silently trusted. The cost of the other direction is a legitimate local
// filesystem producing a boot that will not start with the filesystem's name in
// the message, which one edit fixes.
var (
	localFilesystems = map[string]struct{}{
		"ext2/3/4": {}, // §11 names ext4 explicitly
		"xfs":      {}, // and xfs
		"btrfs":    {},
		"zfs":      {},
		"apfs":     {}, // the development host; the deployment target is Linux
		"hfs":      {},
		"ufs":      {},
	}
	remoteFilesystems = map[string]struct{}{
		"nfs":     {},
		"smb":     {},
		"smb2":    {},
		"cifs":    {},
		"smbfs":   {},
		"afs":     {},
		"ceph":    {},
		"9p":      {},
		"lustre":  {},
		"ocfs2":   {},
		"gfs2":    {},
		"webdav":  {},
		"fuse":    {}, // sshfs, s3fs and every other remote store wearing a local name
		"fusefs":  {},
		"osxfuse": {},
		"macfuse": {},
	}
	volatileFilesystems = map[string]struct{}{
		"tmpfs": {},
		"ramfs": {},
	}
)

// classify says which class a filesystem name belongs to.
//
// A pure function over a name so the decision is testable without a mount: the
// syscall half is per-GOOS and can only be exercised on the host running the
// test, and a check whose whole content is a table would otherwise be verified on
// exactly one filesystem — the one the test machine happens to use.
func classify(name string) filesystemClass {
	name = strings.ToLower(name)
	if _, ok := remoteFilesystems[name]; ok {
		return classRemote
	}
	if _, ok := volatileFilesystems[name]; ok {
		return classVolatile
	}
	if _, ok := localFilesystems[name]; ok {
		return classLocal
	}
	return classUnknown
}

// checkJournalStorage reports whether the journal at path may be trusted as
// durable evidence on this host.
//
// It inspects the DIRECTORY rather than the file, because on a first boot the
// file does not exist yet and the question is about the mount either way. The
// syscall follows symlinks, so a local-looking path linked onto a network mount
// is answered about the mount.
func checkJournalStorage(path string) error {
	dir := filepath.Dir(path)
	name, err := filesystemName(dir)
	if err != nil {
		return fmt.Errorf("journal directory %q: %w", dir, err)
	}
	switch classify(name) {
	case classLocal:
		return nil
	case classRemote:
		return fmt.Errorf("journal directory %q is on %q: %w", dir, name, ErrJournalStorageRemote)
	case classVolatile:
		return fmt.Errorf("journal directory %q is on %q, which does not survive a reboot: %w",
			dir, name, ErrJournalStorageVolatile)
	case classUnknown:
		return fmt.Errorf("journal directory %q is on %q, which is in no admitted set (%s): %w",
			dir, name, admittedNames(), ErrJournalStorageUnknown)
	}
	// Unreachable while classify is total, and named rather than dropped: a class
	// added without a case here must refuse, not pass.
	return fmt.Errorf("journal directory %q is on %q: %w", dir, name, ErrJournalStorageUnknown)
}

// admittedNames is the allowlist in the refusal, so an operator reading it knows
// what to move the journal onto rather than only what it is on now.
func admittedNames() string {
	names := make([]string, 0, len(localFilesystems))
	for n := range localFilesystems {
		names = append(names, n)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
