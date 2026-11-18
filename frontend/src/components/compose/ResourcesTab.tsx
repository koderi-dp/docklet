import { useComposeResources } from "@/hooks/useCompose";
import { ProjectDiskPanel } from "./ProjectDiskPanel";

function Table({ title, empty, head, rows }: { title: string; empty: string; head: string[]; rows: string[][] }) {
  return (
    <section>
      <h3 className="mb-2 text-sm font-semibold">
        {title} <span className="font-normal text-muted-foreground">({rows.length})</span>
      </h3>
      {rows.length === 0 ? (
        <p className="text-muted-foreground">{empty}</p>
      ) : (
        <div className="overflow-x-auto rounded-[10px] border border-border bg-card">
          <table className="w-full border-collapse text-[13px]">
            <thead>
              <tr className="border-b border-border text-left text-xs font-medium text-muted-foreground">
                {head.map((h) => (
                  <th key={h} className="h-10 px-3 font-medium">
                    {h}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {rows.map((r) => (
                <tr key={r[0]} className="h-[42px] border-b border-border/60 last:border-b-0">
                  {r.map((v, i) => (
                    <td key={i} className={i === 0 ? "px-3 font-medium" : "px-3 text-muted-foreground"}>
                      <span className="block max-w-[420px] truncate" title={v}>
                        {v || "—"}
                      </span>
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}

/** Networks and volumes carrying the com.docker.compose.project label of this project. */
export function ResourcesTab({ connectionId, project }: { connectionId: string; project: string }) {
  const q = useComposeResources(connectionId, project, true);
  if (q.isLoading) return <p className="text-muted-foreground">Loading…</p>;
  if (q.error) return <p className="text-destructive">Could not load resources: {(q.error as Error).message}</p>;
  return (
    <div className="space-y-5">
      <ProjectDiskPanel connectionId={connectionId} project={project} />
      <Table
        title="Networks"
        empty="No networks are labelled for this project."
        head={["Name", "Driver", "Scope"]}
        rows={(q.data?.networks ?? []).map((n) => [n.name, n.driver, n.scope])}
      />
      <Table
        title="Volumes"
        empty="No volumes are labelled for this project."
        head={["Name", "Driver", "Mountpoint"]}
        rows={(q.data?.volumes ?? []).map((v) => [v.name, v.driver, v.mountpoint])}
      />
    </div>
  );
}
