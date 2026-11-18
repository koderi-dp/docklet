import { useEffect, useRef } from "react";
import { Copy, Download, Maximize2, Minimize2, MoreHorizontal, Search, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import type { PaneSource, TickedItem } from "@/lib/logPanes";
import { PaneSourceMenu } from "./PaneSourceMenu";

interface Props {
  source: PaneSource;
  items: readonly TickedItem[];
  lineCount: number;
  text: string;
  onText: (t: string) => void;
  textError: string | null;
  searchOpen: boolean;
  onSearchOpen: (open: boolean) => void;
  note: string | null;
  maximised: boolean;
  onToggleMax: () => void;
  canExport: boolean;
  onCopy: () => void;
  onDownload: () => void;
  onAll: (on: boolean) => void;
  onNames: (names: readonly string[], on: boolean) => void;
  empty: boolean;
}

/** Compact (32 px) header of a pane: source dropdown, line count, optional text filter and the pane menu. */
export function PaneHeader(p: Props) {
  const input = useRef<HTMLInputElement>(null);
  useEffect(() => {
    if (p.searchOpen) input.current?.focus();
  }, [p.searchOpen]);
  const filtering = p.text !== "";
  const closeSearch = () => {
    p.onText("");
    p.onSearchOpen(false);
  };
  return (
    <div className="shrink-0 border-b border-border bg-card">
      <div className="flex h-8 items-center gap-1 px-1.5">
        <PaneSourceMenu source={p.source} items={p.items} onAll={p.onAll} onNames={p.onNames} prominent={p.empty} />
        {p.note && (
          <span className="min-w-0 truncate text-[11px] text-muted-foreground" title={`Ticked: ${p.note}`}>
            + {p.note}
          </span>
        )}
        <span className="ml-auto shrink-0 pl-1 text-xs text-muted-foreground tabular-nums">
          {p.lineCount.toLocaleString()} {p.lineCount === 1 ? "line" : "lines"}
        </span>
        <Button
          variant="ghost"
          size="icon-xs"
          aria-label="Filter this pane"
          aria-pressed={p.searchOpen}
          title="Filter this pane: hides the lines that do not match (same patterns as the search)"
          className={cn("shrink-0", (p.searchOpen || filtering) && "bg-primary/10 text-foreground")}
          onClick={() => p.onSearchOpen(!p.searchOpen)}
        >
          <Search />
        </Button>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon-xs" aria-label="Pane menu" title="Pane menu" className="shrink-0">
              <MoreHorizontal />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-48">
            <DropdownMenuItem onSelect={p.onToggleMax}>
              {p.maximised ? <Minimize2 /> : <Maximize2 />} {p.maximised ? "Restore layout" : "Maximise this pane"}
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem disabled={!p.canExport} onSelect={p.onCopy}>
              <Copy /> Copy visible
            </DropdownMenuItem>
            <DropdownMenuItem disabled={!p.canExport} onSelect={p.onDownload}>
              <Download /> Download visible
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
      {p.searchOpen && (
        <div className="relative px-1.5 pb-1.5">
          <Input
            ref={input}
            value={p.text}
            onChange={(e) => p.onText(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Escape") closeSearch();
            }}
            placeholder="Filter this pane: text, GET /api/*, /timeout/"
            aria-label="Filter this pane"
            aria-invalid={!!p.textError}
            className={cn("h-7 pr-7 text-xs", p.textError && "border-destructive")}
          />
          <button type="button" aria-label="Clear and close pane filter" title="Clear and close (Esc)" onClick={closeSearch} className="absolute top-[3px] right-2.5 rounded p-1 text-muted-foreground hover:bg-muted hover:text-foreground">
            <X className="size-3" />
          </button>
          {p.textError && (
            <p role="alert" className="pt-1 text-[11px] text-destructive">
              Invalid regular expression: {p.textError}. Not applied.
            </p>
          )}
        </div>
      )}
    </div>
  );
}
