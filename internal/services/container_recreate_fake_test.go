package services

import (
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"testing"

	"docklet/internal/ops"
)

const oldContainerID = "oldid1234567890abcdef"

var anonVol = strings.Repeat("ab", 32) // 64 hex chars: an anonymous volume name

// oldContainerJSON is a running, standalone container "web" with env (one secret),
// a published port, a named volume, an anonymous volume, a read-only bind and
// settings the form does not expose (privileged).
var oldContainerJSON = `{"Id":"` + oldContainerID + `","Name":"/web","Image":"sha256:imgid",
 "State":{"Status":"running","Running":true},
 "Config":{"Image":"nginx:1.27-alpine","Hostname":"oldid1234567","Cmd":["nginx","-g","daemon off;"],
  "Env":["PATH=/bin","TZ=UTC","DB_PASSWORD=hunter2","MODE=blue"],"Labels":{"owner":"me"%LABELS%}},
 "HostConfig":{"NetworkMode":"bridge","Privileged":true,"RestartPolicy":{"Name":"always"},
  "PortBindings":{"80/tcp":[{"HostIp":"","HostPort":"8080"}]}},
 "Mounts":[{"Type":"volume","Name":"data","Destination":"/data","RW":true},
  {"Type":"volume","Name":"` + anonVol + `","Destination":"/anon","RW":true},
  {"Type":"bind","Source":"/h","Destination":"/c","RW":false}],
 "NetworkSettings":{"Networks":{"bridge":{}}}}`

const imageJSON = `{"Id":"sha256:imgid","Config":{"Env":["PATH=/bin"],"Cmd":["nginx","-g","daemon off;"]}}`

// dockerFake serves just enough of the Engine API for Recreate and records the
// call sequence ("METHOD /path?query"). failOn maps "METHOD /path-suffix" to an
// HTTP 500 message.
type dockerFake struct {
	mu         sync.Mutex
	calls      []string
	failOn     map[string]string
	createBody string
	compose    string
}

var versionPrefix = regexp.MustCompile(`^/v[0-9.]+`)

func (f *dockerFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p := versionPrefix.ReplaceAllString(r.URL.Path, "")
	call := r.Method + " " + p
	if r.URL.Query().Get("name") != "" {
		call += "?name=" + r.URL.Query().Get("name")
	}
	f.mu.Lock()
	f.calls = append(f.calls, call)
	fail, failing := "", false
	for k, msg := range f.failOn {
		if strings.HasPrefix(call, k) {
			fail, failing = msg, true
		}
	}
	f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	if failing {
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`{"message":"` + fail + `"}`))
		return
	}
	switch {
	case r.Method == http.MethodGet && p == "/containers/"+oldContainerID+"/json":
		_, _ = w.Write([]byte(strings.ReplaceAll(oldContainerJSON, "%LABELS%", f.composeLabel())))
	case r.Method == http.MethodGet && strings.HasPrefix(p, "/images/") && strings.HasSuffix(p, "/json"):
		_, _ = w.Write([]byte(imageJSON))
	case p == "/containers/create":
		b, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.createBody = string(b)
		f.mu.Unlock()
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"Id":"newid1234567890abcdef","Warnings":[]}`))
	case r.Method == http.MethodDelete || r.Method == http.MethodPost:
		w.WriteHeader(204)
	default:
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"message":"not found"}`))
	}
}

func (f *dockerFake) composeLabel() string {
	if f.compose == "" {
		return ""
	}
	return `,"com.docker.compose.project":"` + f.compose + `"`
}

func (f *dockerFake) sequence() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

// index returns the position of the first call with the prefix, or -1.
func (f *dockerFake) index(prefix string) int {
	for i, c := range f.sequence() {
		if strings.HasPrefix(c, prefix) {
			return i
		}
	}
	return -1
}

func newRecreateSvc(t *testing.T, f *dockerFake) (*ContainerService, *ops.Manager) {
	t.Helper()
	cli := dockerClient(t, f, "1.45")
	m := ops.New(nil)
	return &ContainerService{Engines: fakeProvider{cli}, Events: &recEmitter{}, Ops: m}, m
}

// indexExact returns the position of the first call equal to call, or -1.
func (f *dockerFake) indexExact(call string) int {
	for i, c := range f.sequence() {
		if c == call {
			return i
		}
	}
	return -1
}
