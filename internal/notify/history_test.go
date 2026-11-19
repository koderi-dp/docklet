package notify

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"docklet/internal/model"
)

func hubWithAlerts(store AlertStore, ids ...string) *Hub {
	h := &Hub{d: Deps{Store: store}}
	for _, id := range ids { // oldest first, so the last id ends up on top
		h.alerts = append([]model.Alert{{ID: id}}, h.alerts...)
	}
	return h
}

func readFlags(h *Hub) string {
	var s []string
	for _, a := range h.List(0) {
		s = append(s, fmt.Sprintf("%s:%v", a.ID, a.Read))
	}
	return fmt.Sprint(s)
}

func TestHistory_ListLimitAndIsolation(t *testing.T) {
	h := hubWithAlerts(nil, "a", "b", "c") // list order: c b a
	for _, tc := range []struct {
		limit int
		want  string
	}{{0, "[c b a]"}, {-1, "[c b a]"}, {2, "[c b]"}, {3, "[c b a]"}, {99, "[c b a]"}} {
		var ids []string
		for _, a := range h.List(tc.limit) {
			ids = append(ids, a.ID)
		}
		if fmt.Sprint(ids) != tc.want {
			t.Errorf("limit %d: %v, want %s", tc.limit, ids, tc.want)
		}
	}
	got := h.List(0)
	got[0].Read = true
	got[0].ID = "mutated"
	if h.List(0)[0].ID != "c" || h.List(0)[0].Read {
		t.Fatal("List must return a copy; callers must not be able to edit the hub's history")
	}
	if got := (&Hub{}).List(5); len(got) != 0 {
		t.Fatalf("empty history: %v", got)
	}
}

func TestHistory_MarkReadOnlyAffectsGivenAlertsAndIsPersisted(t *testing.T) {
	store := &memAlertStore{}
	h := hubWithAlerts(store, "a", "b", "c")
	if err := h.MarkRead(context.Background(), []string{"b", "ghost"}); err != nil {
		t.Fatalf("an unknown id must not fail: %v", err)
	}
	if got := readFlags(h); got != "[c:false b:true a:false]" {
		t.Fatalf("got %s", got)
	}
	if len(store.markIDs) != 1 || fmt.Sprint(store.markIDs[0]) != "[b ghost]" {
		t.Fatalf("store got %v", store.markIDs)
	}
	if err := h.MarkRead(context.Background(), nil); err != nil || readFlags(h) != "[c:false b:true a:false]" {
		t.Fatalf("empty id list changed something: %s %v", readFlags(h), err)
	}
}

func TestHistory_MarkAllReadAndClear(t *testing.T) {
	store := &memAlertStore{}
	h := hubWithAlerts(store, "a", "b")
	if err := h.MarkAllRead(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := readFlags(h); got != "[b:true a:true]" || store.markAll != 1 {
		t.Fatalf("got %s, store calls %d", got, store.markAll)
	}
	if err := h.Clear(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(h.List(0)) != 0 || store.clears != 1 {
		t.Fatalf("clear: %d left, store calls %d", len(h.List(0)), store.clears)
	}
	h.add(model.Alert{ID: "after"})
	if len(h.List(0)) != 1 {
		t.Fatal("the hub must keep working after a clear")
	}
}

func TestHistory_WorksWithoutAStore(t *testing.T) {
	h := hubWithAlerts(nil, "a")
	ctx := context.Background()
	if err := h.MarkRead(ctx, []string{"a"}); err != nil {
		t.Fatal(err)
	}
	if err := h.MarkAllRead(ctx); err != nil {
		t.Fatal(err)
	}
	if err := h.Clear(ctx); err != nil || len(h.List(0)) != 0 {
		t.Fatalf("clear: %v", err)
	}
}

func TestHistory_StoreErrorsSurfaceToTheCaller(t *testing.T) {
	boom := errors.New("db locked")
	h := hubWithAlerts(&memAlertStore{markErr: boom}, "a")
	ctx := context.Background()
	if err := h.MarkRead(ctx, []string{"a"}); !errors.Is(err, boom) {
		t.Errorf("MarkRead: %v", err)
	}
	if err := h.MarkAllRead(ctx); !errors.Is(err, boom) {
		t.Errorf("MarkAllRead: %v", err)
	}
	if err := h.Clear(ctx); !errors.Is(err, boom) {
		t.Errorf("Clear: %v", err)
	}
}
