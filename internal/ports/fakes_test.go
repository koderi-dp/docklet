package ports

import (
	"context"
	"errors"
)

type fakeEngine struct {
	bindings []Binding
	err      error
}

func (f fakeEngine) Bindings(context.Context) ([]Binding, error) { return f.bindings, f.err }

type fakeHost struct {
	where     string
	listeners []Listener
	err       error
}

func (f fakeHost) Where() string { return f.where }
func (f fakeHost) Listeners(context.Context) ([]Listener, error) {
	return f.listeners, f.err
}

var errBoom = errors.New("boom")

func running(id, name, project, ip string, port int, proto string) Binding {
	return Binding{ContainerID: id, Name: name, Project: project, Running: true, HostIP: ip, HostPort: port, Protocol: proto}
}

func check(c *Checker, ip string, port int, proto string, ignore ...string) PortCheck {
	return c.Check(context.Background(), []Request{{HostIP: ip, HostPort: port, Protocol: proto}}, ignore)[0]
}

func local(ls ...Listener) *Checker {
	return &Checker{Engine: fakeEngine{}, Hosts: []HostSource{fakeHost{where: "This machine", listeners: ls}}}
}
