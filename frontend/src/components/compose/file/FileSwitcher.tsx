import type { ComposeFileRef } from "@bindings/model";
import { fileLabel } from "@/lib/composeFileState";
import { cn } from "@/lib/utils";

interface Props {
  files: readonly ComposeFileRef[];
  value: string;
  onChange: (path: string) => void;
}

export function FileSwitcher({ files, value, onChange }: Props) {
  const paths = files.map((f) => f.path);
  return (
    <div role="tablist" aria-label="Project files" className="flex flex-wrap items-center gap-1">
      {files.map((f) => (
        <button
          key={f.path}
          type="button"
          role="tab"
          aria-selected={f.path === value}
          title={f.path}
          onClick={() => onChange(f.path)}
          className={cn(
            "flex items-center gap-1.5 rounded-md border px-2 py-1 font-mono text-xs transition-colors",
            f.path === value ? "border-primary/50 bg-accent text-accent-foreground" : "border-border text-muted-foreground hover:bg-muted",
          )}
        >
          {fileLabel(f.path, paths)}
          {f.kind === "env" && <span className="font-sans text-[10px] uppercase tracking-wide text-muted-foreground">read-only</span>}
        </button>
      ))}
    </div>
  );
}
