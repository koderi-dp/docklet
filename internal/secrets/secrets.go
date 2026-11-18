// Package secrets keeps credentials (SSH passwords, key passphrases) in the OS
// credential store (Windows Credential Manager, macOS Keychain, Linux Secret
// Service). Secrets are never written to SQLite, logs or the frontend.
package secrets

import (
	"errors"
	"fmt"
	"regexp"
	"runtime"
	"sync"

	"github.com/zalando/go-keyring"
)

// Secret kinds.
const (
	KindSSHPassword   = "ssh-password"
	KindSSHPassphrase = "ssh-passphrase"
)

// ErrNotFound is returned by Get when no secret is stored.
var ErrNotFound = errors.New("secret not found")

// Store is a secret vault keyed by strings such as docklet/<connectionID>/<kind>.
type Store interface {
	Set(key, secret string) error
	Get(key string) (string, error)
	Delete(key string) error
}

var idPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// ValidKind reports whether kind is a known secret kind.
func ValidKind(kind string) bool { return kind == KindSSHPassword || kind == KindSSHPassphrase }

// Key builds the credential key docklet/<connectionID>/<kind>.
func Key(connectionID, kind string) (string, error) {
	if !idPattern.MatchString(connectionID) {
		return "", errors.New("invalid connection id")
	}
	if !ValidKind(kind) {
		return "", fmt.Errorf("unknown secret kind %q", kind)
	}
	return "docklet/" + connectionID + "/" + kind, nil
}

const keyringService = "docklet"

// OSStore is backed by the operating system credential store.
type OSStore struct{}

// NewOSStore returns the OS credential store.
func NewOSStore() Store { return OSStore{} }

func (OSStore) Set(key, secret string) error {
	return friendlyKeyringError(keyring.Set(keyringService, key, secret))
}

// friendlyKeyringError explains the usual Linux failure: no Secret Service (GNOME Keyring, KWallet,
// KeePassXC) is running on the session bus. Connections that need no stored secret are unaffected.
func friendlyKeyringError(err error) error {
	if err == nil || errors.Is(err, keyring.ErrNotFound) {
		return err
	}
	if runtime.GOOS == "linux" {
		return fmt.Errorf("the system credential store is not available (a Secret Service such as GNOME Keyring or KWallet must be running); use an SSH key or agent instead: %w", err)
	}
	return err
}

func (OSStore) Get(key string) (string, error) {
	v, err := keyring.Get(keyringService, key)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrNotFound
	}
	return v, friendlyKeyringError(err)
}

func (OSStore) Delete(key string) error {
	err := keyring.Delete(keyringService, key)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return friendlyKeyringError(err)
}

// MemoryStore is an in-memory fake for tests.
type MemoryStore struct {
	mu sync.Mutex
	m  map[string]string
}

// NewMemoryStore returns an empty in-memory store.
func NewMemoryStore() *MemoryStore { return &MemoryStore{m: map[string]string{}} }

func (s *MemoryStore) Set(key, secret string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key] = secret
	return nil
}

func (s *MemoryStore) Get(key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.m[key]
	if !ok {
		return "", ErrNotFound
	}
	return v, nil
}

func (s *MemoryStore) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, key)
	return nil
}

// Lookup returns the secret of a connection, or "" when none is stored.
func Lookup(st Store, connectionID, kind string) string {
	if st == nil {
		return ""
	}
	k, err := Key(connectionID, kind)
	if err != nil {
		return ""
	}
	v, err := st.Get(k)
	if err != nil {
		return ""
	}
	return v
}
