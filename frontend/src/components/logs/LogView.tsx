import { useCallback, useMemo, useState, type ReactNode } from "react";
import { ArrowDownToLine } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { downloadText } from "@/lib/downloadText";
import { formatEntries, type LogEntry } from "@/lib/logEntry";
import type { EvalResult } from "@/lib/logFilter/rules";
import { LogActions } from "./LogActions";
import { LogFilterBar } from "./LogFilterBar";
import { LogLines } from "./LogLines";
import type { LineActions } from "./LineMenu";
import type { LogFilterState } from "./useLogFilter";

interface Props {
  /** Every loaded line (before filtering): used for "n of N lines". */
  total: number;
  filter: LogFilterState;
  result: EvalResult<LogEntry>;
  showContainer: boolean;
  tagWidth: number;
  /** Status text on the left of the action row. */
  status: ReactNode;
  /** Extra controls in the action row. */
  extra?: ReactNode;
  paused: boolean;
  onPause: () => void;
  canPause: boolean;
  onClear: () => void;
  /** Shown in the list area when no line has been loaded. */
  emptyNoLines: ReactNode;
  ariaLabel: string;
  /** File name (without extension) for download. */
  downloadName: string;
  /** Replaces the single list (the split view draws its panes here). Wrap and Time stay global. */
  renderBody?: (ctx: { wrap: boolean; timestamps: boolean; actions: LineActions; empty: ReactNode }) => ReactNode;
  className?: string;
}

/** Filter bar, action row and virtualised list with follow: the part the Logs screen and the single-container viewer share. */
export function LogView(p: Props) {
  const { filter, result } = p;
  const [wrap, setWrap] = useState(true);
  const [timestamps, setTimestamps] = useState(true);
  const [follow, setFollow] = useState(true);
  const visible = result.visible;

  const copyVisible = () => {
    navigator.clipboard
      .writeText(formatEntries(visible, { timestamps, container: p.showContainer }))
      .then(() => toast.success(`Copied ${visible.length.toLocaleString()} visible line${visible.length === 1 ? "" : "s"}`))
      .catch((e: unknown) => toast.error("Could not copy logs", { description: String(e) }));
  };
  const download = () => downloadText(`${p.downloadName}.log`, formatEntries(visible, { timestamps, container: p.showContainer }) + "\n");

  const copyText = useCallback((text: string) => {
    navigator.clipboard.writeText(text).catch((e: unknown) => toast.error("Could not copy", { description: String(e) }));
  }, []);
  const addRule = filter.addRule;
  const actions: LineActions = useMemo(
    () => ({ addRule: (kind, target, pattern) => addRule({ kind, target, pattern }), copy: copyText }),
    [addRule, copyText],
  );

  const filteredOut = p.total > 0 && visible.length === 0;
  const empty = filteredOut ? (
    <div className="flex flex-col items-center gap-2 px-6 py-12 text-center text-sm text-muted-foreground">
      <p className="font-medium text-foreground">No lines match the current filters</p>
      <p>{p.total.toLocaleString()} loaded lines are hidden by the rules, levels or time range.</p>
      <Button size="sm" variant="outline" onClick={filter.clearAll}>
        Clear filters
      </Button>
    </div>
  ) : (
    p.emptyNoLines
  );

  return (
    <div className={cn("flex min-h-0 flex-col gap-2", p.className)}>
      <LogFilterBar
        filter={filter}
        result={result}
        trailing={
          <LogActions
            extra={p.extra}
            paused={p.paused}
            onPause={p.onPause}
            canPause={p.canPause}
            wrap={wrap}
            onWrap={setWrap}
            timestamps={timestamps}
            onTimestamps={setTimestamps}
            hasVisible={visible.length > 0}
            hasLines={p.total > 0}
            onCopy={copyVisible}
            onDownload={download}
            onClear={p.onClear}
          />
        }
      />
      <div className="min-w-0 text-xs text-muted-foreground">{p.status}</div>
      {p.renderBody ? (
        <div className="min-h-0 flex-1">{p.renderBody({ wrap, timestamps, actions, empty })}</div>
      ) : (
        <div className="relative min-h-0 flex-1">
          <LogLines
            entries={visible}
            highlights={result.highlights}
            showContainer={p.showContainer}
            tagWidth={p.tagWidth}
            timestamps={timestamps}
            wrap={wrap}
            follow={follow}
            onFollowChange={setFollow}
            actions={actions}
            ariaLabel={p.ariaLabel}
            empty={empty}
          />
          {!follow && visible.length > 0 && (
            <Button size="sm" className="absolute right-4 bottom-3 shadow-sm" onClick={() => setFollow(true)}>
              <ArrowDownToLine className="size-3.5" /> Jump to latest
            </Button>
          )}
        </div>
      )}
    </div>
  );
}
