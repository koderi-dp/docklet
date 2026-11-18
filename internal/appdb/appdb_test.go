package appdb

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"docklet/internal/model"
)

func TestOpen_MigratesFreshDatabaseAndIsIdempotent(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sub", "x.db")
	d, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := d.SetSetting(ctx, "theme", "dark"); err != nil {
		t.Fatal(err)
	}
	d.Close()
	d, err = Open(p) // idempotent re-run
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	v, ok, err := d.GetSetting(ctx, "theme")
	if err != nil || !ok || v != "dark" {
		t.Fatalf("setting = %q %v %v", v, ok, err)
	}
	var n int
	if err := d.SQL().QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&n); err != nil || n != len(migrations) {
		t.Fatalf("migrations recorded = %d %v", n, err)
	}
}

func TestOpen_RefusesDatabaseFromNewerVersion(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.db")
	d, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.SQL().Exec(`INSERT INTO schema_migrations (id, applied_at) VALUES (999, 0)`); err != nil {
		t.Fatal(err)
	}
	d.Close()
	if _, err := Open(p); err == nil || !strings.Contains(err.Error(), "newer") {
		t.Fatalf("want newer-schema refusal, got %v", err)
	}
}

func TestMigrate_RollsBackFailedMigration(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.db")
	d, _ := Open(p)
	defer d.Close()
	err := migrate(context.Background(), d.SQL(), []migration{{1, ""}, {2, "CREATE TABLE a(x); CREATE TABLE a(x);"}})
	if err == nil {
		t.Fatal("expected failure")
	}
	if _, err := d.SQL().Exec(`SELECT * FROM a`); err == nil {
		t.Fatal("table a must have been rolled back")
	}
}

func TestSaveComposeProject_UpsertsPerConnectionAndName(t *testing.T) {
	d, _ := Open(filepath.Join(t.TempDir(), "x.db"))
	defer d.Close()
	ctx := context.Background()
	a, err := d.SaveComposeProject(ctx, ComposeProjectRecord{ConnectionID: "c1", Name: "web", ConfigFiles: []string{"a.yml"}})
	if err != nil || a.ID == "" {
		t.Fatal(a, err)
	}
	b, _ := d.SaveComposeProject(ctx, ComposeProjectRecord{ConnectionID: "c1", Name: "web", WorkingDir: "/w"})
	if b.ID != a.ID {
		t.Fatal("upsert must keep ID")
	}
	d.SaveComposeProject(ctx, ComposeProjectRecord{ConnectionID: "c2", Name: "web"})
	l, _ := d.ListComposeProjects(ctx, "c1")
	if len(l) != 1 || l[0].WorkingDir != "/w" {
		t.Fatalf("%+v", l)
	}
	d.DeleteComposeProject(ctx, "c1", "web")
	if l, _ := d.ListComposeProjects(ctx, "c1"); len(l) != 0 {
		t.Fatal("not deleted")
	}
}

func TestListOperationHistory_NewestFirstAndLimited(t *testing.T) {
	d, _ := Open(filepath.Join(t.TempDir(), "x.db"))
	defer d.Close()
	ctx := context.Background()
	for i, id := range []string{"a", "b", "c"} {
		d.RecordOperation(ctx, model.Operation{ID: id, Kind: "k", Title: "t", Status: model.OpSucceeded, StartedAt: int64(i + 1), FinishedAt: int64(i + 2)})
	}
	l, _ := d.ListOperationHistory(ctx, 2)
	if len(l) != 2 || l[0].ID != "c" {
		t.Fatalf("%+v", l)
	}
}

func TestReplaceConnections_RoundTripsEveryField(t *testing.T) {
	d, _ := Open(filepath.Join(t.TempDir(), "x.db"))
	defer d.Close()
	ctx := context.Background()
	in := []model.ConnectionProfile{{ID: "1", Name: "n", Type: model.ConnSSH, Endpoint: "ssh://h", SSHUser: "u", SSHPort: 2222, SSHUseAgent: true}}
	if err := d.ReplaceConnections(ctx, in); err != nil {
		t.Fatal(err)
	}
	out, _ := d.LoadConnections(ctx)
	if len(out) != 1 || out[0] != in[0] {
		t.Fatalf("%+v", out)
	}
}
