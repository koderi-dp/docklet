package linuxsandbox

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestDecide(t *testing.T) {
	cases := []struct {
		name string
		s    State
		want Action
	}{
		{"works", State{Probe: ProbeOK}, Proceed},
		{"bwrap missing or unrelated failure", State{Probe: ProbeSkipped, Restricted: true, Label: "unconfined"}, Proceed},
		{"blocked", State{Probe: ProbeBlocked, Restricted: true, Label: "unconfined"}, Refuse},
		{"blocked with opt-out", State{Probe: ProbeBlocked, OptOut: true}, DisableSandbox},
		{"opt-out is ignored when not needed", State{Probe: ProbeOK, OptOut: true}, Proceed},
	}
	for _, c := range cases {
		if got := Decide(c.s); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestClassifyProbe(t *testing.T) {
	fail := errors.New("exit status 1")
	cases := []struct {
		out  string
		err  error
		want Probe
	}{
		{"", nil, ProbeOK},
		{"bwrap: setting up uid map: Permission denied\n", fail, ProbeBlocked},
		{"bwrap: No permissions to creating new namespace, likely because the kernel does not allow non-privileged user namespaces.\n", fail, ProbeBlocked},
		{"bwrap: execvp true: No such file or directory\n", fail, ProbeSkipped},
	}
	for _, c := range cases {
		if got := ClassifyProbe(c.out, c.err); got != c.want {
			t.Errorf("%q: got %v, want %v", c.out, got, c.want)
		}
	}
}

func TestMessage(t *testing.T) {
	m := Message(State{Probe: ProbeBlocked, Restricted: true, Label: "unconfined", Exe: "/opt/docklet/docklet"})
	for _, want := range []string{"AppArmor", "bwrap-userns-restrict", "DOCKLET_DISABLE_WEBKIT_SANDBOX=1", "profile docklet /opt/docklet/docklet flags=(unconfined)"} {
		if !strings.Contains(m, want) {
			t.Errorf("message lacks %q:\n%s", want, m)
		}
	}
	img := Message(State{Probe: ProbeBlocked, Restricted: true, Exe: "/tmp/.mount_x/usr/bin/docklet", AppImage: true})
	if strings.Contains(img, "Allow only this program") {
		t.Error("AppImage message must not offer a per-binary profile")
	}
	other := Message(State{Probe: ProbeBlocked})
	if strings.Contains(other, "kernel.apparmor_restrict_unprivileged_userns") {
		t.Error("AppArmor is blamed although the sysctl is not set")
	}
}

// The package ships a copy of Profile for /usr/bin/docklet; they must not drift apart.
func TestShippedProfileMatches(t *testing.T) {
	data, err := os.ReadFile("../../build/linux/nfpm/docklet.apparmor")
	if err != nil {
		t.Fatal(err)
	}
	got := strings.ReplaceAll(string(data), "\r\n", "\n")
	if got != Profile("/usr/bin/docklet") {
		t.Errorf("build/linux/nfpm/docklet.apparmor differs from Profile(\"/usr/bin/docklet\"):\n%s", got)
	}
}
