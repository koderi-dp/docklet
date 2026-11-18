package appdb

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"docklet/internal/model"
)

func TestBuilds_InsertCapListDelete(t *testing.T) {
	d, err := Open(filepath.Join(t.TempDir(), "x.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	for i := 0; i < MaxBuildsPerConnection+7; i++ {
		r := model.BuildRecord{ID: fmt.Sprintf("a%03d", i), ConnectionID: "c1", StartedAt: int64(1000 + i), Result: model.BuildSuccess,
			Tags: []string{"app:1"}, ArgNames: []string{"API_TOKEN"}, LogTail: []string{"line"}}
		if err := d.InsertBuild(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	if err := d.InsertBuild(ctx, model.BuildRecord{ID: "other", ConnectionID: "c2", StartedAt: 1}); err != nil {
		t.Fatal(err)
	}
	list, err := d.ListBuilds(ctx, "c1")
	if err != nil || len(list) != MaxBuildsPerConnection {
		t.Fatalf("len=%d err=%v", len(list), err)
	}
	if list[0].ID != "a056" || list[len(list)-1].ID != "a007" {
		t.Errorf("newest first, oldest dropped: %s .. %s", list[0].ID, list[len(list)-1].ID)
	}
	if list[0].ArgNames[0] != "API_TOKEN" || list[0].LogTail[0] != "line" {
		t.Errorf("fields lost: %+v", list[0])
	}
	if other, _ := d.ListBuilds(ctx, "c2"); len(other) != 1 {
		t.Errorf("the cap is per connection: %d", len(other))
	}
	if err := d.DeleteBuild(ctx, "c1", "a056"); err != nil {
		t.Fatal(err)
	}
	if list, _ = d.ListBuilds(ctx, "c1"); len(list) != MaxBuildsPerConnection-1 || list[0].ID != "a055" {
		t.Errorf("delete: %d", len(list))
	}
	if err := d.ClearBuilds(ctx, "c1"); err != nil {
		t.Fatal(err)
	}
	if list, _ = d.ListBuilds(ctx, "c1"); len(list) != 0 {
		t.Errorf("clear: %d", len(list))
	}
	if other, _ := d.ListBuilds(ctx, "c2"); len(other) != 1 {
		t.Error("clear must only affect its connection")
	}
}
