package tray

import _ "embed"

// Windows takes the multi-size .ico (the shell picks the size that fits).
//
//go:embed icon.ico
var iconData []byte
