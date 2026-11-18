package services

import (
	"context"
	"errors"
	"strings"
	"time"

	cerrdefs "github.com/containerd/errdefs"
)

// friendlyError prefixes common Engine failures with a readable summary while
// keeping the underlying message.
func friendlyError(err error) error {
	if err == nil {
		return nil
	}
	low := strings.ToLower(err.Error())
	var prefix string
	switch {
	case errors.Is(err, context.Canceled):
		prefix = "Operation canceled"
	case errors.Is(err, context.DeadlineExceeded):
		prefix = "Operation timed out"
	case strings.Contains(low, "port is already allocated"), strings.Contains(low, "address already in use"):
		prefix = "A host port is already in use"
	case cerrdefs.IsNotFound(err), strings.Contains(low, "no such container"):
		prefix = "Container not found (it may have been removed)"
	case cerrdefs.IsConflict(err):
		prefix = "Conflict"
	case strings.Contains(low, "permission denied"):
		prefix = "Permission denied by the Docker Engine"
	}
	if prefix == "" {
		return err
	}
	return &friendly{prefix: prefix, err: err}
}

type friendly struct {
	prefix string
	err    error
}

func (f *friendly) Error() string { return f.prefix + ": " + f.err.Error() }
func (f *friendly) Unwrap() error { return f.err }

// Per-call deadlines for non-streaming Engine requests made from bound methods, so a
// hung daemon or dead SSH tunnel can never leave a UI request pending forever.
const (
	callTimeout     = 30 * time.Second
	slowCallTimeout = 2 * time.Minute // disk accounting walks every layer
)

func callCtx(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, callTimeout)
}
