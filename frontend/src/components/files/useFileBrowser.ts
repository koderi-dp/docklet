import { useEffect, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { DialogService } from "@bindings/services";
import type { FileEntry, Operation } from "@bindings/model";
import { useOnOperationDone, useOperations } from "@/hooks/useOperations";
import { filesKey, useFileListing, useFilePreview, type FileBackend } from "@/hooks/useFiles";
import { toastError } from "@/lib/errors";
import { joinPath, normalizeTyped, parentPath } from "./paths";

const MAX_PREVIEW_SOURCE = 1024 * 1024;

interface Options {
  writeDisabledReason?: string | null;
  deleteDisabledReason?: string | null;
}

/** State and actions of one file browser: navigation, selection, preview and file operations. */
export function useFileBrowser(backend: FileBackend, { writeDisabledReason, deleteDisabledReason }: Options) {
  const qc = useQueryClient();
  const { openOperations } = useOperations();
  const [path, setPath] = useState("/");
  const [pathInput, setPathInput] = useState("/");
  const [selected, setSelected] = useState<FileEntry | null>(null);
  const [mkdirOpen, setMkdirOpen] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState<FileEntry | null>(null);
  const [busy, setBusy] = useState(false);
  const [reindexing, setReindexing] = useState(false);

  const listing = useFileListing(backend, path);
  const previewable = selected && !selected.isDir && !selected.isSymlink && selected.size <= MAX_PREVIEW_SOURCE ? selected.path : null;
  const preview = useFilePreview(backend, previewable);

  useEffect(() => {
    setPathInput(path);
    setSelected(null);
  }, [path]);
  // A different source (container/volume) starts at the root again.
  useEffect(() => {
    setPath("/");
  }, [backend.id]);

  const refresh = () => void qc.invalidateQueries({ queryKey: filesKey(backend) });
  useOnOperationDone("files.", backend.connectionId, refresh);
  useOnOperationDone("volume.import", backend.connectionId, refresh);

  const canWrite = !!backend.upload && !writeDisabledReason;
  const writeReason = !backend.upload ? "This view is read-only." : writeDisabledReason;
  const deleteReason = !backend.remove ? "This view is read-only." : deleteDisabledReason;

  const started = (op: Operation, what: string) =>
    toast.info(`${what} started: ${op.title}`, { description: "Progress is shown in Operations.", action: { label: "View", onClick: () => openOperations(op.id) } });

  /** Runs a toolbar action with the busy flag set and a persistent toast on failure. */
  const guarded = async (failure: string, fn: () => Promise<void>) => {
    setBusy(true);
    try {
      await fn();
    } catch (e) {
      toastError(failure, e);
    } finally {
      setBusy(false);
    }
  };

  const open = async (e: FileEntry) => {
    if (e.isDir) return setPath(e.path);
    if (e.isSymlink) {
      try {
        await qc.fetchQuery({ queryKey: [...filesKey(backend), "list", e.path], queryFn: () => backend.list(e.path), staleTime: 0, retry: false });
        return setPath(e.path);
      } catch {
        /* a link to a file: show it in the preview instead */
      }
    }
    setSelected(e);
  };

  const submitPath = () => {
    const p = normalizeTyped(pathInput);
    if (!p) {
      toast.error("Enter an absolute path starting with /");
      return;
    }
    setPath(p);
  };

  const download = () =>
    selected &&
    guarded("Could not start the download", async () => {
      const dest = await DialogService.PickSavePath("Save to", selected.isDir ? `${selected.name}.tar` : selected.name);
      if (dest) started(await backend.download(selected.path, dest), "Download");
    });

  const upload = (kind: "file" | "dir") =>
    backend.upload &&
    guarded("Could not start the upload", async () => {
      const src = kind === "file" ? await DialogService.PickFile("Choose a file to upload", "", "") : await DialogService.PickDirectory("Choose a folder to upload");
      if (src) started(await backend.upload!(path, src), "Upload");
    });

  const remove = (e: FileEntry) =>
    backend.remove &&
    guarded(`Could not delete ${e.path}`, async () => {
      await backend.remove!(e.path);
      toast.success(`Deleted ${e.path}`);
      setSelected(null);
      refresh();
    });

  const reindex = async () => {
    if (!backend.refreshIndex) return;
    setReindexing(true);
    try {
      await backend.refreshIndex();
      refresh();
    } catch (e) {
      toastError("Could not refresh the file index", e);
    } finally {
      setReindexing(false);
    }
  };

  const createFolder = async (name: string) => {
    await backend.mkdir?.(path, name);
    toast.success(`Created ${joinPath(path, name)}`);
    refresh();
  };

  return {
    path,
    go: setPath,
    goUp: () => setPath(parentPath(path)),
    pathInput,
    setPathInput,
    submitPath,
    selected,
    setSelected,
    listing,
    entries: listing.data?.entries ?? [],
    preview,
    previewable,
    refresh,
    open,
    busy,
    download,
    upload,
    remove,
    reindexing,
    reindex,
    mkdirOpen,
    setMkdirOpen,
    createFolder,
    confirmDelete,
    setConfirmDelete,
    canWrite,
    writeReason,
    deleteReason,
  };
}
