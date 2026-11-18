import { useState } from "react";
import { Link } from "react-router-dom";
import { Copy } from "lucide-react";
import { toast } from "sonner";
import type { Container, ContainerDetail } from "@bindings/model";
import { Button } from "@/components/ui/button";
import { formatCreated, uniquePortsLong } from "./format";
import { PortList } from "./PortLink";
import { ShowFolderButton } from "@/components/host/ShowFolderButton";

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="rounded-[10px] border border-border bg-card p-4">
      <h3 className="mb-2 text-sm font-semibold">{title}</h3>
      {children}
    </section>
  );
}

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="grid grid-cols-[130px_1fr] gap-3 border-b border-border/50 py-1.5 text-[13px] last:border-0">
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="min-w-0 break-words">{children}</dd>
    </div>
  );
}

const none = <span className="text-muted-foreground">None</span>;

export function OverviewPanel({ c, d }: { c: Container; d: ContainerDetail }) {
  const [reveal, setReveal] = useState(false);
  const ports = uniquePortsLong(d.ports?.length ? d.ports : (c.ports ?? []));
  const labels = Object.entries(d.labels ?? {}).sort(([a], [b]) => a.localeCompare(b));
  const copyId = () =>
    navigator.clipboard
      .writeText(c.id)
      .then(() => toast.success("Container ID copied"))
      .catch(() => toast.error("Could not copy container ID"));
  return (
    <div className="grid gap-3 xl:grid-cols-2">
      <Section title="Properties">
        <dl>
          <Row label="Container ID">
            <span className="inline-flex items-center gap-1 break-all font-mono text-xs">
              {c.id}
              <button aria-label="Copy container ID" title="Copy container ID" onClick={copyId} className="text-muted-foreground hover:text-foreground">
                <Copy className="size-3.5" />
              </button>
            </span>
          </Row>
          <Row label="Image"><span className="font-mono text-xs">{d.image || c.image}</span></Row>
          <Row label="Created">{formatCreated(d.created)}</Row>
          <Row label="Started">{d.startedAt ? new Date(d.startedAt).toLocaleString() : "—"}</Row>
          {d.finishedAt && c.state !== "running" && <Row label="Finished">{new Date(d.finishedAt).toLocaleString()}</Row>}
          <Row label="Command"><span className="font-mono text-xs">{d.command || "—"}</span></Row>
          <Row label="Entrypoint"><span className="font-mono text-xs">{d.entrypoint || "—"}</span></Row>
          <Row label="Restart policy">{d.restartPolicy || "no"}</Row>
          <Row label="Project">
            {d.composeProject ? (
              <Link to={`/compose/${encodeURIComponent(d.composeProject)}`} className="text-primary hover:underline">
                {d.composeProject}
              </Link>
            ) : (
              <span className="text-muted-foreground">Standalone</span>
            )}
          </Row>
          {d.composeService && <Row label="Service">{d.composeService}</Row>}
        </dl>
      </Section>

      <div className="space-y-3">
        <Section title="Ports">
          {ports.length ? <PortList ports={d.ports?.length ? d.ports : (c.ports ?? [])} /> : none}
        </Section>
        <Section title="Networks">
          {d.networks?.length ? (
            <div className="flex flex-wrap gap-1.5 font-mono text-xs">
              {d.networks.map((n) => <span key={n} className="rounded bg-muted px-1.5 py-0.5">{n}</span>)}
            </div>
          ) : none}
        </Section>
      </div>

      <Section title="Mounts">
        {d.mounts?.length ? (
          <ul className="flex flex-col gap-1.5 font-mono text-xs">
            {d.mounts.map((m) => (
              <li key={m.destination} className="break-all">
                <span className="mr-1.5 rounded bg-muted px-1 py-0.5 font-sans text-[11px] text-muted-foreground">{m.type}</span>
                {m.source} → {m.destination}
                {m.type === "bind" && <span className="ml-1 inline-block align-middle"><ShowFolderButton connectionId={c.connectionId} path={m.source} /></span>}
                {m.readOnly ? " (ro)" : ""}
              </li>
            ))}
          </ul>
        ) : none}
      </Section>

      <Section title={`Labels (${labels.length})`}>
        {labels.length ? (
          <ul className="max-h-56 overflow-auto font-mono text-xs">
            {labels.map(([k, v]) => (
              <li key={k} className="break-all"><span className="text-muted-foreground">{k}=</span>{v}</li>
            ))}
          </ul>
        ) : none}
      </Section>

      <div className="xl:col-span-2">
        <Section title={`Environment (${d.env?.length ?? 0})`}>
          <div className="mb-1 flex items-center justify-between">
            <span className="text-xs text-muted-foreground">Secret-like keys are always withheld by the backend.</span>
            <Button variant="ghost" size="sm" className="h-6 px-2 text-xs" onClick={() => setReveal((v) => !v)}>
              {reveal ? "Hide" : "Show"} values
            </Button>
          </div>
          {d.env?.length ? (
            <ul className="max-h-64 overflow-auto rounded-md bg-muted/40 p-2 font-mono text-xs">
              {d.env.map((e) => (
                <li key={e.key} className="break-all">
                  <span className="text-muted-foreground">{e.key}=</span>
                  {e.masked ? <span title="Secret-like key: value withheld by the backend">••••••••</span> : reveal ? e.value : "••••••••"}
                </li>
              ))}
            </ul>
          ) : (
            <p className="text-xs text-muted-foreground">No environment variables.</p>
          )}
        </Section>
      </div>
    </div>
  );
}
