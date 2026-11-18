package wsldocker

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// StepError names the step that failed and what had been done before it.
type StepError struct {
	Index, Total int
	Title        string
	Cause        error
	Output       []string // last output lines of the failed command
	Done         []string // titles of the steps that had completed
}

func (e *StepError) Error() string {
	msg := fmt.Sprintf("Step %d of %d failed: %s (%v).", e.Index, e.Total, e.Title, e.Cause)
	if len(e.Output) > 0 {
		msg += " Last output: " + strings.Join(e.Output, " | ")
	}
	if len(e.Done) > 0 {
		msg += " Already done, and left in place: " + strings.Join(e.Done, "; ") + "."
	} else {
		msg += " Nothing was changed before this step."
	}
	return msg
}

func (e *StepError) Unwrap() error { return e.Cause }

// Reporter receives progress. Both functions are optional.
type Reporter struct {
	Log  func(string)
	Step func(index, total int, title string) // index is 1-based, called as a step starts
}

// Run executes steps in order and stops at the first failure. Each command line
// and its output go to log. Cancelling ctx stops before the next step starts and
// interrupts the running command. Nothing is rolled back.
func Run(ctx context.Context, ex Exec, distro string, steps []Step, rep Reporter) error {
	log := rep.Log
	if log == nil {
		log = func(string) {}
	}
	var done []string
	for i, s := range steps {
		if err := ctx.Err(); err != nil {
			return err
		}
		log(fmt.Sprintf("[%d/%d] %s", i+1, len(steps), s.Title))
		if rep.Step != nil {
			rep.Step(i+1, len(steps), s.Title)
		}
		var last []string
		out := func(l string) {
			if l == "" {
				return
			}
			log(l)
			last = append(last, l)
			if len(last) > 8 {
				last = last[1:]
			}
		}
		if err := runStep(ctx, ex, distro, s, out); err != nil {
			if cerr := ctx.Err(); cerr != nil {
				return cerr
			}
			return &StepError{Index: i + 1, Total: len(steps), Title: s.Title, Cause: err, Output: last, Done: done}
		}
		done = append(done, s.Title)
	}
	return nil
}

// runStep runs one step, retrying a failing attempt until s.Wait has passed.
func runStep(ctx context.Context, ex Exec, distro string, s Step, out func(string)) error {
	start := time.Now()
	for {
		err := attempt(ctx, ex, distro, s, out)
		if err == nil || ctx.Err() != nil || time.Since(start) >= s.Wait {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

func attempt(ctx context.Context, ex Exec, distro string, s Step, out func(string)) error {
	actx, cancel := context.WithTimeout(ctx, s.Timeout)
	defer cancel()
	var err error
	if s.Edit != nil {
		err = editFile(actx, ex, distro, s.Edit, out)
	} else {
		out("$ " + Display(s.Cmd))
		err = ex.Run(actx, distro, s.Cmd, out)
	}
	if err != nil && actx.Err() == context.DeadlineExceeded && ctx.Err() == nil {
		return fmt.Errorf("timed out after %s", s.Timeout)
	}
	return err
}

// readScript prints a file when it exists and fails only when reading it fails.
const readScript = `if [ -e "$1" ]; then cat "$1"; fi`

func editFile(ctx context.Context, ex Exec, distro string, e *FileEdit, out func(string)) error {
	out("$ edit " + e.Path)
	var old []string
	read := root("sh", "-c", readScript, "sh", e.Path)
	if err := ex.Run(ctx, distro, read, func(l string) { old = append(old, l) }); err != nil {
		return fmt.Errorf("reading %s: %w", e.Path, err)
	}
	cur := strings.Join(old, "\n")
	if len(old) > 0 {
		cur += "\n"
	}
	next := e.Transform(cur)
	if next == cur {
		out(e.Path + " already has the setting")
		return nil
	}
	write := Cmd{Argv: []string{"tee", e.Path}, Root: true, Stdin: next}
	return ex.Run(ctx, distro, write, func(string) {})
}

// Display renders a command for the user with POSIX quoting where needed.
func Display(c Cmd) string {
	parts := make([]string, len(c.Argv))
	for i, a := range c.Argv {
		parts[i] = quote(a)
	}
	s := strings.Join(parts, " ")
	if c.Host {
		return "wsl.exe " + s
	}
	return s
}

func quote(a string) string {
	if a != "" && strings.IndexFunc(a, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("-_./:=@%+,", r))
	}) < 0 {
		return a
	}
	return "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
}
