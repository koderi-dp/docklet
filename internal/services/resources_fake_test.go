package services

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"docklet/internal/model"
	"docklet/internal/ops"
)

// resFake is a minimal Docker API for volumes/networks/system. It records every
// mutating request ("METHOD /path?query").
type resFake struct {
	mu    sync.Mutex
	calls []string
}

func (f *resFake) called(sub string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.calls {
		if strings.Contains(c, sub) {
			return true
		}
	}
	return false
}

func newResFake(t *testing.T) (*resFake, fakeProvider, *recEmitter) {
	t.Helper()
	f := &resFake{}
	cli := dockerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if strings.HasPrefix(p, "/v1.") {
			p = p[strings.Index(p[1:], "/")+1:]
		}
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodGet {
			f.mu.Lock()
			f.calls = append(f.calls, r.Method+" "+p+"?"+r.URL.RawQuery)
			f.mu.Unlock()
		}
		switch {
		case p == "/containers/json":
			_, _ = w.Write([]byte(`[{"Id":"c1111111111111","Names":["/web"],"State":"running","Mounts":[{"Type":"volume","Name":"data","Destination":"/var/lib/data","RW":true}],
 "NetworkSettings":{"Networks":{"appnet":{"NetworkID":"n-app","IPAddress":"172.20.0.2"}}}}]`))
		case p == "/volumes" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"Volumes":[{"Name":"data","Driver":"local","Mountpoint":"/v/data","Scope":"local","Labels":null},{"Name":"spare","Driver":"local","Mountpoint":"/v/spare","Scope":"local","Labels":{"a":"b"}}]}`))
		case p == "/volumes/data" && r.Method == http.MethodDelete:
			w.WriteHeader(409)
			_, _ = w.Write([]byte(`{"message":"remove data: volume is in use - [c1111111111111]"}`))
		case strings.HasPrefix(p, "/volumes/") && r.Method == http.MethodDelete:
			w.WriteHeader(204)
		case p == "/volumes/prune":
			_, _ = w.Write([]byte(`{"VolumesDeleted":["spare"],"SpaceReclaimed":2048}`))
		case p == "/containers/prune":
			_, _ = w.Write([]byte(`{"ContainersDeleted":["x"],"SpaceReclaimed":10}`))
		case p == "/images/prune":
			_, _ = w.Write([]byte(`{"ImagesDeleted":[{"Untagged":"foo:latest"}],"SpaceReclaimed":99}`))
		case p == "/networks/prune":
			_, _ = w.Write([]byte(`{"NetworksDeleted":["old"]}`))
		case p == "/networks/bridge" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"Name":"bridge","Id":"n-bridge","Driver":"bridge","Scope":"local","IPAM":{"Config":[{"Subnet":"172.17.0.0/16","Gateway":"172.17.0.1"}]},"Containers":{}}`))
		case (p == "/networks/n-app" || p == "/networks/appnet") && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"Name":"appnet","Id":"n-app","Driver":"bridge","Scope":"local","IPAM":{"Config":[{"Subnet":"172.20.0.0/16","Gateway":"172.20.0.1"}]},"Containers":{"c1111111111111":{"Name":"web","IPv4Address":"172.20.0.2/16"}}}`))
		case p == "/networks/n-app" && r.Method == http.MethodDelete:
			w.WriteHeader(403)
			_, _ = w.Write([]byte(`{"message":"error while removing network: network appnet id n-app has active endpoints"}`))
		case p == "/system/df":
			_, _ = w.Write([]byte(`{"ImageUsage":{"ActiveCount":1,"TotalCount":3,"Reclaimable":500,"TotalSize":1000},
 "VolumeUsage":{"ActiveCount":1,"TotalCount":2,"Reclaimable":-1,"TotalSize":-1}}`))
		case p == "/events":
			now := time.Now().Unix()
			var b strings.Builder
			for i, a := range []string{"create", "start", "die"} {
				b.WriteString(`{"Type":"container","Action":"` + a + `","Actor":{"ID":"abcdef0123456789","Attributes":{"name":"web","image":"nginx","secret":"x"}},"time":` + strconv.FormatInt(now-int64(10-i), 10) + "}\n")
			}
			_, _ = w.Write([]byte(b.String()))
		default:
			w.WriteHeader(404)
			_, _ = w.Write([]byte(`{"message":"No such thing: ` + p + `"}`))
		}
	}), "1.52")
	return f, fakeProvider{cli}, &recEmitter{}
}

func waitOp(t *testing.T, m *ops.Manager, id string) model.Operation {
	t.Helper()
	for end := time.Now().Add(3 * time.Second); time.Now().Before(end); time.Sleep(5 * time.Millisecond) {
		for _, o := range m.List() {
			if o.ID == id && o.Status != model.OpRunning {
				return o
			}
		}
	}
	t.Fatal("operation did not finish")
	return model.Operation{}
}
