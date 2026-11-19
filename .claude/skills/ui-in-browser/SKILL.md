---
name: ui-in-browser
description: Use when a UI change in Docklet has to be seen, clicked through or screenshotted and the desktop window is not needed - runs the real Go backend in Wails server mode and the React UI in a normal browser with hot reload. Use it before claiming a frontend change works.
---

# Check the Docklet UI in a browser

`wails3 task dev:browser` runs Docklet without the desktop window: the Go backend in Wails server mode and
Vite with hot reload. The same React code talks to the same Go services, so what you see is real data from
the user's Docker engines.

## Start

Run it as a background command from the repository root:

```
wails3 task dev:browser
```

It builds the frontend and the server binary first (about 20 seconds), then serves:

- UI: `http://localhost:9250` - open this one
- backend: `http://localhost:9251` (`/health` answers `{"status":"ok"}` when it is up)

Wait until both answer before opening the page. Another port: `wails3 task dev:browser BROWSER_PORT=9260 BROWSER_BACKEND_PORT=9261`.
The ports differ from `wails3 dev` (9245), so both can run at once. If a port is taken the backend exits with
"bind: Only one usage of each socket address" while Vite keeps running: stop it and pick other ports.

## Use

- Open `http://localhost:9250` in the browser pane and drive it there.
- Edits under `frontend/src` show up at once (hot reload). Go changes need the task restarted.
- A console warning "Browser Environment Detected" is normal.

## Rules

- **It is the real backend.** Stop, Remove, Prune, Down and file edits act on the user's real containers,
  images and volumes. Look and navigate freely; ask before any action that changes or deletes something.
- **No authentication.** Anything on this machine that reaches the port controls Docker through it. It binds
  to localhost only: never set `WAILS_SERVER_HOST` to another address, and stop the task when done.
- **Stop it properly.** Stopping the task can leave `node` (Vite) or `docklet-server` running. Afterwards check
  that nothing listens on 9250 and 9251 and stop what does. Touch only processes you started.

## What it cannot show

Native parts do not exist in a browser: file and folder dialogs, the tray icon and its menu, window
close-to-tray, OS notifications, opening an external terminal. Check those in the desktop app (`wails3 dev`).
It also proves nothing about WebView2 or WebKitGTK rendering; for a release-relevant visual change, look at
the desktop app too.

## How it works

`frontend/vite.config.ts` forwards everything under `/wails` (runtime calls, the event WebSocket, streams) to
the backend when `DOCKLET_SERVER_URL` is set; the task sets it. The tasks are `dev:browser`,
`dev:browser:backend` and `dev:browser:frontend` in `Taskfile.yml`.
