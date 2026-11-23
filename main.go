package main

import (
	"embed"
	"log"
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"

	"docklet/internal/linuxsandbox"
	"docklet/internal/model"
	"docklet/internal/secrets"
)

//go:embed all:frontend/dist
var assets embed.FS

func init() {
	application.RegisterEvent[model.ContainersChanged](model.EventContainersChanged)
	application.RegisterEvent[model.ConnectionStatus](model.EventConnectionStatus)
	application.RegisterEvent[model.Operation](model.EventOperationUpdated)
	application.RegisterEvent[model.ComposeWatchChanged](model.EventComposeWatch)
	application.RegisterEvent[model.ImagesChanged](model.EventImagesChanged)
	application.RegisterEvent[model.VolumesChanged](model.EventVolumesChanged)
	application.RegisterEvent[model.NetworksChanged](model.EventNetworksChanged)
	application.RegisterEvent[model.OperationLog](model.EventOperationLog)
	application.RegisterEvent[model.AlertNew](model.EventAlertsNew)
	application.RegisterEvent[model.ContainerStatsData](model.EventContainerStatsData)
	application.RegisterEvent[model.UpdateStatus](model.EventAppUpdate)
	application.RegisterEvent[model.TrayOpen](model.EventTrayOpen)
	application.RegisterEvent[model.TraySelectConnection](model.EventTraySelectConnection)
}

func main() {
	// ssh re-runs this executable as its askpass helper for password/passphrase auth.
	if secrets.RunAskpassIfRequested() {
		return
	}
	if handleVersionFlag(os.Args[1:]) {
		return
	}
	// On Linux, explain a blocked WebKit sandbox before WebKit would crash on it.
	linuxsandbox.Guard()
	rt, err := newRuntime()
	if err != nil {
		log.Fatal(err)
	}
	defer rt.close()

	app := application.New(application.Options{
		Name:        "Docklet",
		Description: "Desktop client for existing Docker Engines",
		Services:    rt.bound,
		// The window is attached after New; a launch that arrives before that has nothing to show yet.
		SingleInstance: singleInstance(rt.showWindow),
		Assets:         application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})
	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "Docklet",
		EnableFileDrop:   true, // the Compose page is a drop target (data-file-drop-target)
		Hidden:           rt.startHidden(),
		Width:            1440,
		Height:           900,
		MinWidth:         1000,
		MinHeight:        680,
		BackgroundColour: application.NewRGB(11, 16, 25),
		URL:              "/",
	})
	rt.attach(app, win)
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
