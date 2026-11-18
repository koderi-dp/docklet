package main

import (
	"strconv"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

// appEmitter adapts app.Event.Emit to engine.Emitter. The app is set after
// application.New because services must be constructed first. Tap (optional) sees every event name.
type appEmitter struct {
	app *application.App
	tap func(name string)
}

func (e *appEmitter) Emit(name string, data any) {
	if e.app != nil {
		e.app.Event.Emit(name, data)
	}
	if e.tap != nil {
		e.tap(name)
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
