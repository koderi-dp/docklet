package wsldocker

import (
	"context"
	"errors"
	"fmt"
	"time"

	"docklet/internal/model"
)

// Start starts the Docker daemon of an installed engine as root and waits until
// it answers. It refuses Docker Desktop's proxy and distributions without an engine.
func Start(ctx context.Context, ex Exec, distro string) (Facts, error) {
	f, err := Detect(ctx, ex, distro)
	if err != nil {
		return f, err
	}
	switch f.State() {
	case model.WSLStateRunning:
		return f, nil
	case model.WSLStateDockerDesktop:
		return f, errors.New("this distribution uses Docker Desktop's integration; start Docker Desktop instead")
	case model.WSLStateNotInstalled:
		return f, errors.New("Docker Engine is not installed in this distribution")
	}
	var last []string
	err = ex.Run(ctx, distro, StartCommand(f.Systemd), func(l string) { last = append(last, l) })
	if err != nil {
		return f, fmt.Errorf("could not start the Docker service: %w (%s)", err, tail(last, 3))
	}
	deadline := time.Now().Add(20 * time.Second)
	for {
		if f, err = Detect(ctx, ex, distro); err != nil || f.ServerVersion != "" {
			return f, err
		}
		if time.Now().After(deadline) {
			return f, errors.New("the Docker service was started but the daemon is not answering yet; check `journalctl -u docker` or /var/log/docker.log in the distribution")
		}
		select {
		case <-ctx.Done():
			return f, ctx.Err()
		case <-time.After(time.Second):
		}
	}
}
