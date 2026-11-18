package services

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"sync"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
)

type tarItem struct {
	name, link string
	typ        byte
	body       string
}

func makeTar(t *testing.T, items []tarItem) []byte {
	t.Helper()
	var b bytes.Buffer
	tw := tar.NewWriter(&b)
	for _, it := range items {
		h := &tar.Header{Name: it.name, Typeflag: it.typ, Linkname: it.link, Mode: 0o644, Size: int64(len(it.body)), ModTime: time.Unix(1700000000, 0)}
		if it.typ == tar.TypeDir {
			h.Mode = 0o755
		}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if it.body != "" {
			_, _ = tw.Write([]byte(it.body))
		}
	}
	_ = tw.Close()
	return b.Bytes()
}

type fakeFilesAPI struct {
	mu          sync.Mutex
	running     bool
	execArgv    [][]string
	execExit    int
	execStderr  string
	images      []helperImage
	created     []string
	readOnly    []bool
	removed     []string
	helpers     []helperInfo
	copyFromErr error
	copyFrom    []byte
	copyStat    container.PathStat
	statErr     error
	users       []string
	copied      bytes.Buffer
	startedAt   string
	copyFroms   int
}

func (f *fakeFilesAPI) StatPath(context.Context, string, string) (container.PathStat, error) {
	if f.statErr != nil {
		return container.PathStat{}, f.statErr
	}
	return container.PathStat{Name: "x", Mode: fs.ModeDir | 0o755}, nil
}

func (f *fakeFilesAPI) CopyFrom(context.Context, string, string) (io.ReadCloser, container.PathStat, error) {
	f.mu.Lock()
	f.copyFroms++
	f.mu.Unlock()
	if f.copyFromErr != nil {
		return nil, container.PathStat{}, f.copyFromErr
	}
	return io.NopCloser(bytes.NewReader(f.copyFrom)), f.copyStat, nil
}

func (f *fakeFilesAPI) CopyTo(_ context.Context, _ string, _ string, r io.Reader) error {
	_, err := io.Copy(&f.copied, r)
	return err
}

func (f *fakeFilesAPI) Export(context.Context, string) (io.ReadCloser, error) {
	return nil, errors.New("unused")
}

func (f *fakeFilesAPI) Running(context.Context, string) (bool, string, error) {
	return f.running, "c", nil
}

func (f *fakeFilesAPI) Inspect(context.Context, string) (containerInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return containerInfo{Name: "c", Running: f.running, StartedAt: f.startedAt, FinishedAt: "fin", ImageID: "sha256:img"}, nil
}

func (f *fakeFilesAPI) Exec(_ context.Context, _ string, argv []string, _ int64) ([]byte, string, int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.execArgv = append(f.execArgv, argv)
	return nil, f.execStderr, f.execExit, nil
}

func (f *fakeFilesAPI) Images(context.Context) ([]helperImage, error) { return f.images, nil }

func (f *fakeFilesAPI) CreateHelper(_ context.Context, image, volume, name string, ro bool) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.created = append(f.created, name)
	f.readOnly = append(f.readOnly, ro)
	return "helper-" + name, nil
}

func (f *fakeFilesAPI) RemoveContainer(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.removed = append(f.removed, id)
	return nil
}

func (f *fakeFilesAPI) ListHelpers(context.Context) ([]helperInfo, error) { return f.helpers, nil }

func (f *fakeFilesAPI) VolumeRunningUsers(context.Context, string) ([]string, error) {
	return f.users, nil
}

func newFilesSvc(f *fakeFilesAPI) *FileService {
	return &FileService{newAPI: func(context.Context, string) (filesAPI, error) { return f, nil }}
}

var bg = context.Background()
