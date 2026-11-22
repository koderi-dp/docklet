package compose

import (
	"os"
	"reflect"
	"testing"

	"docklet/internal/model"
)

func TestParseHashes_ReadsServiceAndHashPerLine(t *testing.T) {
	b, err := os.ReadFile("testdata/config_hash.txt")
	if err != nil {
		t.Fatal(err)
	}
	h := ParseHashes(string(b))
	const want = "1f7ebb0f84cb0d3583d100820b1a0564c85c234170ebda1d2125509d2c462ffa"
	if len(h) != 3 || h["db"] != want || h["web"] != want || h["worker"] != want {
		t.Errorf("got %v", h)
	}
	h = ParseHashes("\n  \nweb abc\nlonely\na b c\r\ndb 123\r\n")
	if !reflect.DeepEqual(h, map[string]string{"web": "abc", "db": "123"}) {
		t.Errorf("odd lines: %v", h)
	}
}

func ctr(svc, hash, image, state string, extra ...string) Container {
	l := map[string]string{LabelService: svc, LabelConfigHash: hash, LabelImage: image}
	for i := 0; i+1 < len(extra); i += 2 {
		l[extra[i]] = extra[i+1]
	}
	return Container{State: state, Labels: l}
}

func plan(in PlanInput) []model.ComposeApplyItem { return PlanApply(in) }

func one(t *testing.T, in PlanInput) model.ComposeApplyItem {
	t.Helper()
	items := plan(in)
	if len(items) != 1 {
		t.Fatalf("want one item, got %+v", items)
	}
	return items[0]
}

// wanting sets how many containers web should have.
func wanting(n int, in PlanInput) PlanInput {
	in.Replicas = map[string]int{"web": n}
	return in
}

func webInput(cs ...Container) PlanInput {
	return PlanInput{
		Hashes: map[string]string{"web": "h1"}, AllServices: []string{"web"},
		ImageIDs: map[string][]string{"web": {"img1"}}, Containers: cs,
	}
}

func TestPlanApply_PerServiceRules(t *testing.T) {
	cases := []struct {
		name   string
		in     PlanInput
		action string
		detail string
	}{
		{"no container", webInput(), "new", "not created yet"},
		{"other hash", webInput(ctr("web", "h0", "img1", "running")), "recreate", "configuration changed"},
		{"other hash and image", webInput(ctr("web", "h0", "img0", "running")), "recreate", "configuration changed"},
		{"image differs", webInput(ctr("web", "h1", "img0", "running")), "recreateImage", "image changed"},
		{"exited", webInput(ctr("web", "h1", "img1", "exited")), "start", "stopped"},
		{"created", webInput(ctr("web", "h1", "img1", "created")), "start", "stopped"},
		{"same and running", webInput(ctr("web", "h1", "img1", "running")), "unchanged", ""},
		{"image id unknown", func() PlanInput {
			in := webInput(ctr("web", "h1", "img0", "running"))
			in.ImageIDs = map[string][]string{}
			return in
		}(), "unchanged", ""},
		{"label equals the platform id", func() PlanInput {
			in := webInput(ctr("web", "h1", "platformImg", "running"))
			in.ImageIDs = map[string][]string{"web": {"indexImg", "platformImg"}}
			return in
		}(), "unchanged", ""},
		{"label equals none of the ids", func() PlanInput {
			in := webInput(ctr("web", "h1", "old", "running"))
			in.ImageIDs = map[string][]string{"web": {"indexImg", "platformImg"}}
			return in
		}(), "recreateImage", "image changed"},
		{"scaled up", func() PlanInput {
			in := webInput(ctr("web", "h1", "img1", "running"), ctr("web", "h1", "img1", "running"))
			in.Replicas = map[string]int{"web": 3}
			return in
		}(), "scale", "2 containers now, 3 wanted"},
		{"scaled down", webInput(ctr("web", "h1", "img1", "running"), ctr("web", "h1", "img1", "running")), "scale", "2 containers now, 1 wanted"},
		{"stopped and scaled", func() PlanInput {
			in := webInput(ctr("web", "h1", "img1", "exited"))
			in.Replicas = map[string]int{"web": 2}
			return in
		}(), "scale", "1 container now, 2 wanted"},
		{"recreate beats scale", func() PlanInput {
			in := webInput(ctr("web", "h0", "img1", "running"))
			in.Replicas = map[string]int{"web": 2}
			return in
		}(), "recreate", "configuration changed"},
		{"replicas as wanted", func() PlanInput {
			in := webInput(ctr("web", "h1", "img1", "running"), ctr("web", "h1", "img1", "running"))
			in.Replicas = map[string]int{"web": 2}
			return in
		}(), "unchanged", ""},
		{"image label empty", webInput(ctr("web", "h1", "", "running")), "unchanged", ""},
		{"replicas, one outdated", webInput(
			ctr("web", "h1", "img1", "running"), ctr("web", "h0", "img1", "running"), ctr("web", "h1", "img1", "exited")),
			"recreate", "configuration changed"},
		{"replicas, one stopped", wanting(2, webInput(ctr("web", "h1", "img1", "running"), ctr("web", "h1", "img1", "exited"))), "start", "stopped"},
		{"replicas, image beats stopped", webInput(ctr("web", "h1", "img0", "running"), ctr("web", "h1", "img1", "exited")), "recreateImage", "image changed"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := one(t, c.in)
			if got.Service != "web" || got.Action != c.action || got.Detail != c.detail {
				t.Errorf("got %+v", got)
			}
		})
	}
}

func TestParseReplicas(t *testing.T) {
	got := ParseReplicas(`{"services":{"a":{"deploy":{"replicas":3}},"b":{"scale":2},"c":{"scale":4,"deploy":{"replicas":1}},"d":{"image":"x"},"e":{"deploy":{}}}}`)
	if want := map[string]int{"a": 3, "b": 2, "c": 4}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v", got)
	}
	if got := ParseReplicas("not json"); got == nil || len(got) != 0 {
		t.Errorf("bad input: %#v", got)
	}
}

func TestPlanApply_FinishedOneShotRunsAgain(t *testing.T) {
	zero, nonzero := 0, 1
	exited := func(code *int, policy string) Container {
		c := ctr("web", "h1", "img1", "exited")
		c.ExitCode, c.RestartPolicy = code, policy
		return c
	}
	declared := func(in PlanInput) PlanInput {
		in.OneShots = map[string]bool{"web": true}
		return in
	}
	for name, c := range map[string]struct {
		in   PlanInput
		shot bool
		det  string
	}{
		"ordinary service stopped, policy no": {webInput(exited(&zero, "no")), false, "stopped"},
		"ordinary service stopped, no policy": {webInput(exited(&zero, "")), false, "stopped"},
		"declared, completed":                 {declared(webInput(exited(&zero, "no"))), true, "completed, runs again"},
		"declared, failed":                    {declared(webInput(exited(&nonzero, "no"))), false, "stopped"},
		"declared, stopped next to finished":  {declared(wanting(2, webInput(exited(&zero, "no"), exited(&nonzero, "no")))), false, "stopped"},
	} {
		got := one(t, c.in)
		if got.Action != "start" || got.OneShot != c.shot || got.Detail != c.det {
			t.Errorf("%s: %+v", name, got)
		}
	}
}

func TestPlanApply_OrphansProfilesAndOneOff(t *testing.T) {
	got := one(t, PlanInput{
		Hashes: map[string]string{}, AllServices: []string{"web"},
		Containers: []Container{ctr("old", "h", "i", "running")},
	})
	if got != (model.ComposeApplyItem{Service: "old", Action: "remove", Detail: "no longer in the file"}) {
		t.Errorf("orphan: %+v", got)
	}

	// A service of an unselected profile has a container but no hash: not listed, not an orphan.
	items := plan(PlanInput{
		Hashes: map[string]string{}, AllServices: []string{"debug"},
		Containers: []Container{ctr("debug", "h", "i", "running")},
	})
	if len(items) != 0 {
		t.Errorf("profile service listed: %+v", items)
	}

	// One-off containers (compose run) never count, for a service in the file or a removed one.
	in := webInput(ctr("web", "h0", "img0", "running", LabelOneOff, "True"), ctr("gone", "h", "i", "running", LabelOneOff, "True"))
	got = one(t, in)
	if got.Action != "new" {
		t.Errorf("one-off counted: %+v", got)
	}
}

func TestPlanApply_SortedAndNeverNil(t *testing.T) {
	if got := PlanApply(PlanInput{}); got == nil || len(got) != 0 {
		t.Errorf("empty input: %#v", got)
	}
	items := plan(PlanInput{
		Hashes:      map[string]string{"web": "h", "api": "h"},
		AllServices: []string{"web", "api"},
		Containers:  []Container{ctr("zed", "h", "i", "running"), ctr("beta", "h", "i", "running")},
	})
	var names []string
	for _, it := range items {
		names = append(names, it.Service)
	}
	if !reflect.DeepEqual(names, []string{"api", "beta", "web", "zed"}) {
		t.Errorf("order: %v", names)
	}
}
