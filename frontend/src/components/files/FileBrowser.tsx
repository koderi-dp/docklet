import type { ReactNode } from "react";
import { ArrowUp, Download, FilePlus2, Folder, FolderPlus, RefreshCw, Trash2, Upload } from "lucide-react";
import type { FileBackend } from "@/hooks/useFiles";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { IconButton } from "@/components/shared/IconButton";
import { errorMessage } from "@/lib/errors";
import { cn } from "@/lib/utils";
import { DeleteFileDialog, MkdirDialog } from "./FileBrowserDialogs";
import { FilePathBar } from "./FilePathBar";
import { FilePreviewPane } from "./FilePreviewPane";
import { FileTable } from "./FileTable";
import { useFileBrowser } from "./useFileBrowser";

/** A control that is either enabled or disabled with a visible reason. */
function Reasoned({ reason, children }: { reason?: string | null; children: ReactNode }) {
  if (!reason) return <>{children}</>;
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span tabIndex={0}>{children}</span>
      </TooltipTrigger>
      <TooltipContent>{reason}</TooltipContent>
    </Tooltip>
  );
}

export interface FileBrowserProps {
  backend: FileBackend;
  /** Shown in the breadcrumb root, e.g. the volume name. */
  rootLabel?: string;
  /** Explains why changing files is not possible (hides nothing, disables with this reason). */
  writeDisabledReason?: string | null;
  deleteDisabledReason?: string | null;
  className?: string;
  /** Extra toolbar controls rendered at the right end of the toolbar row (e.g. Export filesystem). */
  toolbarExtra?: ReactNode;
  /** Muted note appended to the status line. */
  note?: ReactNode;
}

export function FileBrowser({ backend, rootLabel, writeDisabledReason, deleteDisabledReason, className, toolbarExtra, note }: FileBrowserProps) {
  const b = useFileBrowser(backend, { writeDisabledReason, deleteDisabledReason });
  const { path, listing, selected } = b;

  return (
    <div className={cn("flex min-h-0 flex-col gap-2", className)}>
      <div className="@container flex h-9 flex-nowrap items-center gap-2">
        <IconButton label="Parent directory" variant="outline" disabled={path === "/"} onClick={b.goUp}>
          <ArrowUp className="size-3.5" />
        </IconButton>
        <IconButton label="Refresh" variant="outline" onClick={b.refresh}>
          <RefreshCw className={cn("size-3.5", listing.isFetching && "animate-spin")} />
        </IconButton>
        <FilePathBar path={path} rootLabel={rootLabel} input={b.pathInput} onInputChange={b.setPathInput} onSubmit={b.submitPath} onGo={b.go} />
        <DropdownMenu>
          <Reasoned reason={b.canWrite ? null : b.writeReason}>
            <DropdownMenuTrigger asChild>
              <Button size="sm" variant="outline" disabled={!b.canWrite || b.busy} aria-label="Upload" title="Upload">
                <Upload className="size-3.5" /> <span className="hidden @[860px]:inline">Upload</span>
              </Button>
            </DropdownMenuTrigger>
          </Reasoned>
          <DropdownMenuContent align="end">
            <DropdownMenuItem onSelect={() => void b.upload("file")}><FilePlus2 className="size-3.5" /> Upload file…</DropdownMenuItem>
            <DropdownMenuItem onSelect={() => void b.upload("dir")}><Folder className="size-3.5" /> Upload folder…</DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
        <Button size="sm" variant="outline" disabled={!selected || b.busy} onClick={() => void b.download()} aria-label="Download" title="Download">
          <Download className="size-3.5" /> <span className="hidden @[860px]:inline">Download</span>
        </Button>
        <Reasoned reason={b.canWrite ? null : b.writeReason}>
          <Button size="sm" variant="outline" disabled={!backend.mkdir || !b.canWrite} onClick={() => b.setMkdirOpen(true)} aria-label="New folder" title="New folder">
            <FolderPlus className="size-3.5" /> <span className="hidden @[860px]:inline">New folder</span>
          </Button>
        </Reasoned>
        <Reasoned reason={b.deleteReason}>
          <Button
            size="sm"
            variant="outline"
            disabled={!!b.deleteReason || !selected || b.busy}
            className="text-destructive hover:text-destructive"
            onClick={() => selected && b.setConfirmDelete(selected)}
            aria-label="Delete"
            title="Delete"
          >
            <Trash2 className="size-3.5" /> <span className="hidden @[860px]:inline">Delete</span>
          </Button>
        </Reasoned>
        {toolbarExtra && (
          <>
            <span aria-hidden className="h-5 w-px shrink-0 bg-border" />
            {toolbarExtra}
          </>
        )}
      </div>

      <div className="flex min-h-0 flex-1 gap-3">
        <div className="min-h-0 flex-1 overflow-auto rounded-[10px] border border-border bg-card">
          {listing.isLoading ? (
            <div className="space-y-2 p-3">
              {Array.from({ length: 6 }).map((_, i) => <Skeleton key={i} className="h-8 w-full" />)}
              {note && <p className="text-xs text-muted-foreground">Reading the file tree. The first listing of a stopped container can take a few seconds.</p>}
            </div>
          ) : listing.error ? (
            <div className="p-6 text-sm">
              <p className="font-medium text-destructive">Could not list {path}</p>
              <p className="mt-1 whitespace-pre-wrap text-muted-foreground">{errorMessage(listing.error)}</p>
              <div className="mt-3 flex gap-2">
                <Button size="sm" variant="outline" onClick={b.refresh}>Retry</Button>
                {path !== "/" && <Button size="sm" variant="outline" onClick={b.goUp}>Go to parent directory</Button>}
              </div>
            </div>
          ) : (
            <FileTable path={path} entries={b.entries} selected={selected} onSelect={b.setSelected} onOpen={(e) => void b.open(e)} onGoUp={b.goUp} />
          )}
        </div>

        {selected && !selected.isDir && (
          <FilePreviewPane entry={selected} previewable={b.previewable} preview={b.preview} onClose={() => b.setSelected(null)} />
        )}
      </div>

      {listing.data?.truncated && (
        <p className="text-xs text-warning" role="status">
          Listing truncated: only the first {listing.data.limit.toLocaleString()} entries of this directory are shown.
        </p>
      )}
      {listing.data && (
        <p className="text-xs text-muted-foreground">
          {b.entries.length} item{b.entries.length === 1 ? "" : "s"}
          {listing.data.method === "archive" ? " · listed through the archive API (also works on stopped containers)" : ""}
          {note ? <> · {note}</> : null}
          {listing.data.method === "index" && listing.data.indexedAt > 0 && (
            <>
              {" · "}Indexed {listing.data.indexedEntries.toLocaleString()} entries · refreshed {new Date(listing.data.indexedAt).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}
              {backend.refreshIndex && (
                <Button size="sm" variant="ghost" className="ml-1 h-5 px-1.5 text-xs" disabled={b.reindexing} onClick={() => void b.reindex()}>
                  <RefreshCw className={cn("size-3", b.reindexing && "animate-spin")} /> {b.reindexing ? "Refreshing…" : "Refresh"}
                </Button>
              )}
            </>
          )}
        </p>
      )}

      <MkdirDialog open={b.mkdirOpen} onOpenChange={b.setMkdirOpen} parent={path} onCreate={b.createFolder} />
      <DeleteFileDialog entry={b.confirmDelete} onCancel={() => b.setConfirmDelete(null)} onConfirm={(e) => void b.remove(e)} />
    </div>
  );
}
