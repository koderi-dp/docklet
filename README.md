# Docklet

A desktop client for **existing** Docker Engines, built with Wails v3, Go, React, TypeScript and shadcn/ui. It does not install, bundle or replace Docker Engine, and it does not need Docker Desktop.

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
wails3 build VERSION=0.2.0                         # embeds the version (default: build/config.yml)
GOARCH=arm64 wails3 package                        # Windows on ARM64 (the architecture comes from GOARCH)
```

`docklet --version` prints the embedded version without starting the app.

```
cd frontend && npx tsc --noEmit && npm test && npm run build && cd ..
go vet . ./internal/... && go test ./internal/...
```

`main.go` embeds `frontend/dist`, so build the frontend before running Go commands on the main package.

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
- Optional system tray icon (Settings > Tray and background, off by default): click toggles the window; the menu switches engines, lists running containers with Stop, Restart and Open, and shows unread alerts. Optionally closing the window keeps Docklet running in the tray, and it can start hidden. Changes apply immediately, except "start hidden", which applies at the next start. Quit is in the tray menu.
- Optional update check (Settings > About and updates, "Never" by default): looks at the latest GitHub release on startup or weekly and only shows a small notice in the status bar with a link to the release notes. It never downloads or installs anything, sends no personal data, and "Check now" always works. A repository without published releases reports "No published releases found".

## Releases

Push a tag to publish a release: `git tag v0.2.0 && git push origin v0.2.0` (a tag with a hyphen, such as `v0.2.0-rc.1`, becomes a pre-release). `.github/workflows/release.yml` can also be started by hand with a version. It builds and attaches:

- `docklet-<version>-windows-amd64-installer.exe`, `docklet-<version>-windows-arm64-installer.exe` and the plain `.exe` files
- `docklet-<version>-linux-amd64.AppImage` and `.deb` (and arm64 when the ARM runner succeeds)
- `docklet-<version>-macos-arm64.zip` (optional: the release goes out without it if that job fails)
- `checksums.txt` (SHA-256 of every file)

Windows and Linux amd64 are required; nothing is published if they fail. Nothing is code signed yet (no certificate): Windows SmartScreen and macOS Gatekeeper will warn. The workflow marks where signing would go.

## Platforms

| | Built in CI | Tested in CI | Used interactively |
|---|---|---|---|
| Windows amd64 | yes | vet, unit tests | yes (Docker in WSL2) |
| Windows arm64 | yes (cross-compiled) | no, never run | no |
| Linux amd64 | yes (GTK 4 and WebKitGTK 6.0, Ubuntu 24.04) | vet, unit tests, and a smoke test: the app is started under Xvfb and must still be running after 15 s | no |
| macOS arm64 | yes, may fail without blocking | vet, unit tests | no |

On Linux, `libgtk-4-dev` and `libwebkitgtk-6.0-dev` are needed to build; a Secret Service (GNOME Keyring or KWallet) is needed to store SSH passwords. On Linux and macOS, well-known local Docker sockets (`/var/run/docker.sock`, rootless Docker, Docker Desktop, Colima, OrbStack) are offered when they exist.

## Known limitations

- Builds use the Engine's classic builder; BuildKit-only Dockerfile features such as `RUN --mount` do not work.
- There is no registry login UI: run `docker login` in a terminal.
- Compose is not supported for Docker context connections that are not SSH, and password-prompting SSH hosts do not work for Compose (key or agent auth is needed there).
- Recreate does not carry anonymous volumes over (the dialog warns), and Compose-managed containers are updated through Compose rather than recreated in place.
- Alerts only fire while Docklet is running and connected.
- Not built: Kubernetes, Swarm, a built-in Compose YAML editor, automatic container updates, vulnerability scanning, automatic app updates (the update check only notifies).
- Developed and used on Windows with Docker running in WSL2. Linux and macOS are built in CI (Linux is smoke-tested) but nobody has used them interactively; the tray icon and its menu have not been checked on Linux or macOS.
- Automated tests are unit tests against fakes; they do not need a Docker daemon.

## Layout

```
main.go                 entry point; app_services.go and app_adapters.go hold the wiring
internal/tray           system tray menu model (pure) and its Wails adapter
internal/update         release check and version comparison
internal/version        build-time version
internal/model          DTOs shared with the frontend (source of the generated TS types)
internal/engine         connections and transports, one Docker SDK client per connection
internal/services       bound services: containers, images, volumes, networks, compose, files, exec, system
internal/compose        Compose discovery, argv construction, execution environment
internal/ops            background operation manager
internal/notify         alert rules and delivery
internal/appdb          SQLite storage and migrations
frontend/src            React app: pages, components, hooks, lib
```

## License

MIT. See [LICENSE](LICENSE).
