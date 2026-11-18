package engine

import (
	"os"
	"path/filepath"
	"runtime"

	"docklet/internal/model"
)

// A discovered socket is only offered when the file exists (read-only detection; nothing is connected).
type socketCandidate struct{ id, name, path string }

// socketCandidates lists the well-known per-user Docker sockets on Linux and macOS.
func socketCandidates(home, xdgRuntimeDir string) []socketCandidate {
	var c []socketCandidate
	if xdgRuntimeDir != "" {
		c = append(c, socketCandidate{"local-rootless", "Rootless Docker", filepath.Join(xdgRuntimeDir, "docker.sock")})
	}
	if home != "" {
		c = append(c,
			socketCandidate{"local-docker-desktop", "Docker Desktop", filepath.Join(home, ".docker", "run", "docker.sock")},
			socketCandidate{"local-docker-desktop-linux", "Docker Desktop", filepath.Join(home, ".docker", "desktop", "docker.sock")},
			socketCandidate{"local-colima", "Colima", filepath.Join(home, ".colima", "default", "docker.sock")},
			socketCandidate{"local-orbstack", "OrbStack", filepath.Join(home, ".orbstack", "run", "docker.sock")},
		)
	}
	return c
}

func isSocket(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.Mode()&os.ModeSocket != 0
}

// discoverSockets returns a Discovered unix profile for every candidate socket that exists.
func discoverSockets(home, xdgRuntimeDir string, exists func(string) bool) []model.ConnectionProfile {
	var out []model.ConnectionProfile
	for _, c := range socketCandidates(home, xdgRuntimeDir) {
		if exists(c.path) {
			out = append(out, model.ConnectionProfile{ID: c.id, Name: c.name, Type: model.ConnUnix,
				Endpoint: "unix://" + filepath.ToSlash(c.path), Discovered: true})
		}
	}
	return out
}

// DiscoveredProfiles is the default local engine plus any other well-known local socket that exists.
func DiscoveredProfiles() []model.ConnectionProfile {
	out := []model.ConnectionProfile{DefaultLocalProfile()}
	if runtime.GOOS == "windows" {
		return out
	}
	home, _ := os.UserHomeDir()
	return append(out, discoverSockets(home, os.Getenv("XDG_RUNTIME_DIR"), isSocket)...)
}
