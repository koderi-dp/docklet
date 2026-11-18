package tray

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"docklet/internal/engine"
	"docklet/internal/model"
)

const (
	refreshDelay = 400 * time.Millisecond
	hintKey      = "tray.hintShown"
)

// Lifecycle runs container actions (satisfied by services.ContainerService).
type Lifecycle interface {
	Stop(ctx context.Context, connectionID string, ids []string) ([]model.ActionResult, error)
	Restart(ctx context.Context, connectionID string, ids []string) ([]model.ActionResult, error)
}

// KV is the tiny persistence the controller needs (satisfied by *appdb.DB).
type KV interface {
	GetSetting(ctx context.Context, key string) (string, bool, error)
	SetSetting(ctx context.Context, key, value string) error
}

// Controller owns the system tray and the close-to-tray behaviour. It is the thin Wails adapter
// around the pure menu model: the tray is created and destroyed at runtime from the settings.
type Controller struct {
	App       *application.App
	Window    *application.WebviewWindow
	Source    *Source
	Lifecycle Lifecycle
	Emit      engine.Emitter
	Notify    func(title, body string) error
	KV        KV

	quitting atomic.Bool

	mu       sync.Mutex
	settings model.AppSettings
	tray     *application.SystemTray
	menu     *application.Menu
	timer    *time.Timer
}

// Apply starts or stops the tray to match the settings; it is safe to call before app.Run.
func (c *Controller) Apply(s model.AppSettings) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.settings = s
	switch {
	case s.ShowTrayIcon && c.tray == nil:
		c.create()
	case !s.ShowTrayIcon && c.tray != nil:
		c.destroy()
	}
}

func (c *Controller) create() {
	c.menu = c.App.NewMenu()
	c.fill(BuildMenu(State{}))
	t := c.App.SystemTray.New()
	t.SetIcon(iconData)
	t.SetTooltip("Docklet")
	t.OnClick(c.Toggle)
	t.SetMenu(c.menu)
	c.tray = t
	c.scheduleRefreshLocked()
}

func (c *Controller) destroy() {
	if c.timer != nil {
		c.timer.Stop()
	}
	c.tray.Destroy()
	c.tray, c.menu = nil, nil
}

// HandleClose is the window closing hook: with the tray and closeToTray on, closing only hides the window.
func (c *Controller) HandleClose(e *application.WindowEvent) {
	if c.quitting.Load() {
		return
	}
	c.mu.Lock()
	hide := c.tray != nil && c.settings.CloseToTray
	c.mu.Unlock()
	if !hide {
		return
	}
	e.Cancel()
	c.Window.Hide()
	c.hintOnce()
}

// hintOnce tells the user, a single time ever, that the app keeps running in the tray.
func (c *Controller) hintOnce() {
	if c.KV == nil || c.Notify == nil {
		return
	}
	ctx := context.Background()
	if _, shown, _ := c.KV.GetSetting(ctx, hintKey); shown {
		return
	}
	_ = c.KV.SetSetting(ctx, hintKey, "1")
	_ = c.Notify("Docklet", "Docklet is still running in the tray")
}

// Quit really quits (the close hook lets the window close) and runs the normal shutdown paths.
func (c *Controller) Quit() {
	c.quitting.Store(true)
	c.App.Quit()
}

// ShowWindow brings the main window back.
func (c *Controller) ShowWindow() {
	c.Window.Show()
	c.Window.UnMinimise()
	c.Window.Focus()
}

// Toggle hides the window when it is in front, otherwise shows it.
func (c *Controller) Toggle() {
	if c.Window.IsVisible() && !c.Window.IsMinimised() {
		c.Window.Hide()
		return
	}
	c.ShowWindow()
}

// SetActive records the engine selected in the app and refreshes the menu.
func (c *Controller) SetActive(id string) {
	c.Source.SetActive(id)
	c.scheduleRefresh()
}

// Tap is called for every backend event; only the ones that change the menu cause a refresh.
func (c *Controller) Tap(name string) {
	switch name {
	case model.EventContainersChanged, model.EventConnectionStatus, model.EventAlertsNew:
		c.scheduleRefresh()
	}
}

func (c *Controller) scheduleRefresh() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.scheduleRefreshLocked()
}

// scheduleRefreshLocked debounces refreshes; it does nothing while there is no tray.
func (c *Controller) scheduleRefreshLocked() {
	if c.tray == nil {
		return
	}
	if c.timer != nil {
		c.timer.Stop()
	}
	c.timer = time.AfterFunc(refreshDelay, c.refresh)
}

func (c *Controller) refresh() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	items := BuildMenu(c.Source.Snapshot(ctx))
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.tray == nil {
		return
	}
	c.fill(items)
}

// fill rebuilds the one tray menu in place from the model. Callers hold c.mu (or are single-threaded).
func (c *Controller) fill(items []Item) {
	c.menu.Clear()
	c.addItems(c.menu, items)
	c.menu.Update()
}

func (c *Controller) addItems(m *application.Menu, items []Item) {
	for _, it := range items {
		switch {
		case it.Separator:
			m.AddSeparator()
		case len(it.Children) > 0:
			c.addItems(m.AddSubmenu(it.Label), it.Children)
		default:
			mi := m.Add(it.Label).SetEnabled(it.Enabled)
			if a := it.Action; a.Kind != "" {
				mi.OnClick(func(*application.Context) { c.run(a) })
			}
		}
	}
}

func (c *Controller) run(a Action) {
	switch a.Kind {
	case ActShow:
		c.ShowWindow()
	case ActQuit:
		c.Quit()
	case ActSelectConnection:
		c.ShowWindow()
		c.Emit.Emit(model.EventTraySelectConnection, model.TraySelectConnection{ConnectionID: a.ConnectionID})
	case ActOpenAlerts:
		c.ShowWindow()
		c.Emit.Emit(model.EventTrayOpen, model.TrayOpen{Target: model.TrayTargetAlerts})
	case ActOpenContainer:
		c.ShowWindow()
		c.Emit.Emit(model.EventTrayOpen, model.TrayOpen{Target: model.TrayTargetContainer, ConnectionID: a.ConnectionID, ContainerID: a.ContainerID})
	case ActStop, ActRestart:
		go c.lifecycle(a)
	}
}

func (c *Controller) lifecycle(a Action) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	do, verb := c.Lifecycle.Stop, "stop"
	if a.Kind == ActRestart {
		do, verb = c.Lifecycle.Restart, "restart"
	}
	res, err := do(ctx, a.ConnectionID, []string{a.ContainerID})
	if err == nil && len(res) > 0 && !res[0].OK {
		err = fmt.Errorf("%s", res[0].Error)
	}
	if err != nil && c.Notify != nil {
		_ = c.Notify("Docklet", fmt.Sprintf("Could not %s %s: %v", verb, a.Name, err))
	}
}
