package engine

import (
	"context"
	"docklet/internal/model"
	"time"

	"github.com/moby/moby/client"
)

// Disconnect stops streams, closes the client and emits the new status.
func (m *Manager) Disconnect(ctx context.Context, id string) error {
	l := m.lockFor(id)
	l.Lock()
	defer l.Unlock()
	m.mu.RLock()
	c := m.conns[id]
	m.mu.RUnlock()
	if c == nil {
		return nil
	}
	m.teardown(id)
	m.setStatus(id, c, model.ConnectionStatus{State: model.StateDisconnected})
	return nil
}

// Connect (re)establishes the client. Failures are reported in the returned
// status (State=error); the Go error is only for unknown IDs.
func (m *Manager) Connect(ctx context.Context, id string) (model.ConnectionStatus, error) {
	p, err := m.resolve(id)
	if err != nil {
		return model.ConnectionStatus{ConnectionID: id, State: model.StateError, Error: err.Error()}, err
	}
	l := m.lockFor(id)
	l.Lock()
	defer l.Unlock()

	m.teardown(id)
	m.mu.Lock()
	c := &conn{profile: p}
	m.conns[id] = c
	m.mu.Unlock()
	m.setStatus(id, c, model.ConnectionStatus{State: model.StateConnecting})

	fail := func(err error) (model.ConnectionStatus, error) {
		st := model.ConnectionStatus{State: model.StateError,
			Error: friendlyError(err, endpointLabel(p)), Diagnostics: Redact(err.Error())}
		m.setStatus(id, c, st)
		st.ConnectionID = id
		return st, nil
	}

	cli, err := m.newClient(p)
	if err != nil {
		return fail(err)
	}
	pctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	st, err := m.probe(pctx, cli, p)
	if err != nil {
		_ = cli.Close()
		return fail(err)
	}

	evCtx, evCancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	m.mu.Lock()
	c.cli, c.cancel, c.done = cli, evCancel, done
	m.mu.Unlock()
	m.setStatus(id, c, st)
	go func() {
		defer close(done)
		m.runEvents(evCtx, id, c, cli)
	}()
	st.ConnectionID = id
	return st, nil
}

// probe pings and gathers info/counts, producing a connected status.
func (m *Manager) probe(ctx context.Context, cli *client.Client, p model.ConnectionProfile) (model.ConnectionStatus, error) {
	ping, err := cli.Ping(ctx, client.PingOptions{NegotiateAPIVersion: true})
	if err != nil {
		return model.ConnectionStatus{}, err
	}
	info, err := cli.Info(ctx, client.InfoOptions{})
	if err != nil {
		return model.ConnectionStatus{}, err
	}
	st := model.ConnectionStatus{
		State:         model.StateConnected,
		ServerVersion: info.Info.ServerVersion,
		APIVersion:    ping.APIVersion,
		OS:            info.Info.OSType,
		Arch:          info.Info.Architecture,
		NCPU:          info.Info.NCPU,
		MemTotal:      info.Info.MemTotal,
		Containers:    info.Info.Containers,
		Images:        info.Info.Images,
	}
	// Best-effort counts; failures leave 0 rather than failing the connection.
	if v, err := cli.VolumeList(ctx, client.VolumeListOptions{}); err == nil {
		st.Volumes = len(v.Items)
	}
	if n, err := cli.NetworkList(ctx, client.NetworkListOptions{}); err == nil {
		st.Networks = len(n.Items)
	}
	ok, reason := composeAvailable(ctx, p)
	st.Capabilities = model.Capabilities{
		CanListResources: true, CanExecuteContainers: true, CanStreamLogs: true, CanStreamStats: true,
		CanBuildImages: true, CanRunCompose: ok, ComposeReason: reason,
	}
	return st, nil
}

// Shutdown disconnects everything (app exit).
func (m *Manager) Shutdown() {
	m.mu.RLock()
	ids := make([]string, 0, len(m.conns))
	for id := range m.conns {
		ids = append(ids, id)
	}
	m.mu.RUnlock()
	for _, id := range ids {
		_ = m.Disconnect(context.Background(), id)
	}
}

// teardown stops streams and closes the client; the entry stays as disconnected.
func (m *Manager) teardown(id string) {
	m.mu.Lock()
	c := m.conns[id]
	m.mu.Unlock()
	if c == nil {
		return
	}
	if c.cancel != nil {
		c.cancel()
	}
	if c.done != nil {
		<-c.done
	}
	m.mu.Lock()
	cli := c.cli
	c.cli, c.cancel, c.done = nil, nil, nil
	m.mu.Unlock()
	if cli != nil {
		_ = cli.Close()
	}
}
