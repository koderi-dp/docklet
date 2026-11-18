package engine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/moby/moby/client"

	"docklet/internal/model"
	"docklet/internal/secrets"
)

// ProfileStore persists user-saved profiles (implemented by internal/store).
type ProfileStore interface {
	Load() ([]model.ConnectionProfile, error)
	Save([]model.ConnectionProfile) error
}

// conn is one connection's isolated runtime state.
type conn struct {
	profile model.ConnectionProfile
	cli     *client.Client // nil unless State == connected
	status  model.ConnectionStatus
	cancel  context.CancelFunc // stops the event stream
	done    chan struct{}      // closed when the event goroutine exits
}

// Manager implements Provider and owns every connection's client lifecycle.
type Manager struct {
	store   ProfileStore
	secrets secrets.Store // OS credential store; nil disables stored SSH secrets
	emit    Emitter

	newClient  clientFactory
	debounce   time.Duration
	backoffMin time.Duration
	backoffMax time.Duration

	mu    sync.RWMutex
	conns map[string]*conn
	locks map[string]*sync.Mutex // per-connection Connect/Disconnect serialization
}

var _ Provider = (*Manager)(nil)

// NewManager creates a manager. emit may be nil.
func NewManager(st ProfileStore, emit Emitter) *Manager {
	m := &Manager{
		store: st, emit: emit,
		debounce: 250 * time.Millisecond, backoffMin: time.Second, backoffMax: 30 * time.Second,
		conns: map[string]*conn{}, locks: map[string]*sync.Mutex{},
	}

	m.newClient = func(p model.ConnectionProfile) (*client.Client, error) { return newSDKClientWith(p, m.secrets) }
	return m
}

func (m *Manager) lockFor(id string) *sync.Mutex {
	m.mu.Lock()
	defer m.mu.Unlock()
	l := m.locks[id]
	if l == nil {
		l = &sync.Mutex{}
		m.locks[id] = l
	}
	return l
}

func (m *Manager) emitEvent(name string, data any) {
	if m.emit != nil {
		m.emit.Emit(name, data)
	}
}

// ---- profiles ----

// List returns saved profiles plus the discovered local default, which stays
// listed (last) unless a saved profile already points at the same endpoint.
func (m *Manager) List(ctx context.Context) ([]model.ConnectionProfile, error) {
	saved, err := m.store.Load()
	if err != nil {
		return nil, err
	}
	def := DefaultLocalProfile()
	out := make([]model.ConnectionProfile, 0, len(saved)+1)
	out = append(out, saved...)
	for _, p := range saved {
		if strings.EqualFold(p.Endpoint, def.Endpoint) {
			return out, nil
		}
	}
	return append(out, def), nil
}

func (m *Manager) resolve(id string) (model.ConnectionProfile, error) {
	saved, err := m.store.Load()
	if err != nil {
		return model.ConnectionProfile{}, err
	}
	for _, p := range saved {
		if p.ID == id {
			return p, nil
		}
	}
	if id == DiscoveredLocalID {
		return DefaultLocalProfile(), nil
	}
	return model.ConnectionProfile{}, fmt.Errorf("unknown connection %q", id)
}

// Save validates and persists a profile (new when ID is empty).
func (m *Manager) Save(ctx context.Context, p model.ConnectionProfile) (model.ConnectionProfile, error) {
	p, err := ValidateProfile(p)
	if err != nil {
		return model.ConnectionProfile{}, err
	}
	saved, err := m.store.Load()
	if err != nil {
		return model.ConnectionProfile{}, err
	}
	if p.ID == "" {
		p.ID = newID()
	}
	replaced := false
	for i := range saved {
		if saved[i].ID == p.ID {
			saved[i], replaced = p, true
		}
	}
	if !replaced {
		saved = append(saved, p)
	}
	if err := m.store.Save(saved); err != nil {
		return model.ConnectionProfile{}, err
	}
	return p, nil
}

// Remove disconnects and deletes a saved profile.
func (m *Manager) Remove(ctx context.Context, id string) error {
	if err := m.Disconnect(ctx, id); err != nil {
		return err
	}
	saved, err := m.store.Load()
	if err != nil {
		return err
	}
	out := saved[:0]
	for _, p := range saved {
		if p.ID != id {
			out = append(out, p)
		}
	}
	if len(out) == len(saved) {
		return nil
	}
	if err := m.store.Save(out); err != nil {
		return err
	}
	m.mu.Lock()
	delete(m.conns, id)
	m.mu.Unlock()
	_ = m.DeleteSecret(id, secrets.KindSSHPassword)
	_ = m.DeleteSecret(id, secrets.KindSSHPassphrase)
	return nil
}

// ---- lifecycle ----

// Client returns the live client for exactly this connection, or an error.
func (m *Manager) Client(ctx context.Context, connectionID string) (*client.Client, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.conns[connectionID]
	if !ok {
		return nil, fmt.Errorf("connection %q is not connected", connectionID)
	}
	if c.cli == nil || c.status.State != model.StateConnected {
		return nil, fmt.Errorf("connection %q is not connected (%s)", connectionID, c.status.State)
	}
	return c.cli, nil
}

// Status returns the last known status without side effects.
func (m *Manager) Status(ctx context.Context, id string) (model.ConnectionStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if c, ok := m.conns[id]; ok {
		return c.status, nil
	}
	return model.ConnectionStatus{ConnectionID: id, State: model.StateDisconnected}, nil
}

func (m *Manager) setStatus(id string, c *conn, st model.ConnectionStatus) {
	st.ConnectionID = id
	m.mu.Lock()
	c.status = st
	m.mu.Unlock()
	m.emitEvent(model.EventConnectionStatus, st)
}

var errNotConnected = errors.New("not connected")

// Profile implements ProfileSource.
func (m *Manager) Profile(connectionID string) (model.ConnectionProfile, error) {
	return m.resolve(connectionID)
}
