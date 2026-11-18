import { Fragment, useCallback, useState, type ReactNode } from "react";
import { ResizableHandle, ResizablePanel, ResizablePanelGroup } from "@/components/ui/resizable";
import type { LogEntry } from "@/lib/logEntry";
import type { HighlightColour } from "@/lib/logFilter/rules";
import type { LayoutId, PaneSource, TickedItem } from "@/lib/logPanes";
import type { GroupSizes } from "@/stores/logSplitStore";
import type { LineActions } from "../LineMenu";
import { LogPane } from "./LogPane";
import type { PaneNote } from "./usePaneAssignments";

const MIN_WIDTH = 220;
const MIN_HEIGHT = 96;

interface SplitProps {
  id: string;
  orientation: "horizontal" | "vertical";
  sizes: Record<string, GroupSizes>;
  onSizes: (group: string, sizes: GroupSizes) => void;
  children: ReactNode[];
}

/** A resizable group of equally sized panels; double-clicking a divider puts the panels back to equal. */
function Split({ id, orientation, sizes, onSizes, children }: SplitProps) {
  const equal = `${100 / children.length}%`;
  return (
    <ResizablePanelGroup orientation={orientation} id={id} defaultLayout={sizes[id]} onLayoutChanged={(l) => onSizes(id, l)}>
      {children.map((c, i) => (
        <Fragment key={i}>
          {i > 0 && (
            <ResizableHandle
              aria-label="Resize panes (double-click to reset)"
              className="bg-transparent transition-colors after:hidden hover:bg-primary/30 data-[separator=active]:bg-primary/40 data-[separator=hover]:bg-primary/30 w-1.5 aria-[orientation=horizontal]:h-1.5"
            />
          )}
          <ResizablePanel id={`p${i}`} defaultSize={equal} minSize={orientation === "horizontal" ? MIN_WIDTH : MIN_HEIGHT}>
            {c}
          </ResizablePanel>
        </Fragment>
      ))}
    </ResizablePanelGroup>
  );
}

interface Props {
  /** The layout as drawn: columns and the grid arrive here already stacked when the window is narrow. */
  layout: LayoutId | "stack2" | "stack4";
  panes: PaneSource[];
  items: readonly TickedItem[];
  visible: LogEntry[];
  highlights: Array<HighlightColour | undefined>;
  wrap: boolean;
  timestamps: boolean;
  actions: LineActions;
  streamEmpty: ReactNode;
  note: PaneNote | null;
  sizes: Record<string, GroupSizes>;
  onSizes: (group: string, sizes: GroupSizes) => void;
  onPaneAll: (index: number, on: boolean) => void;
  onPaneNames: (index: number, names: readonly string[], on: boolean) => void;
  downloadName: string;
}

/** The panes of the split view in their layout. Panes are views over one shared, already filtered stream. */
export function SplitLayout(p: Props) {
  const [maximised, setMaximised] = useState<number | null>(null);
  const [texts, setTexts] = useState<string[]>([]);
  const onText = useCallback((i: number, t: string) => setTexts((prev) => Object.assign([...prev], { [i]: t })), []);
  const onToggleMax = useCallback((i: number) => setMaximised((cur) => (cur === i ? null : i)), []);

  const pane = (i: number) => {
    const source = p.panes[i];
    if (!source) return null;
    return (
      <LogPane
        key={i}
        index={i}
        source={source}
        items={p.items}
        visible={p.visible}
        highlights={p.highlights}
        wrap={p.wrap}
        timestamps={p.timestamps}
        actions={p.actions}
        text={texts[i] ?? ""}
        note={p.note?.pane === i ? p.note.text : null}
        maximised={maximised === i}
        streamEmpty={p.streamEmpty}
        downloadName={p.downloadName}
        onText={onText}
        onToggleMax={onToggleMax}
        onAll={p.onPaneAll}
        onNames={p.onPaneNames}
      />
    );
  };

  const group = (id: string, orientation: "horizontal" | "vertical", nodes: ReactNode[]) => (
    <Split id={id} orientation={orientation} sizes={p.sizes} onSizes={p.onSizes}>
      {nodes}
    </Split>
  );

  if (maximised !== null && p.panes[maximised]) return <div className="h-full">{pane(maximised)}</div>;
  switch (p.layout) {
    case "cols":
      return group("cols", "horizontal", [pane(0), pane(1)]);
    case "rows":
    case "stack2":
      return group(p.layout, "vertical", [pane(0), pane(1)]);
    case "stack4":
      return group("stack4", "vertical", [pane(0), pane(1), pane(2), pane(3)]);
    case "grid":
      return group("grid", "vertical", [group("grid-top", "horizontal", [pane(0), pane(1)]), group("grid-bottom", "horizontal", [pane(2), pane(3)])]);
    default:
      return null;
  }
}
