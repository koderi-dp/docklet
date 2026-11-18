//go:build !windows

package wslinfo

// NewHostReader returns nil: the Windows-side figures do not exist here.
func NewHostReader() HostReader { return nil }
