import { Loader2 } from "lucide-react";
import type { BuildPlan } from "@bindings/model";
import { Button } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import type { BuilderChoice } from "./buildForm";

interface Props {
  plan: BuildPlan | undefined;
  pending: boolean;
  failed: string | null;
  builder: BuilderChoice;
  onBuilder: (b: BuilderChoice) => void;
}

/**
 * One quiet line saying which builder will run and why, with a small override. A folder the chosen
 * builder cannot read is reported here (before anything starts), with a one-click switch to Classic.
 */
export function BuilderLine({ plan, pending, failed, builder, onBuilder }: Props) {
  const cap = plan?.capability;
  const buildkitUnavailable = cap ? !cap.available : false;
  const line = failed ?? plan?.summary ?? "Checking how this connection can build…";
  return (
    <div className="space-y-1.5">
      <div className="flex items-center gap-2">
        <p className="flex min-w-0 flex-1 items-center gap-1.5 truncate text-xs text-muted-foreground" title={line}>
          {pending && !plan && <Loader2 className="size-3 shrink-0 animate-spin" aria-hidden />}
          <span className="truncate" aria-live="polite">
            {line}
          </span>
        </p>
        <Select value={builder} onValueChange={(v) => onBuilder(v as BuilderChoice)}>
          <SelectTrigger size="sm" className="h-7 w-[118px] shrink-0 text-xs" aria-label="Builder">
            <SelectValue />
          </SelectTrigger>
          <SelectContent align="end">
            <SelectItem value="auto">Automatic</SelectItem>
            <SelectItem value="buildkit" disabled={buildkitUnavailable} title={buildkitUnavailable ? cap?.reason : undefined}>
              BuildKit
            </SelectItem>
            <SelectItem value="classic">Classic</SelectItem>
          </SelectContent>
        </Select>
      </div>
      {cap?.contextHint && plan?.builder === "buildkit" && <p className="text-xs text-muted-foreground">{cap.contextHint}</p>}
      {plan?.builder === "classic" && plan.contextProblem === "" && (
        <p className="text-xs text-muted-foreground">The folder is read on this computer and uploaded to the Engine through its API.</p>
      )}
      {plan?.contextProblem && (
        <div className="flex items-start justify-between gap-2 rounded-md border border-destructive/30 bg-destructive/10 px-2.5 py-2 text-xs text-destructive">
          <p className="min-w-0 whitespace-pre-wrap break-words">{plan.contextProblem}</p>
          {plan.offerClassic && builder !== "classic" && (
            <Button type="button" size="sm" variant="outline" className="h-6 shrink-0 px-2 text-xs" onClick={() => onBuilder("classic")}>
              Use Classic builder
            </Button>
          )}
        </div>
      )}
    </div>
  );
}
