package system

import "syscall"

// diskUsage reports each mount once, even when several paths share a
// filesystem.
func diskUsage(paths []string) []Disk {
	disks := []Disk{}
	seen := map[[2]int32]bool{}
	for _, p := range paths {
		var st syscall.Statfs_t
		if err := syscall.Statfs(p, &st); err != nil || st.Blocks == 0 {
			continue
		}
		if seen[st.Fsid.X__val] {
			continue
		}
		seen[st.Fsid.X__val] = true
		bsize := uint64(st.Bsize)
		total := st.Blocks * bsize
		disks = append(disks, Disk{Path: p, Total: total, Used: total - st.Bfree*bsize})
	}
	return disks
}
