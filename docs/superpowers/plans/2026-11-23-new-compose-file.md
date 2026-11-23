# New Compose File Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A "New Compose File" dialog on the Compose page that writes a valid starting `compose.yaml`, saves it as a project and opens it in the existing editor.

**Architecture:** The Go side gains file creation in the execution environment (local and WSL), a renderer that turns several containers into one Compose document, and two `ComposeService` calls: a preview that writes nothing and a create that writes the file, validates it and saves the project record, undoing itself on failure. The React side gains one dialog that shares its location box and path field with Open Compose File and shows the preview in the existing read-only YAML editor.

**Tech Stack:** Go 1.26, `gopkg.in/yaml.v3`, Wails v3 bindings, React + TypeScript, TanStack Query, CodeMirror 6, Vitest, Playwright.

**Spec:** `docs/superpowers/specs/2026-11-23-new-compose-file-design.md` (Part 1). Read it before any task.

## Global Constraints

- The file is always named `compose.yaml` and is written with `\n` line endings.
- The four names that count as "this folder already has a Compose file": `compose.yaml`, `compose.yml`, `docker-compose.yaml`, `docker-compose.yml`.
- Never overwrite or modify an existing file. Never start, stop or change a container.
- Secret values are written only when `IncludeSecrets` is true; otherwise `${VAR}` placeholders.
- A failed creation leaves nothing behind: no file, no folder this call created, no project record.
- Files stay near 500 lines, split by responsibility. Unit tests check real behaviour against fakes; no mocked data ships.
- Commit as the repository's git user, with no Co-Authored-By or other attribution line. Never add `DESIGN.md`, `docker-desktop-alternative-spec.md`, `design-references/` or `docs/ROADMAP.md` to git.
- Live-engine checks touch only `docklet-test-*` resources. Leave the `catalo` containers, the `Ubuntu-24.04` WSL distribution and the Podman machine alone; never run `wsl --shutdown`.
- UI must be aligned and readable in the dark and the light theme, and is checked in the running app, not only by tests.
- Test commands: Go `go vet ./internal/... && go test ./internal/...`; frontend `cd frontend && npx tsc --noEmit && npm test`. TypeScript bindings are generated, not committed: after changing a bound Go method or model, run `wails3 generate bindings -clean=true -ts -i`.

## Review Focus

1. **Folder field holds a file path, a relative path, or a Windows drive path on a WSL connection.** Expected: an inline error in the dialog's words for that connection, nothing written. Pinned in Task 4.
2. **The folder is not writable (permission denied, read-only share).** Expected: the OS error inline, no folder left that this call created. Pinned in Task 4.
3. **"Running containers" selected with nothing ticked.** Expected: Create is disabled with a reason and no preview call is made. Pinned in Task 6.
4. **A ticked container is removed between preview and create, or inspect fails for one of several.** Expected: the preview shows the error for that container by name and Create is disabled; the others are not silently dropped. Pinned in Task 3.
5. **Folder name that normalises to an empty project name (for example `C:\_` or `/srv/!!!`).** Expected: the name field stays empty with the naming rule shown, Create disabled. Pinned in Task 5.

---

### Task 1: Create files and folders where Compose runs

**Files:**
- Create: `internal/compose/filecreate.go`
- Test: `internal/compose/filecreate_test.go`

**Interfaces:**
- Produces:
  ```go
  // ComposeFileNames are the file names Compose looks for in a folder.
  var ComposeFileNames = []string{"compose.yaml", "compose.yml", "docker-compose.yaml", "docker-compose.yml"}
  var ErrFileExists = errors.New("file already exists")

  // FileCreator makes new files where the Compose CLI sees them.
  type FileCreator interface {
      MakeDir(ctx context.Context, dir string) (created bool, err error) // like mkdir -p; created is false when dir existed
      CreateFile(ctx context.Context, path string, data []byte) error    // ErrFileExists when path exists; never replaces
      Remove(ctx context.Context, path string) error                     // one file, or one empty folder
  }
  func JoinPath(kind Kind, dir, name string) string // path.Join for KindWSL, filepath.Join otherwise
  ```
  Both `LocalFS` and `WSLFS` implement `FileCreator`.

- [ ] **Step 1: Write the failing tests** in `filecreate_test.go`, following the fake-runner style of `fileio_test.go` for WSL.

  ```go
  func TestLocalCreateFile_NeverReplaces(t *testing.T)        // create ok, content equals data; second call returns ErrFileExists (errors.Is) and content is unchanged
  func TestLocalMakeDir_ReportsWhetherItCreated(t *testing.T) // nested missing dir -> created true; again -> created false; a path that is a file -> error
  func TestLocalRemove_FileAndEmptyDirOnly(t *testing.T)      // removes file; removes empty dir; non-empty dir -> error and dir still there
  func TestWSLCreateFile_UsesNoclobber(t *testing.T)          // argv is: wsl.exe -d Ubuntu --exec sh -c `set -C; cat > "$1"` sh /p/compose.yaml, stdin == data
  func TestWSLCreateFile_ExistingFile(t *testing.T)           // runner: `test -e` succeeds -> ErrFileExists and no sh call
  func TestWSLMakeDirAndRemove(t *testing.T)                  // MakeDir: `test -d` fails then `mkdir -p -- /p` -> created true; Remove on a dir uses `rmdir --`, on a file `rm -f --`
  func TestWSLCreate_RefusesBadInput(t *testing.T)            // relative path, path starting with '-', invalid distro -> error, runner never called
  func TestJoinPath(t *testing.T)                             // KindWSL: "/a/b"+"compose.yaml" -> "/a/b/compose.yaml"; KindLocal uses filepath.Join
  ```

- [ ] **Step 2: Run** `go test ./internal/compose/ -run "Create|MakeDir|Remove|JoinPath"`. Expected: FAIL, undefined names.

- [ ] **Step 3: Implement.** Local: `os.MkdirAll` (0o755), `os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)`, `os.Remove`. WSL: reuse `w.check` and `w.run`; existence is checked with `test -e` first so the caller gets `ErrFileExists`, and `set -C` still guards the race. `Remove` asks `test -d` to choose `rmdir` or `rm -f`.

- [ ] **Step 4: Run** `go vet ./internal/compose/ && go test ./internal/compose/`. Expected: PASS.

- [ ] **Step 5: Commit** `git commit -m "Compose: create files and folders where the CLI runs"`.

---

### Task 2: Render several containers as one Compose document

**Files:**
- Create: `internal/runspec/composefile.go`
- Modify: `internal/runspec/compose.go` (extract the per-service part of `ToCompose` so both callers use it; `ToCompose` output must not change)
- Test: `internal/runspec/composefile_test.go`

**Interfaces:**
- Consumes: `Spec`, `Options`, `secretVarName` (existing).
- Produces:
  ```go
  // ComposeFile is a whole Compose document made from containers.
  type ComposeFile struct {
      Text        string
      NotIncluded []string // each entry prefixed "<service>: "
      SecretKeys  []string // "<service>.<KEY>", only keys that were replaced by a placeholder or would be
  }
  // ToComposeFile renders specs as one document for a project named project.
  func ToComposeFile(project string, specs []Spec, o Options) ComposeFile
  ```

Rules the tests pin (from the spec's "Starting content"):

- The document starts with `name: <project>`, then `services`, then `volumes` and `networks` when needed.
- Service name: `Spec.serviceName()`; a repeat gets `-2`, `-3`, ...
- A spec whose `ComposeProject` differs from `project` is rendered as if it had no project: `container_name` is written, volumes get `name: <existing volume>`, networks get `external: true`. A spec whose `ComposeProject` equals `project` keeps today's project-local mapping.
- Two different volumes or networks that would get the same top-level key: the later one gets `-2`, and that service's reference uses the new key.
- `Links`, `VolumesFrom` and `NetworkMode: container:<name>` are matched against the other specs by `Spec.Name`. A match is rewritten to the service name (`network_mode: service:<svc>`); no match is dropped and reported in `NotIncluded` as `<service>: <setting> <target> (not in this file)`.
- Secret placeholder: `${` + `secretVarName(KEY)` + `}`. When the same secret key appears in more than one service, each uses `secretVarName(<SERVICE>_<KEY>)` with the service name upper-cased and non-alphanumerics as `_`.
- The trailing `# not included: ...` comment lists every `NotIncluded` entry.

- [ ] **Step 1: Write the failing tests.**

  ```go
  func TestToComposeFile_StartsWithNameAndMergesServices(t *testing.T)
  // specs web(nginx:1.27) + db(postgres:16), project "shop": Text has prefix "name: shop\nservices:\n", contains "  web:" and "  db:", parses as YAML
  func TestToComposeFile_UniqueServiceNames(t *testing.T)
  // two specs Name "app" -> services "app" and "app-2"
  func TestToComposeFile_KeepsExistingDataForForeignProject(t *testing.T)
  // spec{ComposeProject:"old", ComposeService:"db", Name:"old-db-1", volume old_data:/var/lib/postgresql/data, network old_default}, project "shop":
  // top-level volumes.old_data.name == "old_data"; networks.old_default.external == true; service has container_name "old-db-1"
  func TestToComposeFile_SameProjectStaysLocal(t *testing.T)
  // same spec, project "old": volumes key "data" with no name; no container_name; no top-level networks
  func TestToComposeFile_ClashingVolumeKeys(t *testing.T)
  // spec A in project "shop" with volume shop_data, spec B standalone with volume "data", project "shop":
  // keys "data" (no name) and "data-2" (name: data); B's mount is "data-2:/x"
  func TestToComposeFile_ReferencesBetweenTickedAndDangling(t *testing.T)
  // web links ["db:database","ghost"], volumes_from ["db"], sidecar NetworkMode "container:web":
  // web.links == ["db:database"], web.volumes_from == ["db"], sidecar.network_mode == "service:web",
  // NotIncluded contains "web: link ghost (not in this file)"
  func TestToComposeFile_SecretPlaceholders(t *testing.T)
  // one service with secret POSTGRES_PASSWORD -> "${POSTGRES_PASSWORD}"; services "api" and "db-main" both with secret PASSWORD -> "${API_PASSWORD}" and "${DB_MAIN_PASSWORD}";
  // IncludeSecrets true -> literal values, dollars doubled
  func TestToComposeFile_NotIncludedCommentAndPrefix(t *testing.T)
  // spec with AutoRemove -> NotIncluded == ["web: auto-remove (--rm)"], Text ends with "# not included: web: auto-remove (--rm)\n"
  func TestToComposeFile_NoSpecs(t *testing.T)
  // Text == "name: shop\nservices: {}\n"
  ```
  Also run the existing `compose_test.go` unchanged as the guard for `ToCompose`.

- [ ] **Step 2: Run** `go test ./internal/runspec/ -run ToComposeFile`. Expected: FAIL, undefined.

- [ ] **Step 3: Implement `ToComposeFile`.** Extract from `ToCompose` a function that builds one service node and returns its top-level volume and network definitions; `ToComposeFile` calls it per spec (after clearing `ComposeProject` on foreign specs and rewriting references), then merges the top-level maps with the clash rule.

- [ ] **Step 4: Run** `go vet ./internal/runspec/ && go test ./internal/runspec/`. Expected: PASS, including the untouched `ToCompose` tests.

- [ ] **Step 5: Commit** `git commit -m "runspec: render several containers as one Compose file"`.

---

### Task 3: Preview of the starting content

**Files:**
- Create: `internal/compose/starter/starter.go`, `internal/compose/starter/minimal.yaml`
- Create: `internal/model/compose_new.go`
- Create: `internal/services/compose_new.go`
- Modify: `internal/services/copyas.go` (move the inspect-to-`Spec` step into a package-level function both services call)
- Test: `internal/compose/starter/starter_test.go`, `internal/services/compose_new_test.go`

**Interfaces:**
- Consumes: `runspec.ToComposeFile` (Task 2), `runspec.FromInspect`, `ComposeService.client`.
- Produces:
  ```go
  // model
  type ComposeNewSource struct {
      Kind           string   `json:"kind"` // "minimal" | "template" | "containers"
      Template       string   `json:"template,omitempty"`
      ContainerIDs   []string `json:"containerIds"`
      IncludeSecrets bool     `json:"includeSecrets"`
  }
  type ComposeNewPreview struct {
      Text        string   `json:"text"`
      SecretKeys  []string `json:"secretKeys"`
      NotIncluded []string `json:"notIncluded"`
  }
  // starter
  func Minimal(project string) string // "name: <project>\n" + minimal.yaml
  // services
  func (s *ComposeService) NewFilePreview(ctx context.Context, connectionID, name string, src model.ComposeNewSource) (model.ComposeNewPreview, error)
  ```

`minimal.yaml`: one service `app` with `image: nginx:alpine` and `ports: ["8080:80"]`, plus comment lines naming `environment`, `volumes` and `depends_on` as the usual next keys. Its exact wording is the implementer's; it must pass `docker compose config -q`.

Behaviour: `name` must pass `compose.ValidateProjectName`. Kind `minimal` reads nothing from the engine. Kind `containers` inspects each id in the order given; an inspect failure returns an error that names the container id; an empty id list returns an error "choose at least one container". When a spec's `NetworkMode` is `container:<id>` and the id belongs to another requested container, it is rewritten to `container:<that container's name>` before rendering. Kind `template` returns "unknown template" until Task 8. Any other kind is an error. Slices in the result are never nil.

- [ ] **Step 1: Write the failing tests.**

  ```go
  // starter_test.go
  func TestMinimal_StartsWithNameAndParses(t *testing.T) // prefix "name: shop\n"; yaml.Unmarshal ok; services.app.image == "nginx:alpine"
  // compose_new_test.go (engine faked with dockerClient + http handler, as in compose_files_test.go)
  func TestNewFilePreview_Minimal(t *testing.T)                    // Text == starter.Minimal("shop"); engine never called
  func TestNewFilePreview_Containers(t *testing.T)                 // two inspect payloads -> Text has both services and "name: shop"; SecretKeys lists the secret one; placeholder unless IncludeSecrets
  func TestNewFilePreview_InspectFailureNamesTheContainer(t *testing.T) // second id returns 404 -> error contains that id; no partial text
  func TestNewFilePreview_RejectsBadInput(t *testing.T)            // name "Shop!", kind "", kind "containers" with no ids, kind "template" -> errors
  ```

- [ ] **Step 2: Run** `go test ./internal/compose/starter/ ./internal/services/ -run "Minimal|NewFilePreview"`. Expected: FAIL.

- [ ] **Step 3: Implement** the model, the starter package (`//go:embed minimal.yaml`) and `NewFilePreview`. In `copyas.go`, `CopyAsService.load` keeps its behaviour and delegates to the shared function.

- [ ] **Step 4: Run** `go vet ./internal/... && go test ./internal/...`. Expected: PASS.

- [ ] **Step 5: Commit** `git commit -m "Compose: preview the content of a new Compose file"`.

---

### Task 4: Create the file and the project

**Files:**
- Modify: `internal/services/compose_new.go`, `internal/model/compose_new.go`
- Modify: `internal/services/compose_projects.go` (extract the `docker compose config -q` step of `AddProject` into a method both use)
- Test: `internal/services/compose_new_test.go`

**Interfaces:**
- Consumes: `compose.FileCreator`, `compose.ComposeFileNames`, `compose.JoinPath`, `compose.ErrFileExists` (Task 1); `normalizeFiles`, `s.managed`, `s.fsFor`, `s.cliStatus`, `s.project` (existing).
- Produces:
  ```go
  // model
  type ComposeCreateResult struct {
      Project      *ComposeProject `json:"project,omitempty"`
      ExistingFile string          `json:"existingFile,omitempty"` // set, with no project, when the folder already has a Compose file
  }
  // services
  func (s *ComposeService) CreateProject(ctx context.Context, connectionID, dir, name, text string) (model.ComposeCreateResult, error)
  ```

Order of work, each failing step returning before the next:

1. Store present, environment not `KindNone`, CLI status OK: the same checks and messages as `AddProject`.
2. `dir` normalised with `normalizeFiles(env, []string{dir}, "folder")`; empty is "choose a folder". A path that exists as a file is "<path> is a file, not a folder".
3. `name` trimmed and validated with the same message as `AddProject`. A name that has a saved record on this connection is refused: `a project named "<name>" is already saved for this connection`.
4. `len(text)` over `compose.MaxEditableFile`, or empty text, is refused.
5. If one of `compose.ComposeFileNames` exists in `dir`: return `ComposeCreateResult{ExistingFile: <path>}` and a nil error.
6. `MakeDir`, then `CreateFile(<dir>/compose.yaml)`. `ErrFileExists` here is treated as step 5.
7. Validate with `docker compose config -q`. On failure: `Remove` the file, `Remove` the folder if `MakeDir` created it, return the same "not valid" error `AddProject` gives.
8. Save the record. On failure: the same cleanup, then the error.
9. Return the project from `s.project`.

- [ ] **Step 1: Write the failing tests** (temp dirs, `fakeRunner`, `fakeStore`; add a store whose `SaveComposeProject` fails).

  ```go
  func TestCreateProject_WritesFileAndSavesRecord(t *testing.T)
  // new subfolder of TempDir, name "shop": file content == text; record {Name:"shop", WorkingDir:dir, ConfigFiles:[dir/compose.yaml]}; result.Project.Name == "shop"; runner saw `config -q` with -p shop
  func TestCreateProject_ExistingComposeFileIsReported(t *testing.T)
  // table over the four names: result.ExistingFile == that path, err nil, no record, the existing file's bytes unchanged, no compose.yaml added
  func TestCreateProject_RefusesSavedNameButAllowsDiscovered(t *testing.T)
  // store already has "shop" -> error, no file. Engine lists containers labelled project "live" with no record -> CreateProject(name "live") succeeds
  func TestCreateProject_InvalidFileLeavesNothing(t *testing.T)
  // runner fails config -q: error mentions "not valid"; compose.yaml gone; the folder this call created is gone; a folder that existed before stays
  func TestCreateProject_SaveFailureLeavesNothing(t *testing.T)
  // failing store: error returned; file and created folder gone
  func TestCreateProject_RejectsBadFolder(t *testing.T) // Review Focus 1
  // "", "relative/dir", a path that is a file, and on the "wsl" profile `C:\apps\shop` -> errors; nothing written; runner not asked to create anything
  func TestCreateProject_UnwritableFolder(t *testing.T) // Review Focus 2
  // FileCreator whose CreateFile fails with a permission error after MakeDir created the folder: error returned and the folder removed
  func TestCreateProject_RejectsEmptyAndOversizedText(t *testing.T)
  ```
  For the unwritable case the service needs a seam: resolve the creator through one method (`s.creatorFor(env) compose.FileCreator`) that a test can replace with a field, next to the existing `runner` field.

- [ ] **Step 2: Run** `go test ./internal/services/ -run CreateProject`. Expected: FAIL.

- [ ] **Step 3: Implement `CreateProject`** and the extracted validation method; `AddProject` behaviour and its tests stay as they are.

- [ ] **Step 4: Run** `go vet ./internal/... && go test ./internal/...`. Expected: PASS.

- [ ] **Step 5: Commit** `git commit -m "Compose: create a Compose file and save it as a project"`.

---

### Task 5: Shared dialog fields and the dialog's rules

**Files:**
- Create: `frontend/src/components/compose/pathFields.tsx` (the location box and the path row, moved out of `OpenComposeDialog.tsx`)
- Modify: `frontend/src/components/compose/OpenComposeDialog.tsx` (uses the shared pieces; no behaviour change)
- Create: `frontend/src/lib/newComposeFile.ts`
- Test: `frontend/src/lib/newComposeFile.test.ts`

**Interfaces:**
- Produces:
  ```ts
  // pathFields.tsx
  export function ComposeLocationBox(props: { location: ComposeLocation | undefined; error?: unknown }): JSX.Element
  export function PathRow(props: {
    value: string; onChange: (v: string) => void; onRemove?: () => void;
    placeholder: string; label: string;
    pick: { kind: "file"; title: string; pattern: string } | { kind: "folder"; title: string };
  }): JSX.Element // "folder" calls DialogService.PickDirectory(title)

  // newComposeFile.ts
  export function folderName(dir: string): string            // last segment, either slash, trailing slashes ignored
  export function defaultProjectName(dir: string): string    // same rule as Go's compose.NormalizeProjectName on folderName(dir)
  export function nameProblem(name: string): string | undefined // undefined when /^[a-z0-9][a-z0-9_-]*$/ and <= 255
  export type NameState = { kind: "free" } | { kind: "saved" } | { kind: "takeover"; containers: number; containerIds: string[] }
  export function nameState(projects: readonly ComposeProject[], name: string): NameState
  // "saved" when a project of that name has managed === true; "takeover" when it exists with managed === false
  export function createBlocked(a: {
    location: ComposeLocation | undefined; dir: string; name: string; nameState: NameState;
    source: "minimal" | "template" | "containers"; ticked: number; previewReady: boolean;
  }): string | undefined
  ```
  `createBlocked` returns, first match wins: the location reasons `OpenComposeDialog` uses today; `"Choose a folder"`; `"Enter a project name"`; the `nameProblem` text; `"A project with this name is already saved"`; `"Tick at least one container"` (source containers, ticked 0); `"Preparing the preview…"`.

- [ ] **Step 1: Write the failing tests.**

  ```ts
  describe("defaultProjectName", () => {
    it.each([
      ["C:\\apps\\Shop", "shop"], ["/home/dev/my_app/", "my_app"], ["C:\\apps\\My Shop!", "myshop"],
      ["/srv/-edge", "edge"], ["C:\\_", ""], ["/srv/!!!", ""], // Review Focus 5
    ])("%s -> %s", (dir, want) => expect(defaultProjectName(dir)).toBe(want));
  });
  describe("nameProblem", () => { /* "shop" ok; "", "Shop", "-x", "a b" -> text */ });
  describe("nameState", () => { /* free; saved for managed:true; takeover with containerIds and count for managed:false */ });
  describe("createBlocked", () => {
    /* each reason in order; containers source with ticked 0 -> "Tick at least one container" (Review Focus 3);
       empty name from a folder that normalises to "" -> "Enter a project name" (Review Focus 5); undefined when all is well */
  });
  ```

- [ ] **Step 2: Run** `cd frontend && npm test -- newComposeFile`. Expected: FAIL.

- [ ] **Step 3: Implement** the lib, then move the two UI pieces and switch `OpenComposeDialog` to them.

- [ ] **Step 4: Run** `cd frontend && npx tsc --noEmit && npm test`. Expected: PASS, including the existing compose tests.

- [ ] **Step 5: Commit** `git commit -m "Compose: share the dialog path fields and add the New dialog rules"`.

---

### Task 6: The New Compose File dialog

**Files:**
- Create: `frontend/src/components/compose/NewComposeDialog.tsx`, `frontend/src/components/compose/NewComposeSource.tsx` (the "Start from" block with the container list)
- Modify: `frontend/src/hooks/useCompose.ts` (two hooks), `frontend/src/pages/ComposePage.tsx`, `frontend/src/pages/ComposeProjectPage.tsx`
- Test: `frontend/src/components/compose/newCompose.test.tsx`

**Interfaces:**
- Consumes: Task 5 exports; `ComposeService.NewFilePreview`, `ComposeService.CreateProject` (Tasks 3 and 4, regenerate bindings first); `YamlEditor` read-only.
- Produces:
  ```ts
  // useCompose.ts
  export function useNewFilePreview(connectionId: string, name: string, source: ComposeNewSource, enabled: boolean)
  //   query key ["compose", connectionId, "newfile", name, source]; arguments settled for 300 ms before the call; retry false
  export function useCreateComposeProject(connectionId: string)
  //   mutation of { dir, name, text } -> ComposeCreateResult; invalidates ["compose", connectionId] when result.project is set
  // NewComposeDialog.tsx
  export function NewComposeDialog(props: {
    connectionId: string; open: boolean; onOpenChange: (open: boolean) => void;
    projects: readonly ComposeProject[]; containers: readonly Container[] | undefined;
    onOpenExisting: (path: string) => void;
  }): JSX.Element
  ```

Behaviour, with the copy the spec fixes:

- Title "New Compose File". Fields in order: location box, Folder (`PathRow` folder mode, helper text "compose.yaml will be created here."), Project name, Start from, Preview.
- The name field follows `defaultProjectName(dir)` until the user types in it.
- Start from: "Minimal example" (default), "Template" (hidden until Task 8 supplies templates), "Running containers". The container list shows every container of the connection, running first, each with image and its Compose project when it has one; under it the checkbox "Include secret values" and the note: "The original containers keep running. Up will clash with their names and ports until they are stopped or removed."
- Preview: `YamlEditor` read-only with the preview text; "Not included: ..." under it when the list is not empty. A preview error shows in place of the editor and blocks Create.
- No preview request is made while `createBlocked` reports a problem with the name or "Tick at least one container".
- Takeover (`nameState.kind === "takeover"`): the warning above the buttons reads `A project named "<name>" is already running (<n> containers) without a Compose file in Docklet. This file will become its Compose file: Up and Apply will then change those containers.` The button reads "Create and take over"; otherwise "Create and open". When the name first becomes a takeover, the source switches to Running containers with `containerIds` ticked, once, without overriding later changes by the user.
- Create: on `existingFile`, show "This folder already has a Compose file." with a button "Open it instead" that closes this dialog and calls `onOpenExisting(path)`. On a project, close and `navigate("/compose/<name>", { state: { tab: "file" } })`. Errors show inline in the same alert style as `OpenComposeDialog`.
- `ComposePage`: a "New Compose File" button left of "Open Compose File" in the header and inside the empty state; `onOpenExisting` sets the existing `dropped` state to `[path]` and opens `OpenComposeDialog`.
- `ComposeProjectPage`: the initial tab is `location.state?.tab` when it is one of the page's tab values, else `"structure"`.

- [ ] **Step 1: Write the failing tests** (the `renderToStaticMarkup` + `QueryClient.setQueryData` style of `file.test.tsx`; `vi.mock("@bindings/services")`).

  ```tsx
  it("shows the takeover warning and button for a running project without a file")
  // projects [{ name: "shop", managed: false, containerIds: ["a","b","c"] }], initial name "shop":
  // html contains 'already running (3 containers)' and "Create and take over"
  it("offers Create and open for a free name") // no warning text
  it("disables Create with the reason when no container is ticked") // Review Focus 3: reason text present, button disabled
  it("lists running containers first with image and Compose project")
  it("shows Open it instead when the folder already has a Compose file")
  // render the result block with existingFile "/p/compose.yaml"
  it("makes no preview request while the name is invalid or nothing is ticked") // ComposeService.NewFilePreview mock not called
  it("shows a preview error in place of the editor and disables Create") // Review Focus 4: error text naming the container is visible, button disabled
  ```
  To make these testable without events, keep the dialog body a component that takes its state as props (`NewComposeBody`) and let `NewComposeDialog` own the state.

- [ ] **Step 2: Run** `cd frontend && npm test -- newCompose`. Expected: FAIL.

- [ ] **Step 3: Implement** the hooks, the two components and the page changes.

- [ ] **Step 4: Run** `cd frontend && npx tsc --noEmit && npm test && npm run build`. Expected: PASS.

- [ ] **Step 5: Check it in the running app** with `wails3 dev -config ./build/config.yml -port 9245` (the folder picker is native, so the browser mode is not enough). In the dark and the light theme: create from the minimal example into a new folder under a temp directory with a name starting `docklet-test-`; confirm the Compose file tab opens with the text; create again in the same folder and use "Open it instead"; tick a `docklet-test-*` container and confirm the preview. Remove the test folder and forget the test project afterwards.

- [ ] **Step 6: Commit** `git commit -m "Compose: New Compose File dialog"`.

---

### Task 7: End-to-end test

**Files:**
- Modify: `e2e/tests/compose.spec.ts`
- Modify: `e2e/support/engine.ts` only if a helper to read a file inside the Docklet container is missing

- [ ] **Step 1: Write the test** next to the existing one.

  ```ts
  test("creates a new Compose file and opens it in the editor", async ({ page }) => {
    // open /compose on TCP_CONNECTION; click "New Compose File"
    // type the folder /tmp/docklet-test-new (typed, the native picker is not available) and leave the name it derives: "docklet-test-new"
    // expect the Preview to contain "name: docklet-test-new"
    // click "Create and open"
    // expect the URL to end with /compose/docklet-test-new and the "Compose file" tab to be selected, showing "name: docklet-test-new"
    // check in the Docklet container that /tmp/docklet-test-new/compose.yaml exists
    // expect no container named docklet-test-new-* in the engine (nothing was started)
  });
  ```
  Find controls by role and visible name; add accessible labels in the frontend where one is missing.

- [ ] **Step 2: Run** `wails3 task e2e` (or, with the stack up, `cd e2e && npx playwright test tests/compose.spec.ts`). Expected: both tests PASS.

- [ ] **Step 3: Commit** `git commit -m "e2e: create a new Compose file"`.

---

### Task 8: Templates (detachable)

Skip this task to ship without templates; nothing else depends on it.

**Files:**
- Create: `internal/compose/starter/web-postgres.yaml`, `web-postgres-redis.yaml`, `static-site.yaml`
- Modify: `internal/compose/starter/starter.go`, `internal/model/compose_new.go`, `internal/services/compose_new.go`, `frontend/src/components/compose/NewComposeSource.tsx`, `frontend/src/hooks/useCompose.ts`
- Test: `internal/compose/starter/starter_test.go`, `internal/services/compose_new_test.go`, `frontend/src/components/compose/newCompose.test.tsx`

**Interfaces:**
- Produces:
  ```go
  // model
  type ComposeTemplate struct { ID string `json:"id"`; Title string `json:"title"` }
  // starter
  func Templates() []model.ComposeTemplate // ids "web-postgres", "web-postgres-redis", "static-site"; titles "Web app + PostgreSQL", "Web app + PostgreSQL + Redis", "Static site"
  func Template(id, project string) (string, error) // "name: <project>\n" + file; unknown id is an error
  // services
  func (s *ComposeService) NewFileTemplates() []model.ComposeTemplate
  ```
  `NewFilePreview` kind `template` returns `starter.Template(src.Template, name)`.

Each template: a named volume for data, a healthcheck on the database, `${VAR}` placeholders for passwords, `depends_on` with `condition: service_healthy` where a database exists. The exact YAML is the implementer's.

- [ ] **Step 1: Write the failing tests.**

  ```go
  func TestTemplates_EachParsesAndStartsWithName(t *testing.T) // for every Templates() id: prefix "name: shop\n", yaml parses, has at least one service, contains no literal password (every *_PASSWORD value starts with "${")
  func TestTemplate_UnknownID(t *testing.T)
  func TestNewFilePreview_Template(t *testing.T)               // Text == starter.Template("web-postgres", "shop")
  ```
  ```tsx
  it("shows the Template choice with the three titles when templates are available")
  ```

- [ ] **Step 2: Run** the Go and frontend tests. Expected: FAIL.

- [ ] **Step 3: Implement**, embed the files, add `useNewFileTemplates(connectionId)` (static, `staleTime: Infinity`) and show the Template choice.

- [ ] **Step 4: Validate each template with the real CLI** in an empty temp folder: `docker compose -f <file> config -q` exits 0 (unset variables only warn).

- [ ] **Step 5: Run** all Go and frontend tests, check the choice in the running app in both themes, then commit `git commit -m "Compose: templates for new Compose files"`.
