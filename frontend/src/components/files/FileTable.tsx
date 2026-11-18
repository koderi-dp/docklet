import { useEffect, useRef } from "react";
import { File as FileIcon, Folder, Link2 } from "lucide-react";
import type { FileEntry } from "@bindings/model";
import { formatBytes } from "@/components/containers/format";
import { cn } from "@/lib/utils";

const ROW = "h-[42px] cursor-pointer border-b border-border/60 hover:bg-accent/50 focus:bg-accent/50 focus:outline-none";

interface Props {
  /** Current directory; a ".." row is shown unless it is the root. */
  path: string;
  entries: FileEntry[];
  selected: FileEntry | null;
  onSelect: (e: FileEntry) => void;
  onOpen: (e: FileEntry) => void;
  onGoUp: () => void;
}

/** The directory listing: click selects, double click / Enter opens, arrows move, Backspace goes up. */
export function FileTable({ path, entries, selected, onSelect, onOpen, onGoUp }: Props) {
  const selectedRow = useRef<HTMLTableRowElement | null>(null);
  useEffect(() => {
    // Block body on purpose: scrollIntoView returns a Promise in current WebView2, and an
    // effect must return nothing or a cleanup function.
    selectedRow.current?.scrollIntoView({ block: "nearest" });
  }, [selected]);

  const onRowKey = (ev: React.KeyboardEvent, e: FileEntry | "up", idx: number) => {
    const rows = (ev.currentTarget.parentElement?.querySelectorAll("tr[data-row]") ?? []) as NodeListOf<HTMLElement>;
    if (ev.key === "Enter") {
      ev.preventDefault();
      if (e === "up") onGoUp();
      else onOpen(e);
    } else if (ev.key === "ArrowDown") {
      ev.preventDefault();
      rows[idx + 1]?.focus();
    } else if (ev.key === "ArrowUp") {
      ev.preventDefault();
      rows[idx - 1]?.focus();
    } else if (ev.key === "Backspace" && path !== "/") {
      ev.preventDefault();
      onGoUp();
    }
  };

  return (
    <table className="w-full text-[13px]">
      <thead className="sticky top-0 bg-card">
        <tr className="border-b border-border text-left text-xs text-muted-foreground">
          <th scope="col" className="h-9 px-3 font-medium">Name</th>
          <th scope="col" className="h-9 w-24 px-3 text-right font-medium">Size</th>
          <th scope="col" className="h-9 w-28 px-3 font-medium">Mode</th>
          <th scope="col" className="h-9 w-44 px-3 font-medium">Modified</th>
        </tr>
      </thead>
      <tbody>
        {path !== "/" && (
          <tr data-row tabIndex={0} className={ROW} onDoubleClick={onGoUp} onKeyDown={(e) => onRowKey(e, "up", 0)}>
            <td className="px-3" colSpan={4}><span className="flex items-center gap-2 text-muted-foreground"><Folder className="size-4" /> ..</span></td>
          </tr>
        )}
        {entries.map((e, i) => {
          const sel = selected?.path === e.path;
          return (
            <tr
              key={e.path}
              data-row
              ref={sel ? selectedRow : undefined}
              tabIndex={0}
              aria-selected={sel}
              className={cn(ROW, sel && "bg-accent")}
              onClick={() => onSelect(e)}
              onDoubleClick={() => onOpen(e)}
              onKeyDown={(ev) => onRowKey(ev, e, i + (path !== "/" ? 1 : 0))}
            >
              <td className="px-3">
                <span className="flex items-center gap-2">
                  {e.isSymlink ? <Link2 className="size-4 shrink-0 text-info" /> : e.isDir ? <Folder className="size-4 shrink-0 text-primary" /> : <FileIcon className="size-4 shrink-0 text-muted-foreground" />}
                  <span className="truncate" title={e.name}>{e.name}</span>
                  {e.isSymlink && e.linkTarget && <span className="truncate text-xs text-muted-foreground" title={e.linkTarget}>→ {e.linkTarget}</span>}
                </span>
              </td>
              <td className="px-3 text-right tabular-nums text-muted-foreground">{e.isDir || e.isSymlink ? "—" : formatBytes(e.size)}</td>
              <td className="px-3 font-mono text-xs text-muted-foreground">{e.mode}</td>
              <td className="px-3 text-xs text-muted-foreground">{e.modTime ? new Date(e.modTime).toLocaleString() : "—"}</td>
            </tr>
          );
        })}
        {entries.length === 0 && (
          <tr><td colSpan={4} className="px-3 py-10 text-center text-muted-foreground">This directory is empty.</td></tr>
        )}
      </tbody>
    </table>
  );
}
