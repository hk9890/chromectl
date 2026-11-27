// Package build carries the version stamped in by the build task and by
// GoReleaser.
package build

import (
	"fmt"
	"runtime/debug"
	"strings"
)

var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

// Line is the version, commit and date `chromectl version` prints. A build
// without the stamp, as `go build` or `go install`, takes what Go recorded in
// the binary instead.
func Line() string {
	info, _ := debug.ReadBuildInfo()
	return line(info)
}

func line(info *debug.BuildInfo) string {
	version, commit, date := Version, Commit, Date
	if version == "dev" && info != nil {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			version = strings.TrimPrefix(v, "v")
		}
		for _, s := range info.Settings {
			switch s.Key {
			case "vcs.revision":
				commit = s.Value[:min(7, len(s.Value))]
			case "vcs.time":
				date = s.Value
			}
		}
	}
	return fmt.Sprintf("%s (%s, %s)", version, commit, date)
}
