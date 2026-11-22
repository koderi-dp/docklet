import { useCallback, useMemo, useRef, useState } from "react";
import { Network } from "lucide-react";
import type { ComposeProject, Container } from "@bindings/model";
import { EmptyState } from "@/components/shared/EmptyState";
import { Skeleton } from "@/components/ui/skeleton";
import { useComposeGraph } from "@/hooks/useCompose";
import { layoutGraph } from "@/lib/composeGraphLayout";
import { serviceState } from "@/lib/composeGraphView";
import { GraphCanvas, type ServiceLive } from "./GraphCanvas";
import { GraphToolbar } from "./GraphToolbar";
import { useGraphViewport } from "./useGraphViewport";

interface Props {
  connectionId: string;
  project: ComposeProject;
  containers: Container[];
  active: boolean;
  onShowService: (service: string) => void;
}

function names(list: string[] | null | undefined): string {
  const l = list ?? [];
  if (l.length === 0) return "none";
  return l.length > 3 ? `${l.slice(0, 3).join(", ")} +${l.length - 3}` : l.join(", ");
}

function FooterItem({ label, list }: { label: string; list: string[] | null | undefined }) {
  return (
    <span className="flex min-w-0 items-baseline gap-1.5">
      <span className="text-[10px] font-semibold uppercase tracking-wide text-muted-foreground">{label}</span>
      <span className="truncate text-xs" title={(list ?? []).join(", ")}>
        {names(list)}
      </span>
    </span>
  );
}

export function StructureTab({ connectionId, project, containers, active, onShowService }: Props) {
  const q = useComposeGraph(connectionId, project.name, active);
  const graph = q.data;
  const [find, setFind] = useState("");

  // The layout depends on the graph only: container state changes must not move anything.
  const layout = useMemo(
    () => (graph ? layoutGraph((graph.services ?? []).map((s) => ({ name: s.name, dependsOn: (s.dependsOn ?? []).map((d) => d.service) }))) : null),
    [graph],
  );
  const viewport = useGraphViewport(layout && layout.width > 0 ? { width: layout.width, height: layout.height } : null);

  const live = useMemo(() => {
    const by = new Map<string, Container[]>();
    for (const c of containers) by.set(c.composeService ?? "", [...(by.get(c.composeService ?? "") ?? []), c]);
    const out = new Map<string, ServiceLive>();
    for (const s of graph?.services ?? []) {
      const list = by.get(s.name) ?? [];
      out.set(s.name, { state: serviceState(list), containerId: list.length === 1 ? list[0].id : "" });
    }
    return out;
  }, [containers, graph]);

  const latest = useRef(onShowService);
  latest.current = onShowService;
  const show = useCallback((service: string) => latest.current(service), []);

  if (q.isLoading) return <Skeleton className="h-full min-h-[420px] w-full rounded-[10px]" />;
  if (q.error || !graph || !layout)
    return <p className="p-4 text-destructive">{q.error ? (q.error as Error).message : "The service structure could not be read."}</p>;
  if ((graph.services ?? []).length === 0)
    return (
      <EmptyState icon={<Network />} title="No services to draw">
        <p>This project defines no services.</p>
      </EmptyState>
    );

  return (
    <div className="flex h-full min-h-[420px] flex-col overflow-hidden rounded-[10px] border border-border bg-card">
      <GraphToolbar
        find={find}
        onFind={setFind}
        count={(graph.services ?? []).length}
        scale={viewport.view.scale}
        onFit={viewport.fit}
        onZoomIn={viewport.zoomIn}
        onZoomOut={viewport.zoomOut}
      />
      {graph.source === "containers" && (
        <p className="border-b border-border px-3 py-1.5 text-xs text-warning">
          Drawn from the running containers: {graph.note}. Dependencies may be incomplete.
        </p>
      )}
      {layout.cyclic.length > 0 && (
        <p className="border-b border-border px-3 py-1.5 text-xs text-destructive">Dependency cycle: {layout.cyclic.join(", ")}</p>
      )}
      <GraphCanvas graph={graph} layout={layout} live={live} find={find} viewport={viewport} onShowService={show} />
      <div className="flex items-center gap-6 border-t border-border px-3 py-2">
        <FooterItem label="Network" list={graph.networks} />
        <FooterItem label="Volumes" list={graph.volumes} />
        <span className="ml-auto hidden shrink-0 text-xs text-muted-foreground lg:block">
          Arrows point at what a service needs; Compose starts those first.
        </span>
      </div>
    </div>
  );
}
