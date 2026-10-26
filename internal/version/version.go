// Package version holds build information, set at link time with
// -ldflags "-X github.com/haelahpx/haepage/internal/version.Version=...".
package version

import "fmt"

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// String returns the one-line version shown by --version.
func String() string {
	return fmt.Sprintf("haepage %s (%s)", Version, Commit)
}
