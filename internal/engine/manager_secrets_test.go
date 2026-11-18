package engine

import (
	"context"
	"strings"
	"testing"

	"docklet/internal/model"
	"docklet/internal/secrets"
)

func TestSecrets_StayOutOfProfileStoreAndAreRemovedWithConnection(t *testing.T) {
	st := &memStore{}
	m := NewManager(st, nil)
	mem := secrets.NewMemoryStore()
	m.SetSecrets(mem)
	saved, err := m.Save(context.Background(), model.ConnectionProfile{Name: "s", Type: model.ConnSSH, Endpoint: "ssh://me@host"})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.SetSecret(saved.ID, secrets.KindSSHPassword, "hunter2-SECRET"); err != nil {
		t.Fatal(err)
	}
	if ok, _ := m.HasSecret(saved.ID, secrets.KindSSHPassword); !ok {
		t.Fatal("secret should be stored")
	}
	profiles, _ := st.Load()
	if strings.Contains(strings.Join([]string{profiles[0].Name, profiles[0].Endpoint, profiles[0].SSHUser, profiles[0].SSHIdentityFile}, "|"), "hunter2") {
		t.Fatal("secret leaked into profile store")
	}
	if err := m.SetSecret(saved.ID, "bogus", "x"); err == nil {
		t.Fatal("unknown kind must be rejected")
	}
	if err := m.SetSecret("nope", secrets.KindSSHPassword, "x"); err == nil {
		t.Fatal("unknown connection must be rejected")
	}
	if err := m.Remove(context.Background(), saved.ID); err != nil {
		t.Fatal(err)
	}
	if ok, _ := m.HasSecret(saved.ID, secrets.KindSSHPassword); ok {
		t.Fatal("secret must be removed with the connection")
	}
}
