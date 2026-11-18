package tray

import (
	"fmt"
	"testing"

	"docklet/internal/model"
)

func find(items []Item, label string) (Item, bool) {
	for _, it := range items {
		if it.Label == label {
			return it, true
		}
	}
	return Item{}, false
}

func TestBuildMenu_NoEngineShowsOnlyBasics(t *testing.T) {
	m := BuildMenu(State{})
	if m[0].Label != "Show Docklet" || m[len(m)-1].Label != "Quit Docklet" {
		t.Fatalf("show/quit must frame the menu: %+v", m)
	}
	if m[1].Label != "No engine selected" || m[1].Enabled {
		t.Errorf("header must be a disabled note: %+v", m[1])
	}
	if it, ok := find(m, "Engines: none saved"); !ok || it.Enabled {
		t.Error("expected a disabled 'none saved' engines note")
	}
	if _, ok := find(m, "Alerts: 0 unread"); ok {
		t.Error("alerts item must be absent when alerts are off")
	}
}

func TestBuildMenu_DisconnectedEngineListsNoContainers(t *testing.T) {
	m := BuildMenu(State{
		ActiveID:   "a",
		Engines:    []Engine{{"a", "Local", model.StateDisconnected}, {"b", "Remote", model.StateConnected}},
		Containers: []Container{{"1", "web"}},
	})
	if m[1].Label != "Local: not connected" {
		t.Errorf("header: %q", m[1].Label)
	}
	if _, ok := find(m, "Running containers: engine not connected"); !ok {
		t.Error("containers must not be offered for a disconnected engine")
	}
	engines, _ := find(m, "Engines")
	if engines.Children[0].Label != "○ Local" || engines.Children[1].Label != "● Remote" {
		t.Errorf("state marks wrong: %+v", engines.Children)
	}
	if a := engines.Children[1].Action; a.Kind != ActSelectConnection || a.ConnectionID != "b" {
		t.Errorf("engine click must select it: %+v", a)
	}
}

func TestBuildMenu_ManyContainersAreSortedAndCappedAtFifteen(t *testing.T) {
	var cs []Container
	for i := 20; i > 0; i-- {
		cs = append(cs, Container{fmt.Sprint("id", i), fmt.Sprintf("c%02d", i)})
	}
	m := BuildMenu(State{ActiveID: "a", Engines: []Engine{{"a", "Local", model.StateConnected}}, Containers: cs})
	menu, ok := find(m, "Running containers (20)")
	if !ok || len(menu.Children) != MaxContainers+1 {
		t.Fatalf("want 15 containers + overflow note, got %d (%v)", len(menu.Children), ok)
	}
	first := menu.Children[0]
	if first.Label != "c01" || len(first.Children) != 3 {
		t.Fatalf("first container: %+v", first)
	}
	stop := first.Children[0]
	if stop.Action.Kind != ActStop || stop.Action.ContainerID != "id1" || stop.Action.ConnectionID != "a" {
		t.Errorf("stop action: %+v", stop.Action)
	}
	if last := menu.Children[MaxContainers]; last.Label != "5 more in Docklet" || last.Enabled {
		t.Errorf("overflow note: %+v", last)
	}
}

func TestBuildMenu_AlertsItemAppearsOnlyWhenEnabled(t *testing.T) {
	m := BuildMenu(State{AlertsEnabled: true, Unread: 3})
	it, ok := find(m, "Alerts: 3 unread")
	if !ok || it.Action.Kind != ActOpenAlerts {
		t.Fatalf("alerts item missing: %+v", m)
	}
}
