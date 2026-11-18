package ports

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
)

// runFunc runs a command and returns its standard output. Replaced in tests.
type runFunc func(ctx context.Context, name string, args ...string) ([]byte, error)

func execRun(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	hideWindow(cmd)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		if msg := bytes.TrimSpace(errb.Bytes()); len(msg) > 0 && len(msg) < 300 {
			return out.Bytes(), &runError{err: err, msg: string(bytes.ReplaceAll(msg, []byte{0}, nil))}
		}
		return out.Bytes(), err
	}
	return out.Bytes(), nil
}

type runError struct {
	err error
	msg string
}

func (e *runError) Error() string { return e.err.Error() + ": " + e.msg }
func (e *runError) Unwrap() error { return e.err }

func exitCode(err error) int {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return -1
}
