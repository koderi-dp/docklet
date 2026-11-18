import type { Container, ContainerStat } from "@bindings/model";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cpuCores, formatMemoryUsage, formatPercent } from "@/lib/statsFormat";
import { publishedBindings, uptimeLabel } from "./format";
import { PortLink } from "./PortLink";

export function Dash({ why }: { why?: string }) {
  const el = <span className="text-muted-foreground">—</span>;
  if (!why) return el;
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span className="cursor-default text-muted-foreground">—</span>
      </TooltipTrigger>
      <TooltipContent>{why}</TooltipContent>
    </Tooltip>
  );
}

/** First two published ports as links (when openable), "+N" for the rest. */
export function PortsCell({ container }: { container: Container }) {
  const ports = publishedBindings(container.ports ?? []);
  if (ports.length === 0) return <Dash />;
  return (
    <span className="font-mono text-xs" title={ports.map((x) => x.label).join(", ")}>
      {ports.slice(0, 2).map((x, i) => (
        <span key={x.label}>
          {i > 0 && ", "}
          <PortLink binding={x.binding}>{x.label}</PortLink>
        </span>
      ))}
      {ports.length > 2 && ` +${ports.length - 2}`}
    </span>
  );
}

export function CpuCell({ container, stat }: { container: Container; stat?: ContainerStat }) {
  if (container.state !== "running" || stat?.cpuPercent == null) return <Dash />;
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span className="tabular-nums">{stat.cpuPercent.toFixed(1)}%</span>
      </TooltipTrigger>
      <TooltipContent>About {cpuCores(stat.cpuPercent)} (100% = one core)</TooltipContent>
    </Tooltip>
  );
}

/** Memory as "used / limit"; the tooltip adds the percentage of the limit. */
export function MemoryCell({ container, stat }: { container: Container; stat?: ContainerStat }) {
  if (container.state !== "running" || stat?.memoryBytes == null) return <Dash />;
  const m = formatMemoryUsage(stat.memoryBytes, stat.memoryLimit);
  const txt = <span className="tabular-nums">{m.text}</span>;
  if (m.percent == null) return txt;
  return (
    <Tooltip>
      <TooltipTrigger asChild>{txt}</TooltipTrigger>
      <TooltipContent>{formatPercent(m.percent)} of the memory limit</TooltipContent>
    </Tooltip>
  );
}

export function UptimeCell({ container }: { container: Container }) {
  const up = uptimeLabel(container);
  return up ? (
    <span className="whitespace-nowrap tabular-nums" title={container.status}>
      {up}
    </span>
  ) : (
    <Dash why={container.status || undefined} />
  );
}
