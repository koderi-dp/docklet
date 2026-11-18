import { memo, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { ArrowDownToLine, LayoutPanelTop } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { useElementWidth } from "@/hooks/useElementWidth";
import { downloadText } from "@/lib/downloadText";
import { formatEntries, type LogEntry } from "@/lib/logEntry";
import type { HighlightColour } from "@/lib/logFilter/rules";
import { describePane, type PaneLines, type PaneSource, type PaneSummary, type TickedItem } from "@/lib/logPanes";
import { LogLines } from "../LogLines";
import type { LineActions } from "../LineMenu";
import { useLogSearchContext } from "../searchContext";
import { PaneHeader } from "./PaneHeader";
import { usePaneLines } from "./usePaneLines";

export interface PaneProps {
  index: number;
  source: PaneSource;
  items: readonly TickedItem[];
  /** The globally filtered stream (the pane only selects from it) and its highlights. */
  visible: LogEntry[];
  highlights: Array<HighlightColour | undefined>;
  wrap: boolean;
  timestamps: boolean;
  actions: LineActions;
  /** Text filter of this pane only, kept by the layout so it survives maximising. */
  text: string;
  note: string | null;
  maximised: boolean;
  /** Shown when the stream itself has no visible lines (nothing loaded yet, or the global filters hide everything). */
  streamEmpty: ReactNode;
  downloadName: string;
  /** Callbacks take the pane index so the layout can hand every pane the same stable functions. */
  onText: (index: number, t: string) => void;
  onToggleMax: (index: number) => void;
  onAll: (index: number, on: boolean) => void;
  onNames: (index: number, names: readonly string[], on: boolean) => void;
}

/** Tag column width in characters; a narrow pane gives the message more room. */
const tagWidthOf = (names: readonly string[], paneWidth: number) => Math.min(paneWidth > 0 && paneWidth < 520 ? 14 : 24, Math.max(8, ...names.map((n) => n.length)));

interface ViewProps extends Omit<PaneProps, "visible" | "highlights" | "streamEmpty"> {
  lines: PaneLines;
  textError: string | null;
  summary: PaneSummary;
  /** What to show instead of lines (only read when there are none). */
  streamEmpty: ReactNode;
  streamHasLines: boolean;
}

/** One pane of the split view: its share of the shared stream is picked out first, then drawn by PaneView. */
export function LogPane({ visible, highlights, streamEmpty, ...p }: PaneProps) {
  const summary = useMemo(() => describePane(p.source, p.items), [p.source, p.items]);
  const { lines, textError } = usePaneLines(visible, highlights, summary.names, p.source.all, p.text);
  // The search counts the lines the panes show together.
  const { registry } = useLogSearchContext();
  useEffect(() => registry.setPane(p.index, lines.entries), [registry, p.index, lines.entries]);
  useEffect(() => () => registry.setPane(p.index, null), [registry, p.index]);
  // The "empty" node changes with every batch; it only matters while the stream has nothing to show.
  const hasLines = visible.length > 0;
  return <PaneView {...p} lines={lines} textError={textError} summary={summary} streamHasLines={hasLines} streamEmpty={hasLines ? null : streamEmpty} />;
}

/** Re-renders only when this pane's own lines or settings change. */
const PaneView = memo(function PaneView(p: ViewProps) {
  const root = useRef<HTMLElement>(null);
  const width = useElementWidth(root);
  const [follow, setFollow] = useState(true);
  const [searchOpen, setSearchOpen] = useState(p.text !== "");
  const { lines, textError, summary } = p;
  // One container needs no tag column: its name is in the header.
  const showContainer = summary.names.length > 1;
  const none = summary.names.length === 0;
  const count = lines.entries.length;

  const text = () => formatEntries(lines.entries, { timestamps: p.timestamps, container: showContainer });
  const copy = () =>
    navigator.clipboard
      .writeText(text())
      .then(() => toast.success(`Copied ${count.toLocaleString()} visible line${count === 1 ? "" : "s"}`))
      .catch((e: unknown) => toast.error("Could not copy logs", { description: String(e) }));
  const download = () => downloadText(`${p.downloadName}-pane-${p.index + 1}.log`, text() + "\n");

  let empty: ReactNode;
  if (none) {
    empty = (
      <div className="flex flex-col items-center gap-2 px-6 py-10 text-center text-sm text-muted-foreground">
        <LayoutPanelTop className="size-5" />
        <p className="font-medium text-foreground">Nothing selected in this pane</p>
        <p>Use “Showing” above to choose containers from the ticked ones.</p>
      </div>
    );
  } else if (!p.streamHasLines) empty = p.streamEmpty;
  else
    empty = (
      <p className="p-3 text-sm text-muted-foreground">{p.text !== "" && !textError ? "No lines in this pane match its filter." : "These containers have no lines that pass the filters."}</p>
    );

  return (
    <section ref={root} aria-label={`Pane ${p.index + 1}: ${summary.label}`} className="flex h-full min-h-0 min-w-0 flex-col overflow-hidden rounded-md border border-border bg-card">
      <PaneHeader
        source={p.source}
        items={p.items}
        lineCount={count}
        text={p.text}
        onText={(t) => p.onText(p.index, t)}
        textError={textError}
        searchOpen={searchOpen}
        onSearchOpen={setSearchOpen}
        note={p.note}
        maximised={p.maximised}
        onToggleMax={() => p.onToggleMax(p.index)}
        canExport={count > 0}
        onCopy={() => void copy()}
        onDownload={download}
        onAll={(on) => p.onAll(p.index, on)}
        onNames={(n, on) => p.onNames(p.index, n, on)}
        empty={none}
      />
      <div className="relative min-h-0 flex-1">
        <LogLines
          entries={lines.entries}
          highlights={lines.highlights}
          showContainer={showContainer}
          tagWidth={tagWidthOf(summary.names, width)}
          timestamps={p.timestamps}
          wrap={p.wrap}
          follow={follow}
          onFollowChange={setFollow}
          actions={p.actions}
          ariaLabel={`Logs of pane ${p.index + 1}`}
          empty={empty}
          className="min-h-0 rounded-none border-0 bg-muted/30"
        />
        {!follow && count > 0 && (
          <Button size="sm" className="absolute right-3 bottom-2 shadow-sm" onClick={() => setFollow(true)}>
            <ArrowDownToLine className="size-3.5" /> Jump to latest
          </Button>
        )}
      </div>
    </section>
  );
});
