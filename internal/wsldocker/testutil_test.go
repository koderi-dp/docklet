package wsldocker

import (
	"context"
	"strings"
	"sync"
	"testing"
)

// fakeExec records every command and answers from a script keyed by the display form.
type fakeExec struct {
	mu     sync.Mutex
	calls  []Cmd
	output map[string][]string // display string -> output lines
	fail   map[string]error    // display string -> error
	onCall func(c Cmd)
}

func (f *fakeExec) Run(ctx context.Context, distro string, c Cmd, onLine func(string)) error {
	f.mu.Lock()
	f.calls = append(f.calls, c)
	f.mu.Unlock()
	if f.onCall != nil {
		f.onCall(c)
	}
	key := Display(c)
	for _, l := range f.output[key] {
		onLine(l)
	}
	return f.fail[key]
}

func (f *fakeExec) displays() []string {
	var out []string
	for _, c := range f.calls {
		out = append(out, Display(c))
	}
	return out
}

const ubuntuOSRelease = `PRETTY_NAME="Ubuntu 24.04.1 LTS"
NAME="Ubuntu"
VERSION_ID="24.04"
VERSION="24.04.1 LTS (Noble Numbat)"
VERSION_CODENAME=noble
ID=ubuntu
ID_LIKE=debian
UBUNTU_CODENAME=noble
`

const debianOSRelease = `PRETTY_NAME="Debian GNU/Linux 12 (bookworm)"
VERSION_ID="12"
VERSION_CODENAME=bookworm
ID=debian
`

const fedoraOSRelease = `NAME="Fedora Linux"
ID=fedora
VERSION_ID=40
PRETTY_NAME="Fedora Linux 40"
`

func ubuntuFacts() Facts {
	return Facts{Distro: "Ubuntu-24.04", OS: ParseOSRelease(ubuntuOSRelease), Arch: "amd64", Systemd: true, User: "alice"}
}

func debianFacts() Facts {
	return Facts{Distro: "Debian", OS: ParseOSRelease(debianOSRelease), Arch: "arm64", User: "bob"}
}

func commands(steps []Step) []string {
	var out []string
	for _, s := range steps {
		if s.Edit != nil {
			out = append(out, "edit "+s.Edit.Path)
			continue
		}
		out = append(out, Display(s.Cmd))
	}
	return out
}

func mustContain(t *testing.T, haystack []string, want string) {
	t.Helper()
	for _, h := range haystack {
		if strings.Contains(h, want) {
			return
		}
	}
	t.Fatalf("no entry contains %q in:\n%s", want, strings.Join(haystack, "\n"))
}
