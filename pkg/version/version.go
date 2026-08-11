// Package version holds build-time metadata for flakehunter.
package version

import (
	"fmt"
	"runtime"
	"runtime/debug"
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
//
// Release builds inject the values above via -ldflags. A binary produced by
// `go install` gets no ldflags, so it would otherwise report "dev" forever —
// the module version and VCS stamps embedded by the toolchain are read back
// here to fill the gap.
func Current() Info {
	info := Info{
		Version:   Version,
		Commit:    Commit,
		BuildDate: BuildDate,
		GoVersion: runtime.Version(),
		Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}

	build, ok := debug.ReadBuildInfo()
	if !ok {
		return info
	}

	if info.Version == "dev" && build.Main.Version != "" && build.Main.Version != "(devel)" {
		info.Version = build.Main.Version
	}

	for _, setting := range build.Settings {
		switch setting.Key {
		case "vcs.revision":
			if info.Commit == "none" && setting.Value != "" {
				info.Commit = shortCommit(setting.Value)
			}
		case "vcs.time":
			if info.BuildDate == "unknown" && setting.Value != "" {
				info.BuildDate = setting.Value
			}
		}
	}

	return info
}

func shortCommit(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// String renders the version info as a single human-readable line.
func (i Info) String() string {
	return fmt.Sprintf("flakehunter %s (commit %s, built %s) %s %s",
		i.Version, i.Commit, i.BuildDate, i.GoVersion, i.Platform)
}
