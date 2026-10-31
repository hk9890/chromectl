// Package proto is the one message shape both sides of chromectl speak: the
// CLI to the host over the socket, and the host to the extension over native
// messaging. The host assigns Seq; the extension echoes it.
package proto

import "encoding/json"

// Request names a command and carries its arguments flat, as the extension
// reads them. Only the fields a command uses are set. The id is always sent:
// Chrome reads a missing tab id as the current tab, and a 0 as no tab.
type Request struct {
	Seq    uint64 `json:"seq,omitempty"`
	Cmd    string `json:"cmd"`
	Tab    int    `json:"id"`
	URL    string `json:"url,omitempty"`
	Window int    `json:"window,omitempty"`
}

// Reply carries the command's result, or the reason it failed.
type Reply struct {
	Seq    uint64          `json:"seq"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

// Tab is one entry of a list result.
type Tab struct {
	ID     int    `json:"id"`
	Window int    `json:"window"`
	Active bool   `json:"active"`
	Title  string `json:"title"`
	URL    string `json:"url"`
}

// Identity is the result of the identify request the host sends the
// extension when it starts: the profile's id, ProfileID.
type Identity struct {
	Profile string `json:"profile"`
}

// ProfileID is the form of a profile id, as a regular expression: 8 hex
// digits, which the extension makes and the host, the socket name and every
// tab id carry.
const ProfileID = `[0-9a-f]{8}`

// ChromeGone begins the error of a call the host could not finish because
// Chrome closed its pipe, or could no longer be written to.
const ChromeGone = "chrome went away"

// StaleExtension is the answer of a host whose extension is older than its
// binary and does not know identify.
const StaleExtension = "the chromectl extension is older than chromectl: reload it at chrome://extensions"

// ExtensionID is fixed by the "key" field in extension/manifest.json.
const ExtensionID = "mpnpeobidlmllklmdpnkhhhblkbhpmcp"

// HostName is the native messaging host the extension connects to.
const HostName = "com.hk9890.chromectl"
