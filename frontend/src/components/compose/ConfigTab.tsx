import { useComposeConfig } from "@/hooks/useCompose";
import type { ComposeProject } from "@bindings/model";

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="grid grid-cols-[160px_1fr] gap-3 border-b border-border/60 px-4 py-2.5 last:border-b-0">
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="min-w-0 break-all">{children}</dd>
    </div>
  );
}

const NA = <span className="text-muted-foreground">Not available</span>;

export function ConfigTab({ connectionId, project, active }: { connectionId: string; project: ComposeProject; active: boolean }) {
  const cap = project.capability;
  const cfg = useComposeConfig(connectionId, project.name, active && cap.canRun);
  return (
    <div className="space-y-4">
      <dl className="rounded-[10px] border border-border bg-card">
        <Row label="Source">{project.source}</Row>
        <Row label="Execution location">{cap.executionLocation || NA}</Row>
        <Row label="Working directory">{project.workingDir ? <span className="font-mono text-xs">{project.workingDir}</span> : NA}</Row>
        <Row label="Config files">
          {(project.configFiles ?? []).length ? (
            <ul className="space-y-0.5 font-mono text-xs">
              {(project.configFiles ?? []).map((f) => (
                <li key={f}>{f}</li>
              ))}
            </ul>
          ) : (
            NA
          )}
        </Row>
        <Row label="Environment files">
          {(project.envFiles ?? []).length ? (
            <ul className="space-y-0.5 font-mono text-xs">
              {(project.envFiles ?? []).map((f) => (
                <li key={f}>{f}</li>
              ))}
            </ul>
          ) : (
            <span className="text-muted-foreground">None recorded</span>
          )}
        </Row>
        <Row label="Compose CLI">
          {cap.canRun ? (
            <span className="text-success">Can operate this project (Up, Down, Restart, Pull, Build).</span>
          ) : (
            <span className="text-warning">Not available: {cap.reason}</span>
          )}
          {cap.warning && <p className="mt-1 text-xs text-muted-foreground">{cap.warning}</p>}
        </Row>
      </dl>
      {!cap.canRun && (
        <p className="text-xs text-muted-foreground">
          The project was discovered from container labels only. Start, Stop and Restart of its containers still work as Engine operations.
        </p>
      )}
      {cap.canRun && (
        <div>
          <h3 className="mb-2 text-sm font-semibold">Rendered configuration</h3>
          {cfg.isLoading ? (
            <p className="text-muted-foreground">Rendering…</p>
          ) : cfg.error ? (
            <p className="text-destructive">{(cfg.error as Error).message}</p>
          ) : cfg.data?.available ? (
            <>
              <pre className="max-h-[480px] overflow-auto rounded-[10px] border border-border bg-card p-3 font-mono text-xs leading-5">{cfg.data.rendered}</pre>
              <p className="mt-1 text-xs text-muted-foreground">Values of secret-like keys are masked.</p>
            </>
          ) : (
            <p className="text-muted-foreground">{cfg.data?.note || "Rendered configuration is not available."}</p>
          )}
        </div>
      )}
    </div>
  );
}
