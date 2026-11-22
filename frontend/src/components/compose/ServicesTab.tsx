import { useEffect, useMemo, useRef } from "react";
import { useImagesQuery } from "@/hooks/useImages";
import { useComposeVerb, useProjectEngineBatch } from "@/hooks/useCompose";
import { buildRows, countRows, filterHides, filterRows, imageSizeFor, type ServiceFilter, type ServiceRow as Row } from "@/lib/serviceRows";
import type { ComposeProject, Container, ImageRow } from "@bindings/model";
import { ServiceFilterChips } from "./ServiceFilterChips";
import { ServiceRow, type Act } from "./ServiceRow";

const HEADERS = ["Service", "Image", "Status", "Health", "Ports", "Image size", "Created"];

/** Service table: one row per container, filterable by running / stopped. Actions are Engine operations. */
export function ServicesTab({ connectionId, project, containers, focusService, filter, onFilterChange }: {
  connectionId: string;
  project: ComposeProject;
  containers: Container[];
  focusService?: string;
  /** The filter lives in the page so it survives a switch to another tab. */
  filter: ServiceFilter;
  onFilterChange: (f: ServiceFilter) => void;
}) {
  const engine = useProjectEngineBatch(connectionId);
  const verbs = useComposeVerb(connectionId);
  const images = useImagesQuery(connectionId, true).data;
  const cap = project.capability;
  const upReason = !cap.canRun ? cap.reason || "Compose CLI is not available for this project" : verbs.isBusy(project.name) ? "A Compose command is being submitted" : undefined;
  const rows = useMemo(() => buildRows((project.services ?? []).map((s) => s.name), containers), [containers, project.services]);

  const wrapRef = useRef<HTMLDivElement>(null);
  // Rows are read through a ref so a container refetch does not re-run the effect and cut the ring
  // short; `applied` remembers the focused service so a later filter change does not ring again.
  const latestRows = useRef(rows);
  latestRows.current = rows;
  const applied = useRef<string | undefined>(undefined);
  useEffect(() => {
    if (!focusService || applied.current === focusService) return;
    if (filterHides(latestRows.current, focusService, filter)) {
      // The filter gives way first; this effect runs again once the rows of the new filter are rendered.
      onFilterChange("all");
      return;
    }
    applied.current = focusService;
    const ring = ["ring-2", "ring-inset", "ring-primary"];
    const els = Array.from(wrapRef.current?.querySelectorAll<HTMLElement>("[data-service]") ?? []).filter((r) => r.dataset.service === focusService);
    els[0]?.scrollIntoView({ block: "center", behavior: "smooth" });
    els.forEach((el) => el.classList.add(...ring));
    const timer = window.setTimeout(() => els.forEach((el) => el.classList.remove(...ring)), 2000);
    return () => {
      window.clearTimeout(timer);
      els.forEach((el) => el.classList.remove(...ring));
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [focusService, filter]);

  if (rows.length === 0) return <p className="p-4 text-muted-foreground">No containers carry this project label{project.managed ? " and the Compose files define no services we could read" : ""}.</p>;

  return (
    <ServicesView
      rows={rows}
      filter={filter}
      onFilter={onFilterChange}
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
            {shown.map((r, i) => (
              <ServiceRow
                key={r.key}
                row={r}
                firstVisible={shown.findIndex((x) => x.service === r.service) === i}
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
