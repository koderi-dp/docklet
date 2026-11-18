//go:build !windows

package tray

import _ "embed"

// Linux (StatusNotifier) and macOS take a PNG.
//
//go:embed icon.png
var iconData []byte
