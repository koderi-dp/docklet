# New Compose File and Compose autocompletion: design

Date: 2026-11-23. Status: waiting for approval.

Two parts that can be built and merged independently: Part 1, the New Compose File
dialog (everything up to "Part 2"), and Part 2, autocompletion in the Compose file editor.

## Intent

The Compose page can register a Compose file that already exists (Open Compose File) but
cannot create one. Someone who has containers started by hand, or only an idea, has to
leave Docklet, create a file, and come back.

This change adds **New Compose File**: a short dialog that writes a valid starting file,
saves it as a project and opens it in the existing YAML editor.

Decided by the user:

- The user works in YAML. There is no form builder for services.
- Approach A below.
- A new file may take the name of a project that exists only as running containers, with
  a warning (see "Taking over a running project").
- Autocompletion is part of this work and is schema-driven (Part 2).

Everything else in this document is a proposal made while writing it, marked "my call"
where it is a real choice.

## Goals

- From the Compose page to an open, valid, editable Compose file in one dialog.
- The file never starts empty: it comes from running containers, a template, or a minimal
  commented example.
- Works wherever the Compose file editor works today: local and WSL.

## Non-goals

- A form or visual builder for services.
- Writing a `.env` file for secret placeholders.
- Inferring `depends_on` between converted containers.
- Starting the project from the dialog.
- Moving containers into a project they do not already belong to. (Giving a running
  project its file back under its own name is allowed; see below.)

## Anti-goals

- Never overwrite or modify a file that already exists.
- Never write secret values into the file unless the user ticks the box that says so.
- Never leave a half-made result: a file without a project, or a project without a file.
- Never start, stop or change a container.

## Approaches

**A. Create the file, then edit it (chosen).** A dialog collects folder, name and starting
point; Docklet writes `compose.yaml`, saves the project and opens its Compose file tab. It
reuses the editor, outline, validation and Apply unchanged. The cost is that a file exists
on disk before the user has edited it; the starting content being valid makes that
acceptable.

**B. Draft first, choose the location on first save.** Nothing touches disk until Save.
Rejected: the editor, the outline and Apply all assume a saved project that owns files, so
a project-less draft means changing each of them and handling lost drafts.

**C. A with an empty skeleton only.** Smallest change. Rejected: a blank file is the hard
part, and the container-to-Compose conversion that already exists would stay unused.

## What the user sees

A **New Compose File** button in the Compose page header, left of Open Compose File, and
in the empty state. As in Open Compose File, the dialog always opens and its Create
button is disabled with the reason (Compose not available for the connection, application
database not open).

```
New Compose File
  Compose runs on: This machine

  Folder        [ C:\apps\shop                         ] [Browse]
                compose.yaml will be created here.
  Project name  [ shop                                 ]

  Start from    (•) Minimal example
                ( ) Template            [ Web app + PostgreSQL ▾ ]
                ( ) Running containers
                      [x] web      nginx:1.27
                      [x] db       postgres:16
                      [ ] cache    redis:7
                    [ ] Include secret values

  Preview
  ┌────────────────────────────────────────────────────────┐
  │ name: shop                                             │
  │ services:                                              │
  │   web:                                                 │
  │     image: nginx:1.27                                  │
  │   ...                                                  │
  └────────────────────────────────────────────────────────┘
  Not included: auto-remove (--rm) on web

                                 [Cancel]  [Create and open]
```

- **Folder**: typed or picked with the native folder dialog. The same location box and
  path note as Open Compose File (for WSL: a Linux path or a `\\wsl$\<distro>\...` pick).
  A folder that does not exist is created (my call).
- **Project name**: prefilled from the folder name, normalised the way Compose does it,
  until the user edits it. Validated with the existing project-name rules.
- **Start from**: three choices, Minimal selected first.
- **Preview**: read-only, updates as the choices change. It is exactly the text that will
  be written. Notes from the conversion ("not included") show under it.
- **Create and open**: writes the file, saves the project, closes the dialog and opens the
  project page on its Compose file tab. Nothing is started.

Errors show inline in the dialog, as in Open Compose File. Two get their own wording:

- The folder already holds `compose.yaml`, `compose.yml`, `docker-compose.yaml` or
  `docker-compose.yml`: "This folder already has a Compose file." with an **Open it
  instead** button that switches to Open Compose File with that path filled in.
- A saved project of that name already exists on this connection: say so and keep the
  dialog open so the name can be changed.

### Taking over a running project

Docklet lists two kinds of projects: saved ones (a record with files) and ones discovered
only from the labels of running containers. A saved record attaches to a discovered
project of the same name (`compose.Merge`).

When the typed name matches a discovered project that has no saved record, creation is
allowed. As soon as the name matches, the dialog shows a warning above the buttons:

> A project named "shop" is already running (3 containers) without a Compose file in
> Docklet. This file will become its Compose file: Up and Apply will then change those
> containers.

The button reads **Create and take over**. In this case the dialog also preselects
Running containers with that project's containers ticked (my call), since restoring a
lost file is the likely reason. Creating still starts and changes nothing by itself.

The name check runs in the dialog from the project list the page already holds, and again
on the server in `CreateProject`.

When the source is running containers, a note under the list says the original containers
keep running and that Up will clash with their names and ports until they are stopped or
removed.

## Starting content

All three produce a file that begins with `name: <project name>` (my call: the file then
gives the same project name when used outside Docklet).

- **Minimal example**: one service with an image and a published port, and short comments
  pointing at the usual next keys (environment, volumes, depends_on).
- **Template** (my call, easy to cut): three small stacks embedded in the Go binary.
  1. Web app + PostgreSQL
  2. Web app + PostgreSQL + Redis
  3. Static site (nginx with a bind-mounted folder)

  Each uses a named volume for data, a healthcheck on the database, and `${VAR}`
  placeholders for passwords. The exact YAML is the builder's.
- **Running containers**: every container on the connection is listed, running ones
  first, with image and, when it has one, its Compose project. Ticked containers become
  services through the existing conversion (`runspec.FromInspect` then Compose
  rendering), merged into one file:
  - service names are made unique when two containers would get the same one;
  - top-level `volumes` and `networks` are merged, each key once;
  - secret-looking environment values become `${VAR}` placeholders unless **Include
    secret values** is ticked, as in Copy as;
  - the "not included" notes of all containers are returned for the dialog and also kept
    as a trailing comment in the file, as the single-container snippet does today.

  Rules added after review (my calls):

  - **Existing data is kept.** A volume or network of a ticked container is declared
    project-local only when the container already belongs to a Compose project with the
    same name as the new one. In every other case the file points at the existing
    resource (`name:` on the volume, `external: true` on the network), so a database
    does not come up empty under a new project name. `container_name` follows the same
    rule: written unless the container's project equals the new name.
  - **Clashing keys.** When two ticked containers would produce the same top-level key
    for different volumes or networks, the keys are made unique; the builder picks the
    scheme.
  - **Dangling references.** `links`, `volumes_from` and `network_mode: container:`
    entries that point at a container not among the ticked ones are left out and listed
    under "not included", so the file validates on its own. References between ticked
    containers are rewritten to the service names.
  - **Secret placeholders.** When two services have the same secret key, each gets its
    own variable name (service name as prefix), so two different passwords never share
    one placeholder.
  - If validation still fails, the dialog stays open with the CLI's message and nothing
    is left on disk.

Templates are planned as the last, detachable task: the `source` shape and the dialog
layout allow for them from the start, and cutting them removes only the selector, the
embedded files and their tests.

## How it is built

Backend, in `ComposeService` (new file, e.g. `compose_create.go`):

- `NewFilePreview(connectionID, name, source) -> {text, secretKeys, notIncluded}`.
  `source` is `{kind: "minimal" | "template" | "containers", template, containerIDs,
  includeSecrets}`. It reads the engine only for the containers kind and writes nothing.
- `CreateProject(connectionID, dir, name, text) -> ComposeProject`. In order:
  1. the same environment, store and CLI checks as `AddProject`;
  2. normalise `dir` with the rules `normalizeFiles` uses for the connection kind;
  3. validate the name; refuse a name that already has a saved record on this connection
     (today `SaveComposeProject` upserts, so this check is new). A name that matches only
     a discovered project is accepted;
  4. refuse when one of the four Compose file names exists in `dir`;
  5. create `dir` when missing, then create `compose.yaml` so that an existing file is
     never replaced (exclusive create; in WSL the equivalent with `wsl.exe --exec`);
  6. run `docker compose config -q` on it; on failure delete the file just created (and
     the folder, if this call created it) and return the error;
  7. save the project record; on failure delete the file the same way;
  8. return the project as `AddProject` does.

  The text comes from the dialog, so the server treats it as untrusted: size-limited to
  `compose.MaxEditableFile` and accepted only when step 6 passes.

- `compose` package: a create-file operation next to `FileIO.WriteFile` for `LocalFS`
  and `WSLFS` (today's `WriteFile` requires the file to exist), plus make-directory and
  remove for the cleanup. Line endings: the platform default of the execution location
  (`\n` in WSL; my call for local Windows: `\n` too, the editor keeps a file's style).
- `runspec` package: render several specs into one Compose document. `ToCompose` for a
  single spec keeps its current output.
- Templates: text files embedded with `go:embed`.

Frontend:

- `NewComposeDialog.tsx` beside `OpenComposeDialog.tsx`. The location box and the path
  row are shared with it rather than copied; `FileRow` gains a folder mode that calls
  `DialogService.PickDirectory`.
- The preview uses the existing `YamlEditor` in read-only mode.
- The container list comes from the containers query the page already holds.
- After creation, navigate to the project page with the Compose file tab selected. The
  tab is local state today (`useState("structure")` in `ComposeProjectPage.tsx`), so it
  needs a way to be chosen from the link (router state or a query parameter).

## Constraints

- Project rules in `docs/pipeline/rules.md`: files about 500 lines, split by
  responsibility; unit tests that check real behaviour; no mocked data shipped; UI checked
  in both themes and in the running app; live-engine tests only on `docklet-test-*`
  resources.
- File access goes through the connection's execution environment only (local or WSL).
  For other kinds the button is disabled with the environment's reason.

## Tests

- Go: creation refuses an existing file and each of the four names; cleanup after a
  failed validation and after a failed save; duplicate project name; WSL path rules;
  multi-container rendering (name clashes, merged volumes and networks, secrets on and
  off).
- Frontend: the dialog's blocked reasons, name prefill from the folder, preview updates,
  the "Open it instead" hand-off.
- e2e: create from the minimal example and land on the Compose file tab, next to the
  existing `e2e/tests/compose.spec.ts`.

## Part 2: Compose autocompletion

### Intent

The Compose file editor (`YamlEditor.tsx`, CodeMirror 6) highlights YAML but knows
nothing about Compose. Someone writing YAML by hand has to remember every key. The
editor's standard setup already contains the completion popup; this part feeds it.

It applies to every Compose config file opened in the editor, not only new ones.

### Approaches

**A. Schema-driven (chosen).** Bundle the official Compose specification JSON Schema and
suggest from it by cursor position. Complete, carries the schema's descriptions, and
staying current means replacing one file.

**B. Hand-written key table.** Smallest, but incomplete from the first day, drifts as
Compose changes, and has no descriptions unless written by hand.

**C. A YAML language server in the editor.** Completion, hover and validation at once,
but heavy to bundle in a desktop app and it duplicates the validation Docklet already
does with `docker compose config`.

### What the user sees

- Typing a key, or pressing Ctrl+Space, opens a list of the keys valid at that position:
  top level (`services`, `volumes`, `networks`, ...), inside a service (`image`, `ports`,
  `healthcheck`, ...), inside nested maps (`healthcheck` gives `test`, `interval`, ...).
- Keys already present in the same map are not offered again.
- Where the schema lists fixed values they are offered after the colon, for example
  `restart:` gives `no`, `always`, `on-failure`, `unless-stopped`.
- Each entry shows the schema's description when it has one.
- Suggestions that come from the project itself:
  - service names of the file under `depends_on`, `links`, `volumes_from`,
    `network_mode: service:` and `extends.service`;
  - top-level volume and network keys of the file where a service references them;
  - image names (with tags) on the connection's engine after `image:`.
- Enter accepts a suggestion. Tab keeps inserting indentation as it does today (my call).
- Read-only views (env files, the New dialog's preview) get no completion.

### How it is built

- The schema: `compose-spec.json` from the compose-spec repository, vendored under the
  frontend with a note of the version it was taken from. Nothing is fetched at runtime.
- A completion source for CodeMirror in its own module, in three pieces kept apart so
  each is unit-testable without an editor:
  1. cursor to path: from the YAML syntax tree, the list of keys leading to the cursor
     and whether the cursor is at a key or a value;
  2. path to schema node: follows `$ref`, `oneOf`/`anyOf`, `properties`,
     `patternProperties` (service, volume and network names) and `additionalProperties`;
  3. project values: service, volume and network names read from the current document;
     image names from the images query the app already has for the connection.
- Reading the schema (piece 2): an existing library may be used when it works with the
  pinned CodeMirror versions and does not bring its own validation into the editor;
  otherwise a small reader is written. The builder decides after checking (my call to
  leave it there: the outcome for the user is the same).
- `YamlEditor` gains an optional prop carrying the completion extension, so the editor
  stays a generic YAML editor and the Compose knowledge lives beside it.
- In a project with several Compose files, names are taken from the open file only (my
  call).

### Non-goals

- Validation or error marking from the schema: `docker compose config` stays the only
  judge of whether a file is valid.
- Hover documentation.
- Multi-line snippets (a whole service block).
- Completion of environment variable names or of paths on disk.

### Tests

- Unit: cursor-to-path on realistic documents, including a half-typed key, a list item
  and a broken document; schema lookup for top level, service, nested maps, pattern
  keys and enums; keys already present are filtered; project values.
- e2e or in-app check: type inside a service in the running app and accept a suggestion,
  in both themes.

## Left to the builder

- Part 2: the library-or-own-reader choice, what text a suggestion inserts after the key
  (colon, space, new line), and how the list is ordered.
- The exact YAML of the minimal example and the templates.
- How the Compose file tab is selected on navigation.
- How unique service names are derived.
- Component split and naming inside the dialog.
- Debouncing of the preview call.
