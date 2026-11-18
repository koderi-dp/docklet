import { CircleHelp } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";

const EXAMPLE = `develop:
  watch:
    - action: sync
      path: ./src
      target: /app/src`;

/** "What is this?" for a project without develop.watch rules: what watch does and a five-line example. */
export function WatchHelp({ project }: { project: string }) {
  return (
    <Popover>
      <Tooltip>
        <TooltipTrigger asChild>
          <PopoverTrigger asChild>
            <Button variant="ghost" size="icon" className="size-7 text-muted-foreground" aria-label={`What is Compose watch? (${project})`}>
              <CircleHelp className="size-3.5" />
            </Button>
          </PopoverTrigger>
        </TooltipTrigger>
        <TooltipContent>What is this?</TooltipContent>
      </Tooltip>
      <PopoverContent align="end" className="w-96 space-y-2 text-sm">
        <div className="font-medium">Compose watch</div>
        <p className="text-xs text-muted-foreground">
          Watch keeps running and, when files change, copies them into the container or rebuilds / restarts the service. Each service
          says what to do in a <span className="font-mono">develop: watch:</span> block under the service in the Compose file, for example:
        </p>
        <pre className="overflow-x-auto rounded-md border border-border bg-muted/50 p-2 font-mono text-xs leading-5">{EXAMPLE}</pre>
        <p className="text-xs text-muted-foreground">
          Actions: <span className="font-mono">sync</span>, <span className="font-mono">rebuild</span>, <span className="font-mono">sync+restart</span>,{" "}
          <span className="font-mono">sync+exec</span>, <span className="font-mono">restart</span>.
        </p>
      </PopoverContent>
    </Popover>
  );
}
