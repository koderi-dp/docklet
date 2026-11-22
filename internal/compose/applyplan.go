package compose

import (
	"encoding/json"
	"fmt"
	"slices"
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
	// ImageIDs are the Engine image ids that count as a service's current image (the plain
	// id and the platform-specific one); a missing or empty entry means unknown.
	ImageIDs map[string][]string
	// Replicas is the wanted container count per service; a service not in the map wants 1.
	Replicas map[string]int
	// OneShots are the services another service waits on with service_completed_successfully.
	OneShots map[string]bool
}

// ParseReplicas reads `docker compose config --format json` output for the container count
// each service states: services.<name>.scale wins over services.<name>.deploy.replicas.
// Only services that state one are in the map. Unreadable input gives an empty map.
func ParseReplicas(jsonText string) map[string]int {
	out := map[string]int{}
	var doc struct {
		Services map[string]struct {
			Scale  *int `json:"scale"`
			Deploy *struct {
				Replicas *int `json:"replicas"`
			} `json:"deploy"`
		} `json:"services"`
	}
	if json.Unmarshal([]byte(jsonText), &doc) != nil {
		return out
	}
	for name, sv := range doc.Services {
		switch {
		case sv.Scale != nil:
			out[name] = *sv.Scale
		case sv.Deploy != nil && sv.Deploy.Replicas != nil:
			out[name] = *sv.Deploy.Replicas
		}
	}
	return out
}

// Strongest first: a service with several containers gets the strongest action.
var applyRank = map[string]int{"recreate": 4, "recreateImage": 3, "scale": 2, "start": 1, "unchanged": 0}

var applyDetail = map[string]string{
	"recreate":      "configuration changed",
	"recreateImage": "image changed",
	"start":         "stopped",
	"unchanged":     "",
}

const oneShotDetail = "completed, runs again"

// PlanApply works out what `docker compose up -d` would change, from the config hashes,
// the containers' labels and the images' ids. One-off containers are ignored.
func PlanApply(in PlanInput) []model.ComposeApplyItem {
	all := map[string]bool{}
	for _, s := range in.AllServices {
		all[s] = true
	}
	actions := map[string]string{}
	oneShot := map[string]bool{}
	counts := map[string]int{}
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
			counts[svc]++
			a := containerAction(c, want, in.ImageIDs[svc])
			shot := a == "start" && in.OneShots[svc] && finishedOK(c)
			cur, ok := actions[svc]
			switch {
			case !ok || applyRank[a] > applyRank[cur]:
				actions[svc], oneShot[svc] = a, shot
			case a == "start" && cur == "start" && !shot:
				oneShot[svc] = false // a plainly stopped container among the finished ones
			}
		}
	}
	wanted := func(svc string) int {
		if n, ok := in.Replicas[svc]; ok {
			return n
		}
		return 1
	}
	items := []model.ComposeApplyItem{}
	for svc := range in.Hashes {
		if a, ok := actions[svc]; ok {
			it := model.ComposeApplyItem{Service: svc, Action: a, Detail: applyDetail[a]}
			switch {
			case (a == "unchanged" || a == "start") && counts[svc] != wanted(svc):
				now := fmt.Sprintf("%d containers", counts[svc])
				if counts[svc] == 1 {
					now = "1 container"
				}
				it.Action, it.Detail = "scale", fmt.Sprintf("%s now, %d wanted", now, wanted(svc))
			case oneShot[svc]:
				it.Detail, it.OneShot = oneShotDetail, true
			}
			items = append(items, it)
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

// finishedOK is a container that exited with 0. It is a finished one-shot only when the file
// declares its service one (PlanInput.OneShots); `up` runs it again.
func finishedOK(c Container) bool {
	return c.State == "exited" && c.ExitCode != nil && *c.ExitCode == 0
}

func containerAction(c Container, wantHash string, imageIDs []string) string {
	label := c.Labels[LabelImage]
	switch {
	case c.Labels[LabelConfigHash] != wantHash:
		return "recreate"
	case label != "" && len(imageIDs) > 0 && !slices.Contains(imageIDs, label):
		return "recreateImage"
	case c.State == "exited" || c.State == "created":
		return "start"
	}
	return "unchanged"
}
