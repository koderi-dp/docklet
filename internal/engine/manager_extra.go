package engine

import (
	"context"
	"fmt"
	"strings"
	"time"

	"docklet/internal/model"
)

// endpointLabel is a credential-free description used in error messages.
func endpointLabel(p model.ConnectionProfile) string {
	if p.Type == model.ConnWSL {
		return "WSL (" + p.WSLDistro + ")"
	}
	return p.Endpoint
}

// Test connects with a temporary client, pings and reads info, then closes it.
// It never persists the profile, registers a connection, starts an event stream,
// emits events, or becomes the client handed out by Client(). Failures come back
// as State=error with a friendly message and redacted diagnostics.
func (m *Manager) Test(ctx context.Context, p model.ConnectionProfile) (model.ConnectionStatus, error) {
	id := p.ID
	vp, err := ValidateProfile(p)
	if err != nil {
		return model.ConnectionStatus{ConnectionID: id, State: model.StateError, Error: err.Error(), Diagnostics: Redact(err.Error())}, nil
	}
	fail := func(err error) (model.ConnectionStatus, error) {
		return model.ConnectionStatus{ConnectionID: id, State: model.StateError,
			Error: friendlyError(err, endpointLabel(vp)), Diagnostics: Redact(err.Error())}, nil
	}
	cli, err := m.newClient(vp)
	if err != nil {
		return fail(err)
	}
	defer cli.Close()
	pctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	st, err := m.probe(pctx, cli, vp)
	if err != nil {
		return fail(err)
	}
	st.ConnectionID = id
	return st, nil
}

// Duplicate saves a copy of a saved or discovered profile under a new ID and a unique name.
func (m *Manager) Duplicate(ctx context.Context, id string) (model.ConnectionProfile, error) {
	p, err := m.resolve(id)
	if err != nil {
		return model.ConnectionProfile{}, err
	}
	saved, err := m.store.Load()
	if err != nil {
		return model.ConnectionProfile{}, err
	}
	taken := map[string]bool{}
	for _, s := range saved {
		taken[strings.ToLower(s.Name)] = true
	}
	base := p.Name + " copy"
	name := base
	for i := 2; taken[strings.ToLower(name)]; i++ {
		name = fmt.Sprintf("%s %d", base, i)
	}
	p.ID, p.Name, p.Discovered = "", name, false
	return m.Save(ctx, p)
}
