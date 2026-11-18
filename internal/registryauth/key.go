package registryauth

import (
	"errors"
	"regexp"
	"strings"
)

var hostRe = regexp.MustCompile(`^(\[[0-9a-f:.]+\]|[a-z0-9]([a-z0-9._-]*[a-z0-9])?)(:[0-9]{1,5})?$`)

// CleanHost validates a registry host typed by the user ("ghcr.io",
// "https://registry.example.com:5000/", "docker.io") and returns the
// normalized host ("docker.io" for every Docker Hub alias).
func CleanHost(s string) (string, error) {
	h := NormalizeHost(s)
	if h == "" {
		return "", errors.New("enter a registry host, for example ghcr.io")
	}
	if !hostRe.MatchString(h) || strings.Contains(h, "..") {
		return "", errors.New("that is not a valid registry host (use host or host:port, without a path)")
	}
	return h, nil
}

// ServerKey is the key the Docker CLI uses for the registry in config.json
// and credential helpers: the legacy URL for Docker Hub, the host otherwise.
func ServerKey(host string) string {
	h := NormalizeHost(host)
	if h == hubHost {
		return hubServerURL
	}
	return h
}
