import { useEffect, useMemo, useRef, useState } from "react";
import { useImagesQuery } from "@/hooks/useImages";
import { useComposeVerb, useProjectEngineBatch } from "@/hooks/useCompose";
import { buildRows, countRows, filterRows, imageSizeFor, type ServiceFilter, type ServiceRow as Row } from "@/lib/serviceRows";
import type { ComposeProject, Container, ImageRow } from "@bindings/model";
import { ServiceFilterChips } from "./ServiceFilterChips";
import { ServiceRow, type Act } from "./ServiceRow";

const HEADERS = ["Service", "Image", "Status", "Health", "Ports", "Image size", "Created"];

/** Service table: one row per container, filterable by running / stopped. Actions are Engine operations. */
export function ServicesTab({ connectionId, project, containers, focusService }: { connectionId: string; project: ComposeProject; containers: Container[]; focusService?: string }) {
  const engine = useProjectEngineBatch(connectionId);
  const verbs = useComposeVerb(connectionId);
  const images = useImagesQuery(connectionId, true).data;
  const cap = project.capability;
  const upReason = !cap.canRun ? cap.reason || "Compose CLI is not available for this project" : verbs.isBusy(project.name) ? "A Compose command is being submitted" : undefined;
  const rows = useMemo(() => buildRows((project.services ?? []).map((s) => s.name), containers), [containers, project.services]);

  const [filter, setFilter] = useState<ServiceFilter>("all");
  useEffect(() => setFilter("all"), [project.name]);

  const wrapRef = useRef<HTMLDivElement>(null);
  const focused = useRef<string | undefined>(undefined);
  useEffect(() => {
    if (!focusService) {
      focused.current = undefined;
      return;
    }
    if (focused.current === focusService) return;
    // A filter that would hide the service gives way first; the effect runs again with the new filter.
    if (!filterRows(rows, filter).some((r) => r.service === focusService)) {
      if (filter !== "all") setFilter("all");
      return;
    }
    focused.current = focusService;
    const els = Array.from(wrapRef.current?.querySelectorAll<HTMLElement>("[data-service]") ?? []).filter((r) => r.dataset.service === focusService);
    els[0]?.scrollIntoView({ block: "center", behavior: "smooth" });
    const ring = ["ring-2", "ring-inset", "ring-primary"];
    els.forEach((el) => el.classList.add(...ring));
    const t = window.setTimeout(() => els.forEach((el) => el.classList.remove(...ring)), 2000);
    return () => {
      window.clearTimeout(t);
      els.forEach((el) => el.classList.remove(...ring));
    };
  }, [focusService, filter, rows]);

  if (rows.length === 0) return <p className="p-4 text-muted-foreground">No containers carry this project label{project.managed ? " and the Compose files define no services we could read" : ""}.</p>;

  return (
    <ServicesView
      rows={rows}
      filter={filter}
      onFilter={setFilter}
      images={images ?? []}
      busy={engine.pending}
      canRun={cap.canRun}
      upReason={upReason}
      wrapRef={wrapRef}
      onAct={(a, t) => void engine.run(a, t)}
      onUp={(service) => void verbs.run("up", project.name, { service })}
    />
  );
}

export function ServicesView({ rows, filter, onFilter, images, busy, canRun, upReason, wrapRef, onAct, onUp }: {
  rows: Row[];
  filter: ServiceFilter;
  onFilter: (f: ServiceFilter) => void;
  images: ImageRow[];
  busy: boolean;
  canRun: boolean;
  upReason?: string;
  wrapRef?: React.Ref<HTMLDivElement>;
  onAct: (a: Act, targets: Container[]) => void;
  onUp: (service: string) => void;
}) {
  const shown = filterRows(rows, filter);
  return (
    <div className="space-y-2">
      <div className="flex justify-end">
        <ServiceFilterChips value={filter} counts={countRows(rows)} onChange={onFilter} />
      </div>
      <div ref={wrapRef} className="overflow-x-auto rounded-[10px] border border-border bg-card">
        <table className="w-full min-w-[980px] border-collapse text-[13px]">
          <thead>
            <tr className="border-b border-border text-left text-xs font-medium text-muted-foreground">
              {HEADERS.map((h) => (
                <th key={h} className="h-10 whitespace-nowrap px-3 font-medium">
                  {h}
                </th>
              ))}
              <th className="px-3 text-right font-medium">
                <span className="sr-only">Actions</span>
              </th>
            </tr>
          </thead>
          <tbody>
            {shown.length === 0 && (
              <tr>
                <td colSpan={HEADERS.length + 1} className="h-[42px] whitespace-nowrap px-3 text-muted-foreground">
                  No {filter} services.
                </td>
              </tr>
            )}
            {shown.map((r) => (
              <ServiceRow
                key={r.key}
                row={r}
                siblings={rows.filter((x) => x.service === r.service && x.container).map((x) => x.container as Container)}
                imageSize={r.container ? imageSizeFor(r.container, images) : undefined}
                busy={busy}
                upReason={r.service ? upReason : "Containers without a service label cannot be started with Compose"}
                composeAvailable={canRun && !!r.service}
                onAct={onAct}
                onUp={() => onUp(r.service)}
              />
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
