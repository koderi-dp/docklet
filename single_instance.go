package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"

	"docklet/internal/version"
)

// singleInstance keeps Docklet to one running copy per user. A second launch exits at once
// and the running copy brings its window to the front (also when it is hidden in the tray).
// Development builds use their own ID, so `wails3 dev` still starts next to an installed Docklet.
func singleInstance(show func()) *application.SingleInstanceOptions {
	id := "dev.docklet.app"
	if version.Version == "dev" {
		id += ".dev"
	}
	return &application.SingleInstanceOptions{
		UniqueID:               id,
		OnSecondInstanceLaunch: func(application.SecondInstanceData) { show() },
	}
}
