import { ChevronDown, ChevronUp, ListFilter, Search, X } from "lucide-react";
import { IconButton } from "@/components/shared/IconButton";
import { cn } from "@/lib/utils";
import type { LogSearchState } from "./useLogSearch";

interface Counter {
  text: string;
  /** Shorter form for a narrow box. */
  short: string;
  tone: string;
}

function counter(s: LogSearchState): Counter | null {
  if (s.query.trim() === "") return null;
  if (s.error) return { text: "Invalid pattern", short: "Invalid", tone: "text-destructive" };
  if (s.pending) return null;
  const n = s.total.toLocaleString();
  if (s.total === 0) return { text: "No matches", short: "None", tone: "text-muted-foreground" };
  if (s.position === 0) return { text: `${n} ${s.total === 1 ? "match" : "matches"}`, short: n, tone: "text-muted-foreground" };
  const p = s.position.toLocaleString();
  return { text: `${p} of ${n}`, short: `${p}/${n}`, tone: "text-foreground" };
}

interface Props {
  search: LogSearchState;
  /** Adds the text as an Include / Message rule (and clears the search). Absent: no such button. */
  onKeep?: () => void;
  className?: string;
}

/**
 * Search of the log view: marks matches and walks through them, never hides a line. Enter = older match (starting
 * at the newest), Shift+Enter = newer, Esc clears. Clear, "Keep as filter", the counter and the arrows sit inside the
 * right end of the box and appear once there is text.
 */
export function LogSearchBox({ search: s, onKeep, className }: Props) {
  const has = s.query !== "";
  const c = counter(s);
  const canStep = s.valid && s.total > 0;
  const keep = !!onKeep && s.valid;
  return (
    <div
      aria-invalid={!!s.error}
      className={cn(
        "@container flex h-8 min-w-0 flex-1 items-center gap-1 rounded-md border border-input bg-transparent pr-1 pl-2 text-[13px] shadow-xs transition-[color,box-shadow] dark:bg-input/30",
        "focus-within:border-ring focus-within:ring-[3px] focus-within:ring-ring/50",
        "aria-invalid:border-destructive aria-invalid:ring-destructive/20",
        className,
      )}
    >
      <Search className="pointer-events-none size-3.5 shrink-0 text-muted-foreground" />
      <input
        value={s.query}
        onChange={(e) => s.setQuery(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter") {
            e.preventDefault();
            // A log is read from its newest line: Enter walks back in time, starting at the latest match.
            if (e.shiftKey) s.next();
            else s.prev();
          } else if (e.key === "Escape" && has) {
            e.preventDefault();
            s.clear();
          }
        }}
        placeholder="Search logs"
        aria-label="Search logs"
        aria-invalid={!!s.error}
        title="Finds lines without hiding any. Text, GET /api/*, /timeout|refused/"
        className="h-full min-w-0 flex-1 bg-transparent text-[13px] outline-none selection:bg-primary selection:text-primary-foreground placeholder:text-muted-foreground"
      />
      {has && (
        <>
          <button type="button" aria-label="Clear search" title="Clear (Esc)" onClick={s.clear} className="shrink-0 rounded p-1 text-muted-foreground hover:bg-muted hover:text-foreground">
            <X className="size-3" />
          </button>
          {keep && (
            <button
              type="button"
              onClick={onKeep}
              title="Show only lines that match (adds a filter)"
              aria-label="Keep as filter"
              className="inline-flex h-6 shrink-0 items-center gap-1 rounded px-1.5 text-[11px] text-muted-foreground hover:bg-muted hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/60 focus-visible:outline-none"
            >
              <ListFilter className="size-3" />
              <span className="hidden @[440px]:inline">Keep as filter</span>
            </button>
          )}
          {c && (
            <span role="status" aria-label={c.text} className={cn("shrink-0 px-1 text-xs tabular-nums", c.tone)}>
              <span className="hidden @[360px]:inline">{c.text}</span>
              <span className="@[360px]:hidden">{c.short}</span>
            </span>
          )}
          {s.valid && (
            <div className="flex shrink-0 items-center">
              <IconButton label="Older match" tooltip={canStep ? "Older match (Enter)" : "No matches"} disabled={!canStep} onClick={s.prev} className="size-6">
                <ChevronUp className="size-4" />
              </IconButton>
              <IconButton label="Newer match" tooltip={canStep ? "Newer match (Shift+Enter)" : "No matches"} disabled={!canStep} onClick={s.next} className="size-6">
                <ChevronDown className="size-4" />
              </IconButton>
            </div>
          )}
        </>
      )}
    </div>
  );
}
