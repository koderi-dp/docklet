import { Check, CircleAlert, Eye, Hammer, RefreshCw, RotateCw } from "lucide-react";
import type { ComposeWatchEvent } from "@bindings/model";
import { cn } from "@/lib/utils";
import { formatClock } from "./watchFormat";

const KIND: Record<string, { icon: React.ReactNode; tone: string }> = {
  enabled: { icon: <Eye />, tone: "text-success" },
  sync: { icon: <RefreshCw />, tone: "text-info" },
  rebuilding: { icon: <Hammer />, tone: "text-warning" },
  rebuilt: { icon: <Check />, tone: "text-success" },
  restarted: { icon: <RotateCw />, tone: "text-info" },
  error: { icon: <CircleAlert />, tone: "text-destructive" },
};

/** What the Compose CLI reported, newest first. Only lines the CLI printed appear here. */
export function WatchActivity({ events, running }: { events: readonly ComposeWatchEvent[]; running: boolean }) {
  if (events.length === 0) {
    return (
      <p className="rounded-[10px] border border-border bg-card px-3 py-3 text-muted-foreground">
        {running ? "Nothing reported yet. Change a watched file and the CLI's reaction shows up here." : "The CLI reported nothing."}
      </p>
    );
  }
  return (
    <ul className="max-h-[360px] overflow-auto rounded-[10px] border border-border bg-card" aria-label="Watch activity">
      {events.map((e) => {
        const k = KIND[e.kind] ?? KIND.error;
        const isError = e.kind === "error";
        return (
          <li key={e.id} className="grid grid-cols-[72px_96px_16px_1fr] items-baseline gap-x-2 border-b border-border/60 px-3 py-1.5 last:border-b-0">
            <span className="font-mono text-xs tabular-nums text-muted-foreground">{formatClock(e.at)}</span>
            <span className="truncate text-xs text-muted-foreground" title={e.service}>
              {e.service || "—"}
            </span>
            <span className={cn("self-center [&_svg]:size-3.5", k.tone)}>{k.icon}</span>
            <span className={cn("min-w-0 break-words", isError && "font-mono text-xs text-destructive")}>{e.text}</span>
          </li>
        );
      })}
    </ul>
  );
}
