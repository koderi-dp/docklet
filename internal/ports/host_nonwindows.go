//go:build !windows

package ports

import (
	"context"
	"errors"
)

// unavailable is a host source that cannot be read on this platform.
type unavailable struct{ where string }

func (u unavailable) Where() string { return u.where }
func (unavailable) Listeners(context.Context) ([]Listener, error) {
	return nil, errors.New("not supported on this operating system")
}

// NewWindowsHost exists so callers compile everywhere; Windows is only readable on Windows.
func NewWindowsHost() HostSource { return unavailable{where: "Windows"} }
