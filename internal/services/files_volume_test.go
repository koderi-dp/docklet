package services

import (
	"archive/tar"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
)

func TestVolumeBrowse_RemovesReadOnlyHelperEvenOnError(t *testing.T) {
	f := &fakeFilesAPI{images: []helperImage{{Ref: "big:1", Size: 900}, {Ref: "busybox:latest", Size: 4000}}, copyFromErr: errors.New("boom")}
	s := newFilesSvc(f)
	if _, err := s.VolumeBrowse(context.Background(), "c1", "myvol", "/"); err == nil {
		t.Fatal("expected error")
	}
	if len(f.created) != 1 || len(f.removed) != 1 || f.removed[0] != "helper-"+f.created[0] {
		t.Fatalf("created=%v removed=%v", f.created, f.removed)
	}
	if !f.readOnly[0] {
		t.Fatal("browse must mount the volume read-only")
	}
	if !strings.HasPrefix(f.created[0], "docklet-helper-") {
		t.Fatalf("name = %q", f.created[0])
	}
}

func TestVolumeBrowse_WithoutLocalImagesExplainsAndCreatesNoHelper(t *testing.T) {
	f := &fakeFilesAPI{}
	s := newFilesSvc(f)
	_, err := s.VolumeBrowse(context.Background(), "c1", "myvol", "/")
	if err == nil || !strings.Contains(err.Error(), "Pull busybox") || len(f.created) != 0 {
		t.Fatalf("err=%v created=%v", err, f.created)
	}
}

func TestPickHelperImage_PrefersKnownTinyImagesThenSmallest(t *testing.T) {
	got, _ := pickHelperImage([]helperImage{{"node:20", 100}, {"docker.io/library/alpine:3", 7000}, {"tiny:1", 10}})
	if got != "docker.io/library/alpine:3" {
		t.Fatalf("got %s", got)
	}
	got, _ = pickHelperImage([]helperImage{{"node:20", 100}, {"tiny:1", 10}})
	if got != "tiny:1" {
		t.Fatalf("got %s", got)
	}
}

func TestHelperSweep_RemovesOnlyStaleIdleHelpers(t *testing.T) {
	now := time.Now().Unix()
	f := &fakeFilesAPI{
		images:  []helperImage{{"busybox:1", 1}},
		helpers: []helperInfo{{"stale", now - 3600}, {"fresh", now}, {"busy", now - 3600}},
	}
	s := newFilesSvc(f)
	s.active.Store("c1/busy", struct{}{})
	f.copyFrom = makeTar(t, []tarItem{{name: "volume/", typ: tar.TypeDir}})
	f.copyStat = container.PathStat{Mode: fs.ModeDir}
	if _, err := s.VolumeBrowse(context.Background(), "c1", "myvol", "/"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f.removed[:1], []string{"stale"}) || len(f.removed) != 2 {
		t.Fatalf("removed = %v", f.removed)
	}
}

func TestVolumeImport_RefusesWithoutOverwriteConfirmation(t *testing.T) {
	f := &fakeFilesAPI{}
	_, err := newFilesSvc(f).VolumeImport(bg, "c1", "myvol", filepath.Join(t.TempDir(), "x.tar"), false)
	if err == nil || !strings.Contains(err.Error(), "Confirm") || len(f.created) != 0 {
		t.Fatalf("err=%v created=%v", err, f.created)
	}
}

func TestVolumeImport_RefusesWhileRunningContainersUseTheVolume(t *testing.T) {
	tarPath := filepath.Join(t.TempDir(), "x.tar")
	if err := os.WriteFile(tarPath, makeTar(t, []tarItem{{name: "a", typ: tar.TypeReg}}), 0o600); err != nil {
		t.Fatal(err)
	}
	f := &fakeFilesAPI{users: []string{"web"}}
	_, err := newFilesSvc(f).VolumeImport(bg, "c1", "myvol", tarPath, true)
	if err == nil || !strings.Contains(err.Error(), "in use by running containers (web)") || len(f.created) != 0 {
		t.Fatalf("err=%v created=%v", err, f.created)
	}
}
