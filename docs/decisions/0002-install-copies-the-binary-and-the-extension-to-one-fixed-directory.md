# 0002: `install` copies the binary and the extension to one fixed directory, and the native messaging manifest names the copy

A release that mise installs lives in a directory named after its version. One fixed directory
makes an upgrade `mise upgrade`, `chromectl install` and a reload of the extension's card.

Rejected: a manifest that names the binary that ran `install`, with the extension loaded from where
it lies, because after an upgrade the manifest keeps starting the old version, and the loaded
extension vanishes with the old version's directory.
