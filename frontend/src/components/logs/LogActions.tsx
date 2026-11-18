import type { ReactNode } from "react";
import { Clock, Copy, Download, Eraser, Pause, Play, WrapText } from "lucide-react";
import { Button } from "@/components/ui/button";
import { IconButton } from "@/components/shared/IconButton";

interface Props {
  /** Extra controls before the standard ones (e.g. the tail size). */
  extra?: ReactNode;
  paused: boolean;
  onPause: () => void;
  canPause: boolean;
  wrap: boolean;
  onWrap: (v: boolean) => void;
  timestamps: boolean;
  onTimestamps: (v: boolean) => void;
  hasVisible: boolean;
  hasLines: boolean;
  onCopy: () => void;
  onDownload: () => void;
  onClear: () => void;
}

/** Status line and actions under the filter bar; shared by the Logs screen and the single-container viewer. */
export function LogActions(p: Props) {
  return (
    <div className="flex flex-wrap items-center gap-1.5">
        {p.extra}
        <Button size="sm" variant="outline" onClick={p.onPause} disabled={!p.canPause} title={p.paused ? "Show new lines again" : "Freeze the view; new lines keep arriving in the background"}>
          {p.paused ? <Play className="size-3.5" /> : <Pause className="size-3.5" />}
          {p.paused ? "Resume" : "Pause"}
        </Button>
        <Button size="sm" variant={p.wrap ? "secondary" : "outline"} aria-pressed={p.wrap} onClick={() => p.onWrap(!p.wrap)} title="Wrap long lines">
          <WrapText className="size-3.5" /> Wrap
        </Button>
        <Button size="sm" variant={p.timestamps ? "secondary" : "outline"} aria-pressed={p.timestamps} onClick={() => p.onTimestamps(!p.timestamps)} title="Show the Engine timestamp of each line">
          <Clock className="size-3.5" /> Time
        </Button>
        <IconButton variant="outline" label="Copy visible" tooltip="Copy the lines currently shown (after filters)" onClick={p.onCopy} disabled={!p.hasVisible}>
          <Copy className="size-3.5" />
        </IconButton>
        <IconButton variant="outline" label="Download" tooltip="Download the lines currently shown" onClick={p.onDownload} disabled={!p.hasVisible}>
          <Download className="size-3.5" />
        </IconButton>
        <IconButton variant="outline" label="Clear view" tooltip="Clears this view only; the containers' logs are not touched" onClick={p.onClear} disabled={!p.hasLines}>
          <Eraser className="size-3.5" />
        </IconButton>
    </div>
  );
}
