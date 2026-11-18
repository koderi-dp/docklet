package services

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestParseExitCode_ReadsEngineStatusText(t *testing.T) {
	for status, want := range map[string]int{"Exited (0) 3 hours ago": 0, "Exited (137) 2 minutes ago": 137, "Exited (-1) 1 second ago": -1} {
		if got := parseExitCode(status); got == nil || *got != want {
			t.Errorf("%q: got %v want %d", status, got, want)
		}
	}
	for _, status := range []string{"Up 2 hours", "Created", ""} {
		if parseExitCode(status) != nil {
			t.Errorf("%q should have no exit code", status)
		}
	}
}

func TestList_FinishedOneShotIsCompletedAndServiceThatStoppedIsNot(t *testing.T) {
	exitedPolicies = policyCache{}
	inspects := 0
	cli := dockerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/containers/json"):
			_, _ = w.Write([]byte(`[
 {"Id":"mig","Names":["/catalo-migrate-1"],"State":"exited","Status":"Exited (0) 5 minutes ago","Created":1,"Labels":{"com.docker.compose.project":"catalo"}},
 {"Id":"svc","Names":["/catalo-cache-1"],"State":"exited","Status":"Exited (0) 5 minutes ago","Created":2,"Labels":{"com.docker.compose.project":"catalo"}},
 {"Id":"bad","Names":["/catalo-seed-1"],"State":"exited","Status":"Exited (2) 5 minutes ago","Created":3,"Labels":{"com.docker.compose.project":"catalo"}},
 {"Id":"run1","Names":["/catalo-run-1"],"State":"exited","Status":"Exited (0) 1 minute ago","Created":4,"Labels":{"com.docker.compose.project":"catalo","com.docker.compose.oneoff":"True"}}]`))
		case strings.HasSuffix(r.URL.Path, "/mig/json"):
			inspects++
			_, _ = w.Write([]byte(`{"Id":"mig","Name":"/m","HostConfig":{"RestartPolicy":{"Name":""}}}`))
		case strings.HasSuffix(r.URL.Path, "/svc/json"):
			inspects++
			_, _ = w.Write([]byte(`{"Id":"svc","Name":"/s","HostConfig":{"RestartPolicy":{"Name":"unless-stopped"}}}`))
		default:
			w.WriteHeader(404)
		}
	}), "1.47")
	svc := &ContainerService{Engines: fakeProvider{cli}}

	for range 2 { // second call must come from the cache
		rows, err := svc.List(context.Background(), "c1")
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]string{}
		for _, r := range rows {
			code := "-"
			if r.ExitCode != nil {
				code = string(rune('0' + *r.ExitCode))
			}
			got[r.ID] = code + "/" + r.RestartPolicy
		}
		want := map[string]string{"mig": "0/no", "svc": "0/unless-stopped", "bad": "2/", "run1": "0/no"}
		for id, w := range want {
			if got[id] != w {
				t.Errorf("%s: got %q want %q", id, got[id], w)
			}
		}
	}
	if inspects != 2 {
		t.Errorf("expected one inspect per exit-0 container (cached afterwards), got %d", inspects)
	}
}
