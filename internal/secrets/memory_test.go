package secrets

import (
	"errors"
	"fmt"
	"runtime"
	"testing"

	"github.com/zalando/go-keyring"
)

func TestMemoryStore_MissingKeyIsErrNotFound(t *testing.T) {
	s := NewMemoryStore()
	if _, err := s.Get("docklet/a/ssh-password"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestMemoryStore_SetOverwritesAndDeleteRemoves(t *testing.T) {
	s := NewMemoryStore()
	_ = s.Set("k", "one")
	_ = s.Set("k", "two")
	if v, err := s.Get("k"); err != nil || v != "two" {
		t.Fatalf("overwrite: %q %v", v, err)
	}
	if err := s.Delete("k"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get("k"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after delete: %v", err)
	}
	if err := s.Delete("k"); err != nil {
		t.Fatalf("deleting a missing key must be a no-op, got %v", err)
	}
}

func TestMemoryStore_EmptySecretIsStoredNotMissing(t *testing.T) {
	s := NewMemoryStore()
	_ = s.Set("k", "")
	if v, err := s.Get("k"); err != nil || v != "" {
		t.Fatalf("got %q %v", v, err)
	}
}

func TestLookup_IsolatesConnectionsAndKinds(t *testing.T) {
	s := NewMemoryStore()
	put := func(id, kind, val string) {
		k, err := Key(id, kind)
		if err != nil {
			t.Fatal(err)
		}
		_ = s.Set(k, val)
	}
	put("c1", KindSSHPassword, "pw1")
	put("c1", KindSSHPassphrase, "phrase1")
	put("c2", KindSSHPassword, "pw2")
	for _, tc := range []struct{ id, kind, want string }{
		{"c1", KindSSHPassword, "pw1"},
		{"c1", KindSSHPassphrase, "phrase1"},
		{"c2", KindSSHPassword, "pw2"},
		{"c2", KindSSHPassphrase, ""},  // other kind of the same connection
		{"c3", KindSSHPassword, ""},    // unknown connection
		{"../c1", KindSSHPassword, ""}, // an invalid id never reaches the store
		{"c1", "bogus-kind", ""},       // invalid kind
	} {
		if got := Lookup(s, tc.id, tc.kind); got != tc.want {
			t.Errorf("Lookup(%q,%q) = %q, want %q", tc.id, tc.kind, got, tc.want)
		}
	}
	if got := Lookup(nil, "c1", KindSSHPassword); got != "" {
		t.Errorf("nil store: %q", got)
	}
}

type failingStore struct{ Store }

func (failingStore) Get(string) (string, error) { return "secret", errors.New("boom") }

func TestLookup_StoreErrorYieldsEmptyNeverAPartialSecret(t *testing.T) {
	if got := Lookup(failingStore{}, "c1", KindSSHPassword); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestFriendlyKeyringError(t *testing.T) {
	if friendlyKeyringError(nil) != nil {
		t.Error("nil must stay nil")
	}
	if err := friendlyKeyringError(keyring.ErrNotFound); err != keyring.ErrNotFound {
		t.Errorf("ErrNotFound must pass through untouched, got %v", err)
	}
	wrapped := fmt.Errorf("lookup: %w", keyring.ErrNotFound)
	if err := friendlyKeyringError(wrapped); err != wrapped {
		t.Errorf("wrapped ErrNotFound must pass through, got %v", err)
	}
	raw := errors.New("dbus: no secret service")
	got := friendlyKeyringError(raw)
	if !errors.Is(got, raw) {
		t.Fatalf("the original cause must stay reachable: %v", got)
	}
	explained := got.Error() != raw.Error()
	if explained != (runtime.GOOS == "linux") {
		t.Errorf("explanation added=%v on %s; it is meant for linux only: %v", explained, runtime.GOOS, got)
	}
}
