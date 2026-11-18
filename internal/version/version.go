// Package version holds the application version. Release builds set it with
// -ldflags "-X docklet/internal/version.Version=0.1.0"; development builds report "dev".
package version

// Version is the running application version without a leading "v".
var Version = "dev"
