package services

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/client"
)

type helperImage struct {
	Ref  string // tag or ID usable in container create
	Size int64
}

type helperInfo struct {
	ID      string
	Created int64 // unix seconds
}

// filesAPI is the narrow part of the Engine API used by the file browser (test seam).
type filesAPI interface {
	StatPath(ctx context.Context, id, p string) (container.PathStat, error)
	CopyFrom(ctx context.Context, id, p string) (io.ReadCloser, container.PathStat, error)
	CopyTo(ctx context.Context, id, dir string, content io.Reader) error
	Export(ctx context.Context, id string) (io.ReadCloser, error)
	// Running reports the state and display name of the container.
	Running(ctx context.Context, id string) (running bool, name string, err error)
	// Inspect returns the state needed to choose a listing method and to stamp cached indexes.
	Inspect(ctx context.Context, id string) (containerInfo, error)
	// Exec runs argv non-interactively without TTY and returns stdout (capped), stderr and the exit code.
	Exec(ctx context.Context, id string, argv []string, maxOut int64) (stdout []byte, stderr string, exit int, err error)

	Images(ctx context.Context) ([]helperImage, error)
	CreateHelper(ctx context.Context, image, volume, name string, readOnly bool) (string, error)
	RemoveContainer(ctx context.Context, id string) error
	ListHelpers(ctx context.Context) ([]helperInfo, error)
	VolumeRunningUsers(ctx context.Context, volume string) ([]string, error)
}

type sdkFilesAPI struct{ cli *client.Client }

func (a sdkFilesAPI) StatPath(ctx context.Context, id, p string) (container.PathStat, error) {
	r, err := a.cli.ContainerStatPath(ctx, id, client.ContainerStatPathOptions{Path: p})
	return r.Stat, err
}

func (a sdkFilesAPI) CopyFrom(ctx context.Context, id, p string) (io.ReadCloser, container.PathStat, error) {
	r, err := a.cli.CopyFromContainer(ctx, id, client.CopyFromContainerOptions{SourcePath: p})
	if err != nil {
		return nil, r.Stat, err
	}
	return r.Content, r.Stat, nil
}

func (a sdkFilesAPI) CopyTo(ctx context.Context, id, dir string, content io.Reader) error {
	_, err := a.cli.CopyToContainer(ctx, id, client.CopyToContainerOptions{DestinationPath: dir, Content: content})
	return err
}

func (a sdkFilesAPI) Export(ctx context.Context, id string) (io.ReadCloser, error) {
	r, err := a.cli.ContainerExport(ctx, id, client.ContainerExportOptions{})
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (a sdkFilesAPI) Running(ctx context.Context, id string) (bool, string, error) {
	r, err := a.cli.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil {
		return false, "", err
	}
	running := false
	if r.Container.State != nil {
		running = r.Container.State.Running
	}
	return running, strings.TrimPrefix(r.Container.Name, "/"), nil
}

// containerInfo is the part of ContainerInspect used by the file browser.
type containerInfo struct {
	Name       string
	Running    bool
	Paused     bool
	Restarting bool
	StartedAt  string
	FinishedAt string
	ImageID    string
}

// canExec reports whether a command can be started in the container right now.
func (c containerInfo) canExec() bool { return c.Running && !c.Paused && !c.Restarting }

// stamp changes whenever the container filesystem may have changed by running.
func (c containerInfo) stamp() string {
	return c.ImageID + "|" + c.StartedAt + "|" + c.FinishedAt
}

func (a sdkFilesAPI) Inspect(ctx context.Context, id string) (containerInfo, error) {
	r, err := a.cli.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil {
		return containerInfo{}, err
	}
	ci := containerInfo{Name: strings.TrimPrefix(r.Container.Name, "/"), ImageID: r.Container.Image}
	if st := r.Container.State; st != nil {
		ci.Running, ci.Paused, ci.Restarting = st.Running, st.Paused, st.Restarting
		ci.StartedAt, ci.FinishedAt = st.StartedAt, st.FinishedAt
	}
	return ci, nil
}

var errExecOutputTooLarge = errors.New("command output too large")

type capWriter struct {
	buf bytes.Buffer
	max int64
}

func (c *capWriter) Write(p []byte) (int, error) {
	if int64(c.buf.Len()+len(p)) > c.max {
		return 0, errExecOutputTooLarge
	}
	return c.buf.Write(p)
}

func (a sdkFilesAPI) Exec(ctx context.Context, id string, argv []string, maxOut int64) ([]byte, string, int, error) {
	created, err := a.cli.ExecCreate(ctx, id, client.ExecCreateOptions{AttachStdout: true, AttachStderr: true, Cmd: argv})
	if err != nil {
		return nil, "", -1, err
	}
	att, err := a.cli.ExecAttach(ctx, created.ID, client.ExecAttachOptions{})
	if err != nil {
		return nil, "", -1, err
	}
	defer att.Close()
	stop := context.AfterFunc(ctx, func() { att.Close() })
	defer stop()
	out, errw := &capWriter{max: maxOut}, &capWriter{max: 64 << 10}
	_, cerr := stdcopy.StdCopy(out, errw, att.Reader)
	if ctx.Err() != nil {
		return nil, "", -1, ctx.Err()
	}
	if cerr != nil {
		return nil, "", -1, cerr
	}
	// the exit code becomes available shortly after the stream ends
	var res client.ExecInspectResult
	for i := 0; i < 50; i++ {
		res, err = a.cli.ExecInspect(ctx, created.ID, client.ExecInspectOptions{})
		if err != nil {
			return nil, "", -1, err
		}
		if !res.Running {
			break
		}
		select {
		case <-ctx.Done():
			return nil, "", -1, ctx.Err()
		case <-time.After(40 * time.Millisecond):
		}
	}
	return out.buf.Bytes(), errw.buf.String(), res.ExitCode, nil
}

func (a sdkFilesAPI) Images(ctx context.Context) ([]helperImage, error) {
	res, err := a.cli.ImageList(ctx, client.ImageListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]helperImage, 0, len(res.Items))
	for _, im := range res.Items {
		ref := im.ID
		for _, t := range im.RepoTags {
			if t != "" && t != "<none>:<none>" {
				ref = t
				break
			}
		}
		out = append(out, helperImage{Ref: ref, Size: im.Size})
	}
	return out, nil
}

func (a sdkFilesAPI) CreateHelper(ctx context.Context, image, volume, name string, readOnly bool) (string, error) {
	res, err := a.cli.ContainerCreate(ctx, client.ContainerCreateOptions{
		Name: name,
		Config: &container.Config{
			Image:           image,
			Cmd:             []string{"true"},
			Entrypoint:      []string{},
			Labels:          map[string]string{helperLabel: "1"},
			NetworkDisabled: true,
		},
		HostConfig: &container.HostConfig{
			Mounts: []mount.Mount{{Type: mount.TypeVolume, Source: volume, Target: helperMountPath, ReadOnly: readOnly}},
		},
	})
	return res.ID, err
}

func (a sdkFilesAPI) RemoveContainer(ctx context.Context, id string) error {
	_, err := a.cli.ContainerRemove(ctx, id, client.ContainerRemoveOptions{Force: true})
	return err
}

func (a sdkFilesAPI) ListHelpers(ctx context.Context) ([]helperInfo, error) {
	res, err := a.cli.ContainerList(ctx, client.ContainerListOptions{All: true, Filters: make(client.Filters).Add("label", helperLabel+"=1")})
	if err != nil {
		return nil, err
	}
	out := make([]helperInfo, 0, len(res.Items))
	for _, c := range res.Items {
		out = append(out, helperInfo{ID: c.ID, Created: c.Created})
	}
	return out, nil
}

func (a sdkFilesAPI) VolumeRunningUsers(ctx context.Context, volume string) ([]string, error) {
	res, err := a.cli.ContainerList(ctx, client.ContainerListOptions{Filters: make(client.Filters).Add("volume", volume).Add("status", "running")})
	if err != nil {
		return nil, err
	}
	var names []string
	for _, c := range res.Items {
		n := c.ID
		if len(c.Names) > 0 {
			n = strings.TrimPrefix(c.Names[0], "/")
		}
		names = append(names, n)
	}
	return names, nil
}
