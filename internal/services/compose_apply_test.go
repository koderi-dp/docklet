package services

import (
	"errors"
	"slices"
	"testing"

	"docklet/internal/compose"
)

const applyConfigJSON = `{"services":{"web":{"image":"nginx"},"db":{"image":"postgres"},"debug":{"image":"busybox","profiles":["debug"]}}}`

func scriptApply(r *fakeRunner, hashOut string, hashErr error) {
	r.out = func(c compose.Command) (string, error) {
		switch {
		case slices.Contains(c.Args, "--short"):
			return "5.7.1\n", nil
		case slices.Contains(c.Args, "--hash"):
			return hashOut, hashErr
		case slices.Contains(c.Args, "config"):
			return applyConfigJSON, nil
		}
		return "", nil
	}
}

func TestApplyPlan_EveryServiceIsNewWithoutContainers(t *testing.T) {
	s, r, _ := newVerbSvc(t)
	scriptApply(r, "db h1\nweb h2\n", nil)
	plan, err := s.ApplyPlan(bg, "c1", "shop", []string{"p1"})
	if err != nil || plan.Error != "" || len(plan.Items) != 2 {
		t.Fatalf("%+v %v", plan, err)
	}
	for i, svc := range []string{"db", "web"} {
		if it := plan.Items[i]; it.Service != svc || it.Action != "new" {
			t.Errorf("item %d: %+v", i, it)
		}
	}
	// The hash run carries the chosen profiles; the structure run asks for all of them.
	hash := composeArgs(t, r, "--hash")
	if i := slices.Index(hash, "--profile"); i < 0 || hash[i+1] != "p1" {
		t.Errorf("hash argv: %v", hash)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	found := false
	for _, c := range r.cmds {
		if i := slices.Index(c.Args, "--profile"); i >= 0 && c.Args[i+1] == "*" && slices.Contains(c.Args, "json") {
			found = true
		}
	}
	if !found {
		t.Errorf("no json run with all profiles: %+v", r.cmds)
	}
}

func TestApplyPlan_FailingConfigIsAnErrorFieldNotAGoError(t *testing.T) {
	s, r, _ := newVerbSvc(t)
	scriptApply(r, "", errors.New("service \"web\" refers to undefined volume\nsecond line"))
	plan, err := s.ApplyPlan(bg, "c1", "shop", nil)
	if err != nil {
		t.Fatalf("Go error: %v", err)
	}
	if plan.Error != `service "web" refers to undefined volume` || len(plan.Items) != 0 {
		t.Errorf("%+v", plan)
	}
}

func TestApplyPlan_ProjectThatCannotRunReportsWhy(t *testing.T) {
	s, r, _ := newVerbSvc(t)
	r.out = func(c compose.Command) (string, error) { return "", errors.New("docker: not found") }
	capab, err := s.Capability(bg, "c1", "shop")
	if err != nil || capab.CanRun || capab.Reason == "" {
		t.Fatalf("setup: %+v %v", capab, err)
	}
	plan, err := s.ApplyPlan(bg, "c1", "shop", nil)
	if err != nil || plan.Error != capab.Reason || len(plan.Items) != 0 {
		t.Errorf("%+v %v", plan, err)
	}
}
