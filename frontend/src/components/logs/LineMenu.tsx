import { Copy, EyeOff, Filter, Highlighter, ScanSearch } from "lucide-react";
import { ContextMenuContent, ContextMenuItem, ContextMenuLabel, ContextMenuSeparator } from "@/components/ui/context-menu";
import { literalPattern } from "@/lib/logFilter/pattern";
import type { RuleKind, RuleTarget } from "@/lib/logFilter/rules";
import type { LogEntry } from "@/lib/logEntry";

/** The line (and selection, if any) the context menu was opened on. */
export interface LineContext {
  entry: LogEntry;
  selection: string;
}

export interface LineActions {
  addRule: (kind: RuleKind, target: RuleTarget, pattern: string) => void;
  copy: (text: string) => void;
}

const MAX_PATTERN = 300;
const preview = (s: string, n = 44) => (s.length > n ? `${s.slice(0, n)}…` : s);

/** Right-click menu of a log line: build filter rules from the log itself. */
export function LineMenu({ ctx, actions, showContainer }: { ctx: LineContext | null; actions: LineActions; showContainer: boolean }) {
  const entry = ctx?.entry;
  const basis = (ctx?.selection || entry?.text.trim() || "").slice(0, MAX_PATTERN);
  // A whole line usually carries counters, ids and durations: numbers become wildcards so the rule matches its siblings.
  const pattern = ctx?.selection ? literalPattern(basis) : literalPattern(basis).replace(/\d+/g, "*").replace(/\*{2,}/g, "*");
  const usable = !!entry && basis !== "";
  const add = (kind: RuleKind) => () => actions.addRule(kind, "message", pattern);
  return (
    <ContextMenuContent className="w-60">
      <ContextMenuLabel className="truncate font-mono text-xs font-normal text-muted-foreground" title={basis}>
        {usable ? `${ctx?.selection ? "Selection" : "Line"}: ${preview(basis)}` : "No line here"}
      </ContextMenuLabel>
      <ContextMenuItem disabled={!usable} onSelect={add("include")}>
        <Filter /> Include this
      </ContextMenuItem>
      <ContextMenuItem disabled={!usable} onSelect={add("exclude")}>
        <EyeOff /> Exclude this
      </ContextMenuItem>
      <ContextMenuItem disabled={!usable} onSelect={add("highlight")}>
        <Highlighter /> Highlight this
      </ContextMenuItem>
      {showContainer && (
        <>
          <ContextMenuSeparator />
          <ContextMenuItem disabled={!entry} onSelect={() => entry && actions.addRule("include", "container", literalPattern(entry.container))}>
            <ScanSearch /> Only this container
          </ContextMenuItem>
          <ContextMenuItem disabled={!entry} onSelect={() => entry && actions.addRule("exclude", "container", literalPattern(entry.container))}>
            <EyeOff /> Hide this container
          </ContextMenuItem>
        </>
      )}
      <ContextMenuSeparator />
      <ContextMenuItem disabled={!entry} onSelect={() => entry && actions.copy(ctx?.selection || entry.text)}>
        <Copy /> {ctx?.selection ? "Copy selection" : "Copy line"}
      </ContextMenuItem>
    </ContextMenuContent>
  );
}
