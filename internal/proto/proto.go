// Package proto is the one message shape both sides of chromectl speak: the
// CLI to the host over the socket, and the host to the extension over native
// messaging. The host assigns Seq; the extension echoes it.
package proto

import "encoding/json"

// Request names a command and carries its arguments flat, as the extension
// reads them. Only the fields a command uses are set.
type Request struct {
	Seq    uint64 `json:"seq,omitempty"`
	Cmd    string `json:"cmd"`
	Tab    int    `json:"id,omitempty"`
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

// ExtensionID is fixed by the "key" field in extension/manifest.json.
const ExtensionID = "mpnpeobidlmllklmdpnkhhhblkbhpmcp"

// HostName is the native messaging host the extension connects to.
const HostName = "com.hk9890.chromectl"
