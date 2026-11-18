package wslinfo

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"docklet/internal/model"
	"docklet/internal/wsldocker"
)

func val(p *int64) int64 {
	if p == nil {
		return -1
	}
	return *p
}

func TestParseMeminfo_UsedIsTotalMinusAvailableAndCacheMatchesFree(t *testing.T) {
	m := ParseMeminfo("MemTotal:       16000000 kB\nMemFree:         1000000 kB\nMemAvailable:   12000000 kB\nBuffers:          200000 kB\nCached:          3000000 kB\nSReclaimable:     100000 kB\n")
	if val(m.Total) != 16000000*1024 || val(m.Used) != 4000000*1024 || val(m.Cache) != 3300000*1024 {
		t.Fatalf("got total=%d used=%d cache=%d", val(m.Total), val(m.Used), val(m.Cache))
	}
}

func TestParseMeminfo_MissingFieldsStayUnknownNotZero(t *testing.T) {
	m := ParseMeminfo("MemTotal: 1000 kB\n")
	if m.Total == nil || m.Used != nil || m.Cache != nil {
		t.Fatalf("only the total is known: %+v", m)
	}
	if m := ParseMeminfo(""); m.Total != nil || m.Used != nil {
		t.Fatalf("empty input must leave everything unknown: %+v", m)
	}
}

func TestParseDF_ReadsDataRowAndRejectsGarbage(t *testing.T) {
	d := ParseDF("Filesystem     1-blocks         Used    Available Capacity Mounted on\n/dev/sdd      1081101176832 262144000000 765000000000      26% /\n")
	if val(d.Size) != 1081101176832 || val(d.Used) != 262144000000 {
		t.Fatalf("got %d / %d", val(d.Size), val(d.Used))
	}
	for _, bad := range []string{"", "df: /: No such file", "Filesystem 1-blocks Used Available Capacity Mounted on\n"} {
		if d := ParseDF(bad); d.Size != nil || d.Used != nil {
			t.Fatalf("%q must be unknown", bad)
		}
	}
}

func TestSystemdEnabled(t *testing.T) {
	cases := map[string]bool{
		"":                                          false,
		"[boot]\nsystemd=true\n":                    true,
		"[boot]\r\nsystemd = True # on\r\n":         true,
		"[boot]\nsystemd=false\n":                   false,
		"[user]\ndefault=a\n[boot]\ncommand=x\n":    false,
		"[automount]\nsystemd=true\n":               false, // other section
		"[boot]\n# systemd=true\n":                  false,
		"[user]\ndefault=a\n\n[boot]\nsystemd=true": true,
	}
	for in, want := range cases {
		if got := SystemdEnabled(in); got != want {
			t.Errorf("%q: got %v want %v", in, got, want)
		}
	}
}

func TestOtherRunning_ListsOnlyRunningOthersSorted(t *testing.T) {
	all := []model.WSLDistro{
		{Name: "Ubuntu-24.04", State: "Running", Version: 2, Default: true},
		{Name: "docker-desktop", State: "Running", Version: 2},
		{Name: "Alpine", State: "Stopped", Version: 2},
		{Name: "Debian", State: "Running", Version: 2},
	}
	if got, want := OtherRunning(all, "ubuntu-24.04"), []string{"Debian", "docker-desktop"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	if got := OtherRunning(all[:1], "Ubuntu-24.04"); got == nil || len(got) != 0 {
		t.Fatalf("no others must be an empty list, got %#v", got)
	}
}

type fakeExec struct {
	calls  int
	output []string
	err    error
}

func (f *fakeExec) Run(_ context.Context, _ string, _ wsldocker.Cmd, onLine func(string)) error {
	f.calls++
	for _, l := range f.output {
		onLine(l)
	}
	return f.err
}

type fakeHost struct{}

func (fakeHost) VMMemory() (int64, bool) { return 5 << 30, true }
func (fakeHost) VHDX(string) (string, *int64, bool) {
	n := int64(40 << 30)
	return `C:\wsl\Ubuntu\ext4.vhdx`, &n, true
}

const insideOutput = `@@os
PRETTY_NAME="Ubuntu 24.04.1 LTS"
ID=ubuntu
@@init
systemd
@@conf
[boot]
systemd=true
@@mem
MemTotal: 8000000 kB
MemAvailable: 6000000 kB
Buffers: 1000 kB
Cached: 500000 kB
SReclaimable: 0 kB
@@df
Filesystem 1-blocks Used Available Capacity Mounted on
/dev/sdd 1000000000 250000000 700000000 26% /
@@docker
running
@@end
`

func TestCollect_RunningDistributionFillsEveryField(t *testing.T) {
	ex := &fakeExec{output: strings.Split(insideOutput, "\n")}
	all := []model.WSLDistro{{Name: "Ubuntu-24.04", State: "Running", Version: 2, Default: true}, {Name: "Debian", State: "Running", Version: 2}}
	info := Collector{Exec: ex, Host: fakeHost{}}.Collect(context.Background(), all[0], all)
	if !info.Running || !info.IsDefault || info.Version != 2 || info.OSName != "Ubuntu 24.04.1 LTS" || info.Docker != "running" {
		t.Fatalf("unexpected: %+v", info)
	}
	if info.SystemdEnabled == nil || !*info.SystemdEnabled || info.SystemdRunning == nil || !*info.SystemdRunning {
		t.Fatalf("systemd not read: %+v", info)
	}
	if val(info.MemUsed) != 2000000*1024 || val(info.DiskUsed) != 250000000 || val(info.VMMemory) != 5<<30 || val(info.VHDXSize) != 40<<30 {
		t.Fatalf("figures wrong: %+v", info)
	}
	if !reflect.DeepEqual(info.OtherRunning, []string{"Debian"}) || len(info.Actions) != 4 {
		t.Fatalf("others/actions wrong: %+v", info)
	}
	for _, a := range info.Actions {
		if a.Disabled != "" {
			t.Errorf("%s should be available: %s", a.ID, a.Disabled)
		}
	}
}

func TestCollect_StoppedDistributionIsNeverExecuted(t *testing.T) {
	ex := &fakeExec{}
	d := model.WSLDistro{Name: "Debian", State: "Stopped", Version: 2}
	info := Collector{Exec: ex, Host: fakeHost{}}.Collect(context.Background(), d, []model.WSLDistro{d})
	if ex.calls != 0 {
		t.Fatal("a stopped distribution must not be started just to read it")
	}
	if info.MemTotal != nil || info.DiskSize != nil || info.VMMemory != nil || info.Docker != "" {
		t.Fatalf("unread values must stay unknown: %+v", info)
	}
	for _, a := range info.Actions {
		if a.ID != model.WSLActionShutdown && a.Disabled == "" {
			t.Errorf("%s must be disabled for a stopped distribution", a.ID)
		}
	}
}

func TestActions_DockerStateDecidesRestartDocker(t *testing.T) {
	base := model.WSLInfo{Distro: "U", Running: true, Version: 2}
	for state, wantDisabled := range map[string]bool{"running": false, "stopped": false, "not-installed": true, "docker-desktop": true, "": true} {
		i := base
		i.Docker = state
		a, _ := Action(Actions(i), model.WSLActionRestartDocker)
		if (a.Disabled != "") != wantDisabled {
			t.Errorf("docker=%q: disabled=%q", state, a.Disabled)
		}
	}
	i := base
	i.Version = 1
	if a, _ := Action(Actions(i), model.WSLActionReclaimMemory); a.Disabled == "" {
		t.Error("WSL 1 cannot drop caches")
	}
}

func TestCommands_AreExactAndRunAsRootWithoutShell(t *testing.T) {
	if c := ReclaimCommand(); !c.Root || strings.Join(c.Argv, " ") != "sh -c sync; echo 1 > /proc/sys/vm/drop_caches" {
		t.Fatalf("reclaim: %+v", c)
	}
	if got := wsldocker.Display(RestartDockerCommand(true)); got != "systemctl restart docker" {
		t.Fatal(got)
	}
	if got := wsldocker.Display(RestartDockerCommand(false)); got != "/usr/sbin/service docker restart" {
		t.Fatal(got)
	}
	if got := wsldocker.Display(TerminateCommand("Ubuntu-24.04")); got != "wsl.exe --terminate Ubuntu-24.04" {
		t.Fatal(got)
	}
	if c := ShutdownCommand(); !c.Host || !reflect.DeepEqual(c.Argv, []string{"--shutdown"}) {
		t.Fatalf("shutdown: %+v", c)
	}
	if got := Show("U", ReclaimCommand()); !strings.HasPrefix(got, "wsl.exe -d U -u root --exec sh -c ") {
		t.Fatal(got)
	}
}
