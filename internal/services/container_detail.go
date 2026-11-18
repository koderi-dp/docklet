package services

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"

	"docklet/internal/model"
)

// StatsDetail takes one detailed sample of a running container. Values the
// Engine does not report stay nil.
func (s *ContainerService) StatsDetail(ctx context.Context, connectionID string, id string) (model.ContainerStatDetail, error) {
	cli, err := s.client(ctx, connectionID)
	if err != nil {
		return model.ContainerStatDetail{}, err
	}
	res, err := cli.ContainerStats(ctx, id, client.ContainerStatsOptions{IncludePreviousSample: true})
	if err != nil {
		return model.ContainerStatDetail{}, friendlyError(err)
	}
	defer res.Body.Close()
	var sr container.StatsResponse
	if err := json.NewDecoder(res.Body).Decode(&sr); err != nil {
		return model.ContainerStatDetail{}, friendlyError(err)
	}
	return detailFromResponse(id, sr), nil
}

func detailFromResponse(id string, sr container.StatsResponse) model.ContainerStatDetail {
	base := statFromResponse(id, sr)
	d := model.ContainerStatDetail{
		ID: id, CPUPercent: base.CPUPercent, MemoryBytes: base.MemoryBytes, MemoryLimit: base.MemoryLimit,
	}
	if !sr.Read.IsZero() && sr.Read.Year() > 1 {
		d.Timestamp = sr.Read.Format(time.RFC3339Nano)
	}
	if len(sr.Networks) > 0 {
		var rx, tx uint64
		for _, n := range sr.Networks {
			rx += n.RxBytes
			tx += n.TxBytes
		}
		d.NetRxBytes, d.NetTxBytes = &rx, &tx
	}
	if entries := sr.BlkioStats.IoServiceBytesRecursive; len(entries) > 0 {
		var r, w uint64
		for _, e := range entries {
			switch strings.ToLower(e.Op) {
			case "read":
				r += e.Value
			case "write":
				w += e.Value
			}
		}
		d.BlockReadBytes, d.BlockWriteBytes = &r, &w
	} else if st := sr.StorageStats; st.ReadSizeBytes > 0 || st.WriteSizeBytes > 0 {
		r, w := st.ReadSizeBytes, st.WriteSizeBytes
		d.BlockReadBytes, d.BlockWriteBytes = &r, &w
	}
	if sr.PidsStats.Current > 0 {
		p := sr.PidsStats.Current
		d.Pids = &p
	}
	return d
}

// Top lists the processes of a running container (Engine "top").
func (s *ContainerService) Top(ctx context.Context, connectionID string, id string) (model.ContainerProcesses, error) {
	cli, err := s.client(ctx, connectionID)
	if err != nil {
		return model.ContainerProcesses{}, err
	}
	res, err := cli.ContainerTop(ctx, id, client.ContainerTopOptions{})
	if err != nil {
		return model.ContainerProcesses{}, friendlyError(err)
	}
	out := model.ContainerProcesses{Titles: res.Titles, Processes: res.Processes}
	if out.Titles == nil {
		out.Titles = []string{}
	}
	if out.Processes == nil {
		out.Processes = [][]string{}
	}
	return out, nil
}

// InspectRaw returns the pretty-printed Engine inspect JSON with the values of
// secret-like environment variables masked (same rule as Inspect).
func (s *ContainerService) InspectRaw(ctx context.Context, connectionID string, id string) (string, error) {
	cli, err := s.client(ctx, connectionID)
	if err != nil {
		return "", err
	}
	res, err := cli.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil {
		return "", friendlyError(err)
	}
	return maskInspectJSON(res.Raw)
}

func maskInspectJSON(raw []byte) (string, error) {
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return "", err
	}
	if cfg, ok := doc["Config"].(map[string]any); ok {
		if env, ok := cfg["Env"].([]any); ok {
			for i, e := range env {
				str, _ := e.(string)
				k, _, found := strings.Cut(str, "=")
				if found && isSecretKey(k) {
					env[i] = k + "=" + maskedValue
				}
			}
		}
	}
	b, err := json.MarshalIndent(doc, "", "  ")
	return string(b), err
}

// Pause freezes containers (per-ID results, like Start/Stop).
func (s *ContainerService) Pause(ctx context.Context, connectionID string, ids []string) ([]model.ActionResult, error) {
	return s.lifecycle(ctx, connectionID, ids, actionPause)
}

// Unpause resumes paused containers.
func (s *ContainerService) Unpause(ctx context.Context, connectionID string, ids []string) ([]model.ActionResult, error) {
	return s.lifecycle(ctx, connectionID, ids, actionUnpause)
}

// Kill sends SIGKILL to running containers. The UI must have confirmed.
func (s *ContainerService) Kill(ctx context.Context, connectionID string, ids []string) ([]model.ActionResult, error) {
	return s.lifecycle(ctx, connectionID, ids, actionKill)
}

// Rename changes a container name; emits containers:changed on success.
func (s *ContainerService) Rename(ctx context.Context, connectionID string, id string, newName string) error {
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return errEmptyName
	}
	cli, err := s.client(ctx, connectionID)
	if err != nil {
		return err
	}
	if _, err := cli.ContainerRename(ctx, id, client.ContainerRenameOptions{NewName: newName}); err != nil {
		return friendlyError(err)
	}
	if s.Events != nil {
		s.Events.Emit(model.EventContainersChanged, model.ContainersChanged{ConnectionID: connectionID})
	}
	return nil
}
