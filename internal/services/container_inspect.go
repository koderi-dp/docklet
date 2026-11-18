package services

import (
	"context"
	"docklet/internal/model"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

var secretKeyRe = regexp.MustCompile(`(?i)(PASSWORD|PASS|SECRET|TOKEN|KEY|CREDENTIAL|AUTH|PRIVATE)`)

func isSecretKey(k string) bool { return secretKeyRe.MatchString(k) }

func (s *ContainerService) Inspect(ctx context.Context, connectionID string, id string) (model.ContainerDetail, error) {
	ctx, cancel := callCtx(ctx)
	defer cancel()
	cli, err := s.client(ctx, connectionID)
	if err != nil {
		return model.ContainerDetail{}, err
	}
	res, err := cli.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil {
		return model.ContainerDetail{}, friendlyError(err)
	}
	return mapInspect(connectionID, res.Container), nil
}

func mapInspect(connectionID string, in container.InspectResponse) model.ContainerDetail {
	d := model.ContainerDetail{
		Container: model.Container{
			ConnectionID: connectionID,
			ID:           in.ID,
			Name:         trimName(in.Name),
			Image:        in.Image,
			Health:       model.HealthNone,
			Ports:        []model.PortBinding{},
		},
		Mounts:   []model.Mount{},
		Networks: []string{},
		Labels:   map[string]string{},
		Env:      []model.EnvVar{},
	}
	if t, err := time.Parse(time.RFC3339Nano, in.Created); err == nil {
		d.Created = t.Unix()
	}
	if in.State != nil {
		d.State = string(in.State.Status)
		if d.State == "exited" {
			code := in.State.ExitCode
			d.ExitCode = &code
		}
		d.StartedAt = zeroTime(in.State.StartedAt)
		d.FinishedAt = zeroTime(in.State.FinishedAt)
		if in.State.Health != nil {
			d.Health = mapHealthStatus(in.State.Health.Status)
		}
	}
	if c := in.Config; c != nil {
		if c.Image != "" {
			d.Image = c.Image
		}
		d.Command = strings.Join(c.Cmd, " ")
		d.Entrypoint = strings.Join(c.Entrypoint, " ")
		for k, v := range c.Labels {
			d.Labels[k] = v
		}
		d.ComposeProject = c.Labels["com.docker.compose.project"]
		d.ComposeService = c.Labels["com.docker.compose.service"]
		d.ComposeWorkingDir = c.Labels["com.docker.compose.project.working_dir"]
		d.ComposeConfigFiles = c.Labels["com.docker.compose.project.config_files"]
		for _, e := range c.Env {
			k, v, _ := strings.Cut(e, "=")
			if isSecretKey(k) {
				d.Env = append(d.Env, model.EnvVar{Key: k, Masked: true})
			} else {
				d.Env = append(d.Env, model.EnvVar{Key: k, Value: v})
			}
		}
	}
	if hc := in.HostConfig; hc != nil {
		d.RestartPolicy = string(hc.RestartPolicy.Name)
		if d.RestartPolicy == "" {
			d.RestartPolicy = "no"
		}
		if hc.RestartPolicy.Name == "on-failure" && hc.RestartPolicy.MaximumRetryCount > 0 {
			d.RestartPolicy += ":" + strconv.Itoa(hc.RestartPolicy.MaximumRetryCount)
		}
	}
	for _, m := range in.Mounts {
		src := m.Source
		if src == "" {
			src = m.Name
		}
		d.Mounts = append(d.Mounts, model.Mount{Type: string(m.Type), Source: src, Destination: m.Destination, ReadOnly: !m.RW})
	}
	if ns := in.NetworkSettings; ns != nil {
		for n := range ns.Networks {
			d.Networks = append(d.Networks, n)
		}
		sort.Strings(d.Networks)
		for p, bs := range ns.Ports {
			if len(bs) == 0 {
				d.Ports = append(d.Ports, model.PortBinding{ContainerPort: int(p.Num()), Protocol: string(p.Proto())})
				continue
			}
			for _, b := range bs {
				pb := model.PortBinding{ContainerPort: int(p.Num()), Protocol: string(p.Proto())}
				if b.HostIP.IsValid() {
					pb.HostIP = b.HostIP.String()
				}
				pb.HostPort, _ = strconv.Atoi(b.HostPort)
				d.Ports = append(d.Ports, pb)
			}
		}
		sort.Slice(d.Ports, func(i, j int) bool {
			if d.Ports[i].ContainerPort != d.Ports[j].ContainerPort {
				return d.Ports[i].ContainerPort < d.Ports[j].ContainerPort
			}
			return d.Ports[i].HostPort < d.Ports[j].HostPort
		})
	}
	return d
}

func zeroTime(s string) string {
	if strings.HasPrefix(s, "0001-01-01") {
		return ""
	}
	return s
}
