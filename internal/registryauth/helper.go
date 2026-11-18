package registryauth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Helper speaks the Docker credential helper protocol
// (https://github.com/docker/docker-credential-helpers) with docker-credential-<Name>.
type Helper struct {
	Name   string
	Runner Runner
}

func (h Helper) run(ctx context.Context, verb, stdin string) ([]byte, error) {
	if !helperNameRe.MatchString(h.Name) {
		return nil, fmt.Errorf("invalid credential helper name %q", h.Name)
	}
	runner := h.Runner
	if runner == nil {
		runner = execRunner{}
	}
	stdout, stderr, err := runner.Run(ctx, "docker-credential-"+h.Name, stdin, verb)
	if err != nil {
		return stdout, h.failure(verb, stdout, stderr, err)
	}
	return stdout, nil
}

// failure builds an error from a helper's output. Helpers print their message
// on stdout; the text is redacted and never includes what was sent on stdin.
func (h Helper) failure(verb string, stdout, stderr []byte, err error) error {
	msg := strings.TrimSpace(firstLine(string(stdout)) + " " + firstLine(string(stderr)))
	if msg == "" {
		msg = errText(err)
	}
	return &helperError{verb: verb, name: h.Name, msg: Redact(msg), notFound: isNotFound(string(stdout) + " " + string(stderr))}
}

type helperError struct {
	verb, name, msg string
	notFound        bool
}

func (e *helperError) Error() string {
	return fmt.Sprintf("credential helper %s (%s) failed: %s", e.name, e.verb, e.msg)
}

func isNotFoundErr(err error) bool {
	var he *helperError
	return errors.As(err, &he) && he.notFound
}

// Get returns the stored username and secret ("" and "" when nothing is stored).
func (h Helper) Get(ctx context.Context, serverURL string) (user, secret string, err error) {
	out, err := h.run(ctx, "get", serverURL)
	if err != nil {
		if isNotFoundErr(err) {
			return "", "", nil
		}
		return "", "", err
	}
	var v struct{ Username, Secret string }
	if err := json.Unmarshal(out, &v); err != nil {
		return "", "", fmt.Errorf("credential helper %s returned unreadable output", h.Name)
	}
	return v.Username, v.Secret, nil
}

// Store saves credentials. An identity token is stored with username "<token>".
func (h Helper) Store(ctx context.Context, serverURL, username, secret string) error {
	in, _ := json.Marshal(map[string]string{"ServerURL": serverURL, "Username": username, "Secret": secret})
	_, err := h.run(ctx, "store", string(in))
	return err
}

// Erase removes the entry; an entry that does not exist is not an error.
func (h Helper) Erase(ctx context.Context, serverURL string) error {
	_, err := h.run(ctx, "erase", serverURL)
	if isNotFoundErr(err) {
		return nil
	}
	return err
}

// List returns server URL -> username for everything the helper holds.
func (h Helper) List(ctx context.Context) (map[string]string, error) {
	out, err := h.run(ctx, "list", "")
	if err != nil {
		return nil, err
	}
	out = bytes.TrimSpace(out)
	m := map[string]string{}
	if len(out) == 0 || string(out) == "null" {
		return m, nil
	}
	if err := json.Unmarshal(out, &m); err != nil {
		return nil, fmt.Errorf("credential helper %s returned an unreadable list", h.Name)
	}
	return m, nil
}

// helperOnPath reports whether docker-credential-<name> can be executed.
func helperOnPath(lookPath func(string) (string, error), name string) bool {
	if !helperNameRe.MatchString(name) {
		return false
	}
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	_, err := lookPath("docker-credential-" + name)
	return err == nil
}
