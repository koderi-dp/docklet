# Compose Autocompletion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The Compose file editor suggests the Compose keys and values valid at the cursor, plus names taken from the open file and the engine.

**Architecture:** Three pure TypeScript modules with no editor in them: one turns text and a cursor offset into a key path, one looks that path up in the bundled Compose specification schema, one collects names from the document. A thin CodeMirror completion source joins them and is passed to `YamlEditor` as an optional extension, so the editor stays a generic YAML editor.

**Tech Stack:** TypeScript, CodeMirror 6 (`@codemirror/autocomplete`), Vitest. No Go changes.

**Spec:** `docs/superpowers/specs/2026-11-23-new-compose-file-design.md` (Part 2). Read it before any task.

## Decisions the spec left to the build

- **The schema is read by a small reader of our own**, not a library. The spec rules out schema validation and hover in the editor, which is most of what the available library adds, and a reader that only lists keys and values is small.
- **The cursor position is worked out from indentation, line by line**, not from the YAML syntax tree. A file being typed is usually not valid YAML at the cursor, and indentation still answers "which map am I in".
- **A small supplement lists values for keys the schema types as free text** (`restart`, `pull_policy`, `network_mode`). The spec's example `restart: unless-stopped` needs it: the Compose schema does not enumerate those values. If the vendored schema does carry an enum for one of them, the two lists are merged without repeats.

## Global Constraints

- Nothing is fetched at runtime. The schema is a file in the repository with a note of the version it came from.
- `docker compose config` stays the only judge of validity: no error marking, no hover, no multi-line snippets, no completion of variable names or paths.
- Enter accepts a suggestion. Tab keeps inserting two spaces as it does today.
- Read-only views (env files, the New dialog's preview) get no completion.
- In a project with several Compose files, names come from the open file only.
- New dependency: `@codemirror/autocomplete` pinned to `6.20.4`, the version already in `frontend/package-lock.json` through `codemirror`. No other new dependency.
- Files stay near 500 lines. Unit tests check real behaviour. Commit as the repository's git user with no attribution line.
- UI must be readable in the dark and the light theme, and is checked in the running app.
- Test command: `cd frontend && npx tsc --noEmit && npm test`.

## Review Focus

1. **Cursor inside a comment, a quoted string, a block scalar (`command: |`) or a flow collection (`ports: ["80:80"]`).** Expected: no suggestions, never an exception. Pinned in Task 1.
2. **Tabs or mixed indentation, and Windows line endings.** Expected: `\r` is ignored; a line indented with a tab yields no suggestions rather than a wrong path. Pinned in Task 1.
3. **Keys the schema does not know (`x-` extensions, a typo as parent).** Expected: an empty list under them, no exception. Pinned in Task 2.
4. **A large file (several thousand lines) and fast typing.** Expected: the path scan walks upward from the cursor line only and stops at indentation 0; no full parse per keystroke. Pinned in Task 1 by a test on a 5,000-line document that asserts the result and a bounded number of line reads.
5. **Images without a tag (`<none>:<none>`) and the same tag on several images.** Expected: untagged entries are left out and names are listed once. Pinned in Task 3.

---

### Task 1: From cursor to key path

**Files:**
- Create: `frontend/src/lib/composeComplete/cursorPath.ts`
- Test: `frontend/src/lib/composeComplete/cursorPath.test.ts`

**Interfaces:**
- Produces:
  ```ts
  export interface CursorContext {
    /** Keys from the document root to the map or list the cursor is in. A list level is the string "[]". */
    path: string[];
    /** "key": typing a key in a map. "value": after `key:` on the same line, or a plain list item. */
    kind: "key" | "value";
    /** For kind "value" on a `key: ` line: that key. Undefined for a list item. */
    key?: string;
    /** What is already typed, and where it starts (document offset): the range a suggestion replaces. */
    prefix: string;
    from: number;
    /** Keys already present in the same map (kind "key" only), without the one being typed. */
    siblings: string[];
  }
  /** Null where nothing should be suggested. `readLine` (1-based line number to line text) exists so a test can count reads; by default lines are read from `text`. */
  export function cursorContext(text: string, offset: number, readLine?: (lineNumber: number) => string): CursorContext | null
  ```

Rules:

- Indentation is the count of leading spaces. A line whose indentation contains a tab makes the result `null`.
- A parent is the nearest line above with smaller indentation that is `key:` with nothing (or only a comment) after the colon; walk upward repeating until indentation 0. Blank lines and comment lines are skipped.
- A line starting with `- ` is a list item of its parent: `"[]"` is pushed after the parent key, and a `- key: value` item puts the cursor in a map whose path ends with `"[]"`.
- `null` when the cursor is after a `#` that starts a comment, inside quotes, inside `[...]` or `{...}`, or on a line that belongs to a block scalar (a parent line ending in `|`, `>` or their `-`/`+` forms).
- `\r` at a line end is ignored.

- [ ] **Step 1: Write the failing tests.** `at(doc)` is a test helper that takes a document with one `|` marking the cursor and returns `cursorContext(textWithoutMarker, offset)`.

  ```ts
  it("top level key", () => expect(at("services:\n  web:\n    image: nginx\nvol|")).toMatchObject({ path: [], kind: "key", prefix: "vol", siblings: ["services"] }));
  it("key inside a service", () => expect(at("services:\n  web:\n    image: nginx\n    he|")).toMatchObject({ path: ["services", "web"], kind: "key", prefix: "he", siblings: ["image"] }));
  it("empty line takes its path from the cursor column", () => expect(at("services:\n  web:\n    image: nginx\n    |")).toMatchObject({ path: ["services", "web"], kind: "key", prefix: "" }));
  it("nested map", () => expect(at("services:\n  web:\n    healthcheck:\n      in|")).toMatchObject({ path: ["services", "web", "healthcheck"] }));
  it("value after a key", () => expect(at("services:\n  web:\n    restart: un|")).toMatchObject({ path: ["services", "web"], kind: "value", key: "restart", prefix: "un" }));
  it("plain list item", () => expect(at("services:\n  web:\n    depends_on:\n      - d|")).toMatchObject({ path: ["services", "web", "depends_on", "[]"], kind: "value", key: undefined, prefix: "d" }));
  it("key inside a list item map", () => expect(at("services:\n  web:\n    volumes:\n      - type: bind\n        so|")).toMatchObject({ path: ["services", "web", "volumes", "[]"], kind: "key", siblings: ["type"] }));
  it("from is the start of the typed word", () => { const c = at("services:\n  we|"); expect(c!.from).toBe("services:\n  ".length); });
  it.each([
    ["comment", "services:\n  web: # no|"], ["double quotes", 'services:\n  web:\n    image: "ng|"'],
    ["flow list", 'services:\n  web:\n    ports: ["80|"]'], ["flow map", "services:\n  web: { ima| }"],
    ["block scalar", "services:\n  web:\n    command: |-\n      echo h|"], // the first | here is YAML; the helper uses the last one
    ["tab indentation", "services:\n\tweb:\n\t\tim|"],
  ])("no suggestions in a %s", (_n, doc) => expect(at(doc)).toBeNull()); // Review Focus 1 and 2
  it("ignores carriage returns", () => expect(at("services:\r\n  web:\r\n    im|")).toMatchObject({ path: ["services", "web"], prefix: "im" })); // Review Focus 2
  it("reads only the lines above the cursor up to the root", () => {
    // Review Focus 4: 5,000 lines of other services above; the cursor is in the last service.
    // cursorContext(text, offset, countingReader) returns the right path and the reader was asked for fewer than 5,100 lines.
  });
  it("offset 0 and an empty document", () => expect(at("|")).toMatchObject({ path: [], kind: "key", prefix: "" }));
  ```
  The counting reader is an optional third parameter `(lineNumber: number) => string` used only by that test.

- [ ] **Step 2: Run** `cd frontend && npm test -- cursorPath`. Expected: FAIL, module not found.

- [ ] **Step 3: Implement `cursorContext`.**

- [ ] **Step 4: Run** `cd frontend && npx tsc --noEmit && npm test -- cursorPath`. Expected: PASS.

- [ ] **Step 5: Commit** `git commit -m "Compose completion: key path at the cursor"`.

---

### Task 2: Keys and values from the Compose schema

**Files:**
- Create: `frontend/src/lib/composeComplete/compose-spec.json` (vendored), `frontend/src/lib/composeComplete/SCHEMA.md` (two lines: where it came from and the commit)
- Create: `frontend/src/lib/composeComplete/schema.ts`
- Test: `frontend/src/lib/composeComplete/schema.test.ts`

**Interfaces:**
- Produces:
  ```ts
  export interface Suggestion { label: string; detail?: string; info?: string }
  /** Keys valid in the map at `path` (see CursorContext.path), sorted by label. */
  export function keysAt(path: readonly string[]): Suggestion[]
  /** Fixed values for `key` in the map at `path`, or for an item of the list at `path` when key is undefined. */
  export function valuesAt(path: readonly string[], key: string | undefined): Suggestion[]
  ```

- [ ] **Step 1: Vendor the schema.** Download `schema/compose-spec.json` from the `compose-spec/compose-spec` repository on GitHub (main branch) into `compose-spec.json` and write the commit hash and date into `SCHEMA.md`. The file is Apache-2.0; say so in `SCHEMA.md`.

- [ ] **Step 2: Write the failing tests** against the real vendored file.

  ```ts
  const labels = (s: Suggestion[]) => s.map((x) => x.label);
  it("top level", () => expect(labels(keysAt([]))).toEqual(expect.arrayContaining(["name", "services", "networks", "volumes", "secrets", "configs"])));
  it("service keys through the pattern key", () => expect(labels(keysAt(["services", "web"]))).toEqual(expect.arrayContaining(["image", "ports", "environment", "healthcheck", "depends_on", "restart", "volumes"])));
  it("nested map", () => expect(labels(keysAt(["services", "web", "healthcheck"]))).toEqual(expect.arrayContaining(["test", "interval", "timeout", "retries", "start_period"])));
  it("alternatives are merged", () => expect(labels(keysAt(["services", "web", "build"]))).toEqual(expect.arrayContaining(["context", "dockerfile", "args"]))); // build is string | object
  it("map form of depends_on", () => expect(labels(keysAt(["services", "web", "depends_on", "db"]))).toEqual(expect.arrayContaining(["condition"])));
  it("enum from the schema", () => expect(labels(valuesAt(["services", "web", "depends_on", "db"], "condition"))).toEqual(expect.arrayContaining(["service_started", "service_healthy", "service_completed_successfully"])));
  it("supplement for free-text keys", () => {
    expect(labels(valuesAt(["services", "web"], "restart"))).toEqual(["always", "no", "on-failure", "unless-stopped"]);
    expect(labels(valuesAt(["services", "web"], "pull_policy"))).toEqual(expect.arrayContaining(["always", "never", "missing", "build"]));
    expect(labels(valuesAt(["services", "web"], "network_mode"))).toEqual(expect.arrayContaining(["bridge", "host", "none"]));
  });
  it("booleans", () => expect(labels(valuesAt(["services", "web"], "privileged"))).toEqual(["false", "true"]));
  it("list item keys", () => expect(labels(keysAt(["services", "web", "volumes", "[]"]))).toEqual(expect.arrayContaining(["type", "source", "target", "read_only"])));
  it("top-level volume definition", () => expect(labels(keysAt(["volumes", "data"]))).toEqual(expect.arrayContaining(["driver", "external", "name"])));
  it("carries the schema's description", () => expect(keysAt([]).find((s) => s.label === "services")?.info).toBeTruthy());
  it("no duplicates and sorted", () => { const l = labels(keysAt(["services", "web"])); expect(l).toEqual([...new Set(l)].sort()); });
  it.each([[["services", "web", "x-custom"]], [["nonsense"]], [["services", "web", "image", "deeper"]]])("unknown path %j gives nothing", (p) => {
    expect(keysAt(p)).toEqual([]); expect(valuesAt(p, "anything")).toEqual([]); // Review Focus 3
  });
  ```
  If an assertion on a key name fails because the vendored schema names it differently, fix the reader, not the assertion; if the schema truly lacks that key, replace it in the assertion with a neighbouring key that exists and note it in the commit message.

- [ ] **Step 3: Run** `cd frontend && npm test -- composeComplete/schema`. Expected: FAIL.

- [ ] **Step 4: Implement the reader.** Resolve a path to a set of schema nodes, following `$ref` (with a guard against cycles), every branch of `oneOf` / `anyOf` / `allOf`, `properties`, `patternProperties` (any key matches), `additionalProperties` when it is a schema, and `items` for `"[]"`. `keysAt` unions `properties` of the nodes, leaving out pattern keys such as `^x-`. `valuesAt` unions `enum` and `const`, adds `true`/`false` for a boolean type, and merges the supplement table. `info` is the node's `description`; `detail` is its type when it is a single plain type.

- [ ] **Step 5: Run** `cd frontend && npx tsc --noEmit && npm test -- composeComplete`. Expected: PASS.

- [ ] **Step 6: Commit** `git commit -m "Compose completion: keys and values from the Compose schema"`.

---

### Task 3: Names from the open file and the engine

**Files:**
- Create: `frontend/src/lib/composeComplete/projectValues.ts`
- Test: `frontend/src/lib/composeComplete/projectValues.test.ts`

**Interfaces:**
- Consumes: `CursorContext` (Task 1), `Suggestion` (Task 2).
- Produces:
  ```ts
  export interface DocumentNames { services: string[]; volumes: string[]; networks: string[] }
  /** Child keys of the top-level services, volumes and networks maps, in document order. */
  export function documentNames(text: string): DocumentNames
  /** Image references usable after `image:`: every repo:tag once, untagged entries left out, sorted. */
  export function imageNames(repoTags: readonly (readonly string[] | null | undefined)[]): string[]
  /** Suggestions that come from the project for this position; empty when the position has none. */
  export function projectValues(ctx: CursorContext, names: DocumentNames, images: readonly string[]): Suggestion[]
  ```

Positions (`S` is `["services", <any>]`):

| Position | Suggestions |
|---|---|
| value of `image` in `S` | `images` |
| item of `S.depends_on`, `S.links`, `S.volumes_from`; key in the map `S.depends_on`; value of `service` in `S.extends` | service names except the service being edited |
| value of `network_mode` in `S` | `service:<name>` for every other service |
| item of `S.networks`; key in the map `S.networks` | network names |
| item of `S.volumes` | `<volume name>:` for every volume name |

- [ ] **Step 1: Write the failing tests.**

  ```ts
  const doc = "services:\n  web:\n    image: nginx\n  db:\n    image: postgres\nvolumes:\n  data:\n  cache: {}\nnetworks:\n  back:\n";
  it("reads names from the document", () => expect(documentNames(doc)).toEqual({ services: ["web", "db"], volumes: ["data", "cache"], networks: ["back"] }));
  it("survives a document with no such maps or broken text", () => expect(documentNames("serv")).toEqual({ services: [], volumes: [], networks: [] }));
  it("lists image names once and skips untagged ones", () => // Review Focus 5
    expect(imageNames([["nginx:1.27", "nginx:latest"], ["<none>:<none>"], null, ["nginx:latest"], []])).toEqual(["nginx:1.27", "nginx:latest"]));
  it("offers images after image:", () => /* ctx value/key "image" at ["services","web"] -> labels equal images */);
  it("offers other services under depends_on, list and map form", () => /* editing web -> ["db"], never "web" */);
  it("offers service: targets for network_mode", () => /* ["service:db"] */);
  it("offers networks and volumes where a service references them", () => /* ["back"]; ["cache:", "data:"] */);
  it("offers nothing elsewhere", () => /* key position at ["services","web"] -> [] */);
  ```

- [ ] **Step 2: Run** `cd frontend && npm test -- projectValues`. Expected: FAIL.

- [ ] **Step 3: Implement.** `documentNames` scans lines by indentation the same way Task 1 does (top-level key at indentation 0, its children at the first deeper indentation); it does not parse YAML.

- [ ] **Step 4: Run** `cd frontend && npx tsc --noEmit && npm test -- composeComplete`. Expected: PASS.

- [ ] **Step 5: Commit** `git commit -m "Compose completion: names from the file and the engine"`.

---

### Task 4: The completion source in the editor

**Files:**
- Modify: `frontend/package.json`, `frontend/package-lock.json` (`@codemirror/autocomplete` `6.20.4` as a direct dependency)
- Create: `frontend/src/components/compose/file/composeCompletion.ts`
- Modify: `frontend/src/components/compose/file/YamlEditor.tsx`, `frontend/src/components/compose/file/ComposeFileTab.tsx`, `frontend/src/components/compose/file/yamlEditorTheme.ts`
- Test: `frontend/src/components/compose/file/composeCompletion.test.ts`

**Interfaces:**
- Consumes: `cursorContext`, `keysAt`, `valuesAt`, `documentNames`, `imageNames`, `projectValues` (Tasks 1 to 3); `useImagesQuery` from `frontend/src/hooks/useImages.ts`.
- Produces:
  ```ts
  // composeCompletion.ts
  import type { CompletionContext, CompletionResult } from "@codemirror/autocomplete";
  import type { Extension } from "@codemirror/state";
  /** The source alone, for tests. `images` is read on every call so the list can change without rebuilding the editor. */
  export function composeSource(images: () => readonly string[]): (ctx: CompletionContext) => CompletionResult | null
  /** The editor extension: `autocompletion({ override: [composeSource(images)], activateOnTyping: true })`. */
  export function composeCompletion(images: () => readonly string[]): Extension
  // YamlEditor.tsx
  interface Props { /* existing */ completion?: Extension }
  ```

Behaviour:

- Position "key": `keysAt(path)` without `siblings`, then `projectValues`. Accepting a key inserts `label` followed by `: `, unless a colon already follows the cursor on that line.
- Position "value": `projectValues` first, then `valuesAt(path, key)`. Accepting inserts the label only.
- `from` is `CursorContext.from`; `validFor` is `/^[\w.:/-]*$/` so typing narrows the list without a new lookup.
- Returns `null` when `cursorContext` is `null`, and when there is nothing to offer and the request was not explicit (Ctrl+Space).
- Entries show `detail` beside the label and `info` as the side text. Project values rank above schema values (`boost`).
- `YamlEditor` adds `completion` to its extension list when given. It is read once when the view is created, like the other extensions, so `ComposeFileTab` passes a stable value (`useMemo`) and feeds images through a ref. The existing `Prec.highest` keymap keeps Tab as indentation; Enter accepts through the standard completion keymap already in `basicSetup`.
- `ComposeFileTab` passes the extension only for an editable config file; read-only files pass none.
- The popup takes its colours from the same tokens as the editor theme so it is readable in both themes.

- [ ] **Step 1: Add the dependency.** `cd frontend && npm install --save-exact @codemirror/autocomplete@6.20.4`. Expected: `package.json` gains the line and the lock file keeps a single copy of the package.

- [ ] **Step 2: Write the failing tests.** No DOM is needed: build an `EditorState` and a `CompletionContext` by hand.

  ```ts
  const run = (doc: string, images: string[] = [], explicit = false) => {
    const pos = doc.indexOf("|");
    const state = EditorState.create({ doc: doc.replace("|", "") });
    return composeSource(() => images)(new CompletionContext(state, pos, explicit));
  };
  const labels = (r: CompletionResult | null) => (r ? r.options.map((o) => o.label) : null);

  it("offers service keys without the ones already present", () => {
    const l = labels(run("services:\n  web:\n    image: nginx\n    he|"))!;
    expect(l).toContain("healthcheck"); expect(l).not.toContain("image");
  });
  it("replaces from the start of the typed word", () => expect(run("services:\n  web:\n    he|")!.from).toBe("services:\n  web:\n    ".length));
  it("inserts the key with a colon and a space", () => /* option for "healthcheck": apply (string or function result) yields "healthcheck: " */);
  it("does not add a second colon", () => /* doc "    he|:" -> the applied text is "healthcheck" */);
  it("offers restart values", () => expect(labels(run("services:\n  web:\n    restart: |"))).toEqual(expect.arrayContaining(["unless-stopped"])));
  it("offers images after image:", () => expect(labels(run("services:\n  web:\n    image: |", ["nginx:1.27"]))).toEqual(["nginx:1.27"]));
  it("offers other services under depends_on before anything else", () => expect(labels(run("services:\n  web:\n    depends_on:\n      - |\n  db:\n    image: postgres"))![0]).toBe("db"));
  it("stays silent in comments, strings and block scalars", () => expect(run("services:\n  web: # he|", [], true)).toBeNull());
  it("stays silent on an unknown parent unless asked", () => {
    expect(run("services:\n  web:\n    x-custom:\n      a|")).toBeNull();
    expect(run("services:\n  web:\n    x-custom:\n      a|", [], true)).toBeNull();
  });
  ```
  And, in the existing `file.test.tsx`, one test that `ComposeFileTab` renders for a read-only file without requesting images (the images query mock is not called).

- [ ] **Step 3: Run** `cd frontend && npm test -- composeCompletion`. Expected: FAIL.

- [ ] **Step 4: Implement** the source, the extension, the `YamlEditor` prop, the wiring in `ComposeFileTab` and the popup colours.

- [ ] **Step 5: Run** `cd frontend && npx tsc --noEmit && npm test && npm run build`. Expected: PASS; the build output shows the schema in the chunk that holds the Compose file editor, not in the entry chunk (import `schema.ts` only from `composeCompletion.ts`).

- [ ] **Step 6: Check it in the running app** with `wails3 task dev:browser` (no native part is involved), on a project whose Compose file is editable, in the dark and the light theme: type `he` inside a service and accept `healthcheck` with Enter; type `restart: ` and pick a value; add `depends_on:` with a list item and see the other services; press Tab and confirm it still indents; open a `.env` file of the project and confirm nothing pops up. Do not save the file.

- [ ] **Step 7: Commit** `git commit -m "Compose file editor: autocompletion from the Compose schema and the project"`.
