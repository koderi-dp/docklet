import { X } from "lucide-react";
import type { FileEntry, FileText } from "@bindings/model";
import { Skeleton } from "@/components/ui/skeleton";
import { IconButton } from "@/components/shared/IconButton";
import { formatBytes } from "@/components/containers/format";
import { errorMessage } from "@/lib/errors";

interface Props {
  entry: FileEntry;
  /** Path to preview inline, or null when the entry cannot be previewed (link, too large). */
  previewable: string | null;
  preview: { isLoading: boolean; error: unknown; data?: FileText };
  onClose: () => void;
}

export function FilePreviewPane({ entry, previewable, preview, onClose }: Props) {
  return (
    <aside className="flex min-h-0 w-[360px] shrink-0 flex-col rounded-[10px] border border-border bg-card" aria-label="Preview">
      <div className="flex items-center gap-2 border-b border-border px-3 py-2">
        <span className="min-w-0 flex-1 truncate font-mono text-xs" title={entry.path}>{entry.path}</span>
        <IconButton label="Close preview" className="size-6" onClick={onClose}><X className="size-3.5" /></IconButton>
      </div>
      <div className="min-h-0 flex-1 overflow-auto p-3 text-xs">
        {!previewable ? (
          <p className="text-muted-foreground">
            {entry.isSymlink ? `Symbolic link to ${entry.linkTarget || "?"}. Open the target to preview it.` : `Too large for an inline preview (${formatBytes(entry.size)}). Download it instead.`}
          </p>
        ) : preview.isLoading ? (
          <Skeleton className="h-32 w-full" />
        ) : preview.error ? (
          <p className="text-muted-foreground">{errorMessage(preview.error)}</p>
        ) : (
          <>
            <pre className="whitespace-pre-wrap break-all font-mono">{preview.data?.content}</pre>
            {preview.data?.truncated && <p className="mt-2 text-warning">Showing only the first part of the file ({formatBytes(preview.data.size)} total).</p>}
          </>
        )}
      </div>
    </aside>
  );
}
