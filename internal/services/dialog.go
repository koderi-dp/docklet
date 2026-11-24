package services

import "context"

// FilePicker opens native file dialogs. Implemented in main.go on top of Wails.
// An empty result with a nil error means the user cancelled.
type FilePicker interface {
	OpenFile(title, filterName, pattern string) (string, error)
	OpenDirectory(title string) (string, error)
	SaveFile(title, defaultName string) (string, error)
}

// DialogService exposes native file dialogs to the frontend. Paths always come
// from an explicit user choice; services receiving a path must still validate it.
type DialogService struct {
	Picker FilePicker
}

// PickFile lets the user choose one existing file. pattern is e.g. "*.yml;*.yaml" ("" = any).
func (s *DialogService) PickFile(ctx context.Context, title, filterName, pattern string) (string, error) {
	return s.Picker.OpenFile(title, filterName, pattern)
}

// PickDirectory lets the user choose one existing directory.
func (s *DialogService) PickDirectory(ctx context.Context, title string) (string, error) {
	return s.Picker.OpenDirectory(title)
}

// PickSavePath lets the user choose where to save a new file.
func (s *DialogService) PickSavePath(ctx context.Context, title, defaultName string) (string, error) {
	return s.Picker.SaveFile(title, defaultName)
}
