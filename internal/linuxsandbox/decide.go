// Package linuxsandbox detects, before WebKitGTK starts, that its bubblewrap sandbox cannot work
// (typically Ubuntu 24.04+ restricting unprivileged user namespaces through AppArmor) and turns the
// otherwise cryptic SIGTRAP crash into a clear message. The decision logic is pure and builds everywhere.
package linuxsandbox

import (
	"fmt"
	"strings"
)

// ExitCode is the process exit status when Docklet refuses to start (EX_CONFIG).
const ExitCode = 78

// Probe is the outcome of trying to start bubblewrap the way WebKitGTK does.
type Probe int

const (
	ProbeSkipped Probe = iota // bubblewrap missing, or it failed for an unrelated reason: do not block the app
	ProbeOK
	ProbeBlocked // user namespaces are denied
)

// Action is what the app should do before starting WebKit.
type Action int

const (
	Proceed Action = iota
	Refuse
	DisableSandbox
)

// State is everything the decision and the message depend on.
type State struct {
	Probe      Probe
	OptOut     bool   // the user explicitly asked for the sandbox to be off
	Restricted bool   // kernel.apparmor_restrict_unprivileged_userns == 1
	Label      string // AppArmor label of this process ("unconfined" when no profile applies)
	Exe        string // path of this executable
	AppImage   bool   // running from an AppImage (no profile can be installed for it)
}

// Decide returns the action for s. The sysctl and the label only explain a block; the probe decides it.
func Decide(s State) Action {
	if s.Probe != ProbeBlocked {
		return Proceed
	}
	if s.OptOut {
		return DisableSandbox
	}
	return Refuse
}

// ClassifyProbe maps the result of running bubblewrap to a Probe. Only the errors that mean
// "user namespaces are denied" block the app; anything else is not ours to judge.
func ClassifyProbe(output string, err error) Probe {
	if err == nil {
		return ProbeOK
	}
	if strings.Contains(output, "uid map") || strings.Contains(output, "new namespace") {
		return ProbeBlocked
	}
	return ProbeSkipped
}

// Profile is the AppArmor profile that lets the binary at exe create user namespaces.
// It is shipped as build/linux/nfpm/docklet.apparmor for /usr/bin/docklet (a test keeps them equal).
func Profile(exe string) string {
	return fmt.Sprintf(`abi <abi/4.0>,
include <tunables/global>

profile docklet %s flags=(unconfined) {
  userns,

  include if exists <local/docklet>
}
`, exe)
}

// Message is the text shown when Docklet refuses to start.
func Message(s State) string {
	var b strings.Builder
	b.WriteString("Docklet cannot start: this system does not allow WebKitGTK to create its sandbox.\n\n")
	b.WriteString("WebKitGTK isolates its helper processes with bubblewrap, which needs unprivileged user namespaces.\n")
	if s.Restricted && (s.Label == "" || s.Label == "unconfined") {
		b.WriteString("Here they are blocked by AppArmor (kernel.apparmor_restrict_unprivileged_userns=1, the Ubuntu 24.04+\ndefault), and Docklet has no AppArmor profile that allows them.\n")
	} else {
		b.WriteString("Here they are blocked by the system configuration.\n")
	}
	b.WriteString("\nFix it with one of these:\n\n")
	b.WriteString("1. Install the .deb package from the Releases page. It ships the AppArmor profile.\n\n")
	b.WriteString("2. Use Ubuntu's own bubblewrap profile (this also works for the AppImage):\n")
	b.WriteString("     sudo apt install apparmor-profiles\n")
	b.WriteString("     sudo cp /usr/share/apparmor/extra-profiles/bwrap-userns-restrict /etc/apparmor.d/\n")
	b.WriteString("     sudo apparmor_parser -r /etc/apparmor.d/bwrap-userns-restrict\n\n")
	if !s.AppImage && s.Exe != "" {
		b.WriteString("3. Allow only this program. Save this as /etc/apparmor.d/docklet and run\n   sudo apparmor_parser -r /etc/apparmor.d/docklet\n\n")
		for _, line := range strings.Split(strings.TrimRight(Profile(s.Exe), "\n"), "\n") {
			b.WriteString("     " + line + "\n")
		}
		b.WriteString("\n")
	}
	b.WriteString("4. Last resort: start Docklet with DOCKLET_DISABLE_WEBKIT_SANDBOX=1 to turn the WebKit sandbox off.\n")
	b.WriteString("   Docklet only loads its own bundled interface, but a bug in the web view would then not be contained.\n")
	return b.String()
}
