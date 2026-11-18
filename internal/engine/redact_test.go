package engine

import (
	"strings"
	"testing"
)

func TestRedact_RemovesSecretsAndKeepsBenignText(t *testing.T) {
	tests := []struct{ in, notContains string }{
		{"dial tcp://admin:hunter2@host:2376 failed", "hunter2"},
		{"Authorization: Bearer abc.def.ghi", "abc.def.ghi"},
		{"password=s3cr3t other", "s3cr3t"},
		{`{"IdentityToken":"tok123"}`, "tok123"},
		{"-----BEGIN OPENSSH PRIVATE KEY-----\nAAAA\n-----END OPENSSH PRIVATE KEY-----", "AAAA"},
	}
	for _, tc := range tests {
		if out := Redact(tc.in); strings.Contains(out, tc.notContains) {
			t.Errorf("Redact(%q) = %q still contains %q", tc.in, out, tc.notContains)
		}
	}
	if out := Redact("connection refused at npipe:////./pipe/docker_engine"); !strings.Contains(out, "connection refused") {
		t.Errorf("benign text mangled: %q", out)
	}
}
