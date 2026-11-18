package services

import (
	"context"
	"sync"

	"github.com/moby/moby/client"

	"docklet/internal/compose"
	"docklet/internal/ports"
)

const (
	maxInspected    = 40 // stopped containers read for their configured bindings
	inspectParallel = 8
)

// engineSource reports the published ports of the containers of one Engine.
// Running containers come straight from the list; stopped ones need an inspect, because
// the list only shows ports that are bound.
type engineSource struct {
	cli    *client.Client
	filter client.Filters
	label  string // "Podman Engine"; empty means Docker
}

func (e engineSource) Bindings(ctx context.Context) ([]ports.Binding, error) {
	res, err := e.cli.ContainerList(ctx, client.ContainerListOptions{All: true, Filters: e.filter})
	if err != nil {
		return nil, err
	}
	var out []ports.Binding
	var stopped []int
	for i, it := range res.Items {
		name, project := firstName(it.Names), it.Labels[compose.LabelProject]
		if it.State == "running" || it.State == "restarting" || it.State == "paused" {
			for _, p := range it.Ports {
				if p.PublicPort == 0 {
					continue
				}
				ip := ""
				if p.IP.IsValid() {
					ip = p.IP.String()
				}
				out = append(out, ports.Binding{ContainerID: it.ID, Name: name, Project: project, Running: true,
					HostIP: ip, HostPort: int(p.PublicPort), Protocol: p.Type, Engine: e.label})
			}
			continue
		}
		stopped = append(stopped, i)
	}
	if len(stopped) > maxInspected {
		stopped = stopped[:maxInspected]
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, inspectParallel)
	for _, i := range stopped {
		it := res.Items[i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			insp, err := e.cli.ContainerInspect(ctx, it.ID, client.ContainerInspectOptions{})
			if err != nil || insp.Container.HostConfig == nil {
				return // a note about a stopped container is optional
			}
			mu.Lock()
			defer mu.Unlock()
			for port, bs := range insp.Container.HostConfig.PortBindings {
				for _, b := range bs {
					n := 0
					for _, c := range b.HostPort {
						if c < '0' || c > '9' {
							n = 0
							break
						}
						n = n*10 + int(c-'0')
					}
					if n == 0 {
						continue
					}
					ip := ""
					if b.HostIP.IsValid() {
						ip = b.HostIP.String()
					}
					out = append(out, ports.Binding{ContainerID: it.ID, Name: firstName(it.Names), Project: it.Labels[compose.LabelProject],
						HostIP: ip, HostPort: n, Protocol: string(port.Proto()), Engine: e.label})
				}
			}
		}()
	}
	wg.Wait()
	return out, nil
}
