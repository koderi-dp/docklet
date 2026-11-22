import { useCallback, useMemo, useState } from "react";
import type { ComposeGraph } from "@bindings/model";
import type { GraphLayout } from "@/lib/composeGraphLayout";
import { matchesFind, portLabel, related, serviceKind, volumeLabel, type GraphState } from "@/lib/composeGraphView";
import { cn } from "@/lib/utils";
import { CARD_HEIGHT, CARD_WIDTH, ServiceCard } from "./ServiceCard";
import type { useGraphViewport } from "./useGraphViewport";

export interface ServiceLive {
  state: GraphState;
  /** The container id when the service has exactly one container. */
  containerId: string;
}

interface Props {
  graph: ComposeGraph;
  layout: GraphLayout;
  live: Map<string, ServiceLive>;
  find: string;
  viewport: ReturnType<typeof useGraphViewport>;
  onShowService: (service: string) => void;
}

const CONDITION_NOTE: Record<string, string> = {
  service_healthy: " (waits until healthy)",
  service_completed_successfully: " (waits until it has completed)",
};

type Mark = "default" | "active" | "cycle";
const MARK_FILL: Record<Mark, string> = {
  default: "var(--muted-foreground)",
  active: "var(--primary)",
  cycle: "var(--destructive)",
};

export function GraphCanvas({ graph, layout, live, find, viewport, onShowService }: Props) {
  const [hover, setHover] = useState<string | null>(null);
  const onHover = useCallback((name: string | null) => setHover(name), []);
  const { boxRef, view, ready, pan } = viewport;

  const byName = useMemo(() => new Map((graph.services ?? []).map((s) => [s.name, s])), [graph]);
  const conditions = useMemo(() => {
    const m = new Map<string, string>();
    for (const s of graph.services ?? []) for (const d of s.dependsOn ?? []) m.set(`${s.name}\0${d.service}`, d.condition ?? "");
    return m;
  }, [graph]);
  const relatedSet = useMemo(() => (hover ? related(hover, layout.edges) : null), [hover, layout.edges]);

  return (
    <div
      ref={boxRef}
      {...pan}
      className="relative min-h-0 flex-1 touch-none select-none overflow-hidden"
      style={{ backgroundImage: "radial-gradient(var(--border) 1px, transparent 1px)", backgroundSize: "20px 20px" }}
    >
      <div
        className="absolute left-0 top-0"
        style={{
          width: layout.width,
          height: layout.height,
          transform: `translate(${view.x}px, ${view.y}px) scale(${view.scale})`,
          transformOrigin: "0 0",
          visibility: ready ? "visible" : "hidden",
        }}
      >
        <svg className="absolute left-0 top-0 overflow-visible" width={layout.width} height={layout.height}>
          <defs>
            {(Object.keys(MARK_FILL) as Mark[]).map((m) => (
              <marker key={m} id={`graph-arrow-${m}`} viewBox="0 0 10 10" refX="9" refY="5" markerWidth="8" markerHeight="8" orient="auto-start-reverse">
                <path d="M 0 0 L 10 5 L 0 10 z" fill={MARK_FILL[m]} />
              </marker>
            ))}
          </defs>
          {layout.edges.map((e) => {
            const active = !!relatedSet && relatedSet.has(e.from) && relatedSet.has(e.to);
            const mark: Mark = e.inCycle ? "cycle" : active ? "active" : "default";
            const dim = !!relatedSet && !active;
            const cond = conditions.get(`${e.from}\0${e.to}`) ?? "";
            return (
              <g key={`${e.from}\0${e.to}`} className={cn("transition-opacity", dim && "opacity-30")}>
                <path
                  d={e.path}
                  fill="none"
                  stroke={MARK_FILL[mark]}
                  strokeOpacity={active || e.inCycle ? 1 : 0.55}
                  strokeWidth={1.5}
                  strokeDasharray={e.inCycle ? "5 4" : undefined}
                  markerEnd={`url(#graph-arrow-${mark})`}
                />
                <path d={e.path} fill="none" stroke="transparent" strokeWidth={14} pointerEvents="stroke">
                  <title>{`${e.from} needs ${e.to}${CONDITION_NOTE[cond] ?? ""}`}</title>
                </path>
              </g>
            );
          })}
        </svg>
        {layout.nodes.map((n) => {
          const s = byName.get(n.name);
          if (!s) return null;
          const l = live.get(n.name);
          const matches = matchesFind(s, find);
          const dimmed = relatedSet ? !relatedSet.has(n.name) : !matches;
          const vols = s.volumes ?? [];
          return (
            <div key={n.name} className="absolute" style={{ left: n.x, top: n.y, width: CARD_WIDTH, height: CARD_HEIGHT }}>
              <ServiceCard
                name={s.name}
                image={s.image ?? ""}
                build={s.build}
                kind={serviceKind({ image: s.image, ports: s.ports ?? [] })}
                state={l?.state ?? "notCreated"}
                ports={(s.ports ?? []).slice(0, 2).map(portLabel).join(" ")}
                volume={vols.length ? volumeLabel(vols[0]) : ""}
                moreVolumes={Math.max(0, vols.length - 1)}
                profile={(s.profiles ?? [])[0] ?? ""}
                containerId={l?.containerId ?? ""}
                dimmed={dimmed}
                onShowService={onShowService}
                onHover={onHover}
              />
            </div>
          );
        })}
      </div>
    </div>
  );
}
