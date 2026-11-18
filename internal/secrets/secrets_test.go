package secrets

import (
	"strings"
	"testing"
)

func TestKey_RejectsPathLikeIDsAndUnknownKinds(t *testing.T) {
	if k, err := Key("abc123", KindSSHPassword); err != nil || k != "docklet/abc123/ssh-password" {
		t.Fatal(k, err)
	}
	for _, bad := range []string{"", "../x", "a/b", "-x"} {
		if _, err := Key(bad, KindSSHPassword); err == nil {
			t.Errorf("id %q accepted", bad)
		}
	}
	if _, err := Key("a", "other"); err == nil {
		t.Error("kind accepted")
	}
}

func TestAskpass_AnswersOnlyTokenHolderOncePerPromptKind(t *testing.T) {
	srv, err := StartAskpass("pw-1", "phrase-1")
	if err != nil {
		t.Skip("unix sockets unavailable:", err)
	}
	defer srv.Close()
	env, err := srv.Env()
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(env, "\n")
	if strings.Contains(joined, "pw-1") || strings.Contains(joined, "phrase-1") {
		t.Fatal("secret must not be in the child environment")
	}
	if !strings.Contains(joined, "SSH_ASKPASS_REQUIRE=force") {
		t.Fatal("force missing")
	}
	// wrong token is refused
	if got, _ := askParent(srv.sock, "wrong", "Password:"); got != "" {
		t.Fatalf("wrong token got %q", got)
	}
	if got, err := askParent(srv.sock, srv.token, "me@host's password: "); err != nil || got != "pw-1" {
		t.Fatalf("password: %q %v", got, err)
	}
	// one-shot per kind: a retry is refused
	if got, _ := askParent(srv.sock, srv.token, "me@host's password: "); got != "" {
		t.Fatalf("second password request got %q", got)
	}
	if got, _ := askParent(srv.sock, srv.token, "Enter passphrase for key '/k': "); got != "phrase-1" {
		t.Fatalf("passphrase: %q", got)
	}
	// host key trust questions are never answered
	if got, _ := askParent(srv.sock, srv.token, "Are you sure you want to continue connecting (yes/no)? "); got != "" {
		t.Fatalf("host key prompt answered: %q", got)
	}
}
