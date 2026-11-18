package wsldocker

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os/exec"
	"regexp"
	"strings"
)

// Cmd is one command to run. Argv is passed to wsl.exe as separate arguments,
// never through a shell, so no part of it is ever re-parsed.
type Cmd struct {
	Argv  []string
	Root  bool   // run as root inside the distribution (-u root)
	Stdin string // optional text for the command's standard input
	Host  bool   // Argv are wsl.exe arguments themselves (for example --terminate), not a command in the distribution
}

// Exec runs commands for one distribution and reports each output line.
type Exec interface {
	Run(ctx context.Context, distro string, c Cmd, onLine func(string)) error
}

// wslExec runs commands through wsl.exe.
type wslExec struct{}

// NewExec returns the real executor.
func NewExec() Exec { return wslExec{} }

// wslArgs builds the wsl.exe argument list for a command.
func wslArgs(distro string, c Cmd) []string {
	if c.Host {
		return c.Argv
	}
	args := []string{"-d", distro}
	if c.Root {
		args = append(args, "-u", "root")
	}
	args = append(args, "--exec")
	return append(args, c.Argv...)
}

func (wslExec) Run(ctx context.Context, distro string, c Cmd, onLine func(string)) error {
	cmd := exec.CommandContext(ctx, "wsl.exe", wslArgs(distro, c)...)
	hideWindow(cmd)
	if c.Stdin != "" {
		cmd.Stdin = strings.NewReader(c.Stdin)
	}
	pr, pw := io.Pipe()
	cmd.Stdout, cmd.Stderr = pw, pw
	done := make(chan struct{})
	go func() {
		defer close(done)
		sc := bufio.NewScanner(pr)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		sc.Split(splitLines)
		for sc.Scan() {
			onLine(cleanLine(sc.Bytes())) // empty lines are kept: file contents are read through here
		}
		_, _ = io.Copy(io.Discard, pr)
	}()
	err := cmd.Run()
	_ = pw.Close()
	<-done
	if err != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

// splitLines splits on \n and on \r so progress output (curl, apt) becomes lines.
func splitLines(data []byte, atEOF bool) (int, []byte, error) {
	if i := bytes.IndexAny(data, "\r\n"); i >= 0 {
		return i + 1, data[:i], nil
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// cleanLine strips NULs (wsl.exe's own messages are UTF-16LE: for ASCII text
// removing the zero bytes yields the text), a leading BOM and ANSI escapes.
func cleanLine(b []byte) string {
	b = bytes.ReplaceAll(b, []byte{0}, nil)
	b = bytes.TrimPrefix(b, []byte{0xFF, 0xFE})
	b = bytes.TrimPrefix(b, []byte{0xEF, 0xBB, 0xBF})
	s := string(bytes.ToValidUTF8(b, []byte("?")))
	s = ansiRe.ReplaceAllString(s, "")
	return strings.TrimRight(s, " \t")
}

var ansiRe = regexp.MustCompile(`[\x1b]\[[0-9;?]*[A-Za-z]`)
