package ports

import (
	"strings"
	"testing"
)

func TestOverlap_WildcardsCoverEverySpecificAddress(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"", "127.0.0.1", true},
		{"0.0.0.0", "127.0.0.1", true},
		{"127.0.0.1", "0.0.0.0", true},
		{"0.0.0.0", "::", true}, // Docker treats the two wildcards as overlapping
		{"::", "0.0.0.0", true},
		{"::", "192.168.1.5", true},
		{"[::]", "10.0.0.1", true},
		{"127.0.0.1", "127.0.0.1", true},
		{"127.0.0.1", "127.0.0.2", false},
		{"127.0.0.1", "::1", false},
		{"::1", "::1", true},
		{"::ffff:127.0.0.1", "127.0.0.1", true},
		{"fe80::1%eth0", "fe80::1", true},
	}
	for _, c := range cases {
		if got := overlap(c.a, c.b); got != c.want {
			t.Errorf("overlap(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestCheck_ProtocolsAreSeparate(t *testing.T) {
	c := local(Listener{IP: "0.0.0.0", Port: 53, Protocol: "udp", PID: 9, Name: "dnsmasq", Location: "This machine"})
	if r := check(c, "", 53, "tcp"); r.Status != StatusFree {
		t.Errorf("tcp 53 = %s, want free", r.Status)
	}
	r := check(c, "127.0.0.1", 53, "udp")
	if r.Status != StatusInUse || r.Holder.ProcessName != "dnsmasq" {
		t.Errorf("udp 53 = %+v", r)
	}
	if !strings.Contains(r.Detail, "53/udp is used by dnsmasq (PID 9)") {
		t.Errorf("detail = %q", r.Detail)
	}
}

func TestCheck_SpecificAddressConflictsOnlyWhenAddressesOverlap(t *testing.T) {
	c := local(Listener{IP: "127.0.0.1", Port: 8080, Protocol: "tcp", PID: 1, Name: "node.exe"})
	if check(c, "127.0.0.1", 8080, "tcp").Status != StatusInUse {
		t.Error("same address must conflict")
	}
	if check(c, "", 8080, "tcp").Status != StatusInUse {
		t.Error("a wildcard request must conflict with a loopback listener")
	}
	if check(c, "127.0.0.2", 8080, "tcp").Status != StatusFree {
		t.Error("a different specific address must not conflict")
	}
	if check(c, "::1", 8080, "tcp").Status != StatusFree {
		t.Error("IPv6 loopback must not conflict with IPv4 loopback")
	}
}

func TestCheck_RunningContainerHoldsThePortButStoppedOneIsOnlyANote(t *testing.T) {
	c := &Checker{Engine: fakeEngine{bindings: []Binding{
		running("a", "web-1", "shop", "0.0.0.0", 3000, "tcp"),
		{ContainerID: "b", Name: "old-web", HostIP: "", HostPort: 3001, Protocol: "tcp"},
	}}}
	r := check(c, "", 3000, "tcp")
	if r.Status != StatusInUse || r.Holder.Kind != HolderContainer || r.Holder.ComposeProject != "shop" {
		t.Fatalf("got %+v", r)
	}
	if r.Detail != "3000 is used by container web-1 (project shop)" {
		t.Errorf("detail = %q", r.Detail)
	}
	r = check(c, "", 3001, "tcp")
	if r.Status != StatusFree || !strings.Contains(r.Note, "old-web") {
		t.Errorf("stopped container: %+v", r)
	}
}

func TestCheck_ContainerBeingRecreatedIsIgnored(t *testing.T) {
	c := &Checker{Engine: fakeEngine{bindings: []Binding{running("a", "web-1", "", "0.0.0.0", 3000, "tcp")}}}
	if r := check(c, "", 3000, "tcp", "a"); r.Status != StatusFree {
		t.Errorf("own binding counted as a conflict: %+v", r)
	}
	if r := check(c, "", 3000, "tcp", "other"); r.Status != StatusInUse {
		t.Errorf("unrelated ignore id hid a conflict: %+v", r)
	}
}

func dockerDesktopSetup() *Checker {
	proxy := Listener{IP: "0.0.0.0", Port: 3001, Protocol: "tcp", PID: 5120, Name: "com.docker.backend.exe", Location: "This machine"}
	return &Checker{
		Engine: fakeEngine{bindings: []Binding{running("a", "catalo-web-1", "catalo", "0.0.0.0", 3001, "tcp")}},
		Hosts:  []HostSource{fakeHost{where: "This machine", listeners: []Listener{proxy}}},
	}
}

func TestCheck_DockerDesktopProxyIsAttributedToTheContainer(t *testing.T) {
	r := check(dockerDesktopSetup(), "", 3001, "tcp")
	if r.Status != StatusInUse || r.Holder.Kind != HolderContainer || r.Holder.ContainerName != "catalo-web-1" || len(r.Also) != 0 {
		t.Fatalf("got %+v holder=%+v", r, r.Holder)
	}
}

func TestCheck_ProxyOfAnIgnoredContainerIsNotAConflict(t *testing.T) {
	if r := check(dockerDesktopSetup(), "", 3001, "tcp", "a"); r.Status != StatusFree {
		t.Errorf("a project's own published port reported as conflict: %+v", r)
	}
}

func TestCheck_ProxyWithoutAMatchingContainerIsReportedAsTheProcess(t *testing.T) {
	proxy := Listener{IP: "127.0.0.1", Port: 9000, Protocol: "tcp", PID: 77, Name: "wslrelay.exe", Location: "This machine"}
	r := check(local(proxy), "", 9000, "tcp")
	if r.Status != StatusInUse || r.Holder.Kind != HolderProcess || r.Holder.ProcessName != "wslrelay.exe" {
		t.Errorf("got %+v", r)
	}
}

func TestCheck_WSLReportsWhichSideHoldsThePort(t *testing.T) {
	wsl := fakeHost{where: "WSL (Ubuntu)", listeners: []Listener{{IP: "0.0.0.0", Port: 5432, Protocol: "tcp", PID: 31, Name: "postgres", Location: "WSL (Ubuntu)"}}}
	win := fakeHost{where: "Windows", listeners: []Listener{{IP: "127.0.0.1", Port: 8080, Protocol: "tcp", PID: 4321, Name: "nginx.exe", Location: "Windows"}}}
	c := &Checker{Engine: fakeEngine{}, Hosts: []HostSource{wsl, win}}
	r := check(c, "", 8080, "tcp")
	if r.Status != StatusInUse || r.Holder.Location != "Windows" || !strings.Contains(r.Detail, "on Windows") {
		t.Errorf("windows side: %+v", r)
	}
	r = check(c, "", 5432, "tcp")
	if r.Status != StatusInUse || r.Holder.Location != "WSL (Ubuntu)" {
		t.Errorf("distro side: %+v", r)
	}
}

func TestCheck_RemoteEngineStillFindsContainersButNeverReportsFree(t *testing.T) {
	c := &Checker{Engine: fakeEngine{bindings: []Binding{running("a", "api", "", "0.0.0.0", 8000, "tcp")}},
		HostUnknown: "the Engine runs on another computer"}
	if r := check(c, "", 8000, "tcp"); r.Status != StatusInUse || r.Holder.ContainerName != "api" {
		t.Errorf("engine conflict lost: %+v", r)
	}
	r := check(c, "", 8001, "tcp")
	if r.Status != StatusUnknown || !strings.Contains(r.Detail, "another computer") {
		t.Errorf("remote free port: %+v", r)
	}
}

func TestCheck_UnreadableSourceMakesTheAnswerUnknownNotFree(t *testing.T) {
	c := &Checker{Engine: fakeEngine{}, Hosts: []HostSource{fakeHost{where: "Windows", err: errBoom}}}
	r := check(c, "", 8080, "tcp")
	if r.Status != StatusUnknown || !strings.Contains(r.Detail, "Windows") || !strings.Contains(r.Detail, "boom") {
		t.Errorf("got %+v", r)
	}
	c = &Checker{Engine: fakeEngine{err: errBoom}, Hosts: []HostSource{fakeHost{where: "Windows"}}}
	if r := check(c, "", 8080, "tcp"); r.Status != StatusUnknown {
		t.Errorf("engine failure reported as %s", r.Status)
	}
}

func TestCheck_AConflictIsStillReportedWhenAnotherSourceFailed(t *testing.T) {
	bad := fakeHost{where: "WSL (Ubuntu)", err: errBoom}
	ok := fakeHost{where: "Windows", listeners: []Listener{{IP: "0.0.0.0", Port: 80, Protocol: "tcp", PID: 4, Name: "System", System: true, Location: "Windows"}}}
	r := check(&Checker{Engine: fakeEngine{}, Hosts: []HostSource{bad, ok}}, "", 80, "tcp")
	if r.Status != StatusInUse || r.Holder.Kind != HolderSystem || r.Holder.PID != 4 {
		t.Errorf("got %+v", r)
	}
}

func TestCheck_SCTPCannotBeReadOnTheHostSoItIsUnknown(t *testing.T) {
	if r := check(local(), "", 9999, "sctp"); r.Status != StatusUnknown {
		t.Errorf("sctp = %s", r.Status)
	}
}

func TestNextFree_SkipsHeldPortsAndIsVerified(t *testing.T) {
	c := &Checker{
		Engine: fakeEngine{bindings: []Binding{running("a", "x", "", "", 8081, "tcp")}},
		Hosts:  []HostSource{fakeHost{where: "This machine", listeners: []Listener{{IP: "0.0.0.0", Port: 8082, Protocol: "tcp", PID: 1, Name: "x"}}}},
	}
	got := c.NextFree(t.Context(), Request{HostPort: 8080, Protocol: "tcp"}, nil)
	if got.Port != 8083 || !got.Verified {
		t.Errorf("got %+v, want 8083 verified", got)
	}
}

func TestNextFree_OnARemoteEngineIsOfferedButNotVerified(t *testing.T) {
	c := &Checker{Engine: fakeEngine{bindings: []Binding{running("a", "x", "", "", 8081, "tcp")}}, HostUnknown: "other computer"}
	got := c.NextFree(t.Context(), Request{HostPort: 8080, Protocol: "tcp"}, nil)
	if got.Port != 8082 || got.Verified {
		t.Errorf("got %+v, want 8082 unverified (8081 is a known container port)", got)
	}
}

func TestList_HidesProxiesOfContainersButShowsOtherListeners(t *testing.T) {
	c := &Checker{
		Engine: fakeEngine{bindings: []Binding{running("a", "db", "shop", "0.0.0.0", 5432, "tcp")}},
		Hosts: []HostSource{fakeHost{where: "This machine", listeners: []Listener{
			{IP: "0.0.0.0", Port: 5432, Protocol: "tcp", PID: 5, Name: "com.docker.backend.exe"},
			{IP: "127.0.0.1", Port: 3000, Protocol: "tcp", PID: 6, Name: "node.exe"},
			{IP: "0.0.0.0", Port: 445, Protocol: "tcp", PID: 4, Name: "System", System: true},
		}}},
	}
	l := c.List(t.Context())
	if len(l.Entries) != 3 {
		t.Fatalf("entries = %+v", l.Entries)
	}
	if l.Entries[0].HostPort != 445 || l.Entries[1].HostPort != 3000 || l.Entries[2].Holder.Kind != HolderContainer {
		t.Errorf("order/kinds = %+v", l.Entries)
	}
}

func TestList_RemoteEngineSaysWhatItCannotSee(t *testing.T) {
	l := (&Checker{Engine: fakeEngine{}, HostUnknown: "the Engine runs on another computer"}).List(t.Context())
	if len(l.Notes) != 1 || !strings.Contains(l.Notes[0], "another computer") {
		t.Errorf("notes = %v", l.Notes)
	}
}

func TestCheck_UnidentifiedListenerOnAContainersPortIsTheContainer(t *testing.T) {
	c := &Checker{
		Engine: fakeEngine{bindings: []Binding{running("a", "catalo-web-1", "catalo", "0.0.0.0", 3001, "tcp")}},
		Hosts:  []HostSource{fakeHost{where: "WSL (Ubuntu)", listeners: []Listener{{IP: "::", Port: 3001, Protocol: "tcp", Hidden: true, Location: "WSL (Ubuntu)"}}}},
	}
	if r := check(c, "", 3001, "tcp"); r.Holder.Kind != HolderContainer || len(r.Also) != 0 {
		t.Errorf("got %+v", r)
	}
	if r := check(local(Listener{IP: "::", Port: 7000, Protocol: "tcp", Hidden: true}), "", 7000, "tcp"); r.Status != StatusInUse || r.Holder.ProcessName != "" {
		t.Errorf("an unidentified listener alone must still be reported: %+v", r)
	}
}
