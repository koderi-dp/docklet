import { Lightbulb } from "lucide-react";
import type { ImageLayerHint } from "@bindings/model";

/** Heuristic pointers derived from instruction text only; wording stays tentative. */
export function LayerHints({ hints, onJump }: { hints: ImageLayerHint[]; onJump: (step: number) => void }) {
  if (hints.length === 0) return null;
  return (
    <section aria-label="Things to check" className="space-y-1.5">
      <h3 className="flex items-center gap-1.5 text-sm font-semibold">
        <Lightbulb className="size-4 text-warning" /> Things to check
      </h3>
      <p className="text-xs text-muted-foreground">Hints based only on the instruction text, not measurements. They may not apply to your build.</p>
      <ul className="space-y-1.5">
        {hints.map((h) => (
          <li key={`${h.step}-${h.title}`} className="rounded-md border border-border bg-muted/30 px-3 py-2 text-[13px]">
            <div className="flex items-baseline gap-2">
              <button type="button" onClick={() => onJump(h.step)} className="shrink-0 font-mono text-xs text-primary hover:underline" title="Show this step">
                Step {h.step}
              </button>
              <span className="font-medium">{h.title}</span>
            </div>
            <p className="mt-0.5 text-xs text-muted-foreground">{h.detail}</p>
          </li>
        ))}
      </ul>
    </section>
  );
}
