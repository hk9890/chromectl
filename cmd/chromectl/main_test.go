package main

import "testing"

// The wire omits an id of 0, and Chrome reads a missing tab id as the
// current tab, so the CLI must refuse it before it calls the host.
func TestATabOrWindowIdBelowOneIsRefused(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	for _, arg := range []string{"0", "-3"} {
		if err := tabCommand("activate", []string{arg}); err == nil || err.Error() != `tab id "`+arg+`" is not a positive number` {
			t.Errorf("activate %s: err = %v", arg, err)
		}
	}
	if err := open([]string{"https://example.com", "0"}); err == nil || err.Error() != `window id "0" is not a positive number` {
		t.Errorf("open url 0: err = %v", err)
	}
}
