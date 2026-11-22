import { Button } from "@/components/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useComposeVerb } from "@/hooks/useCompose";
import { applyDisabledReason, type ApplyOptions } from "@/lib/applyPlanView";

interface Props {
  connectionId: string;
  name: string;
  options: ApplyOptions;
  dirty: boolean;
  planError: boolean;
  planLoading: boolean;
  canRun: boolean;
  capReason: string;
  pending: boolean;
}

/** Runs docker compose up -d for the saved file. Off while anything makes that unsafe or pointless. */
export function ApplyButton({ connectionId, name, options, dirty, planError, planLoading, canRun, capReason, pending }: Props) {
  const verbs = useComposeVerb(connectionId);
  const reason = applyDisabledReason({ dirty, planError, planLoading, canRun, capReason, busy: verbs.isBusy(name), pending });
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span className="inline-flex">
          <Button size="sm" disabled={!!reason} onClick={() => void verbs.run("up", name, { up: { build: false, ...options } })}>
            Apply
          </Button>
        </span>
      </TooltipTrigger>
      <TooltipContent className="max-w-sm">{reason ?? "Runs docker compose up -d. Unchanged services keep running."}</TooltipContent>
    </Tooltip>
  );
}
