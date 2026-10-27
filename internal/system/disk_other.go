//go:build !linux

package system

// diskUsage is only implemented on Linux, the platform Haepage targets.
func diskUsage(paths []string) []Disk { return []Disk{} }
