package engine

import (
	"strings"
	"testing"

	"docklet/internal/model"
)

func TestSSHArgs_VerifyHostKeysAndPassUserInputAfterDoubleDash(t *testing.T) {
	p := model.ConnectionProfile{Type: model.ConnSSH, Endpoint: "ssh://me@host:2200", SSHUser: "bob", SSHPort: 2222, SSHProxyJump: "jump"}
	a, err := sshArgs(p, false)
	if err != nil {
		t.Fatal(err)
	}
	j := strings.Join(a, " ")
	for _, want := range []string{"StrictHostKeyChecking=yes", "BatchMode=yes", "-l bob", "-p 2222", "-J jump", "-- host"} {
		if !strings.Contains(j, want) {
			t.Errorf("missing %q in %s", want, j)
		}
	}
	if strings.Contains(j, "StrictHostKeyChecking=no") || strings.Contains(j, "accept-new") {
		t.Error("host keys must be verified")
	}
	a, _ = sshArgs(p, true)
	if j = strings.Join(a, " "); !strings.Contains(j, "BatchMode=no") || strings.Contains(j, "BatchMode=yes") {
		t.Errorf("askpass args: %s", j)
	}
	if _, err := sshArgs(model.ConnectionProfile{Endpoint: "ssh://-oProxyCommand=x@host"}, false); err == nil {
		t.Error("injection must be rejected")
	}
}
