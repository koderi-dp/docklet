import { useState } from "react";
import { Link } from "react-router-dom";
import { Copy } from "lucide-react";
import { toast } from "sonner";
import type { Container, ContainerDetail } from "@bindings/model";
import { Button } from "@/components/ui/button";
import { formatCreated, uniquePortsLong, uptimeLabel } from "./format";
import { LiveStats } from "./LiveStats";
import { PortList } from "./PortLink";
import { CopyAsButton } from "./copyas/CopyAsMenu";
import { ShowFolderButton } from "@/components/host/ShowFolderButton";

/** One flat property line: muted label on the left, value on the right. No card around it. */
function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="grid grid-cols-[110px_1fr] gap-4 text-[13px]">
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="min-w-0 break-words">{children}</dd>
    </div>
  );
}

const mono = "font-mono text-xs";
const dash = <span className="text-muted-foreground">—</span>;

export function OverviewPanel({ c, d, connectionId }: { c: Container; d: ContainerDetail; connectionId: string }) {
  const [reveal, setReveal] = useState(false);
  const [showLabels, setShowLabels] = useState(false);
  const allPorts = d.ports?.length ? d.ports : (c.ports ?? []);
  const labels = Object.entries(d.labels ?? {}).sort(([a], [b]) => a.localeCompare(b));
  const copyId = () =>
    navigator.clipboard
      .writeText(c.id)
      .then(() => toast.success("Container ID copied"))
      .catch(() => toast.error("Could not copy container ID"));
  const uptime = uptimeLabel(c);
  return (
    <div className="space-y-5">
      <LiveStats connectionId={connectionId} containerId={c.id} running={c.state === "running"} />

      <div className="flex justify-end">
        <CopyAsButton containerId={c.id} name={c.name} />
      </div>
      <dl className="space-y-2.5 pb-2">
        <Row label="Image"><span className={mono}>{d.image || c.image}</span></Row>
        <Row label="Command"><span className={mono}>{d.command || "—"}</span></Row>
        {d.entrypoint && <Row label="Entrypoint"><span className={mono}>{d.entrypoint}</span></Row>}
        <Row label="ID">
          <span className="inline-flex items-center gap-1.5 break-all">
            <span className={mono}>{c.id}</span>
            <button aria-label="Copy container ID" title="Copy container ID" onClick={copyId} className="text-muted-foreground hover:text-foreground">
              <Copy className="size-3.5" />
            </button>
          </span>
        </Row>
        <Row label="Created">{formatCreated(d.created)}</Row>
        <Row label="Started">
          {d.startedAt ? new Date(d.startedAt).toLocaleString() : "—"}
          {uptime && c.state === "running" && <span className="ml-2 text-muted-foreground">· up {uptime.replace(/^up\s+/i, "")}</span>}
        </Row>
        {d.finishedAt && c.state !== "running" && <Row label="Finished">{new Date(d.finishedAt).toLocaleString()}</Row>}
        <Row label="Ports">{uniquePortsLong(allPorts).length ? <PortList ports={allPorts} /> : dash}</Row>
        <Row label="Restart">{d.restartPolicy || "no"}</Row>
        <Row label="Compose">
          {d.composeProject ? (
            <>
              <Link to={`/compose/${encodeURIComponent(d.composeProject)}`} className="text-primary hover:underline">{d.composeProject}</Link>
              {d.composeService && <span className="text-muted-foreground"> · service {d.composeService}</span>}
            </>
          ) : (
            <span className="text-muted-foreground">Standalone</span>
          )}
        </Row>
        <Row label="Networks">
          {d.networks?.length ? (
            <span className="flex flex-wrap gap-x-3 gap-y-1">
              {d.networks.map((n) => <span key={n} className={mono}>{n}</span>)}
            </span>
          ) : dash}
        </Row>
        <Row label="Mounts">
          {d.mounts?.length ? (
            <ul className="flex flex-col gap-1">
              {d.mounts.map((m) => (
                <li key={m.destination} className={`${mono} break-all`}>
                  <span className="mr-1.5 rounded bg-muted px-1 py-0.5 font-sans text-[11px] text-muted-foreground">{m.type}</span>
                  {m.source} → {m.destination}
                  {m.type === "bind" && <span className="ml-1 inline-block align-middle"><ShowFolderButton connectionId={c.connectionId} path={m.source} /></span>}
                  {m.readOnly && <span className="ml-1.5 font-sans text-[11px] text-muted-foreground">read-only</span>}
                </li>
              ))}
            </ul>
          ) : dash}
        </Row>
        <Row label="Labels">
          {labels.length ? (
            <>
              <span className="text-muted-foreground">{labels.length} labels</span>
              <button className="ml-2 text-primary hover:underline" onClick={() => setShowLabels((v) => !v)}>{showLabels ? "Hide" : "Show"}</button>
              {showLabels && (
                <ul className="mt-1.5 max-h-56 overflow-auto font-mono text-xs">
                  {labels.map(([k, v]) => (
                    <li key={k} className="break-all"><span className="text-muted-foreground">{k}=</span>{v}</li>
                  ))}
                </ul>
              )}
            </>
          ) : dash}
        </Row>
        <Row label="Environment">
          {d.env?.length ? (
            <>
              <span className="text-muted-foreground">{d.env.length} variables</span>
              <Button variant="link" size="sm" className="ml-1 h-auto p-0 text-[13px]" onClick={() => setReveal((v) => !v)}>
                {reveal ? "Hide" : "Show"} values
              </Button>
              <ul className="mt-1.5 font-mono text-xs">
                {d.env.map((e) => (
                  <li key={e.key} className="break-all">
                    <span className="text-muted-foreground">{e.key}=</span>
                    {e.masked ? <span title="Secret-like key: value withheld by the backend">••••••••</span> : reveal ? e.value : "••••••••"}
                  </li>
                ))}
              </ul>
            </>
          ) : (
            <span className="text-muted-foreground">None</span>
          )}
        </Row>
      </dl>
    </div>
  );
}
