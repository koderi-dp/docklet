package services

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

const dfFixture = `{
 "ContainerUsage":{"TotalCount":4,"Items":[
  {"Id":"a1","ImageID":"sha256:img1","SizeRw":1000,"Labels":{"com.docker.compose.project":"shop"}},
  {"Id":"a2","ImageID":"sha256:img1","SizeRw":500,"Labels":{"com.docker.compose.project":"shop"}},
  {"Id":"a3","ImageID":"sha256:img2","SizeRw":-1,"Labels":{"com.docker.compose.project":"shop"}},
  {"Id":"b1","ImageID":"sha256:img1","SizeRw":7,"Labels":{}}]},
 "ImageUsage":{"TotalCount":2,"Items":[{"Id":"sha256:img1","Size":10000},{"Id":"sha256:img2","Size":-1}]},
 "VolumeUsage":{"TotalCount":3,"Items":[
  {"Name":"shop_db","Labels":{"com.docker.compose.project":"shop"},"UsageData":{"Size":4000,"RefCount":1}},
  {"Name":"shop_cache","Labels":{"com.docker.compose.project":"shop"},"UsageData":{"Size":-1,"RefCount":1}},
  {"Name":"other","Labels":{},"UsageData":{"Size":99,"RefCount":0}}]}
}`

func TestProjectDiskUsage_GroupsByLabelAndKeepsUnknownNil(t *testing.T) {
	cli := dockerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/system/df") && r.URL.Query().Get("verbose") != "" {
			_, _ = w.Write([]byte(dfFixture))
			return
		}
		w.WriteHeader(404)
	}), "1.52")
	s := &ComposeService{Engines: fakeProvider{cli}}
	du, err := s.ProjectDiskUsage(context.Background(), "c1")
	if err != nil {
		t.Fatal(err)
	}
	if len(du.Projects) != 1 || du.Projects[0].Project != "shop" {
		t.Fatalf("only labelled projects expected: %+v", du.Projects)
	}
	p := du.Projects[0]
	if p.ContainersBytes == nil || *p.ContainersBytes != 1500 || p.ContainerCount != 3 || p.ContainersKnown != 2 {
		t.Errorf("containers: %+v", p)
	}
	if p.VolumesBytes == nil || *p.VolumesBytes != 4000 || p.VolumeCount != 2 || p.VolumesKnown != 1 {
		t.Errorf("volumes: %+v", p)
	}
	// img1 counted once although two containers use it; img2 reports no size.
	if p.ImagesBytes == nil || *p.ImagesBytes != 10000 || p.ImageCount != 2 || p.ImagesKnown != 1 {
		t.Errorf("images: %+v", p)
	}
}

func TestProjectDiskFromUsage_NoSizesMeansNil(t *testing.T) {
	cli := dockerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ContainerUsage":{"Items":[{"Id":"a","SizeRw":-1,"Labels":{"com.docker.compose.project":"x"}}]}}`))
	}), "1.52")
	s := &ComposeService{Engines: fakeProvider{cli}}
	du, err := s.ProjectDiskUsage(context.Background(), "c1")
	if err != nil {
		t.Fatal(err)
	}
	p := du.Projects[0]
	if p.ContainersBytes != nil || p.VolumesBytes != nil || p.ImagesBytes != nil {
		t.Errorf("unknown sizes must stay nil: %+v", p)
	}
}
