package services

import (
	"context"
	"testing"

	"docklet/internal/model"
	"docklet/internal/ops"
)

func TestSystemPrune_NeverPrunesVolumesWithoutExplicitConfirmation(t *testing.T) {
	f, prov, em := newResFake(t)
	m := ops.New(nil)
	s := &SystemService{Engines: prov, Events: em, Ops: m}
	ctx := context.Background()
	if _, err := s.SystemPrune(ctx, "c1", false, true, false); err == nil {
		t.Fatal("volumes without confirmation must be refused")
	}
	if _, err := s.PruneVolumes(ctx, "c1", false); err == nil {
		t.Fatal("PruneVolumes without confirmation must be refused")
	}
	op, err := s.SystemPrune(ctx, "c1", false, false, false)
	if err != nil {
		t.Fatal(err)
	}
	waitOp(t, m, op.ID)
	if f.called("/volumes/prune") {
		t.Fatal("default system prune must not touch volumes")
	}
	if !f.called("/images/prune?filters=") || !f.called("dangling") {
		t.Errorf("images must default to dangling only: %v", f.calls)
	}
	op, err = s.SystemPrune(ctx, "c1", true, true, true)
	if err != nil {
		t.Fatal(err)
	}
	waitOp(t, m, op.ID)
	if !f.called("/volumes/prune") {
		t.Fatal("volumes explicitly requested but not pruned")
	}
}

func TestDiskUsage_LeavesUnavailableFiguresNil(t *testing.T) {
	_, prov, em := newResFake(t)
	s := &SystemService{Engines: prov, Events: em}
	du, err := s.DiskUsage(context.Background(), "c1")
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]model.DiskCategory{}
	for _, c := range du.Categories {
		by[c.Key] = c
	}
	if by["images"].Size == nil || *by["images"].Size != 1000 || *by["images"].Reclaimable != 500 || by["images"].ActiveCount != 1 {
		t.Errorf("images: %+v", by["images"])
	}
	if by["volumes"].Size != nil || by["volumes"].Reclaimable != nil {
		t.Errorf("unavailable volume figures must be nil: %+v", by["volumes"])
	}
}

func TestRecentEvents_NewestFirstWithAttributeSubset(t *testing.T) {
	_, prov, em := newResFake(t)
	s := &SystemService{Engines: prov, Events: em}
	ev, err := s.RecentEvents(context.Background(), "c1", 3600, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(ev) != 2 || ev[0].Action != "die" || ev[0].ActorName != "web" || ev[0].Attributes["image"] != "nginx" {
		t.Errorf("events: %+v", ev)
	}
	if _, leaked := ev[0].Attributes["secret"]; leaked {
		t.Error("only a subset of attributes may be returned")
	}
}
