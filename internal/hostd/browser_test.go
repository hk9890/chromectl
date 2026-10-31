package hostd

import "testing"

func TestBrowserNameComesFromTheExecutablesDirectory(t *testing.T) {
	for exe, want := range map[string]string{
		"/opt/google/chrome/chrome":                           "chrome",
		"/opt/google/chrome-beta/chrome":                      "chrome-beta",
		"/opt/google/chrome-unstable/chrome":                  "chrome-unstable",
		"/opt/google/chrome/chrome (deleted)":                 "chrome",
		"/opt/brave.com/brave/brave":                          "brave",
		"/opt/brave.com/brave-beta/brave":                     "brave-beta",
		"/usr/lib/chromium/chromium":                          "chromium",
		"/snap/chromium/3049/usr/lib/chromium-browser/chrome": "chromium",
		"/usr/lib64/chromium-browser/chromium-browser":        "chromium",
		"/usr/bin/vivaldi.bin":                                "vivaldi_bin",
	} {
		if got := browserName(exe); got != want {
			t.Errorf("browserName(%q) = %q, want %q", exe, got, want)
		}
	}
}
