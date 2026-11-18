package services

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"docklet/internal/model"
)

func TestImageRemove_ReportsPerItemFriendlyErrors(t *testing.T) {
	cli := dockerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		p := r.URL.Path
		switch {
		case r.Method == http.MethodDelete && strings.HasSuffix(p, "/images/busy:1"):
			w.WriteHeader(409)
			_, _ = w.Write([]byte(`{"message":"conflict: unable to delete 0123456789ab (must be forced) - image is being used by running container 5a5a5a5a5a5a"}`))
		case r.Method == http.MethodDelete && strings.HasSuffix(p, "/images/gone:1"):
			w.WriteHeader(404)
			_, _ = w.Write([]byte(`{"message":"No such image: gone:1"}`))
		case r.Method == http.MethodDelete && strings.HasSuffix(p, "/images/free:1"):
			_, _ = w.Write([]byte(`[{"Untagged":"free:1"},{"Deleted":"sha256:fff"}]`))
		case strings.HasSuffix(p, "/images/busy:1/json"):
			_, _ = w.Write([]byte(`{"Id":"sha256:busy","RepoTags":["busy:1"]}`))
		case strings.HasSuffix(p, "/containers/json"):
			_, _ = w.Write([]byte(`[{"Id":"5a5a5a5a5a5a5a","Names":["/web-1"],"ImageID":"sha256:busy","State":"running"},{"Id":"6b","Names":["/web-2"],"ImageID":"sha256:busy","State":"exited"}]`))
		default:
			w.WriteHeader(404)
		}
	}), "1.45")
	em := &recEmitter{}
	s := &ImageService{Engines: fakeProvider{cli}, Events: em}
	res, err := s.Remove(context.Background(), "c1", []string{"busy:1", "gone:1", "free:1", "bad ref"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 4 {
		t.Fatalf("one result per item, got %d", len(res))
	}
	if res[0].OK || !strings.Contains(res[0].Error, "used by containers web-1, web-2") {
		t.Errorf("conflict message: %+v", res[0])
	}
	if res[1].OK || !strings.Contains(res[1].Error, "not found") {
		t.Errorf("not found: %+v", res[1])
	}
	if !res[2].OK {
		t.Errorf("free: %+v", res[2])
	}
	if res[3].OK || res[3].Error == "" {
		t.Errorf("invalid ref must fail without calling the engine: %+v", res[3])
	}
	if len(em.names()) != 1 || em.names()[0] != model.EventImagesChanged {
		t.Errorf("events = %v", em.names())
	}
	if _, err := s.Remove(context.Background(), "nope", []string{"x:1"}, false); err == nil {
		t.Error("unknown connection must error")
	}
}
