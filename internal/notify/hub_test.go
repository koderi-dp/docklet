package notify

import (
	"context"
	"strings"
	"testing"

	"docklet/internal/model"
)

func TestCleanDetailRedactsAndTruncatesLastLine(t *testing.T) {
	got := cleanDetail("first\nconnect failed password=hunter2\n\n")
	if got != "connect failed password=***" {
		t.Fatalf("got %q", got)
	}
	long := cleanDetail(strings.Repeat("x", 500))
	if len([]rune(long)) > detailMaxLen+3 {
		t.Fatalf("detail not truncated: %d runes", len([]rune(long)))
	}
}

func TestSaveRejectsOSNotificationsWithoutNotifier(t *testing.T) {
	h := NewHub(Deps{KV: &fakeKV{m: map[string]string{}}})
	s := h.Settings()
	s.OSNotifications = true
	if _, err := h.Save(context.Background(), s); err == nil {
		t.Fatal("expected an error when no OS notifier is wired")
	}
}

func TestAlertsAreOffByDefaultAndMutesAreDeduplicated(t *testing.T) {
	kv := &fakeKV{m: map[string]string{}}
	h := NewHub(Deps{KV: kv})
	if h.Settings().Enabled {
		t.Fatal("alerts must be opt-in")
	}
	s := h.Settings()
	m := model.MutedContainer{ConnectionID: "c1", ContainerName: "web"}
	s.MutedContainers = []model.MutedContainer{m, m, {ConnectionID: "c1", ContainerName: " "}}
	got, err := h.Save(context.Background(), s)
	if err != nil || len(got.MutedContainers) != 1 {
		t.Fatalf("mutes = %+v, err %v", got.MutedContainers, err)
	}
	if reloaded := NewHub(Deps{KV: kv}).Settings(); len(reloaded.MutedContainers) != 1 {
		t.Fatal("settings were not persisted")
	}
}

type fakeKV struct{ m map[string]string }

func (k *fakeKV) GetSetting(_ context.Context, key string) (string, bool, error) {
	v, ok := k.m[key]
	return v, ok, nil
}
func (k *fakeKV) SetSetting(_ context.Context, key, v string) error { k.m[key] = v; return nil }
