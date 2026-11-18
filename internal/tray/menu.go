// Package tray builds the system tray menu. This file is the pure part: a small state struct goes in,
// a menu model comes out. Nothing here imports Wails; controller.go wires the model to the real tray.
package tray

import (
	"fmt"
	"sort"
	"strings"

	"docklet/internal/model"
)

// MaxContainers is how many running containers the tray menu lists.
const MaxContainers = 15

// ActionKind says what a clicked menu item does.
type ActionKind string

const (
	ActShow             ActionKind = "show"
	ActSelectConnection ActionKind = "selectConnection"
	ActStop             ActionKind = "stop"
	ActRestart          ActionKind = "restart"
	ActOpenContainer    ActionKind = "openContainer"
	ActOpenAlerts       ActionKind = "openAlerts"
	ActQuit             ActionKind = "quit"
)

// Action is what happens when an item is clicked; the zero value means nothing.
type Action struct {
	Kind         ActionKind
	ConnectionID string
	ContainerID  string
	Name         string // container name, for messages
}

// Item is one row of the menu model. Children makes it a submenu.
type Item struct {
	Label     string
	Enabled   bool
	Separator bool
	Children  []Item
	Action    Action
}

// Engine is one saved connection and its state.
type Engine struct {
	ID    string
	Name  string
	State model.ConnectionState
}

// Container is a running container of the active engine.
type Container struct{ ID, Name string }

// State is everything the menu depends on.
type State struct {
	Engines       []Engine
	ActiveID      string
	Containers    []Container // running containers of the active engine
	AlertsEnabled bool
	Unread        int
}

func (s State) active() (Engine, bool) {
	for _, e := range s.Engines {
		if e.ID == s.ActiveID {
			return e, true
		}
	}
	return Engine{}, false
}

func leaf(label string, a Action) Item { return Item{Label: label, Enabled: true, Action: a} }
func note(label string) Item           { return Item{Label: label} }
func sep() Item                        { return Item{Separator: true} }

// BuildMenu turns the state into the tray menu model.
func BuildMenu(s State) []Item {
	items := []Item{leaf("Show Docklet", Action{Kind: ActShow}), note(header(s)), sep(), enginesMenu(s)}
	items = append(items, containersMenu(s))
	if s.AlertsEnabled {
		items = append(items, leaf(fmt.Sprintf("Alerts: %d unread", s.Unread), Action{Kind: ActOpenAlerts}))
	}
	return append(items, sep(), leaf("Quit Docklet", Action{Kind: ActQuit}))
}

func stateText(st model.ConnectionState) string {
	switch st {
	case model.StateConnected:
		return "connected"
	case model.StateConnecting:
		return "connecting"
	case model.StateError:
		return "error"
	}
	return "not connected"
}

func header(s State) string {
	e, ok := s.active()
	if !ok {
		return "No engine selected"
	}
	return e.Name + ": " + stateText(e.State)
}

func enginesMenu(s State) Item {
	if len(s.Engines) == 0 {
		return note("Engines: none saved")
	}
	sub := make([]Item, 0, len(s.Engines))
	for _, e := range s.Engines {
		mark := "○ "
		switch {
		case e.State == model.StateConnected:
			mark = "● "
		case e.State == model.StateError:
			mark = "! "
		}
		sub = append(sub, leaf(mark+e.Name, Action{Kind: ActSelectConnection, ConnectionID: e.ID}))
	}
	return Item{Label: "Engines", Enabled: true, Children: sub}
}

func containersMenu(s State) Item {
	e, ok := s.active()
	if !ok || e.State != model.StateConnected {
		return note("Running containers: engine not connected")
	}
	if len(s.Containers) == 0 {
		return note("Running containers: none")
	}
	list := append([]Container(nil), s.Containers...)
	sort.Slice(list, func(i, j int) bool { return strings.ToLower(list[i].Name) < strings.ToLower(list[j].Name) })
	more := 0
	if len(list) > MaxContainers {
		more = len(list) - MaxContainers
		list = list[:MaxContainers]
	}
	sub := make([]Item, 0, len(list)+1)
	for _, c := range list {
		a := Action{ConnectionID: e.ID, ContainerID: c.ID, Name: c.Name}
		sub = append(sub, Item{Label: c.Name, Enabled: true, Children: []Item{
			leaf("Stop", withKind(a, ActStop)),
			leaf("Restart", withKind(a, ActRestart)),
			leaf("Open in Docklet", withKind(a, ActOpenContainer)),
		}})
	}
	if more > 0 {
		sub = append(sub, note(fmt.Sprintf("%d more in Docklet", more)))
	}
	return Item{Label: fmt.Sprintf("Running containers (%d)", len(s.Containers)), Enabled: true, Children: sub}
}

func withKind(a Action, k ActionKind) Action {
	a.Kind = k
	return a
}
