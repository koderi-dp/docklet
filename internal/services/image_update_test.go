package services

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"docklet/internal/model"
)

const (
	digA = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	digB = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

// updateFake serves image inspect (with the given RepoDigests) and distribution
// inspect (answering the remote digest) and counts registry round trips.
type updateFake struct {
	mu      sync.Mutex
	repoDig string // JSON array body for RepoDigests
	remote  string
	hits    int
}

func (u *updateFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch {
	case strings.Contains(r.URL.Path, "/distribution/"):
		u.mu.Lock()
		u.hits++
		u.mu.Unlock()
		_, _ = w.Write([]byte(`{"Descriptor":{"mediaType":"application/vnd.oci.image.index.v1+json","digest":"` + u.remote + `","size":1},"Platforms":[]}`))
	case strings.Contains(r.URL.Path, "/images/"):
		_, _ = w.Write([]byte(`{"Id":"sha256:img","RepoDigests":` + u.repoDig + `}`))
	default:
		w.WriteHeader(404)
	}
}

func (u *updateFake) registryHits() int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.hits
}

func newUpdateSvc(t *testing.T, u *updateFake) *ImageService {
	t.Helper()
	return &ImageService{Engines: fakeProvider{dockerClient(t, u, "1.45")}, Events: &recEmitter{}}
}

func TestCheckUpdate_ReportsNoUpdate_WhenRemoteMatchesLocalIndexDigest(t *testing.T) {
	// A multi-arch tag is pulled by its index digest, which is what RepoDigests records.
	u := &updateFake{repoDig: `["nginx@` + digA + `"]`, remote: digA}
	st, err := newUpdateSvc(t, u).CheckUpdate(context.Background(), "c1", "nginx:1.27-alpine")
	if err != nil || st.Error != "" || st.UpdateAvailable || st.LocalDigest != digA || st.RemoteDigest != digA {
		t.Fatalf("st=%+v err=%v", st, err)
	}
}

func TestCheckUpdate_ReportsUpdate_WhenRemoteDigestDiffers(t *testing.T) {
	u := &updateFake{repoDig: `["nginx@` + digA + `"]`, remote: digB}
	st, _ := newUpdateSvc(t, u).CheckUpdate(context.Background(), "c1", "nginx")
	if !st.UpdateAvailable || st.RemoteDigest != digB || st.CheckedAt == 0 {
		t.Fatalf("%+v", st)
	}
}

func TestCheckUpdate_IgnoresDigestsOfOtherRepositories_WhenImageHasSeveralRepoDigests(t *testing.T) {
	u := &updateFake{repoDig: `["other/nginx@` + digB + `","nginx@` + digA + `"]`, remote: digA}
	st, _ := newUpdateSvc(t, u).CheckUpdate(context.Background(), "c1", "nginx:latest")
	if st.UpdateAvailable || st.Error != "" {
		t.Fatalf("%+v", st)
	}
}

func TestCheckUpdate_ExplainsLocalImage_WithoutAskingTheRegistry(t *testing.T) {
	u := &updateFake{repoDig: `[]`, remote: digA}
	st, _ := newUpdateSvc(t, u).CheckUpdate(context.Background(), "c1", "myapp:dev")
	if !strings.Contains(st.Error, "Not from a registry") || st.UpdateAvailable || u.registryHits() != 0 {
		t.Fatalf("%+v hits=%d", st, u.registryHits())
	}
}

func TestCheckUpdate_ExplainsDigestPinnedReference_WithoutAskingTheRegistry(t *testing.T) {
	u := &updateFake{repoDig: `["nginx@` + digA + `"]`, remote: digB}
	st, _ := newUpdateSvc(t, u).CheckUpdate(context.Background(), "c1", "nginx@"+digA)
	if !strings.Contains(st.Error, "Pinned by digest") || u.registryHits() != 0 {
		t.Fatalf("%+v", st)
	}
}

func TestCheckUpdates_UsesCacheUntilItExpires(t *testing.T) {
	u := &updateFake{repoDig: `["nginx@` + digA + `"]`, remote: digB}
	s := newUpdateSvc(t, u)
	now := time.Now()
	s.updates.now = func() time.Time { return now }
	check := func() model.ImageUpdateStatus {
		res, err := s.CheckUpdates(context.Background(), "c1", []string{"nginx"})
		if err != nil || len(res) != 1 {
			t.Fatalf("res=%v err=%v", res, err)
		}
		return res[0]
	}
	check()
	now = now.Add(14 * time.Minute)
	check()
	if u.registryHits() != 1 {
		t.Fatalf("within 15 minutes the registry must not be asked again, hits=%d", u.registryHits())
	}
	now = now.Add(2 * time.Minute)
	check()
	if u.registryHits() != 2 {
		t.Fatalf("after expiry the registry must be asked again, hits=%d", u.registryHits())
	}
	if _, err := s.CheckUpdate(context.Background(), "c1", "nginx"); err != nil || u.registryHits() != 3 {
		t.Fatalf("an explicit single check always asks the registry, hits=%d", u.registryHits())
	}
}
