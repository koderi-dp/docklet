package appdb

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"docklet/internal/model"
)

func openAlertsDB(t *testing.T) (*DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "alerts.db")
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d, path
}

func alert(id string, at int64) model.Alert {
	return model.Alert{ID: id, ConnectionID: "c1", ConnectionName: "Local", ContainerID: "cid-" + id, ContainerName: "web",
		ComposeProject: "shop", Rule: model.RuleCrash, Severity: model.SeverityError, Title: "Container crashed", Detail: "exit code 1", At: at}
}

func alertIDs(list []model.Alert) []string {
	var out []string
	for _, a := range list {
		out = append(out, a.ID)
	}
	return out
}

func TestAlerts_RoundTripKeepsEveryFieldAndListsNewestFirst(t *testing.T) {
	d, _ := openAlertsDB(t)
	ctx := context.Background()
	for _, a := range []model.Alert{alert("old", 100), alert("new", 300), alert("mid", 200)} {
		if err := d.AddAlert(ctx, a); err != nil {
			t.Fatal(err)
		}
	}
	list, err := d.ListAlerts(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := alertIDs(list); fmt.Sprint(got) != "[new mid old]" {
		t.Fatalf("order must follow At, not insertion: %v", got)
	}
	if list[0] != alert("new", 300) {
		t.Errorf("fields lost in the round trip: %+v", list[0])
	}
}

func TestAlerts_ListLimit(t *testing.T) {
	d, _ := openAlertsDB(t)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		if err := d.AddAlert(ctx, alert(fmt.Sprint("a", i), int64(i))); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		limit int
		want  int
	}{{2, 2}, {5, 5}, {100, 5}, {0, 5}, {-3, 5}} {
		list, err := d.ListAlerts(ctx, tc.limit)
		if err != nil || len(list) != tc.want {
			t.Errorf("limit %d: got %d alerts (err %v), want %d", tc.limit, len(list), err, tc.want)
		}
	}
	if list, _ := d.ListAlerts(ctx, 2); alertIDs(list)[0] != "a4" {
		t.Errorf("a limit keeps the newest: %v", alertIDs(list))
	}
}

func TestAlerts_ListEmptyIsNotAnError(t *testing.T) {
	d, _ := openAlertsDB(t)
	list, err := d.ListAlerts(context.Background(), 10)
	if err != nil || len(list) != 0 {
		t.Fatalf("empty history: %v %v", list, err)
	}
}

func TestAlerts_HistoryIsCappedAndDropsTheOldest(t *testing.T) {
	d, _ := openAlertsDB(t)
	ctx := context.Background()
	for i := 0; i < alertsKeep+15; i++ {
		if err := d.AddAlert(ctx, alert(fmt.Sprintf("a%03d", i), int64(1000+i))); err != nil {
			t.Fatal(err)
		}
	}
	list, err := d.ListAlerts(ctx, 0)
	if err != nil || len(list) != alertsKeep {
		t.Fatalf("len=%d err=%v", len(list), err)
	}
	if list[0].ID != "a214" || list[len(list)-1].ID != "a015" {
		t.Errorf("kept %s .. %s, want a214 .. a015", list[0].ID, list[len(list)-1].ID)
	}
}

func TestAlerts_SameIDReplacesInsteadOfDuplicating(t *testing.T) {
	d, _ := openAlertsDB(t)
	ctx := context.Background()
	if err := d.AddAlert(ctx, alert("x", 1)); err != nil {
		t.Fatal(err)
	}
	changed := alert("x", 1)
	changed.Detail = "exit code 2"
	if err := d.AddAlert(ctx, changed); err != nil {
		t.Fatal(err)
	}
	list, _ := d.ListAlerts(ctx, 0)
	if len(list) != 1 || list[0].Detail != "exit code 2" {
		t.Fatalf("got %+v", list)
	}
}

func TestAlerts_MarkReadTouchesOnlyTheGivenIDs(t *testing.T) {
	d, _ := openAlertsDB(t)
	ctx := context.Background()
	for i, id := range []string{"a", "b", "c"} {
		if err := d.AddAlert(ctx, alert(id, int64(i))); err != nil {
			t.Fatal(err)
		}
	}
	// unknown ids and an empty list are harmless
	if err := d.MarkAlertsRead(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if err := d.MarkAlertsRead(ctx, []string{"missing", "b"}); err != nil {
		t.Fatalf("unknown id must not fail the call: %v", err)
	}
	list, _ := d.ListAlerts(ctx, 0)
	read := map[string]bool{}
	for _, a := range list {
		read[a.ID] = a.Read
	}
	if read["a"] || !read["b"] || read["c"] {
		t.Fatalf("only b should be read: %v", read)
	}
	if err := d.MarkAlertsRead(ctx, []string{"b"}); err != nil {
		t.Fatalf("marking twice: %v", err)
	}
}

func TestAlerts_MarkAllReadClearAndPersistence(t *testing.T) {
	d, path := openAlertsDB(t)
	ctx := context.Background()
	for i, id := range []string{"a", "b"} {
		if err := d.AddAlert(ctx, alert(id, int64(i))); err != nil {
			t.Fatal(err)
		}
	}
	if err := d.MarkAllAlertsRead(ctx); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	d2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d2.Close()
	list, _ := d2.ListAlerts(ctx, 0)
	if len(list) != 2 || !list[0].Read || !list[1].Read {
		t.Fatalf("alerts and read flags must survive a reopen: %+v", list)
	}
	if err := d2.ClearAlerts(ctx); err != nil {
		t.Fatal(err)
	}
	if list, _ := d2.ListAlerts(ctx, 0); len(list) != 0 {
		t.Fatalf("clear left %d", len(list))
	}
	if err := d2.ClearAlerts(ctx); err != nil {
		t.Fatalf("clearing an empty history: %v", err)
	}
	if err := d2.MarkAllAlertsRead(ctx); err != nil {
		t.Fatalf("mark all on empty: %v", err)
	}
}
