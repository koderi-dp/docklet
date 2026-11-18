package services

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"docklet/internal/model"
	"docklet/internal/update"
)

type memKV struct {
	mu sync.Mutex
	m  map[string]string
}

func (k *memKV) GetSetting(_ context.Context, key string) (string, bool, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	v, ok := k.m[key]
	return v, ok, nil
}

func (k *memKV) SetSetting(_ context.Context, key, v string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.m == nil {
		k.m = map[string]string{}
	}
	k.m[key] = v
	return nil
}

type nameEmitter struct{ names []string }

func (r *nameEmitter) Emit(name string, _ any) { r.names = append(r.names, name) }

type releaseServer struct {
	*httptest.Server
	tag  string
	hits int
}

func newReleaseServer(t *testing.T, tag string) *releaseServer {
	rs := &releaseServer{tag: tag}
	rs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		rs.hits++
		_, _ = w.Write([]byte(`{"tag_name":"` + rs.tag + `","html_url":"https://github.com/koderi-dp/docklet/releases"}`))
	}))
	t.Cleanup(rs.Close)
	return rs
}

func newApp(srv *releaseServer, now *time.Time) (*AppService, *nameEmitter) {
	ev := &nameEmitter{}
	return &AppService{
		KV: &memKV{}, Events: ev, Version: "0.1.0",
		Checker: &update.Checker{URL: srv.URL, Now: func() time.Time { return *now }},
		Now:     func() time.Time { return *now },
	}, ev
}

func TestAppSettings_DefaultToEverythingOff(t *testing.T) {
	got, _ := (&AppService{KV: &memKV{}}).GetAppSettings(context.Background())
	if got != (model.AppSettings{CheckForUpdates: model.UpdateCheckNever}) {
		t.Fatalf("defaults must be all off: %+v", got)
	}
}

func TestScheduledCheck_NeverMakesNoRequest(t *testing.T) {
	srv := newReleaseServer(t, "v0.2.0")
	now := time.Now()
	app, _ := newApp(srv, &now)
	if app.RunScheduledCheck(context.Background()) || srv.hits != 0 {
		t.Fatal("update checks are off by default and must not touch the network")
	}
}

func TestScheduledCheck_RunsAtMostOncePerInterval(t *testing.T) {
	srv := newReleaseServer(t, "v0.2.0")
	now := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	app, ev := newApp(srv, &now)
	ctx := context.Background()
	_, _ = app.SaveAppSettings(ctx, model.AppSettings{CheckForUpdates: model.UpdateCheckStartup})

	if !app.RunScheduledCheck(ctx) || len(ev.names) != 1 {
		t.Fatalf("first run must check and announce the update (events %v)", ev.names)
	}
	now = now.Add(2 * time.Hour)
	if app.RunScheduledCheck(ctx) || srv.hits != 1 {
		t.Fatal("startup mode must not check twice within 24h")
	}
	now = now.Add(25 * time.Hour)
	if !app.RunScheduledCheck(ctx) || srv.hits != 2 {
		t.Fatal("startup mode must check again after 24h")
	}
}

func TestDismissUpdate_HidesOnlyThatVersion(t *testing.T) {
	srv := newReleaseServer(t, "v0.2.0")
	now := time.Now()
	app, ev := newApp(srv, &now)
	ctx := context.Background()
	st, _ := app.CheckForUpdate(ctx)
	if !st.UpdateAvailable || st.Dismissed {
		t.Fatalf("expected a fresh update: %+v", st)
	}
	_ = app.DismissUpdate(ctx, "0.2.0")
	st, _ = app.GetUpdateStatus(ctx)
	if !st.Dismissed || !st.UpdateAvailable {
		t.Fatalf("0.2.0 must be dismissed but still known: %+v", st)
	}
	_, _ = app.SaveAppSettings(ctx, model.AppSettings{CheckForUpdates: model.UpdateCheckWeekly})
	now = now.Add(8 * 24 * time.Hour)
	app.RunScheduledCheck(ctx)
	if len(ev.names) != 0 {
		t.Fatal("a dismissed version must not be announced again")
	}
	srv.tag = "v0.3.0"
	if st, _ = app.CheckForUpdate(ctx); st.Dismissed {
		t.Fatalf("a newer version must not inherit the dismissal: %+v", st)
	}
}

func TestCheckForUpdate_DevBuildNeverReportsUpdate(t *testing.T) {
	srv := newReleaseServer(t, "v9.0.0")
	now := time.Now()
	app, _ := newApp(srv, &now)
	app.Version = "dev"
	if st, _ := app.CheckForUpdate(context.Background()); st.UpdateAvailable {
		t.Fatalf("dev builds must not report updates: %+v", st)
	}
}
