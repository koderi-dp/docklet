package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func serve(t *testing.T, h http.HandlerFunc) *Checker {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &Checker{URL: srv.URL, Now: func() time.Time { return time.Unix(1_700_000_000, 0) }}
}

func TestCheck_NewerReleaseIsReportedWithPlainNotes(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "docklet" || r.Header.Get("Authorization") != "" || r.URL.RawQuery != "" {
			t.Errorf("request must be anonymous and carry no user data: %v %q", r.Header, r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"tag_name":"v0.2.0","html_url":"https://github.com/koderi-dp/docklet/releases/tag/v0.2.0","body":"## New\n**Tray** support, see [docs](https://x.y)","published_at":"2026-01-02T03:04:05Z"}`))
	})
	st := c.Check(context.Background(), "0.1.0")
	if !st.UpdateAvailable || st.LatestVersion != "0.2.0" || st.Error != "" {
		t.Fatalf("unexpected status: %+v", st)
	}
	if st.ReleaseNotes != "New\nTray support, see docs" {
		t.Errorf("notes not plain text: %q", st.ReleaseNotes)
	}
}

func TestCheck_SameVersionIsUpToDate(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"v0.1.0","html_url":"https://github.com/x/y"}`))
	})
	if st := c.Check(context.Background(), "0.1.0"); st.UpdateAvailable || st.LatestVersion != "0.1.0" {
		t.Fatalf("same version must not be an update: %+v", st)
	}
}

func TestCheck_ForeignReleaseURLIsDropped(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"v9.0.0","html_url":"https://evil.example/x"}`))
	})
	if st := c.Check(context.Background(), "0.1.0"); st.ReleaseURL != "" {
		t.Fatalf("release url must stay on github.com: %q", st.ReleaseURL)
	}
}

func TestCheck_NoReleasesIsAMessageNotAnError(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, _ *http.Request) { http.NotFound(w, nil) })
	st := c.Check(context.Background(), "0.1.0")
	if st.Error != "" || st.Message != "No published releases found." || st.UpdateAvailable {
		t.Fatalf("404 must be a calm message: %+v", st)
	}
}

func TestCheck_RateLimitMentionsWaitTime(t *testing.T) {
	for _, status := range []int{429, 403} {
		c := serve(t, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Retry-After", "120")
			w.WriteHeader(status)
		})
		st := c.Check(context.Background(), "0.1.0")
		if !strings.Contains(st.Error, "2 min") {
			t.Errorf("%d: want a retry hint, got %+v", status, st)
		}
	}
}

func TestCheck_PlainForbiddenIsNotCalledRateLimit(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(403) })
	if st := c.Check(context.Background(), "0.1.0"); strings.Contains(st.Error, "rate") || st.Error == "" {
		t.Fatalf("unexpected: %+v", st)
	}
}

func TestCheck_TimeoutAndOfflineReportErrors(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() })
	c.Client = &http.Client{Timeout: 50 * time.Millisecond}
	if st := c.Check(context.Background(), "0.1.0"); st.Error == "" {
		t.Fatalf("timeout must be reported: %+v", st)
	}
	off := &Checker{URL: "http://127.0.0.1:1"}
	if st := off.Check(context.Background(), "0.1.0"); st.Error == "" || st.UpdateAvailable {
		t.Fatalf("offline must be reported: %+v", st)
	}
}
