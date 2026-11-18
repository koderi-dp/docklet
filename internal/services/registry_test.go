package services

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/moby/moby/client"

	"docklet/internal/registryauth"
	"docklet/internal/secrets"
)

type oneClient struct{ cli *client.Client }

func (o oneClient) Client(context.Context, string) (*client.Client, error) { return o.cli, nil }

func newRegistrySvc(t *testing.T, login func(string, string) (string, error)) (*RegistryService, *secrets.MemoryStore) {
	t.Helper()
	cli, err := client.New(client.WithHost("tcp://127.0.0.1:1"))
	if err != nil {
		t.Fatal(err)
	}
	st := secrets.NewMemoryStore()
	return &RegistryService{
		Engines: oneClient{cli},
		Auth:    &registryauth.Resolver{ConfigDir: t.TempDir(), Store: st, LookPath: func(string) (string, error) { return "", errors.New("none") }},
		Login: func(_ context.Context, _ *client.Client, _, user, secret string) (string, error) {
			return login(user, secret)
		},
	}, st
}

func TestSignIn_RejectedCredentialsAreNotStoredAndErrorOmitsThePassword(t *testing.T) {
	s, st := newRegistrySvc(t, func(_, secret string) (string, error) {
		return "", errors.New("unauthorized: incorrect username or password (sent " + secret + ")")
	})
	_, err := s.SignIn(context.Background(), "c1", "127.0.0.1:5055", "alice", "hunter2-pw")
	if err == nil || !strings.Contains(err.Error(), "unauthorized") || strings.Contains(err.Error(), "hunter2-pw") {
		t.Fatalf("want the registry's message without the password, got %v", err)
	}
	if l, _ := registryauth.ListDocklet(st); len(l) != 0 {
		t.Errorf("nothing may be stored on failure: %+v", l)
	}
}

func TestSignIn_StoresVerifiedCredentialsAndListHidesSecrets(t *testing.T) {
	s, st := newRegistrySvc(t, func(_, _ string) (string, error) { return "", nil })
	e, err := s.SignIn(context.Background(), "c1", "https://Docker.io", "alice", "tok-123456")
	if err != nil || e.Host != "docker.io" || e.Store != registryauth.KindDocklet {
		t.Fatalf("%+v %v", e, err)
	}
	if v, _ := st.Get("registry/docker.io"); !strings.Contains(v, "tok-123456") {
		t.Error("secret should be in the credential store")
	}
	l, _ := s.List(context.Background())
	if len(l.Entries) != 1 || l.Entries[0].Username != "alice" {
		t.Fatalf("%+v", l)
	}
	if err := s.SignOut(context.Background(), "docker.io", registryauth.KindDocklet); err != nil {
		t.Fatal(err)
	}
	if l, _ = s.List(context.Background()); len(l.Entries) != 0 {
		t.Errorf("after sign out %+v", l)
	}
}

func TestSignIn_IdentityTokenIsStoredAsTokenUser(t *testing.T) {
	s, st := newRegistrySvc(t, func(_, _ string) (string, error) { return "idtoken-xyz", nil })
	e, err := s.SignIn(context.Background(), "c1", "quay.io", "bob", "pw-123456")
	if err != nil || e.Username != "<token>" {
		t.Fatalf("%+v %v", e, err)
	}
	if v, _ := st.Get("registry/quay.io"); strings.Contains(v, "pw-123456") || !strings.Contains(v, "idtoken-xyz") {
		t.Errorf("stored %q", v)
	}
}
