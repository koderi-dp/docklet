package notify

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/moby/moby/api/types/events"

	"docklet/internal/model"
)

func crashEvent(name string) events.Message {
	return containerEvent("die", name, map[string]string{"exitCode": "1", "com.docker.compose.project": "shop"})
}

func TestFlow_CrashEventBecomesStoredEmittedAndListedAlert(t *testing.T) {
	store, em := &memAlertStore{}, &recEmitter{}
	now := time.Unix(1_700_000_000, 0)
	f := newFlowHub(Deps{Store: store, Emitter: em, Profiles: fakeProfiles{"c1": "Local"}, Now: func() time.Time { return now }}, allOn())

	f.ObserveEvent("c1", crashEvent("web"))
	f.finish(t)

	got := store.stored()
	if len(got) != 1 {
		t.Fatalf("want 1 stored alert, got %d", len(got))
	}
	a := got[0]
	if a.ID == "" || a.ConnectionID != "c1" || a.ConnectionName != "Local" || a.ContainerID != "id-web" || a.ContainerName != "web" ||
		a.ComposeProject != "shop" || a.Rule != model.RuleCrash || a.Severity != model.SeverityError ||
		a.Detail != "exit code 1" || a.At != now.UnixMilli() || a.Read {
		t.Fatalf("unexpected alert %+v", a)
	}
	if list := f.List(0); len(list) != 1 || list[0] != a {
		t.Fatalf("hub list must match the stored alert: %+v", list)
	}
	if em.count() != 1 || em.names[0] != model.EventAlertsNew {
		t.Fatalf("emitted %v", em.names)
	}
	if n, ok := em.data[0].(model.AlertNew); !ok || n.Alert != a {
		t.Fatalf("event payload %+v", em.data[0])
	}
}

func TestFlow_EventsThatMustNotAlert(t *testing.T) {
	muted := allOn()
	muted.MutedContainers = []model.MutedContainer{{ConnectionID: "c1", ContainerName: "web"}}
	disabled := allOn()
	disabled.Enabled = false
	crashOff := allOn()
	crashOff.Rules.Crash = false

	for _, tc := range []struct {
		name string
		s    model.NotificationSettings
		ev   events.Message
	}{
		{"alerts globally disabled", disabled, crashEvent("web")},
		{"crash rule switched off", crashOff, crashEvent("web")},
		{"muted container", muted, crashEvent("web")},
		{"clean exit", allOn(), containerEvent("die", "web", map[string]string{"exitCode": "0"})},
		{"a die without exit code", allOn(), containerEvent("die", "web", nil)},
		{"non container event", allOn(), events.Message{Type: events.NetworkEventType, Action: "die",
			Actor: events.Actor{ID: "n", Attributes: map[string]string{"name": "web", "exitCode": "1"}}}},
		{"unrelated action", allOn(), containerEvent("exec_start", "web", nil)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, em := &memAlertStore{}, &recEmitter{}
			os := newFakeOS()
			s := tc.s
			s.OSNotifications = true
			f := newFlowHub(Deps{Store: store, Emitter: em, OS: os}, s)
			f.ObserveEvent("c1", tc.ev)
			f.finish(t)
			if len(store.stored()) != 0 || len(f.List(0)) != 0 || em.count() != 0 || len(os.calls) != 0 {
				t.Fatalf("an alert was produced: stored=%v emitted=%d os=%d", store.stored(), em.count(), len(os.calls))
			}
		})
	}
}

func TestFlow_MuteIsPerContainerNotPerConnectionWide(t *testing.T) {
	s := allOn()
	s.MutedContainers = []model.MutedContainer{{ConnectionID: "c1", ContainerName: "web"}}
	store := &memAlertStore{}
	f := newFlowHub(Deps{Store: store}, s)
	f.ObserveEvent("c1", crashEvent("web")) // muted
	f.ObserveEvent("c1", crashEvent("db"))  // other container
	f.ObserveEvent("c2", crashEvent("web")) // same name, other connection
	f.finish(t)
	var names []string
	for _, a := range store.stored() {
		names = append(names, a.ConnectionID+"/"+a.ContainerName)
	}
	if fmt.Sprint(names) != "[c2/web c1/db]" {
		t.Fatalf("got %v", names)
	}
}

func TestFlow_RepeatedCrashWithinTheCoalesceWindowAlertsOnce(t *testing.T) {
	store := &memAlertStore{}
	f := newFlowHub(Deps{Store: store}, allOn())
	for range 3 {
		f.ObserveEvent("c1", crashEvent("web"))
	}
	f.finish(t)
	if n := len(store.stored()); n != 1 {
		t.Fatalf("flapping container produced %d alerts, want 1", n)
	}
}

func TestFlow_UnhealthyAlertCarriesTheLastHealthCheckOutput(t *testing.T) {
	state := `{"Status":"running","Running":true,"Health":{"Status":"unhealthy","FailingStreak":3,"Log":[` +
		`{"ExitCode":1,"Output":"old failure"},{"ExitCode":1,"Output":"curl: (7) failed\ntoken=abc123\n"}]}}`
	prov := inspectProvider(t, `{"Id":"id-web","Name":"/web","State":`+state+`}`)
	store := &memAlertStore{}
	f := newFlowHub(Deps{Store: store, Engines: prov}, allOn())
	f.ObserveEvent("c1", containerEvent("health_status: unhealthy", "web", nil))
	f.finish(t)
	got := store.stored()
	if len(got) != 1 || got[0].Rule != model.RuleUnhealthy {
		t.Fatalf("got %+v", got)
	}
	if got[0].Detail != "unhealthy: token=***" {
		t.Fatalf("detail must be the newest run's last line, redacted: %q", got[0].Detail)
	}
}

func TestHealthDetail_FallsBackToPlainUnhealthy(t *testing.T) {
	ev := Event{ConnectionID: "c1", ContainerID: "id-web"}
	noLog := inspectProvider(t, `{"Id":"x","State":{"Running":true,"Health":{"Status":"unhealthy","Log":[]}}}`)
	emptyOut := inspectProvider(t, `{"Id":"x","State":{"Running":true,"Health":{"Status":"unhealthy","Log":[{"ExitCode":1,"Output":" \n\n"}]}}}`)
	noState := inspectProvider(t, `{"Id":"x"}`)
	for name, h := range map[string]*Hub{
		"no engine provider":     {},
		"engine not connected":   {d: Deps{Engines: failingProvider{}}},
		"empty health log":       {d: Deps{Engines: noLog}},
		"whitespace only output": {d: Deps{Engines: emptyOut}},
		"no state in inspect":    {d: Deps{Engines: noState}},
	} {
		if got := h.healthDetail(ev); got != "unhealthy" {
			t.Errorf("%s: %q", name, got)
		}
	}
}

func TestFlow_OSNotificationIsSentOnlyWhenEnabled(t *testing.T) {
	os := newFakeOS()
	s := allOn()
	s.OSNotifications = true
	f := newFlowHub(Deps{OS: os, Profiles: fakeProfiles{"c1": "Local"}}, s)
	f.ObserveEvent("c1", crashEvent("web"))
	f.finish(t)
	if c := os.next(t); c.title != "Container crashed: web" || c.body != "exit code 1 (Local)" {
		t.Fatalf("got %+v", c)
	}

	off := newFakeOS()
	f = newFlowHub(Deps{OS: off}, allOn()) // OSNotifications left false
	f.ObserveEvent("c1", crashEvent("web"))
	f.finish(t)
	if len(f.List(0)) != 1 {
		t.Fatal("the in-app alert must still be recorded")
	}
	select {
	case c := <-off.calls:
		t.Fatalf("OS notification sent although switched off: %+v", c)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestFlow_BrokenOSNotifierNeverLosesTheAlertOrCrashesTheHub(t *testing.T) {
	s := allOn()
	s.OSNotifications = true
	for name, os := range map[string]*fakeOS{"panics": {calls: make(chan osCall, 1), boom: true}, "errors": {calls: make(chan osCall, 1), err: errors.New("no dbus")}} {
		store := &memAlertStore{}
		f := newFlowHub(Deps{Store: store, OS: os}, s)
		f.ObserveEvent("c1", crashEvent("web"))
		f.ObserveEvent("c1", crashEvent("db"))
		f.finish(t)
		if len(store.stored()) != 2 {
			t.Errorf("%s: stored %d alerts, want 2", name, len(store.stored()))
		}
	}
}

func TestFlow_StoreFailureStillDeliversInAppAlert(t *testing.T) {
	store, em := &memAlertStore{addErr: errors.New("disk full")}, &recEmitter{}
	f := newFlowHub(Deps{Store: store, Emitter: em}, allOn())
	f.ObserveEvent("c1", crashEvent("web"))
	f.finish(t)
	if len(f.List(0)) != 1 || em.count() != 1 {
		t.Fatalf("alert lost on persist error: list=%d emitted=%d", len(f.List(0)), em.count())
	}
}

func TestHub_RingKeepsNewest200AndRestoresFromStore(t *testing.T) {
	store := &memAlertStore{}
	h := &Hub{d: Deps{Store: store}}
	for i := 0; i < ringSize+5; i++ {
		h.add(model.Alert{ID: fmt.Sprint("a", i)})
	}
	list := h.List(0)
	if len(list) != ringSize || list[0].ID != fmt.Sprint("a", ringSize+4) || list[ringSize-1].ID != "a5" {
		t.Fatalf("ring: len=%d first=%s last=%s", len(list), list[0].ID, list[len(list)-1].ID)
	}
	if len(store.stored()) != ringSize+5 {
		t.Fatalf("the store applies its own cap, the hub must not drop writes: %d", len(store.stored()))
	}

	reloaded := NewHub(Deps{Store: store, KV: &fakeKV{m: map[string]string{}}})
	if got := reloaded.List(0); len(got) != ringSize+5 || got[0].ID != list[0].ID {
		t.Fatalf("history not restored at start: %d", len(got))
	}
	if store.lastList != ringSize {
		t.Fatalf("hub must ask the store for at most ringSize alerts, asked %d", store.lastList)
	}
}

func TestObserveEvent_MapsAttributesAndNeverBlocksOnAFullQueue(t *testing.T) {
	h := &Hub{in: make(chan Event, queueSize)}
	h.ObserveEvent("c9", containerEvent("die", "web", map[string]string{"exitCode": "137", "com.docker.compose.project": "shop"}))
	ev := <-h.in
	want := Event{ConnectionID: "c9", ContainerID: "id-web", ContainerName: "web", Project: "shop", Action: "die", ExitCode: "137"}
	if ev != want {
		t.Fatalf("got %+v want %+v", ev, want)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for range queueSize + 50 { // nobody consumes: the overflow must be dropped, not block
			h.ObserveEvent("c9", crashEvent("web"))
		}
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("ObserveEvent blocked the event loop on a full queue")
	}
	if len(h.in) != queueSize {
		t.Fatalf("queue holds %d, want %d", len(h.in), queueSize)
	}
}

func TestHub_TestAlert(t *testing.T) {
	store, em := &memAlertStore{}, &recEmitter{}
	h := &Hub{d: Deps{Store: store, Emitter: em, Profiles: fakeProfiles{"c1": "Local"}}}
	if err := h.Test("c1"); err != nil {
		t.Fatalf("without an OS notifier the test alert is in-app only: %v", err)
	}
	got := h.List(0)
	if len(got) != 1 || got[0].Rule != "test" || got[0].ConnectionName != "Local" || got[0].Severity != model.SeverityWarning || got[0].ID == "" {
		t.Fatalf("got %+v", got)
	}
	if len(store.stored()) != 1 || em.count() != 1 {
		t.Fatal("the test alert must be stored and emitted like a real one")
	}

	os := newFakeOS()
	h.d.OS = os
	if err := h.Test("c1"); err != nil {
		t.Fatal(err)
	}
	if c := os.next(t); c.title != "Test notification" || c.body != "Container alerts are working." {
		t.Fatalf("got %+v", c)
	}

	os.err = errors.New("denied")
	err := h.Test("c1")
	if err == nil || err.Error() != "the operating system rejected the notification: denied" {
		t.Fatalf("got %v", err)
	}
	if len(h.List(0)) != 3 {
		t.Fatal("the in-app alert is kept even when the OS rejects the notification")
	}
	if a, b := h.List(0)[0].ID, h.List(0)[1].ID; a == b {
		t.Fatal("alert ids must be unique")
	}
}

func TestConnectionName(t *testing.T) {
	for _, tc := range []struct {
		name string
		h    *Hub
		id   string
		want string
	}{
		{"no profile source", &Hub{}, "c1", ""},
		{"known", &Hub{d: Deps{Profiles: fakeProfiles{"c1": "Prod"}}}, "c1", "Prod"},
		{"unknown id", &Hub{d: Deps{Profiles: fakeProfiles{"c1": "Prod"}}}, "zz", ""},
	} {
		if got := tc.h.connectionName(tc.id); got != tc.want {
			t.Errorf("%s: %q", tc.name, got)
		}
	}
}
