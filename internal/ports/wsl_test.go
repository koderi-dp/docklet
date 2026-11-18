package ports

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestWSLHost_UsesSSAsRootInTheDistribution(t *testing.T) {
	var calls [][]string
	h := WSLHost{Distro: "Ubuntu-24.04", run: func(_ context.Context, name string, args ...string) ([]byte, error) {
		calls = append(calls, append([]string{name}, args...))
		return []byte(`tcp LISTEN 0 4096 0.0.0.0:3001 0.0.0.0:* users:(("docker-proxy",pid=1,fd=7))` + "\n"), nil
	}}
	got, err := h.Listeners(t.Context())
	if err != nil || len(got) != 1 || got[0].Location != "WSL (Ubuntu-24.04)" {
		t.Fatalf("got %+v, %v", got, err)
	}
	if strings.Join(calls[0], " ") != "wsl.exe -d Ubuntu-24.04 -u root --exec ss -H -ltnup" {
		t.Errorf("command = %v", calls[0])
	}
}

func TestWSLHost_FallsBackToProcWhenSSIsMissingAndDoesNotInventNames(t *testing.T) {
	n := 0
	h := WSLHost{Distro: "Ubuntu", run: func(context.Context, string, ...string) ([]byte, error) {
		n++
		if n == 1 {
			return nil, errors.New("execvpe(ss) failed: No such file or directory")
		}
		return []byte("## tcp\n" + procTCP + "## tcp6\n" + procTCP6), nil
	}}
	got, err := h.Listeners(t.Context())
	if err != nil || len(got) != 5 {
		t.Fatalf("got %d, %v", len(got), err)
	}
	for _, l := range got {
		if !l.Hidden || l.Name != "" {
			t.Errorf("fallback must not invent program names: %+v", l)
		}
	}
}

func TestWSLHost_FailureOfBothReadersIsAnErrorNotAnEmptyList(t *testing.T) {
	h := WSLHost{Distro: "Ubuntu", run: func(context.Context, string, ...string) ([]byte, error) { return nil, errBoom }}
	if _, err := h.Listeners(t.Context()); err == nil {
		t.Error("expected an error, not an empty (free-looking) list")
	}
}
