# Developing Docklet

Docklet is built with Wails v3, Go, React, TypeScript, Tailwind and shadcn/ui.

## Prerequisites

- Go 1.26+, Node 22+, and the Wails v3 CLI: `go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0`
- Windows installer: NSIS (`makensis`)
- Linux: `build-essential pkg-config libgtk-4-dev libwebkitgtk-6.0-dev`

## Run and build

```
wails3 dev -config ./build/config.yml -port 9245   # development, hot reload
wails3 build                                       # production binary in bin/
wails3 package                                     # installer or package for this OS
wails3 build VERSION=0.2.0                         # embed a version (default: build/config.yml)
GOARCH=arm64 wails3 package                        # Windows on ARM64 (architecture comes from GOARCH)
```

`docklet --version` prints the embedded version without starting the app.

## Tests

```
cd frontend && npx tsc --noEmit && npm test && npm run build && cd ..
go vet . ./internal/... && go test ./internal/...
```

- `main.go` embeds `frontend/dist`, so build the frontend before running Go commands on the main package.
- TypeScript bindings are generated from the Go services (`wails3 generate bindings -clean=true -ts -i`). They are not committed; the build generates them.
- Tests are unit tests against fakes and do not need a Docker daemon.

## Releases

Push a version tag to publish a release:

```
git tag v0.2.0
git push origin v0.2.0
```

A tag with a hyphen, such as `v0.2.0-rc.1`, becomes a pre-release. The release workflow can also be started by hand with a version.

It attaches Windows installers and binaries (x64 and ARM64), Linux AppImage and `.deb`, a macOS `.zip`, and `checksums.txt`. Only the Windows build is required; Linux and macOS files are included when their builds succeed.

Nothing is code signed yet. The workflow has comments where signing would go.

## Platform status

| | Built in CI | Tested in CI | Used day to day |
|---|---|---|---|
| Windows x64 | yes | unit tests | yes, with Docker in WSL2 |
| Windows ARM64 | yes, cross-compiled | not run | no |
| Linux x64 | yes | unit tests, and the app is started under Xvfb | no |
| macOS ARM64 | yes, optional | unit tests | no |

On Linux a Secret Service (GNOME Keyring or KWallet) is needed to store SSH passwords. On Linux and macOS, well-known local Docker sockets (`/var/run/docker.sock`, rootless Docker, Docker Desktop, Colima, OrbStack) are offered when they exist.

## Known limitations

- Image builds use the Engine's classic builder; BuildKit-only Dockerfile features such as `RUN --mount` do not work.
- There is no registry login screen: run `docker login` in a terminal. Credentials are read from the Docker CLI config and credential helpers.
- Compose is not supported for Docker context connections that are not SSH, and SSH hosts that prompt for a password do not work for Compose (use key or agent auth).
- Recreate does not carry anonymous volumes over (the dialog warns). Compose-managed containers are updated through Compose, not recreated in place.
- Alerts only fire while Docklet is running and connected.
- The update check only notifies. It reads the latest GitHub release without credentials, so it finds nothing while releases are not public.

## Project layout

```
main.go                 entry point; app_services.go and app_adapters.go hold the wiring
internal/model          DTOs shared with the frontend (source of the generated TS types)
internal/engine         connections and transports, one Docker SDK client per connection
internal/services       bound services: containers, images, volumes, networks, compose, files, exec, system
internal/compose        Compose discovery, command construction, execution environment
internal/ops            background operation manager
internal/notify         alert rules and delivery
internal/tray           system tray menu model and its Wails adapter
internal/update         release check and version comparison
internal/appdb          SQLite storage and migrations
frontend/src            React app: pages, components, hooks, lib
```

## Design rules worth knowing

- Go owns all Docker communication, credentials and process execution; React is presentation only.
- Every service call names its connection explicitly. Data from different engines is never mixed.
- Compose projects are grouped only by the `com.docker.compose.project` label, never by name.
- Health is never inferred from "running". Unknown values show as "—", not zero.
- Secret-like environment values are masked in Go before they reach the UI.
- Destructive actions need an explicit confirmation that lists what will be affected.
