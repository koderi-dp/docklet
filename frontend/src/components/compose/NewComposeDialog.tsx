import { lazy, Suspense, useEffect, useMemo, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { AlertTriangle } from "lucide-react";
import type { ComposeLocation, ComposeNewSource, ComposeProject, Container } from "@bindings/model";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useComposeLocation, useCreateComposeProject, useNewFilePreview, useNewFileTemplates } from "@/hooks/useCompose";
import { errorMessage } from "@/lib/errors";
import { createBlocked, defaultProjectName, nameState as nameStateOf, previewWanted, type NameState, type NewSourceKind } from "@/lib/newComposeFile";
import { NewComposeSource, type NewComposeSourceProps } from "./NewComposeSource";
import { ComposeLocationBox, PathRow } from "./pathFields";

// CodeMirror stays out of the Compose page's chunk until a preview is shown.
const YamlEditor = lazy(() => import("./file/YamlEditor").then((m) => ({ default: m.YamlEditor })));

export interface NewComposePreview {
  text?: string;
  notIncluded?: readonly string[];
  error?: string;
  loading: boolean;
}

export interface NewComposeBodyProps extends NewComposeSourceProps {
  location: ComposeLocation | undefined;
  locationError?: unknown;
  dir: string;
  onDir: (v: string) => void;
  name: string;
  onName: (v: string) => void;
  nameState: NameState;
  preview: NewComposePreview;
  /** What the last Create attempt came back with. */
  result: { existingFile?: string; error?: string };
  blocked: string | undefined;
  pending: boolean;
  onCancel: () => void;
  onCreate: () => void;
  onOpenExisting: (path: string) => void;
}

const noop = () => {};

function Preview({ preview }: { preview: NewComposePreview }) {
  if (preview.error) {
    return (
      <div role="alert" className="max-h-40 overflow-auto whitespace-pre-wrap rounded-md border border-destructive/40 bg-destructive/5 p-3 text-xs text-destructive">
        {preview.error}
      </div>
    );
  }
  if (preview.text === undefined) {
    return <p className="rounded-md border border-border p-3 text-xs text-muted-foreground">{preview.loading ? "Preparing the preview…" : "The preview appears once the fields above are filled in."}</p>;
  }
  const text = preview.text;
  return (
    <>
      <div className="h-44 overflow-hidden rounded-md border border-border" aria-label="Preview of compose.yaml">
        <Suspense fallback={<pre className="h-full overflow-auto p-3 font-mono text-xs">{text}</pre>}>
          <YamlEditor value={text} readOnly onChange={noop} onSave={noop} />
        </Suspense>
      </div>
      {!!preview.notIncluded?.length && <p className="text-xs text-muted-foreground">Not included: {preview.notIncluded.join("; ")}</p>}
    </>
  );
}

/** Everything inside the dialog; all state comes in through props. */
export function NewComposeBody(p: NewComposeBodyProps) {
  const takeover = p.nameState.kind === "takeover" ? p.nameState : null;
  return (
    <>
      <div className="space-y-3 text-sm">
        <ComposeLocationBox location={p.location} error={p.locationError} />

        <div className="space-y-1.5">
          <Label>Folder</Label>
          <PathRow
            value={p.dir}
            onChange={p.onDir}
            placeholder={p.location?.kind === "wsl" ? "/home/user/app" : "Absolute path to the project folder"}
            pick={{ kind: "folder", title: "Choose the folder for the new Compose file" }}
            label="Folder"
          />
          <p className="text-xs text-muted-foreground">compose.yaml will be created here. A folder that does not exist yet is created.</p>
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="compose-new-name">Project name</Label>
          <Input id="compose-new-name" value={p.name} onChange={(e) => p.onName(e.target.value)} placeholder="Taken from the folder name" spellCheck={false} className="h-8" />
        </div>

        <NewComposeSource
          source={p.source}
          onSource={p.onSource}
          containers={p.containers}
          ticked={p.ticked}
          onToggle={p.onToggle}
          includeSecrets={p.includeSecrets}
          onIncludeSecrets={p.onIncludeSecrets}
          templates={p.templates}
          template={p.template}
          onTemplate={p.onTemplate}
        />

        <div className="space-y-1.5">
          <Label>Preview</Label>
          <Preview preview={p.preview} />
        </div>

        {takeover && (
          <div role="status" className="flex gap-2 rounded-md border border-warning/40 bg-warning/10 p-3 text-xs">
            <AlertTriangle className="mt-0.5 size-3.5 shrink-0 text-warning" aria-hidden />
            <span>
              A project named "{p.name}" is already running ({takeover.containers} {takeover.containers === 1 ? "container" : "containers"}) without a Compose file in Docklet. This file will become its
              Compose file: Up and Apply will then change those containers.
            </span>
          </div>
        )}

        {p.result.existingFile && (
          <div role="status" className="flex flex-wrap items-center gap-2 rounded-md border border-border bg-muted/40 p-3 text-xs">
            <span className="min-w-0 flex-1">
              This folder already has a Compose file. <span className="break-all font-mono text-muted-foreground">{p.result.existingFile}</span>
            </span>
            <Button type="button" size="sm" variant="outline" onClick={() => p.onOpenExisting(p.result.existingFile as string)}>
              Open it instead
            </Button>
          </div>
        )}

        {p.result.error && (
          <div role="alert" className="max-h-40 overflow-auto whitespace-pre-wrap rounded-md border border-destructive/40 bg-destructive/5 p-3 text-xs text-destructive">
            {p.result.error}
          </div>
        )}
      </div>

      <DialogFooter className="items-center gap-2 sm:justify-between">
        <span className="text-xs text-muted-foreground">{p.blocked ?? ""}</span>
        <div className="flex gap-2">
          <Button variant="outline" size="sm" onClick={p.onCancel}>
            Cancel
          </Button>
          <Button size="sm" disabled={!!p.blocked || p.pending} onClick={p.onCreate}>
            {p.pending ? "Creating…" : takeover ? "Create and take over" : "Create and open"}
          </Button>
        </div>
      </DialogFooter>
    </>
  );
}

interface Props {
  connectionId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  projects: readonly ComposeProject[];
  containers: readonly Container[] | undefined;
  /** The folder already has a Compose file and the user wants that one. */
  onOpenExisting: (path: string) => void;
}

/** Creates compose.yaml in a folder, saves it as a project and opens it in the editor. Starts nothing. */
export function NewComposeDialog({ connectionId, open, onOpenChange, projects, containers, onOpenExisting }: Props) {
  const loc = useComposeLocation(connectionId, open);
  const create = useCreateComposeProject(connectionId);
  const navigate = useNavigate();
  const [dir, setDir] = useState("");
  const [typedName, setTypedName] = useState<string | null>(null);
  const [source, setSource] = useState<NewSourceKind>("minimal");
  const [ticked, setTicked] = useState<ReadonlySet<string>>(new Set());
  const [includeSecrets, setIncludeSecrets] = useState(false);
  const templates = useNewFileTemplates(open).data ?? [];
  const [pickedTemplate, setPickedTemplate] = useState("");
  const template = pickedTemplate || templates[0]?.id || "";
  const [result, setResult] = useState<{ existingFile?: string; error?: string }>({});

  // The name follows the folder until the user types one.
  const name = (typedName ?? defaultProjectName(dir)).trim();
  const state = useMemo(() => nameStateOf(projects, name), [projects, name]);

  // Naming a project that only runs: offer its containers once, then leave the choice to the user.
  const offeredFor = useRef<string | null>(null);
  useEffect(() => {
    if (!open || state.kind !== "takeover" || offeredFor.current === name) return;
    offeredFor.current = name;
    setSource("containers");
    setTicked(new Set(state.containerIds));
  }, [open, state, name]);

  const src = useMemo(
    () =>
      ({
        kind: source,
        template: source === "template" ? template : "",
        containerIds: source === "containers" ? [...ticked] : [],
        includeSecrets: source === "containers" && includeSecrets,
      }) as ComposeNewSource,
    [source, template, ticked, includeSecrets],
  );
  const wanted = open && !!loc.data?.available && previewWanted({ name, source, ticked: ticked.size });
  const pv = useNewFilePreview(connectionId, name, src, wanted);
  const ready = wanted && pv.current && !!pv.data && !pv.error && !pv.isFetching;
  const preview: NewComposePreview = !wanted
    ? { loading: false }
    : pv.current && pv.error
      ? { error: errorMessage(pv.error), loading: false }
      : ready
        ? { text: pv.data.text, notIncluded: pv.data.notIncluded ?? [], loading: false }
        : { loading: true };

  const base = createBlocked({ location: loc.data, dir, name, nameState: state, source, ticked: ticked.size, previewReady: true });
  const blocked = base ?? (preview.error ? "The preview could not be made" : ready ? undefined : "Preparing the preview…");

  const reset = () => {
    setDir("");
    setTypedName(null);
    setSource("minimal");
    setTicked(new Set());
    setIncludeSecrets(false);
    setPickedTemplate("");
    setResult({});
    offeredFor.current = null;
  };
  const close = () => {
    reset();
    onOpenChange(false);
  };
  // Any change to the inputs makes the last attempt's outcome stale.
  const edit = <T,>(set: (v: T) => void) => (v: T) => {
    setResult({});
    set(v);
  };

  const submit = async () => {
    if (!ready) return;
    setResult({});
    try {
      const r = await create.mutateAsync({ dir: dir.trim(), name, text: pv.data.text });
      if (r.existingFile) return setResult({ existingFile: r.existingFile });
      if (r.project) {
        const created = r.project.name;
        close();
        navigate(`/compose/${encodeURIComponent(created)}`, { state: { tab: "file" } });
      }
    } catch (e) {
      setResult({ error: errorMessage(e) });
    }
  };

  return (
    <Dialog open={open} onOpenChange={(o) => (o ? onOpenChange(true) : close())}>
      <DialogContent className="max-h-[92vh] overflow-y-auto sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>New Compose File</DialogTitle>
          <DialogDescription>
            Create a <span className="font-mono">compose.yaml</span> to start from and open it in the editor. The file is validated with <span className="font-mono">docker compose config</span>; nothing
            is started.
          </DialogDescription>
        </DialogHeader>
        <NewComposeBody
          location={loc.data}
          locationError={loc.error}
          dir={dir}
          onDir={edit(setDir)}
          name={typedName ?? defaultProjectName(dir)}
          onName={edit<string>(setTypedName)}
          nameState={state}
          source={source}
          onSource={edit(setSource)}
          containers={containers}
          ticked={ticked}
          onToggle={(id, on) => {
            setResult({});
            setTicked((cur) => {
              const next = new Set(cur);
              if (on) next.add(id);
              else next.delete(id);
              return next;
            });
          }}
          includeSecrets={includeSecrets}
          onIncludeSecrets={edit(setIncludeSecrets)}
          templates={templates}
          template={template}
          onTemplate={edit(setPickedTemplate)}
          preview={preview}
          result={result}
          blocked={blocked}
          pending={create.isPending}
          onCancel={close}
          onCreate={() => void submit()}
          onOpenExisting={(path) => {
            close();
            onOpenExisting(path);
          }}
        />
      </DialogContent>
    </Dialog>
  );
}
