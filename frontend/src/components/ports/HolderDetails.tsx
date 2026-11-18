import { Link } from "react-router-dom";
import type { Holder, PortCheck } from "@bindings/ports";
import { CopyValue } from "@/components/shared/CopyValue";

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="grid grid-cols-[84px_1fr] gap-2 py-1">
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="min-w-0 break-words">{children}</dd>
    </div>
  );
}

/** What holds a port: the main holder and any others, with path, owner and bind address. */
export function HolderDetails({ check }: { check: PortCheck }) {
  const all = [...(check.holder ? [check.holder] : []), ...(check.also ?? [])];
  return (
    <div className="space-y-3">
      {all.map((h, i) => (
        <HolderBlock key={i} holder={h} />
      ))}
      {check.note && <p className="text-xs text-muted-foreground">{check.note}</p>}
    </div>
  );
}

function HolderBlock({ holder: h }: { holder: Holder }) {
  const bind = h.bindIp || "all addresses";
  return (
    <dl className="divide-y divide-border/50 [&:not(:first-child)]:border-t [&:not(:first-child)]:pt-2">
      <Row label="Holder">
        {h.kind === "container" ? (
          <Link to={`/containers/${h.containerId}`} className="font-medium hover:underline">{h.containerName}</Link>
        ) : (
          <span className="font-medium">{h.processName || "Unidentified program"}</span>
        )}
      </Row>
      {h.kind === "container" && h.composeProject && <Row label="Project">{h.composeProject}</Row>}
      {h.kind !== "container" && <Row label="PID">{h.pid > 0 ? h.pid : "—"}</Row>}
      {h.kind !== "container" && (
        <Row label="Path">{h.executablePath ? <CopyValue value={h.executablePath} what="Path" /> : <span className="text-muted-foreground">Not available ({h.kind === "system" ? "part of the system" : "access denied or not reported"})</span>}</Row>
      )}
      {h.kind !== "container" && <Row label="User">{h.user || <span className="text-muted-foreground">Not available</span>}</Row>}
      <Row label="Address">{bind}</Row>
      {h.location && <Row label="Where">{h.location}</Row>}
    </dl>
  );
}
