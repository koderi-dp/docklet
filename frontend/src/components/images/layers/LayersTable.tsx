import { useEffect, useMemo, useRef, useState } from "react";
import type { ImageLayersReport } from "@bindings/model";
import { Switch } from "@/components/ui/switch";
import { Label } from "@/components/ui/label";
import { LAYER_GRID, LayerRow } from "./LayerRow";
import { LayerHints } from "./LayerHints";

export function LayersTable({ report }: { report: ImageLayersReport }) {
  const [hideEmpty, setHideEmpty] = useState(true);
  const [expanded, setExpanded] = useState<number | null>(null);
  const [flash, setFlash] = useState<number | null>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const steps = useMemo(() => report.steps ?? [], [report.steps]);
  const visible = useMemo(() => (hideEmpty ? steps.filter((s) => s.size > 0) : steps), [steps, hideEmpty]);
  const hiddenCount = steps.length - visible.length;

  useEffect(() => {
    if (flash == null) return;
    listRef.current?.querySelector(`[data-step="${flash}"]`)?.scrollIntoView({ block: "nearest" });
  }, [flash, visible]);

  const jump = (step: number) => {
    const target = steps.find((s) => s.index === step);
    if (target && target.size <= 0) setHideEmpty(false);
    setExpanded(step);
    setFlash(step);
  };

  return (
    <div className="space-y-4">
      <div>
        <div className="mb-2 flex items-center justify-between">
          <h3 className="text-sm font-semibold">Build steps</h3>
          <div className="flex items-center gap-2">
            <Switch id="layers-hide-empty" checked={hideEmpty} onCheckedChange={setHideEmpty} />
            <Label htmlFor="layers-hide-empty" className="text-[13px] font-normal">
              Hide empty steps{hideEmpty && hiddenCount > 0 ? ` (${hiddenCount})` : ""}
            </Label>
          </div>
        </div>
        <div className="overflow-hidden rounded-[10px] border border-border bg-card">
          <div className={`${LAYER_GRID} h-8 border-b border-border bg-muted/40 text-xs text-muted-foreground`}>
            <span>#</span>
            <span>Type</span>
            <span>Instruction</span>
            <span className="text-right">Size</span>
            <span>Share</span>
            <span className="text-right">Age</span>
          </div>
          <div ref={listRef} className="max-h-[40vh] overflow-auto">
            {visible.length === 0 && <p className="px-3 py-6 text-center text-[13px] text-muted-foreground">No steps to show.</p>}
            {visible.map((s) => (
              <div key={s.index} data-step={s.index} className={flash === s.index ? "ring-1 ring-inset ring-primary/50" : undefined}>
                <LayerRow step={s} expanded={expanded === s.index} isLargest={s.index === report.largestStep} onToggle={() => setExpanded(expanded === s.index ? null : s.index)} />
              </div>
            ))}
          </div>
        </div>
      </div>
      <LayerHints hints={report.hints ?? []} onJump={jump} />
    </div>
  );
}
