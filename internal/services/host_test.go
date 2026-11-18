package services

import (
	"context"
	"reflect"
	"testing"

	"docklet/internal/model"
)

type oneProfile model.ConnectionProfile

func (p oneProfile) Profile(string) (model.ConnectionProfile, error) {
	return model.ConnectionProfile(p), nil
}

func hostSvc(p model.ConnectionProfile, exists map[string]bool, started *[][]string) *HostService {
	return &HostService{
		Profiles: oneProfile(p),
		goos:     "windows",
		stat:     func(path string) (bool, bool) { return true, exists[path] },
		look:     func(string) (string, error) { return "", context.Canceled },
		start:    func(argv []string) error { *started = append(*started, argv); return nil },
	}
}

func TestShowInFileManagerOpensTheWSLShareOfTheProjectFolder(t *testing.T) {
	var started [][]string
	unc := `\\wsl.localhost\Ubuntu\home\u\catalo`
	s := hostSvc(model.ConnectionProfile{Type: model.ConnWSL, WSLDistro: "Ubuntu"}, map[string]bool{unc: true}, &started)
	if err := s.ShowInFileManager(context.Background(), "c1", "/home/u/catalo"); err != nil {
		t.Fatal(err)
	}
	if want := [][]string{{"explorer.exe", unc}}; !reflect.DeepEqual(started, want) {
		t.Fatalf("got %q", started)
	}
}

func TestRemoteEngineNeverLaunchesAnything(t *testing.T) {
	var started [][]string
	s := hostSvc(model.ConnectionProfile{Type: model.ConnSSH}, nil, &started)
	if err := s.ShowInFileManager(context.Background(), "c1", "/srv/app"); err == nil {
		t.Fatal("expected an error")
	}
	if res, err := s.OpenInEditor(context.Background(), "c1", "/srv/app", model.EditorSetting{Mode: "auto"}); err == nil || res.Started {
		t.Fatalf("got %+v, %v", res, err)
	}
	if len(started) != 0 {
		t.Fatalf("started %q", started)
	}
}

func TestOpenInEditorWithoutAnEditorPointsToTheSetting(t *testing.T) {
	var started [][]string
	unc := `\\wsl.localhost\Ubuntu\srv`
	s := hostSvc(model.ConnectionProfile{Type: model.ConnWSL, WSLDistro: "Ubuntu"}, map[string]bool{unc: true}, &started)
	res, err := s.OpenInEditor(context.Background(), "c1", "/srv", model.EditorSetting{Mode: "auto"})
	if err != nil || !res.NeedsSetting || res.Message == "" || len(started) != 0 {
		t.Fatalf("got %+v, %v, started %q", res, err, started)
	}
}
