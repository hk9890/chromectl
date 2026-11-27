package build

import (
	"runtime/debug"
	"testing"
)

func TestLineWithoutTheStampTakesWhatGoRecorded(t *testing.T) {
	checkout := &debug.BuildInfo{
		Main: debug.Module{Version: "v0.4.1-0.20261127161425-986fc60a1b2c+dirty"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "986fc60a1b2c3d4e5f60718293a4b5c6d7e8f901"},
			{Key: "vcs.time", Value: "2026-11-27T16:14:25Z"},
		},
	}
	goInstall := &debug.BuildInfo{Main: debug.Module{Version: "v0.5.0"}}
	noModule := &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}
	for _, c := range []struct {
		name string
		info *debug.BuildInfo
		want string
	}{
		{"a build in a git checkout", checkout, "0.4.1-0.20261127161425-986fc60a1b2c+dirty (986fc60, 2026-11-27T16:14:25Z)"},
		{"go install of a version", goInstall, "0.5.0 (unknown, unknown)"},
		{"no version recorded", noModule, "dev (unknown, unknown)"},
		{"no build info", nil, "dev (unknown, unknown)"},
	} {
		if got := line(c.info); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

func TestLineKeepsTheStamp(t *testing.T) {
	version, commit, date := Version, Commit, Date
	t.Cleanup(func() { Version, Commit, Date = version, commit, date })
	Version, Commit, Date = "0.4.0", "986fc60", "2026-11-27T16:14:25Z"

	info := &debug.BuildInfo{Main: debug.Module{Version: "v9.9.9"}, Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "ffffffffff"}}}
	if got := line(info); got != "0.4.0 (986fc60, 2026-11-27T16:14:25Z)" {
		t.Errorf("%q", got)
	}
}
