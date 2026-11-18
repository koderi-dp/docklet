//go:build linux

package linuxsandbox

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	optOutEnv   = "DOCKLET_DISABLE_WEBKIT_SANDBOX"
	webkitEnv   = "WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS"
	sysctlPath  = "/proc/sys/kernel/apparmor_restrict_unprivileged_userns"
	probeBudget = 5 * time.Second
)

// Guard must run before Wails starts WebKit. It either returns (the sandbox works, or the user opted out
// of it) or prints the explanation and exits with ExitCode.
func Guard() {
	s := State{
		Probe:      probe(),
		OptOut:     enabled(os.Getenv(optOutEnv)) || enabled(os.Getenv(webkitEnv)),
		Restricted: strings.TrimSpace(readFile(sysctlPath)) == "1",
		Label:      label(),
		AppImage:   os.Getenv("APPIMAGE") != "",
	}
	s.Exe, _ = os.Executable()
	switch Decide(s) {
	case DisableSandbox:
		fmt.Fprintln(os.Stderr, "Docklet: the WebKit sandbox is disabled for this run.")
		os.Setenv(webkitEnv, "1")
	case Refuse:
		msg := Message(s)
		fmt.Fprint(os.Stderr, msg)
		showDialog(msg)
		os.Exit(ExitCode)
	}
}

// probe starts bubblewrap with a new user namespace, as WebKitGTK does for its helper processes.
// Running bwrap itself (not creating a namespace from this process) is deliberate: AppArmor lets the
// namespace be created but strips its capabilities, so the failure appears only when bwrap writes its
// uid map, and bwrap is the process whose confinement matters.
func probe() Probe {
	bwrap, err := exec.LookPath("bwrap")
	if err != nil {
		return ProbeSkipped
	}
	ctx, cancel := context.WithTimeout(context.Background(), probeBudget)
	defer cancel()
	var out bytes.Buffer
	cmd := exec.CommandContext(ctx, bwrap, "--unshare-user", "--ro-bind", "/", "/", "true")
	cmd.Stdout, cmd.Stderr = &out, &out
	err = cmd.Run() // the output is read only after bwrap has finished
	return ClassifyProbe(out.String(), err)
}

// label is the AppArmor profile this process runs under, "unconfined" if none.
func label() string {
	for _, p := range []string{"/proc/self/attr/apparmor/current", "/proc/self/attr/current"} {
		if l := strings.TrimSpace(readFile(p)); l != "" {
			return l
		}
	}
	return "unconfined"
}

func readFile(path string) string {
	b, _ := os.ReadFile(path)
	return string(b)
}

func enabled(v string) bool { return v != "" && v != "0" }

// showDialog helps when Docklet was started from a launcher and nobody sees stderr. Wails dialogs need
// the running application, so use zenity when it exists; otherwise stderr is all there is.
func showDialog(msg string) {
	if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		return
	}
	zenity, err := exec.LookPath("zenity")
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	_ = exec.CommandContext(ctx, zenity, "--error", "--no-markup", "--title=Docklet", "--text="+msg).Run()
}
