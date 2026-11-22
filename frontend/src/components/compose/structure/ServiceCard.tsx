import { memo } from "react";
import { HardDrive } from "lucide-react";
import { cn } from "@/lib/utils";
import { kindLabel, stateLabel, type GraphKind, type GraphState } from "@/lib/composeGraphView";

export const CARD_WIDTH = 226;
export const CARD_HEIGHT = 92;

type Tone = "success" | "warning" | "destructive" | "muted";
const TONE: Record<GraphState, Tone> = {
  running: "success",
  unhealthy: "destructive",
  failed: "destructive",
  starting: "warning",
  partial: "warning",
  paused: "warning",
  stopped: "muted",
  completed: "muted",
  notCreated: "muted",
};
const BG: Record<Tone, string> = {
  success: "bg-success",
  warning: "bg-warning",
  destructive: "bg-destructive",
  muted: "bg-muted-foreground/60",
};

export interface ServiceCardProps {
  name: string;
  image: string;
  build: boolean;
  kind: GraphKind;
  state: GraphState;
  /** First two port labels, already formatted and joined by a space. */
  ports: string;
  /** First volume label and how many more there are. */
  volume: string;
  moreVolumes: number;
  /** First profile when the service has profiles; "" otherwise. */
  profile: string;
  dimmed: boolean;
  selected: boolean;
  onSelect: (service: string) => void;
  onHover: (service: string | null) => void;
}

export const ServiceCard = memo(function ServiceCard(p: ServiceCardProps) {
  const idle = p.profile !== "" && p.state === "notCreated";
  const tone = TONE[p.state];
  const label = stateLabel(p.state);
  const image = p.image || (p.build ? "built from source" : "");
  return (
    <button
      type="button"
      onClick={(e) => {
        e.stopPropagation();
        p.onSelect(p.name);
      }}
      data-graph-card
      aria-pressed={p.selected}
      aria-label={`${p.name}, ${label}`}
      onMouseEnter={() => p.onHover(p.name)}
      onMouseLeave={() => p.onHover(null)}
      onFocus={() => p.onHover(p.name)}
      onBlur={() => p.onHover(null)}
      style={{ width: CARD_WIDTH, height: CARD_HEIGHT }}
      className={cn(
        "relative flex flex-col justify-between overflow-hidden rounded-[10px] border border-border bg-background py-2.5 pl-4 pr-3 text-left transition-opacity",
        "outline-none hover:border-primary/50 focus-visible:ring-2 focus-visible:ring-ring",
        "cursor-pointer",
        idle && "border-dashed opacity-60",
        p.selected && "ring-2 ring-primary",
        p.dimmed && "opacity-30",
      )}
    >
      <span aria-hidden className={cn("absolute inset-y-0 left-0 w-[3px]", BG[tone])} />
      <span className="flex w-full items-center gap-2">
        <span className="min-w-0 flex-1 truncate text-[13px] font-semibold" title={p.name}>
          {p.name}
        </span>
        {idle ? (
          <span className="shrink-0 truncate text-xs text-muted-foreground">profile: {p.profile}</span>
        ) : (
          <span className="flex shrink-0 items-center gap-1.5 text-xs text-muted-foreground">
            <span aria-hidden className={cn("size-2 rounded-full", BG[tone])} />
            {label}
          </span>
        )}
      </span>
      <span className="block w-full truncate font-mono text-[11px] text-muted-foreground" title={image}>
        {image || " "}
      </span>
      <span className="flex w-full items-center gap-1.5">
        <span className="shrink-0 rounded border border-border px-1.5 py-px text-[10px] font-medium uppercase tracking-wide text-muted-foreground">
          {kindLabel(p.kind)}
        </span>
        <span className="min-w-0 flex-1 truncate font-mono text-[11px] text-muted-foreground">{p.ports}</span>
        {p.volume && (
          <span className="flex min-w-0 shrink items-center gap-1 text-[11px] text-muted-foreground" title={p.volume}>
            <HardDrive aria-hidden className="size-3 shrink-0" />
            <span className="max-w-[56px] truncate">{p.volume}</span>
            {p.moreVolumes > 0 && <span className="shrink-0">+{p.moreVolumes}</span>}
          </span>
        )}
      </span>
    </button>
  );
});
