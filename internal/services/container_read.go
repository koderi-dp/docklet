package services

import (
	"strings"

	"github.com/moby/moby/api/types/container"

	"docklet/internal/model"
)

const (
	maxLogTail     = 2000
	defaultLogTail = 500
)

func trimName(n string) string { return strings.TrimPrefix(n, "/") }

func mapSummary(connectionID string, it container.Summary) model.Container {
	name := ""
	if len(it.Names) > 0 {
		name = trimName(it.Names[0])
	}
	return model.Container{
		ConnectionID:       connectionID,
		ID:                 it.ID,
		Name:               name,
		Image:              it.Image,
		State:              string(it.State),
		Status:             it.Status,
		Health:             summaryHealth(it),
		Created:            it.Created,
		Ports:              mapPorts(it.Ports),
		ComposeProject:     it.Labels["com.docker.compose.project"],
		ComposeService:     it.Labels["com.docker.compose.service"],
		ComposeWorkingDir:  it.Labels["com.docker.compose.project.working_dir"],
		ComposeConfigFiles: it.Labels["com.docker.compose.project.config_files"],
	}
}

func mapHealthStatus(s container.HealthStatus) model.Health {
	switch s {
	case container.Healthy:
		return model.HealthHealthy
	case container.Unhealthy:
		return model.HealthUnhealthy
	case container.Starting:
		return model.HealthStarting
	}
	return model.HealthNone
}

// summaryHealth derives health only from real Engine data: the structured summary
// when present, else the "(healthy)" / "(unhealthy)" / "(health: starting)" status text.
func summaryHealth(it container.Summary) model.Health {
	if it.Health != nil {
		return mapHealthStatus(it.Health.Status)
	}
	st := strings.ToLower(it.Status)
	switch {
	case strings.Contains(st, "(unhealthy)"):
		return model.HealthUnhealthy
	case strings.Contains(st, "(healthy)"):
		return model.HealthHealthy
	case strings.Contains(st, "(health: starting)"):
		return model.HealthStarting
	}
	return model.HealthNone
}

func mapPorts(in []container.PortSummary) []model.PortBinding {
	out := make([]model.PortBinding, 0, len(in))
	for _, p := range in {
		pb := model.PortBinding{
			ContainerPort: int(p.PrivatePort),
			HostPort:      int(p.PublicPort),
			Protocol:      p.Type,
		}
		if p.IP.IsValid() {
			pb.HostIP = p.IP.String()
		}
		out = append(out, pb)
	}
	return out
}

// ---- Stats ----

// ---- Inspect ----

// ---- Logs ----
