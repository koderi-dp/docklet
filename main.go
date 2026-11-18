package main

import (
	"context"
	"embed"
	"log"
	"strconv"
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
)

// appEmitter adapts app.Event.Emit to engine.Emitter. The app is set after
// application.New because services must be constructed first.
type appEmitter struct{ app *application.App }

func (e *appEmitter) Emit(name string, data any) {
	if e.app != nil {
		e.app.Event.Emit(name, data)
	}
}

// wailsPicker implements services.FilePicker with native dialogs.
type wailsPicker struct{ app *application.App }

func (p *wailsPicker) OpenFile(title, filterName, pattern string) (string, error) {
	d := p.app.Dialog.OpenFile().SetTitle(title).CanChooseFiles(true)
	if pattern != "" {
		d = d.AddFilter(filterName, pattern)
	}
	return d.PromptForSingleSelection()
}

func (p *wailsPicker) OpenDirectory(title string) (string, error) {
	return p.app.Dialog.OpenFile().SetTitle(title).CanChooseFiles(false).CanChooseDirectories(true).PromptForSingleSelection()
}

func (p *wailsPicker) SaveFile(title, defaultName string) (string, error) {
	return p.app.Dialog.SaveFile().SetMessage(title).SetFilename(defaultName).PromptForSingleSelection()
}

// osNotifier adapts the Wails notification service to notify.OSNotifier.
type osNotifier struct {
	svc *notifications.NotificationService
}

func (o *osNotifier) Notify(title, body string) error {
	return o.svc.SendNotification(notifications.NotificationOptions{
		ID:    strconv.FormatInt(time.Now().UnixNano(), 36),
		Title: title,
		Body:  body,
	})
}

//go:embed all:frontend/dist
var assets embed.FS

func init() {
	application.RegisterEvent[model.ContainersChanged](model.EventContainersChanged)
	application.RegisterEvent[model.ConnectionStatus](model.EventConnectionStatus)
	application.RegisterEvent[model.Operation](model.EventOperationUpdated)
	application.RegisterEvent[model.ImagesChanged](model.EventImagesChanged)
	application.RegisterEvent[model.VolumesChanged](model.EventVolumesChanged)
	application.RegisterEvent[model.NetworksChanged](model.EventNetworksChanged)
	application.RegisterEvent[model.OperationLog](model.EventOperationLog)
	application.RegisterEvent[model.AlertNew](model.EventAlertsNew)
	application.RegisterEvent[model.ContainerStatsData](model.EventContainerStatsData)
}

func main() {
	// ssh re-runs this executable as its askpass helper for password/passphrase auth.
	if secrets.RunAskpassIfRequested() {
		return
	}
	dbPath, err := appdb.DefaultPath()
	if err != nil {
		log.Fatal(err)
	}
	db, err := appdb.Open(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	legacyPath, err := store.DefaultPath()
	if err != nil {
		log.Fatal(err)
	}
	profiles, err := store.NewWithDB(db, legacyPath)
	if err != nil {
		log.Fatal(err)
	}
	emitter := &appEmitter{}
	picker := &wailsPicker{}
	manager := engine.NewManager(profiles, emitter)
	manager.SetSecrets(secrets.NewOSStore())
	defer manager.Shutdown()
	opm := ops.New(emitter)
	opm.OnFinish = func(op model.Operation) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := db.RecordOperation(ctx, op); err != nil {
			log.Printf("operation history: %v", err)
		}
	}
	defer opm.Shutdown()

	// Container health alerts (off by default). OS notifications go through the Wails service.
	osNotifications := notifications.New()
	hub := notify.NewHub(notify.Deps{
		Engines: manager, Profiles: manager, Emitter: emitter, KV: db, Store: db,
		OS: &osNotifier{svc: osNotifications},
	})
	manager.SetEventObserver(hub)

	app := application.New(application.Options{
		Name:        "Docklet",
		Description: "Desktop client for existing Docker Engines",
		Services: []application.Service{
			application.NewService(&services.ConnectionService{Manager: manager}),
			application.NewService(&services.ContainerService{Engines: manager, Events: emitter, Ops: opm}),
			application.NewService(&services.OperationService{Ops: opm, History: db.ListOperationHistory}),
			application.NewService(&services.DialogService{Picker: picker}),
			application.NewService(&services.ImageService{Engines: manager, Events: emitter, Ops: opm}),
			application.NewService(&services.VolumeService{Engines: manager, Events: emitter, Ops: opm}),
			application.NewService(&services.NetworkService{Engines: manager, Events: emitter, Ops: opm}),
			application.NewService(&services.SystemService{Engines: manager, Events: emitter, Ops: opm}),
			application.NewService(&services.ComposeService{Engines: manager, Events: emitter, Ops: opm, Profiles: manager, Projects: db}),
			application.NewService(&services.ExecService{Engines: manager, Events: emitter, Ops: opm, Profiles: manager}),
			application.NewService(&services.FileService{Engines: manager, Events: emitter, Ops: opm}),
			application.NewService(&services.NotificationService{Hub: hub}),
			application.NewService(osNotifications),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	emitter.app = app
	picker.app = app

	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "Docklet",
		EnableFileDrop:   true, // the Compose page is a drop target (data-file-drop-target)
		Width:            1440,
		Height:           900,
		MinWidth:         1000,
		MinHeight:        680,
		BackgroundColour: application.NewRGB(11, 16, 25),
		URL:              "/",
	})

	// Forward dropped file paths to the frontend (only elements marked data-file-drop-target report drops).
	win.OnWindowEvent(events.Common.WindowFilesDropped, func(e *application.WindowEvent) {
		emitter.Emit("files-dropped", map[string]any{"files": e.Context().DroppedFiles()})
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
