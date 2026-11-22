import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";
import { toast } from "sonner";
import type { ComposeFileRef, ComposeProject, Container } from "@bindings/model";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { useComposeFile, useComposeFiles, useFileOutline, useInvalidateAfterSave, useSaveComposeFile } from "@/hooks/useComposeFiles";
import { afterSave, isDirty, pickFile, type EditState } from "@/lib/composeFileState";
import { errorMessage, toastError } from "@/lib/errors";
import { DiscardChangesDialog } from "./DiscardChangesDialog";
import { FileSwitcher } from "./FileSwitcher";
import { SaveConflictDialog } from "./SaveConflictDialog";
import { ServiceOutline } from "./ServiceOutline";
import { useUnsavedGuard } from "./useUnsavedGuard";
import { YamlEditor } from "./YamlEditor";

interface Props {
  connectionId: string;
  project: ComposeProject;
  containers: readonly Container[];
  active: boolean;
  onDirtyChange: (dirty: boolean) => void;
  panel?: ReactNode;
}

/** The Compose file editor, or the reason the project's files cannot be edited. */
export function ComposeFileTab({ connectionId, project, active, onDirtyChange, panel }: Props) {
  const files = useComposeFiles(connectionId, project.name, active);
  if (files.isLoading) return <Skeleton className="h-full min-h-64 w-full" />;
  if (files.error || !files.data) return <p className="text-destructive">{errorMessage(files.error ?? "The files could not be listed.")}</p>;
  if (!files.data.canEdit) return <p className="text-muted-foreground">{files.data.reason || "The Compose file cannot be edited here."}</p>;
  return <FileEditor connectionId={connectionId} name={project.name} files={files.data.files ?? []} active={active} onDirtyChange={onDirtyChange} panel={panel} />;
}

/** The open file: its saved text, the text in the editor, and the file it belongs to. */
type Loaded = EditState & { path: string; readOnly: boolean };

interface EditorProps {
  connectionId: string;
  name: string;
  files: readonly ComposeFileRef[];
  active: boolean;
  onDirtyChange: (dirty: boolean) => void;
  panel?: ReactNode;
}

function FileEditor({ connectionId, name, files, active, onDirtyChange, panel }: EditorProps) {
  const [path, setPath] = useState(() => pickFile(files, ""));
  const [loaded, setLoaded] = useState<Loaded | null>(null);
  const [ask, setAsk] = useState<null | { kind: "reload" } | { kind: "file"; path: string }>(null);
  const [jump, setJump] = useState<{ line: number; nonce: number } | undefined>();
  const read = useComposeFile(connectionId, name, path, active);
  const save = useSaveComposeFile(connectionId, name);
  const invalidate = useInvalidateAfterSave(connectionId, name);

  const ready = loaded?.path === path;
  const readOnly = !ready || loaded.readOnly;
  const dirty = ready && isDirty(loaded.loadedText, loaded.text);
  const outline = useFileOutline(loaded?.text ?? "", ready && !readOnly);
  const guard = useUnsavedGuard(dirty);

  // A file arrives: take it when it is a different file, or a newer version while nothing is unsaved.
  const data = read.data;
  useEffect(() => {
    if (!data || data.path !== path) return;
    if (loaded?.path === data.path && (data.version === loaded.version || dirty)) return;
    setLoaded({ path: data.path, loadedText: data.text, text: data.text, version: data.version, conflict: false, error: null, readOnly: data.readOnly });
  }, [data, path, loaded, dirty]);

  const dirtyCb = useRef(onDirtyChange);
  dirtyCb.current = onDirtyChange;
  useEffect(() => dirtyCb.current(dirty), [dirty]);
  useEffect(() => () => dirtyCb.current(false), []);

  const reload = useCallback(async () => {
    const r = await read.refetch();
    if (r.data && r.data.path === path)
      setLoaded({ path: r.data.path, loadedText: r.data.text, text: r.data.text, version: r.data.version, conflict: false, error: null, readOnly: r.data.readOnly });
    else if (r.error) toastError("Could not reload the file", r.error);
  }, [read, path]);

  const doSave = (overwrite: boolean) => {
    if (!ready || readOnly || save.isPending || (!dirty && !overwrite)) return;
    const sent = loaded.text;
    save.mutate(
      { path, text: sent, baseVersion: loaded.version, overwrite },
      {
        onSuccess: (r) => {
          setLoaded((cur) => (cur && cur.path === path ? afterSave(r, cur, sent) : cur));
          if (r.saved) {
            invalidate(path);
            toast.success("Saved");
          }
        },
        onError: (e) => toastError("Could not save the file", e),
      },
    );
  };

  const patch = (p: Partial<Loaded>) => setLoaded((cur) => (cur ? { ...cur, ...p } : cur));
  const jumpTo = (line: number) => setJump({ line, nonce: (jump?.nonce ?? 0) + 1 });
  const switchFile = (p: string) => {
    if (p === path) return;
    if (dirty) setAsk({ kind: "file", path: p });
    else {
      setLoaded(null);
      setPath(p);
    }
  };
  const confirmDiscard = () => {
    const a = ask;
    setAsk(null);
    if (a?.kind === "reload") void reload();
    else if (a) {
      setLoaded(null);
      setPath(a.path);
    }
  };

  const error = loaded?.error ?? (readOnly ? null : (outline.data?.error ?? null));
  const showOutline = ready && !readOnly;

  return (
    <div className="flex h-full min-h-0 gap-3">
      {showOutline && (
        <div className="w-[200px] shrink-0">
          <ServiceOutline services={outline.data?.services ?? []} onJump={jumpTo} />
        </div>
      )}
      <div className="flex min-w-0 flex-1 flex-col overflow-hidden rounded-[10px] border border-border bg-card">
        <div className="flex flex-wrap items-center gap-2 border-b border-border px-3 py-2">
          {files.length > 1 && <FileSwitcher files={files} value={path} onChange={switchFile} />}
          <span className="min-w-0 flex-1 truncate font-mono text-xs text-muted-foreground" title={path}>
            {path}
          </span>
          <span className={dirty ? "text-xs text-warning" : "text-xs text-muted-foreground"}>
            {readOnly && ready ? "Read-only" : dirty ? "Unsaved changes" : "Saved"}
          </span>
          <Button size="sm" variant="outline" disabled={!ready} onClick={() => (dirty ? setAsk({ kind: "reload" }) : void reload())}>
            Reload
          </Button>
          <Button size="sm" disabled={!dirty || readOnly || save.isPending} onClick={() => doSave(false)}>
            Save
          </Button>
        </div>
        {error && (
          <button type="button" onClick={() => jumpTo(error.line)} className="border-b border-border px-3 py-1.5 text-left text-xs text-destructive hover:bg-muted">
            Line {error.line}: {error.message}
          </button>
        )}
        <div className="min-h-0 flex-1">
          {read.error && !ready ? (
            <p className="p-3 text-destructive">{errorMessage(read.error)}</p>
          ) : (
            <YamlEditor
              value={ready ? loaded.text : ""}
              readOnly={readOnly}
              onChange={(text) => patch({ text, error: null })}
              onSave={() => doSave(false)}
              jumpTo={jump}
            />
          )}
        </div>
      </div>
      {panel && <div className="w-[300px] shrink-0 overflow-auto">{panel}</div>}
      <SaveConflictDialog
        open={!!loaded?.conflict}
        onReload={() => {
          patch({ conflict: false });
          void reload();
        }}
        onOverwrite={() => {
          patch({ conflict: false });
          doSave(true);
        }}
        onCancel={() => patch({ conflict: false })}
      />
      <DiscardChangesDialog open={ask !== null} onDiscard={confirmDiscard} onKeep={() => setAsk(null)} />
      {guard}
    </div>
  );
}
