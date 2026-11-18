# Docklet

A desktop client for **existing** Docker Engines, built with Wails v3, Go, React, TypeScript and shadcn/ui. It does not install, bundle or replace Docker Engine, and it does not need Docker Desktop.

`DESIGN.md` is the source of truth for visuals and interaction; `docker-desktop-alternative-spec.md` for backend behaviour.

## Requirements

- Go 1.26+, Node 22+, and the Wails v3 CLI: `go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0`
- A reachable Docker Engine: local named pipe or Unix socket, a WSL2 distribution, TCP/TLS, or SSH
- Optional, per feature:
  - Docker CLI with the Compose v2 plugin — Compose Up/Down/Pull/Build, and "Open in external terminal"
  - `wsl.exe` — WSL2 connections
  - OpenSSH client — SSH connections
  - NSIS (`makensis`) — building the Windows installer

## Run, build, test

```
wails3 dev -config ./build/config.yml -port 9245   # development, hot reload
wails3 build                                       # production binary in bin/
wails3 package                                     # Windows installer (needs NSIS)
```

```
go vet . ./internal/... && go test ./internal/...
cd frontend && npx tsc --noEmit && npm test && npm run build
```

TypeScript bindings are generated from the Go services in interface mode (`wails3 generate bindings -clean=true -ts -i`). They are not committed; the build generates them.

## Features

**Connections**
- Named pipe, Unix socket, TCP with TLS (verification always on; plain TCP to a remote host needs an explicit opt-in), WSL2 (through `wsl.exe … docker system dial-stdio`, no TCP listener), SSH (system OpenSSH with key, agent, password or passphrase auth; host keys always verified), import of Docker CLI contexts.
- Test before save, friendly errors with redacted diagnostics.
- Profiles are stored in SQLite in the user config directory. SSH passwords and passphrases go to the OS credential store, never to the database.

**Containers**
- Compose-grouped and flat lists (grouping strictly by the `com.docker.compose.project` label), filters, sorting, selection with bulk actions, inspector.
- Create, start, stop, restart, pause, kill, remove, and **Edit & recreate** (new container created first; the original is restored if the new one fails to start).
- Detail page: live logs (regex search, level and time filters), interactive terminal (xterm.js), open in an external terminal, live stats, file browser (also for stopped containers), processes, inspect JSON with secrets masked.
- Published ports open in the browser when the host can be determined safely.

**Images**
- List, inspect, history, tag, remove, pull, push, build (classic builder), save and load, remove dangling.
- Manual update check ("update available") and an update flow that pulls and recreates the containers using the image.
- Registry credentials are read from the Docker CLI config and credential helpers; Docklet does not store them.

**Compose**
- Projects discovered from labels, plus projects opened from Compose files on disk.
- Up, Down, Start, Stop, Restart, Pull, Build and per-service actions when the Compose CLI can run where the project files live: this machine, or inside the WSL distribution. Container start/stop/restart is always available as plain Engine operations.
- A finished one-shot service (exit code 0, restart policy "no") does not mark a project as partially running.

**Volumes, networks, system**
- Volumes: list, inspect, create, remove, browse, export, import, guarded prune.
- Networks: list, inspect, create, remove, connect and disconnect containers, guarded prune.
- Engine info, Docker disk usage, recent events, cleanup with explicit confirmation.

**App**
- Background operations in an activity center with history, opt-in alerts (crash, out of memory, unhealthy, restart loop), command palette (Ctrl/Cmd+K), settings, dark and light themes in two palettes (Docklet and stock shadcn/ui).

## Known limitations

- Builds use the Engine's classic builder; BuildKit-only Dockerfile features such as `RUN --mount` do not work.
- There is no registry login UI: run `docker login` in a terminal.
- Compose is not supported for Docker context connections that are not SSH, and password-prompting SSH hosts do not work for Compose (key or agent auth is needed there).
- Recreate does not carry anonymous volumes over (the dialog warns), and Compose-managed containers are updated through Compose rather than recreated in place.
- Alerts only fire while Docklet is running and connected.
- Not built: Kubernetes, Swarm, a built-in Compose YAML editor, automatic container updates, vulnerability scanning, a tray icon, an app update check.
- Developed and tested on Windows with Docker running in WSL2. Linux and macOS builds are untested.
- Automated tests are unit tests against fakes; they do not need a Docker daemon.

## Layout

```
main.go                 app wiring: services, events, window
internal/model          DTOs shared with the frontend (source of the generated TS types)
internal/engine         connections and transports, one Docker SDK client per connection
internal/services       bound services: containers, images, volumes, networks, compose, files, exec, system
internal/compose        Compose discovery, argv construction, execution environment
internal/ops            background operation manager
internal/notify         alert rules and delivery
internal/appdb          SQLite storage and migrations
frontend/src            React app: pages, components, hooks, lib
```
