// Package version holds build-time metadata for flakehunter.
package version

import (
	"fmt"
	"runtime"
)

// Populated at build time via -ldflags.
var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

// Info bundles the version metadata reported by the version command.
type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"buildDate"`
	GoVersion string `json:"goVersion"`
	Platform  string `json:"platform"`
}

// Current returns the version metadata for this binary.
func Current() Info {
	return Info{
		Version:   Version,
		Commit:    Commit,
		BuildDate: BuildDate,
		GoVersion: runtime.Version(),
		Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}

// String renders the version info as a single human-readable line.
func (i Info) String() string {
	return fmt.Sprintf("flakehunter %s (commit %s, built %s) %s %s",
		i.Version, i.Commit, i.BuildDate, i.GoVersion, i.Platform)
}
