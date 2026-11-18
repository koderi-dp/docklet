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

To bump the version, edit `version` in `build/config.yml` and run `wails3 task common:update:build-assets`. That task regenerates `build/linux/nfpm/nfpm.yaml` from its template, which drops the AppArmor profile entry: restore that file with `git checkout -- build/linux/nfpm/nfpm.yaml` afterwards. CI and the release workflow both fail if the `.deb` lacks the profile.

Nothing is code signed yet. The workflow has comments where signing would go.

## Platform status

| | Built in CI | Tested in CI | Used day to day |
|---|---|---|---|
| Windows x64 | yes | unit tests | yes, with Docker in WSL2 |
| Windows ARM64 | yes, cross-compiled | not run | installed and run on an ARM64 device (0.1.0) |
| Linux x64 | yes | unit tests, and the app is started under Xvfb | no |
| macOS ARM64 | yes, optional | unit tests | no |

On Linux a Secret Service (GNOME Keyring or KWallet) is needed to store SSH passwords. On Linux and macOS, well-known local Docker sockets (`/var/run/docker.sock`, rootless Docker, Docker Desktop, Colima, OrbStack) are offered when they exist.

## Linux sandbox

WebKitGTK 6.0 starts its helper processes inside bubblewrap, which needs unprivileged user namespaces. Ubuntu 24.04 restricts them (`kernel.apparmor_restrict_unprivileged_userns=1`): AppArmor lets the namespace be created but strips its capabilities, so `bwrap` fails with `setting up uid map: Permission denied` and WebKit aborts with SIGTRAP. CI runs on such a runner and does not relax the setting.

- The `.deb` installs `/etc/apparmor.d/docklet` (`build/linux/nfpm/docklet.apparmor`, an `unconfined` profile with `userns` for `/usr/bin/docklet`). `postinstall.sh` loads it when AppArmor is running and removes it if the parser rejects it (22.04 does not know `abi <abi/4.0>` and does not need it); `preremove.sh` unloads it. Child processes such as `bwrap` inherit the profile. Ubuntu 25.04+ also ships its own `bwrap-userns-restrict` profile.
- `internal/linuxsandbox` runs first in `main` and probes by running `bwrap --unshare-user` the way WebKit does. If user namespaces are denied it prints the fixes (the `.deb`, Ubuntu's `bwrap-userns-restrict` profile, a per-binary profile, the opt-out) and exits with status 78. A dev build or the AppImage hits this path.
- `DOCKLET_DISABLE_WEBKIT_SANDBOX=1` sets `WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS=1` for that run, only when the probe fails. It removes the isolation of the web view's helper processes. The view only loads the bundled UI (links open in the system browser, there are no iframes), so the exposure is a bug in WebKit or the UI rendering untrusted container output.

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
