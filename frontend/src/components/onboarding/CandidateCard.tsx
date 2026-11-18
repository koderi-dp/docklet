import { Check } from "lucide-react";
import { TypeIcon } from "@/components/connections/connectionMeta";
import { Badge } from "@/components/ui/badge";
import type { Candidate, CandidateState } from "@/lib/onboardingCandidates";
import { cn } from "@/lib/utils";

const BADGE: Record<CandidateState, { label: string; tone: string }> = {
  running: { label: "Running", tone: "border-success/30 bg-success/10 text-success" },
  "docker-stopped": { label: "Docker stopped", tone: "border-warning/30 bg-warning/10 text-warning" },
  "distro-stopped": { label: "Stopped", tone: "border-border bg-muted text-muted-foreground" },
  "no-docker": { label: "No Docker", tone: "border-border bg-muted text-muted-foreground" },
  unsupported: { label: "No Docker", tone: "border-border bg-muted text-muted-foreground" },
  "desktop-down": { label: "Docker Desktop off", tone: "border-warning/30 bg-warning/10 text-warning" },
  unchecked: { label: "Not tested yet", tone: "border-border bg-muted text-muted-foreground" },
};

interface Props {
  c: Candidate;
  selected: boolean;
  recommended: boolean;
  onSelect: () => void;
}

/** One engine the wizard found: type icon, name, state and one line of facts. A radio in a group. */
export function CandidateCard({ c, selected, recommended, onSelect }: Props) {
  const b = BADGE[c.state];
  return (
    <button
      type="button"
      role="radio"
      data-candidate={c.id}
      aria-checked={selected}
      tabIndex={selected ? 0 : -1}
      onClick={onSelect}
      className={cn(
        "flex w-full items-center gap-3 rounded-lg border bg-card p-3.5 text-left outline-none transition-colors hover:bg-accent/40 focus-visible:ring-[3px] focus-visible:ring-ring/50",
        selected && "border-primary bg-accent/40",
      )}
    >
      <span className={cn("flex size-9 shrink-0 items-center justify-center rounded-md bg-muted text-muted-foreground", selected && "bg-primary/15 text-primary")}>
        <TypeIcon type={c.profile.type} className="size-[18px]" />
      </span>
      <span className="min-w-0 flex-1">
        <span className="flex flex-wrap items-center gap-x-2 gap-y-1">
          <span className="text-sm font-medium">{c.name}</span>
          <Badge variant="outline" className={cn("rounded-md px-1.5 py-0 text-[11px]", b.tone)}>
            {b.label}
          </Badge>
          {recommended && (
            <Badge variant="outline" className="rounded-md border-primary/30 bg-primary/10 px-1.5 py-0 text-[11px] text-primary">
              Recommended
            </Badge>
          )}
        </span>
        <span className="mt-0.5 block text-xs break-words text-muted-foreground">{c.facts}</span>
      </span>
      <span
        aria-hidden
        className={cn("flex size-5 shrink-0 items-center justify-center rounded-full border", selected ? "border-primary bg-primary text-primary-foreground" : "text-transparent")}
      >
        <Check className="size-3" strokeWidth={3} />
      </span>
    </button>
  );
}
