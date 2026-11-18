package services

import (
	"context"
	"strings"
	"testing"

	"docklet/internal/model"
	"docklet/internal/ops"
)

func TestVolumeRemove_ReportsInUseWithoutForce(t *testing.T) {
	f, prov, em := newResFake(t)
	s := &VolumeService{Engines: prov, Events: em}
	vols, err := s.List(context.Background(), "c1")
	if err != nil || len(vols) != 2 {
		t.Fatalf("list: %v %v", vols, err)
	}
	if len(vols[0].InUseBy) != 1 || vols[0].InUseBy[0].Name != "web" || vols[0].InUseBy[0].Destination != "/var/lib/data" {
		t.Errorf("data in-use: %+v", vols[0].InUseBy)
	}
	if len(vols[1].InUseBy) != 0 || vols[1].Labels["a"] != "b" || vols[0].Labels == nil {
		t.Errorf("spare: %+v", vols[1])
	}
	res, err := s.Remove(context.Background(), "c1", []string{"data", "spare"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if res[0].OK || !strings.Contains(res[0].Error, "in use by web") {
		t.Errorf("in-use removal must fail friendly: %+v", res[0])
	}
	if !res[1].OK {
		t.Errorf("spare: %+v", res[1])
	}
	if f.called("force=1") {
		t.Error("force must not be sent by default")
	}
}

func TestVolumePrune_RequiresConfirmationBeforeCallingEngine(t *testing.T) {
	f, prov, em := newResFake(t)
	m := ops.New(nil)
	s := &VolumeService{Engines: prov, Events: em, Ops: m}
	if _, err := s.PruneUnused(context.Background(), "c1", false); err == nil {
		t.Fatal("prune without confirmation must be refused")
	}
	if f.called("/volumes/prune") {
		t.Fatal("engine was called without confirmation")
	}
	op, err := s.PruneUnused(context.Background(), "c1", true)
	if err != nil {
		t.Fatal(err)
	}
	done := waitOp(t, m, op.ID)
	if done.Status != model.OpSucceeded || !strings.Contains(done.Detail, "2.0 KB") {
		t.Errorf("op: %+v", done)
	}
	if logs, _ := m.Logs(op.ID); !strings.Contains(strings.Join(logs, "\n"), "spare") {
		t.Errorf("logs should name removed volume: %v", logs)
	}
}

func TestValidateVolume_RejectsOneCharacterName(t *testing.T) {
	if _, err := validateVolume("x", "", nil); err == nil {
		t.Error("1-char volume name must be rejected")
	}
}
