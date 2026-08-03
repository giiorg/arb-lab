//go:build darwin

package reconcile

import (
	"fmt"
	"syscall"
)

// filesystemName is the darwin half of #168's check.
//
// darwin is easier than Linux and the difference is worth naming, because it is
// why this file is three lines of work and the Linux one is a table: BSD's statfs
// carries the filesystem's NAME ("apfs", "nfs", "smbfs") rather than a magic
// number, so there is nothing to look up and nothing to keep in step with a
// kernel header.
//
// This is the DEVELOPMENT host and not the deployment target (§11: Linux,
// ap-northeast-1). It exists so the check is exercised where the tests are run
// rather than compiled out into a stub that would pass — a check that is a no-op
// on the machine a developer sees is a check nobody notices is broken.
func filesystemName(dir string) (string, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return "", fmt.Errorf("statfs: %w", err)
	}
	name := make([]byte, 0, len(st.Fstypename))
	for _, c := range st.Fstypename {
		if c == 0 {
			break
		}
		name = append(name, byte(c))
	}
	return string(name), nil
}
