package wslinfo

import (
	"strings"

	"docklet/internal/model"
	"docklet/internal/wsldocker"
)

const dropCachesScript = "sync; echo 1 > /proc/sys/vm/drop_caches"

// ReclaimCommand drops the page cache only (value 1): no dentries, inodes or application memory.
func ReclaimCommand() wsldocker.Cmd {
	return wsldocker.Cmd{Argv: []string{"sh", "-c", dropCachesScript}, Root: true}
}

// RestartDockerCommand restarts the Docker service the way the distribution runs services.
func RestartDockerCommand(systemd bool) wsldocker.Cmd {
	if systemd {
		return wsldocker.Cmd{Argv: []string{"systemctl", "restart", "docker"}, Root: true}
	}
	return wsldocker.Cmd{Argv: []string{"/usr/sbin/service", "docker", "restart"}, Root: true}
}

// TerminateCommand stops one distribution; others keep running.
func TerminateCommand(distro string) wsldocker.Cmd {
	return wsldocker.Cmd{Argv: []string{"--terminate", distro}, Host: true}
}

// ShutdownCommand stops the whole WSL virtual machine, so every distribution.
func ShutdownCommand() wsldocker.Cmd { return wsldocker.Cmd{Argv: []string{"--shutdown"}, Host: true} }

// Show renders the full wsl.exe command line of c, as shown in the confirmations.
func Show(distro string, c wsldocker.Cmd) string {
	if c.Host {
		return wsldocker.Display(c)
	}
	s := "wsl.exe -d " + distro
	if c.Root {
		s += " -u root"
	}
	return s + " --exec " + wsldocker.Display(c)
}

// Actions says which maintenance actions can run for the state in info, and why not.
func Actions(info model.WSLInfo) []model.WSLAction {
	systemd := info.SystemdRunning != nil && *info.SystemdRunning
	stopped := "The distribution is stopped."

	reclaim := model.WSLAction{ID: model.WSLActionReclaimMemory, Command: Show(info.Distro, ReclaimCommand())}
	switch {
	case !info.Running:
		reclaim.Disabled = stopped
	case info.Version == 1:
		reclaim.Disabled = "WSL 1 has no separate Linux memory cache to drop."
	}

	docker := model.WSLAction{ID: model.WSLActionRestartDocker, Command: Show(info.Distro, RestartDockerCommand(systemd))}
	switch {
	case !info.Running:
		docker.Disabled = stopped
	case info.Docker == model.WSLDockerNotInstalled:
		docker.Disabled = "Docker Engine is not installed in this distribution."
	case info.Docker == model.WSLDockerDesktopIntegrated:
		docker.Disabled = "This distribution uses Docker Desktop's integration; restart Docker Desktop instead."
	case info.Docker == "":
		docker.Disabled = "The state of the Docker service could not be read."
	}

	distro := model.WSLAction{ID: model.WSLActionRestartDistro, Command: wsldocker.Display(TerminateCommand(info.Distro)) + ", then start it again"}
	if !info.Running {
		distro.Disabled = stopped
	}

	shutdown := model.WSLAction{ID: model.WSLActionShutdown, Command: wsldocker.Display(ShutdownCommand())}
	if !info.Running && len(info.OtherRunning) == 0 {
		shutdown.Disabled = "No distribution is running."
	}
	return []model.WSLAction{reclaim, docker, distro, shutdown}
}

// Action returns the entry with the given ID.
func Action(list []model.WSLAction, id string) (model.WSLAction, bool) {
	for _, a := range list {
		if strings.EqualFold(a.ID, id) {
			return a, true
		}
	}
	return model.WSLAction{}, false
}
