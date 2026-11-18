package engine

import (
	"docklet/internal/model"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateProfile_AcceptsOrRejectsPerConnectionType(t *testing.T) {
	tests := []struct {
		name    string
		p       model.ConnectionProfile
		wantErr string
	}{
		{"npipe ok", model.ConnectionProfile{Name: "a", Type: model.ConnNamedPipe, Endpoint: "npipe:////./pipe/docker_engine"}, ""},
		{"unix ok", model.ConnectionProfile{Name: "a", Type: model.ConnUnix, Endpoint: "unix:///var/run/docker.sock"}, ""},
		{"scheme mismatch", model.ConnectionProfile{Name: "a", Type: model.ConnUnix, Endpoint: "tcp://h:1"}, "must start with unix://"},
		{"no name", model.ConnectionProfile{Type: model.ConnUnix, Endpoint: "unix:///x"}, "name is required"},
		{"embedded creds", model.ConnectionProfile{Name: "a", Type: model.ConnTCP, Endpoint: "tcp://u:p@h:2376", TLSVerify: true}, "credentials"},
		{"tcp tls ok", model.ConnectionProfile{Name: "a", Type: model.ConnTCP, Endpoint: "tcp://h.example.com:2376", TLSVerify: true}, ""},
		{"tcp plain remote rejected", model.ConnectionProfile{Name: "a", Type: model.ConnTCP, Endpoint: "tcp://10.0.0.5:2375"}, "not allowed"},
		{"tcp plain loopback ok", model.ConnectionProfile{Name: "a", Type: model.ConnTCP, Endpoint: "tcp://127.0.0.1:2375"}, ""},
		{"ssh password rejected", model.ConnectionProfile{Name: "a", Type: model.ConnSSH, Endpoint: "ssh://u:pw@host"}, "password"},
		{"ssh option injection", model.ConnectionProfile{Name: "a", Type: model.ConnSSH, Endpoint: "ssh://-oProxyCommand=x@host"}, "invalid ssh"},
		{"ssh ok", model.ConnectionProfile{Name: "a", Type: model.ConnSSH, Endpoint: "ssh://me@host:2222"}, ""},
		{"wsl ok", model.ConnectionProfile{Name: "a", Type: model.ConnWSL, WSLDistro: "Ubuntu-24.04"}, ""},
		{"wsl injection", model.ConnectionProfile{Name: "a", Type: model.ConnWSL, WSLDistro: "-e sh"}, "invalid WSL"},
		{"unknown type", model.ConnectionProfile{Name: "a", Type: "bogus", Endpoint: "x://"}, "unknown connection type"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ValidateProfile(tc.p)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got.Discovered {
					t.Fatal("Discovered must be cleared")
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestValidateProfile_RemoteTCPNeedsTLSOrExplicitInsecureOptIn(t *testing.T) {
	remote := model.ConnectionProfile{Name: "a", Type: model.ConnTCP, Endpoint: "tcp://10.0.0.5:2375"}
	if _, err := ValidateProfile(remote); err == nil {
		t.Fatal("plain remote tcp must be rejected by default")
	}
	remote.AllowInsecure = true
	got, err := ValidateProfile(remote)
	if err != nil || !got.AllowInsecure {
		t.Fatalf("explicit opt-in must be accepted and kept: %+v %v", got, err)
	}
	remote.TLSVerify = true
	if got, _ = ValidateProfile(remote); got.AllowInsecure {
		t.Fatal("AllowInsecure must be cleared when TLS is on")
	}
	// The opt-in is meaningless for other transports.
	u := model.ConnectionProfile{Name: "a", Type: model.ConnUnix, Endpoint: "unix:///x", AllowInsecure: true, TLSVerify: true}
	if got, _ = ValidateProfile(u); got.AllowInsecure || got.TLSVerify {
		t.Fatalf("tcp-only fields leaked: %+v", got)
	}
}

func TestValidateProfile_RejectsSSHOptionInjection(t *testing.T) {
	key := filepath.Join(t.TempDir(), "id")
	os.WriteFile(key, []byte("k"), 0o600)
	base := model.ConnectionProfile{Name: "a", Type: model.ConnSSH, Endpoint: "ssh://me@host"}
	cases := []struct {
		name string
		mod  func(*model.ConnectionProfile)
		want string
	}{
		{"ok", func(p *model.ConnectionProfile) {
			p.SSHUser, p.SSHPort, p.SSHIdentityFile, p.SSHProxyJump = "bob", 2222, key, "j@jump:22,j2"
		}, ""},
		{"user dash", func(p *model.ConnectionProfile) { p.SSHUser = "-oProxyCommand=x" }, "invalid ssh user"},
		{"user space", func(p *model.ConnectionProfile) { p.SSHUser = "a b" }, "invalid ssh user"},
		{"port high", func(p *model.ConnectionProfile) { p.SSHPort = 70000 }, "port"},
		{"jump dash", func(p *model.ConnectionProfile) { p.SSHProxyJump = "-oProxyCommand=calc" }, "ProxyJump"},
		{"jump space", func(p *model.ConnectionProfile) { p.SSHProxyJump = "a b" }, "ProxyJump"},
		{"jump ctrl", func(p *model.ConnectionProfile) { p.SSHProxyJump = "a\nb" }, "ProxyJump"},
		{"identity missing", func(p *model.ConnectionProfile) { p.SSHIdentityFile = filepath.Join(t.TempDir(), "nope") }, "does not exist"},
		{"identity dash", func(p *model.ConnectionProfile) { p.SSHIdentityFile = "-oFoo" }, "identity file"},
	}
	for _, c := range cases {
		p := base
		c.mod(&p)
		_, err := ValidateProfile(p)
		if c.want == "" && err != nil || c.want != "" && (err == nil || !strings.Contains(err.Error(), c.want)) {
			t.Errorf("%s: err=%v want %q", c.name, err, c.want)
		}
	}
	// SSH fields are dropped for other types.
	p, err := ValidateProfile(model.ConnectionProfile{Name: "a", Type: model.ConnTCP, Endpoint: "tcp://127.0.0.1:2375", SSHUser: "x", SSHProxyJump: "h"})
	if err != nil || p.SSHUser != "" || p.SSHProxyJump != "" {
		t.Fatalf("%+v %v", p, err)
	}
}
