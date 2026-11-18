import { HardDrive, MemoryStick, Server, Database } from "lucide-react";
import type { WSLInfo } from "@bindings/model";
import { MetricCard } from "@/components/shared/MetricCard";
import { formatBytes } from "@/components/containers/format";

const bytes = (n: number | null | undefined) => (n == null ? "—" : formatBytes(n));
const of = (used: number | null, total: number | null) => (used == null || total == null ? undefined : `of ${formatBytes(total)}`);
const pct = (used: number | null, total: number | null) => (used == null || total == null || total === 0 ? "" : `${Math.round((used / total) * 100)}% used`);

/** Four KPI cards; every figure that could not be read shows an em dash, never 0. */
export function WslSummary({ info }: { info: WSLInfo }) {
  const memHint = [of(info.memUsed, info.memTotal), info.memCache != null ? `${formatBytes(info.memCache)} cache` : ""].filter(Boolean).join(" · ");
  const diskHint = [of(info.diskUsed, info.diskSize), pct(info.diskUsed, info.diskSize)].filter(Boolean).join(" · ");
  return (
    <div className="grid grid-cols-2 gap-3 xl:grid-cols-4">
      <MetricCard icon={<MemoryStick />} label="Memory used" value={bytes(info.memUsed)} hint={memHint || undefined} />
      <MetricCard icon={<Server />} tone="info" label="WSL VM memory" value={bytes(info.vmMemory)} hint="vmmem · all distributions" />
      <MetricCard icon={<HardDrive />} tone="success" label="Disk used" value={bytes(info.diskUsed)} hint={diskHint || undefined} />
      <MetricCard icon={<Database />} tone="warning" label="Virtual disk" value={bytes(info.vhdxSize)} hint="ext4.vhdx on Windows" />
    </div>
  );
}
