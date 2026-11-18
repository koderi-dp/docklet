package engine

import (
	"docklet/internal/secrets"
	"errors"
)

// SetSecrets installs the credential store used for SSH password/passphrase auth.
func (m *Manager) SetSecrets(st secrets.Store) { m.secrets = st }

// SetSecret stores a secret for a saved connection in the OS credential store.
func (m *Manager) SetSecret(id, kind, value string) error {
	if m.secrets == nil {
		return errors.New("no credential store is available")
	}
	key, err := secrets.Key(id, kind)
	if err != nil {
		return err
	}
	if value == "" {
		return errors.New("secret must not be empty")
	}
	if _, err := m.resolve(id); err != nil {
		return err
	}
	return m.secrets.Set(key, value)
}

// HasSecret reports whether a secret is stored (never returns the value).
func (m *Manager) HasSecret(id, kind string) (bool, error) {
	if m.secrets == nil {
		return false, nil
	}
	key, err := secrets.Key(id, kind)
	if err != nil {
		return false, err
	}
	_, err = m.secrets.Get(key)
	if errors.Is(err, secrets.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

// DeleteSecret removes a stored secret.
func (m *Manager) DeleteSecret(id, kind string) error {
	if m.secrets == nil {
		return nil
	}
	key, err := secrets.Key(id, kind)
	if err != nil {
		return err
	}
	return m.secrets.Delete(key)
}
