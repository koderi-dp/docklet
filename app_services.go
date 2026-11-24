package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"

	"docklet/internal/appdb"
	"docklet/internal/engine"
	"docklet/internal/model"
	"docklet/internal/notify"
	"docklet/internal/ops"
	"docklet/internal/secrets"
	"docklet/internal/services"
	"docklet/internal/store"
	"docklet/internal/tray"
	"docklet/internal/update"
	"docklet/internal/version"
)

// appRuntime owns everything main wires together: storage, engines, background work and the tray.
type appRuntime struct {
	db       *appdb.DB
	manager  *engine.Manager
	ops      *ops.Manager
	hub      *notify.Hub
	emitter  *appEmitter
	picker   *wailsPicker
	notifier *osNotifier
	tray     *tray.Controller
	settings model.AppSettings // as stored at startup
	appSvc   *services.AppService
	bound    []application.Service
	stop     context.CancelFunc
	ctx      context.Context
}

func newRuntime() (*appRuntime, error) {
	dbPath, err := appdb.DefaultPath()
	if err != nil {
		return nil, err
	}
	db, err := appdb.Open(dbPath)
	if err != nil {
		return nil, err
	}
	legacyPath, err := store.DefaultPath()
	if err != nil {
		return nil, err
	}
	profiles, err := store.NewWithDB(db, legacyPath)
	if err != nil {
		return nil, err
	}
	r := &appRuntime{db: db, emitter: &appEmitter{}, picker: &wailsPicker{}}
	r.ctx, r.stop = context.WithCancel(context.Background())
	r.manager = engine.NewManager(profiles, r.emitter)
	r.manager.SetSecrets(secrets.NewOSStore())
	r.ops = ops.New(r.emitter)
	r.ops.OnFinish = func(op model.Operation) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := db.RecordOperation(ctx, op); err != nil {
			log.Printf("operation history: %v", err)
		}
	}
	// Container health alerts (off by default). OS notifications go through the Wails service.
	osNotifications := notifications.New()
	r.notifier = &osNotifier{svc: osNotifications}
	r.hub = notify.NewHub(notify.Deps{Engines: r.manager, Profiles: r.manager, Emitter: r.emitter, KV: db, Store: db, OS: r.notifier})
	r.manager.SetEventObserver(r.hub)
	r.settings = services.LoadAppSettings(context.Background(), db)
	r.bound = r.buildServices(osNotifications)
	return r, nil
}

func (r *appRuntime) buildServices(osNotifications *notifications.NotificationService) []application.Service {
	m, e, o := r.manager, r.emitter, r.ops
	containers := &services.ContainerService{Engines: m, Events: e, Ops: o}
	r.tray = &tray.Controller{
		Source:    &tray.Source{Engines: m, Containers: containers, Alerts: r.hub},
		Lifecycle: containers, Emit: e, Notify: r.notifier.Notify, KV: r.db,
	}
	e.tap = r.tray.Tap
	r.appSvc = &services.AppService{
		KV: r.db, Tray: r.tray, Events: e, Version: version.Version,
		Checker: &update.Checker{URL: os.Getenv("DOCKLET_UPDATE_URL")}, // empty = GitHub; override is for testing
	}
	if v := os.Getenv("DOCKLET_UPDATE_CURRENT_VERSION"); v != "" {
		r.appSvc.Version = v // testing aid: pretend to run another version
	}
	connections := &services.ConnectionService{Manager: m, Ops: o}
	if os.Getenv("DOCKLET_FAKE_ENVIRONMENT") == "nothing" {
		// Testing aid: first-run detection finds nothing, so the "nothing usable" screen can be seen
		// without uninstalling anything. Nothing else changes.
		connections.EnvironmentHook = func(r model.EnvironmentReport) model.EnvironmentReport {
			return model.EnvironmentReport{OS: r.OS, Local: []model.LocalEngineReport{}, Contexts: []model.DockerContextInfo{},
				Saved: []model.ConnectionProfile{}, WSL: model.WSLReport{Distros: []model.WSLDistroReport{}, Detail: "No WSL distributions are installed (or WSL itself is not installed)."}}
		}
	}
	composeSvc := &services.ComposeService{Engines: m, Events: e, Ops: o, Profiles: m, Projects: r.db, Picker: r.picker}
	return []application.Service{
		application.NewService(connections),
		application.NewService(containers),
		application.NewService(&services.CopyAsService{Engines: m, Profiles: m}),
		application.NewService(&services.OperationService{Ops: o, History: r.db.ListOperationHistory}),
		application.NewService(&services.DialogService{Picker: r.picker}),
		application.NewService(&services.ImageService{Engines: m, Events: e, Ops: o, Profiles: m, Builds: r.db}),
		application.NewService(&services.RegistryService{Engines: m}),
		application.NewService(&services.VolumeService{Engines: m, Events: e, Ops: o}),
		application.NewService(&services.NetworkService{Engines: m, Events: e, Ops: o}),
		application.NewService(&services.SystemService{Engines: m, Events: e, Ops: o}),
		application.NewService(composeSvc),
		application.NewService(&services.PortService{Engines: m, Profiles: m, Compose: composeSvc}),
		application.NewService(&services.ExecService{Engines: m, Events: e, Ops: o, Profiles: m}),
		application.NewService(&services.HostService{Profiles: m}),
		application.NewService(&services.WSLService{Manager: m, Ops: o}),
		application.NewService(&services.FileService{Engines: m, Events: e, Ops: o}),
		application.NewService(&services.NotificationService{Hub: r.hub}),
		application.NewService(r.appSvc),
		application.NewService(osNotifications),
	}
}

// showWindow brings the main window to the front; a no-op until attach has wired the window.
func (r *appRuntime) showWindow() {
	if r.tray.Window != nil {
		r.tray.ShowWindow()
	}
}

// startHidden: the window starts hidden only when the tray can bring it back.
func (r *appRuntime) startHidden() bool {
	return r.settings.ShowTrayIcon && r.settings.StartMinimizedToTray
}

// attach connects the runtime to the created app and window and starts the tray and background checks.
func (r *appRuntime) attach(app *application.App, win *application.WebviewWindow) {
	r.emitter.app, r.picker.app = app, app
	r.tray.App, r.tray.Window = app, win
	win.RegisterHook(events.Common.WindowClosing, r.tray.HandleClose)
	// Forward dropped file paths to the frontend (only elements marked data-file-drop-target report drops).
	win.OnWindowEvent(events.Common.WindowFilesDropped, func(e *application.WindowEvent) {
		r.emitter.Emit("files-dropped", map[string]any{"files": e.Context().DroppedFiles()})
	})
	r.tray.Apply(r.settings)
	services.StartBackgroundChecks(r.ctx, r.appSvc)
}

// close runs the shutdown paths: background work, engines, operations, then the database.
func (r *appRuntime) close() {
	r.stop()
	r.ops.Shutdown()
	r.manager.Shutdown()
	r.db.Close()
}
