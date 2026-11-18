# Docklet

A desktop client for **existing** Docker Engines, built with Wails v3, Go, React, TypeScript and shadcn/ui. It does not install, bundle or replace Docker Engine and does not need Docker Desktop.

`DESIGN.md` is the source of truth for visuals and interaction; `docker-desktop-alternative-spec.md` for backend behaviour.

## Requirements

- Go 1.26+, Node 20+, and the Wails v3 CLI: `go install github.com/wailsapp/wails/v3/cmd/wails3@latest`
- A reachable Docker Engine (local named pipe / Unix socket, WSL2 distribution, TCP/TLS, or SSH)
- Optional: the Docker CLI with the Compose v2 plugin (needed only for Compose Up/Down/Pull/Build), `wsl.exe` for WSL2 connections, OpenSSH client for SSH connections

## Run and build

```
wails3 dev -config ./build/config.yml -port 9245   # development, hot reload
wails3 build                                       # production binary in bin/
wails3 package                                     # platform installer
```

Checks:

```
go vet . ./internal/... && go test ./internal/...
cd frontend && npx tsc --noEmit && npm test && npm run build
```

Bindings are generated in interface mode (`wails3 generate bindings -clean=true -ts -i`); the build does this automatically.

## What works

- **Connections:** named pipe, Unix socket, TCP with TLS (verification on; plain TCP to a remote host needs an explicit opt-in), WSL2 (through `wsl.exe … docker system dial-stdio`, no TCP listener), SSH (system OpenSSH, key/agent auth, host-key checking), import of Docker CLI contexts. Test before save, diagnostics with redaction. Profiles are stored in the user config directory as JSON without secrets.
- **Containers:** Compose-grouped and flat list (grouping strictly by the `com.docker.compose.project` label), filters, selection and bulk actions, inspector, create, start/stop/restart/pause/kill/remove, detail page with live logs, interactive terminal (xterm.js), live stats, processes and masked inspect JSON.
- **Images:** list, inspect, history, tag, remove, pull with background progress, remove dangling.
- **Volumes and networks:** list, inspect, create, remove, connect/disconnect, guarded prune.
- **Compose:** projects discovered from labels; Up/Down/Restart/Pull/Build only when the Compose CLI can run where the project files live (locally, or inside the WSL distribution); container start/stop/restart always available as Engine operations.
- **System and dashboard:** engine info, Docker disk usage, recent events, cleanup with explicit confirmation.
- **Shell:** activity center for background operations, command palette (Ctrl/Cmd+K), settings, light/dark/system theme.

## Known limitations

- Not implemented yet (shown disabled with a reason): image build, push, save and load; container file browser; volume browse/export; opening a Compose file that is not already running; SSH password authentication; Compose over SSH connections.
- Image pull sends no registry credentials, so private registries fail.
- Connection profiles use a JSON file, not SQLite; there is no OS credential-store integration because no secrets are stored.
- Packaging and smoke testing have only been done on Windows. SSH and remote TLS connections are covered by mocked tests only.
- Tests are deliberately limited to critical mocked unit tests (no real daemon needed).
