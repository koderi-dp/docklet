package compose

import (
	"sort"
	"strings"

	"docklet/internal/model"
)

// ParseHashes reads `docker compose config --hash "*"` output: one "service hash" pair
// per line. Blank lines and lines without exactly two fields are skipped.
func ParseHashes(out string) map[string]string {
	h := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) == 2 {
			h[f[0]] = f[1]
		}
	}
	return h
}

// PlanInput is what PlanApply compares.
type PlanInput struct {
	// Hashes are the config hashes of the services that `up` would handle (selected profiles).
	Hashes map[string]string
	// AllServices are all services of the file, whatever their profile.
	AllServices []string
	Containers  []Container
	// ImageIDs are the Engine image ids of the services' images; a missing entry means unknown.
	ImageIDs map[string]string
}

// Strongest first: a service with several containers gets the strongest action.
var applyRank = map[string]int{"recreate": 3, "recreateImage": 2, "start": 1, "unchanged": 0}

var applyDetail = map[string]string{
	"recreate":      "configuration changed",
	"recreateImage": "image changed",
	"start":         "stopped",
	"unchanged":     "",
}

// PlanApply works out what `docker compose up -d` would change, from the config hashes,
// the containers' labels and the images' ids. One-off containers are ignored.
func PlanApply(in PlanInput) []model.ComposeApplyItem {
	all := map[string]bool{}
	for _, s := range in.AllServices {
		all[s] = true
	}
	actions := map[string]string{}
	orphans := map[string]bool{}
	for _, c := range in.Containers {
		if isOneOff(c.Labels) {
			continue
		}
		svc := c.Labels[LabelService]
		want, selected := in.Hashes[svc]
		switch {
		case !all[svc] && !selected:
			orphans[svc] = true
		case !selected:
			// The service is in the file but behind a profile that is not selected.
		default:
			a := containerAction(c, want, in.ImageIDs[svc])
			if cur, ok := actions[svc]; !ok || applyRank[a] > applyRank[cur] {
				actions[svc] = a
			}
		}
	}
	items := []model.ComposeApplyItem{}
	for svc := range in.Hashes {
		if a, ok := actions[svc]; ok {
			items = append(items, model.ComposeApplyItem{Service: svc, Action: a, Detail: applyDetail[a]})
		} else {
			items = append(items, model.ComposeApplyItem{Service: svc, Action: "new", Detail: "not created yet"})
		}
	}
	for svc := range orphans {
		items = append(items, model.ComposeApplyItem{Service: svc, Action: "remove", Detail: "no longer in the file"})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Service < items[j].Service })
	return items
}

func containerAction(c Container, wantHash, imageID string) string {
	switch {
	case c.Labels[LabelConfigHash] != wantHash:
		return "recreate"
	case imageID != "" && c.Labels[LabelImage] != "" && c.Labels[LabelImage] != imageID:
		return "recreateImage"
	case c.State == "exited" || c.State == "created":
		return "start"
	}
	return "unchanged"
}
