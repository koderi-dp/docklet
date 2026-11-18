import { Browser } from "@wailsio/runtime";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useActiveConnection } from "@/hooks/useActiveConnection";
import { toastError } from "@/lib/errors";
import { portUrl } from "@/lib/portUrl";
import { cn } from "@/lib/utils";
import type { PortBinding } from "@bindings/model";
import { uniquePortBindings } from "./format";

/**
 * A published TCP port becomes a link that opens the default browser, but only when the host is
 * safely known (see lib/portUrl). Otherwise it is plain text with a tooltip saying why.
 */
export function PortLink({ binding, children, className }: { binding: PortBinding; children: React.ReactNode; className?: string }) {
  const { profile } = useActiveConnection();
  const link = portUrl(profile, binding);
  if (!link.url) {
    if (!binding.hostPort) return <span className={className}>{children}</span>;
    return (
      <Tooltip>
        <TooltipTrigger asChild>
          <span className={className}>{children}</span>
        </TooltipTrigger>
        <TooltipContent>{link.reason}</TooltipContent>
      </Tooltip>
    );
  }
  const url = link.url;
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          type="button"
          className={cn("cursor-pointer rounded text-primary hover:underline focus-visible:ring-2 focus-visible:ring-ring/60 focus-visible:outline-none", className)}
          onClick={(e) => {
            e.stopPropagation();
            Browser.OpenURL(url).catch((err) => toastError("Could not open the browser", err));
          }}
        >
          {children}
        </button>
      </TooltipTrigger>
      <TooltipContent>Open {url}</TooltipContent>
    </Tooltip>
  );
}

/** Port rows of the inspector / overview: long form text, each published TCP port clickable. */
export function PortList({ ports }: { ports: readonly PortBinding[] }) {
  return (
    <div className="flex flex-col items-start gap-0.5 font-mono text-xs">
      {uniquePortBindings(ports).map(({ label, binding }) => (
        <PortLink key={label} binding={binding}>
          {label}
        </PortLink>
      ))}
    </div>
  );
}
