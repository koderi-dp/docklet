package compose

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Runner executes commands. Production uses ExecRunner; tests use a fake.
type Runner interface {
	// Run streams merged stdout/stderr lines to onLine and returns when the process
	// ends. Cancelling ctx kills the process.
	Run(ctx context.Context, c Command, onLine func(string)) error
	// Output runs to completion and returns stdout; stderr is added to the error.
	Output(ctx context.Context, c Command) (string, error)
}

// ExecRunner runs real processes without a shell and without a console window.
type ExecRunner struct{}

func newCmd(ctx context.Context, c Command) *exec.Cmd {
	cmd := exec.CommandContext(ctx, c.Name, c.Args...)
	if len(c.EnvSet) > 0 || len(c.EnvUnset) > 0 {
		cmd.Env = MergeEnv(os.Environ(), c.EnvSet, c.EnvUnset)
	}
	if len(c.Stdin) > 0 {
		cmd.Stdin = bytes.NewReader(c.Stdin)
	}
	cmd.WaitDelay = 3 * time.Second
	hideWindow(cmd)
	killTree(cmd)
	return cmd
}

func (ExecRunner) Output(ctx context.Context, c Command) (string, error) {
	cmd := newCmd(ctx, c)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return out.String(), ctx.Err()
		}
		if msg := strings.TrimSpace(errb.String()); msg != "" {
			return out.String(), fmt.Errorf("%w: %s", err, msg)
		}
		return out.String(), err
	}
	return out.String(), nil
}

func (ExecRunner) Run(ctx context.Context, c Command, onLine func(string)) error {
	cmd := newCmd(ctx, c)
	pr, pw := io.Pipe()
	cmd.Stdout, cmd.Stderr = pw, pw
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		sc := bufio.NewScanner(pr)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		sc.Split(splitLines)
		for sc.Scan() {
			if t := strings.TrimSpace(sc.Text()); t != "" {
				onLine(t)
			}
		}
		_, _ = io.Copy(io.Discard, pr)
	}()
	err := cmd.Wait()
	_ = pw.Close()
	<-done
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

// splitLines splits on \n, \r\n and bare \r (Compose progress output redraws with \r).
func splitLines(data []byte, atEOF bool) (int, []byte, error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexAny(data, "\r\n"); i >= 0 {
		adv := i + 1
		if data[i] == '\r' && adv < len(data) && data[adv] == '\n' {
			adv++
		}
		return adv, data[:i], nil
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}
