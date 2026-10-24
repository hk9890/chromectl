// Package chromectl carries the extension inside the binary, so that a
// released binary can install it without a checkout.
package chromectl

import "embed"

// Extension holds extension/ as Chrome loads it unpacked.
//
//go:embed extension
var Extension embed.FS
