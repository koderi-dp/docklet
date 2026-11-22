import { useEffect, useMemo, useRef } from "react";
import { useImagesQuery } from "@/hooks/useImages";
import { useComposeVerb, useProjectEngineBatch } from "@/hooks/useCompose";
import { buildRows, countRows, filterRows, imageSizeFor, type ServiceFilter, type ServiceRow as Row } from "@/lib/serviceRows";
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
  // The ring is keyed on focusService only; the latest rows and filter are read through refs so a
  // container refetch does not cut the two seconds short.
  const latest = useRef({ rows, filter });
  latest.current = { rows, filter };
  useEffect(() => {
    if (!focusService) return;
    const ring = ["ring-2", "ring-inset", "ring-primary"];
    let els: HTMLElement[] = [];
    let timer: number | undefined;
    let frame: number | undefined;
    const apply = () => {
      els = Array.from(wrapRef.current?.querySelectorAll<HTMLElement>("[data-service]") ?? []).filter((r) => r.dataset.service === focusService);
      els[0]?.scrollIntoView({ block: "center", behavior: "smooth" });
      els.forEach((el) => el.classList.add(...ring));
      timer = window.setTimeout(() => els.forEach((el) => el.classList.remove(...ring)), 2000);
    };
    const { rows: rs, filter: f } = latest.current;
    const visible = filterRows(rs, f).filter((r) => r.service === focusService).length;
    if (visible < rs.filter((r) => r.service === focusService).length) {
      // A filter that hides any row of the service gives way first; the ring waits for the new render.
      onFilterChange("all");
      frame = window.requestAnimationFrame(apply);
    } else {
      apply();
    }
    return () => {
      if (frame !== undefined) window.cancelAnimationFrame(frame);
      window.clearTimeout(timer);
      els.forEach((el) => el.classList.remove(...ring));
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [focusService]);

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
