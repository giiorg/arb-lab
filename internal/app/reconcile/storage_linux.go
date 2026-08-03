//go:build linux

package reconcile

import (
	"fmt"
	"syscall"
)

// filesystemMagics maps Linux's statfs f_type to the names classify() decides on.
//
// The magic numbers are the kernel's own (uapi/linux/magic.h) and are stable ABI:
// a filesystem's magic cannot change without breaking every program that reads
// it, which is what makes this table a fact rather than a heuristic. What it is
// NOT is complete — that is classify's default-deny's whole job.
var filesystemMagics = map[int64]string{
	0xEF53:     "ext2/3/4",
	0x58465342: "xfs",
	0x9123683E: "btrfs",
	0x2FC12FC1: "zfs",
	0x01021994: "tmpfs",
	0x858458F6: "ramfs",
	0x6969:     "nfs",
	0x517B:     "smb",
	0xFE534D42: "smb2",
	0xFF534D42: "cifs",
	0x5346414F: "afs",
	0x6B414653: "afs",
	0x00C36400: "ceph",
	0x01021997: "9p",
	0x0BD00BD0: "lustre",
	0x7461636F: "ocfs2",
	0x01161970: "gfs2",
	0x65735546: "fuse",
	0x794C7630: "overlay",
}

// filesystemName is the Linux half of #168's check.
//
// It returns the kernel's name for the filesystem under dir, or the magic in hex
// when the table does not hold it — which classify then refuses. The hex is
// deliberately carried into the refusal rather than replaced by "unknown": it is
// the one thing an operator can look up, and it is what a later edit to the table
// above is written from.
func filesystemName(dir string) (string, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return "", fmt.Errorf("statfs: %w", err)
	}
	magic := int64(st.Type)
	if name, ok := filesystemMagics[magic]; ok {
		return name, nil
	}
	return fmt.Sprintf("0x%x", uint64(magic)), nil
}
