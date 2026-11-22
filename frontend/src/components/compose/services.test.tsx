import { describe, expect, it, vi } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router-dom";
import { TooltipProvider } from "@/components/ui/tooltip";
import { buildRows } from "@/lib/serviceRows";
import type { ComposeProject, Container } from "@bindings/model";
import { ServicesTab, ServicesView } from "./ServicesTab";

vi.mock("@bindings/services", () => ({ ComposeService: {}, ImageService: {}, ContainerService: {} }));
vi.mock("@/hooks/useCompose", () => ({
  useProjectEngineBatch: () => ({ pending: false, run: vi.fn() }),
  useComposeVerb: () => ({ run: vi.fn(), isBusy: () => false }),
}));
vi.mock("@/hooks/useImages", () => ({ useImagesQuery: () => ({ data: [{ id: "sha256:aaaaaaaaaaaaaaaa", repository: "web", tag: "1", size: 2048 }] }) }));

const ctr = (name: string, service: string, over: Partial<Container> = {}): Container =>
  ({ id: `id-${name}`, name, image: "web:1", state: "running", status: "Up 1 hour", health: "none", created: 1_700_000_000, ports: null, composeService: service, ...over }) as Container;

const project = (canRun: boolean, services: string[]): ComposeProject =>
  ({ name: "shop", managed: true, capability: { canRun, reason: canRun ? "" : "no compose" }, services: services.map((name) => ({ name })) }) as unknown as ComposeProject;

const wrap = (node: React.ReactNode) => renderToStaticMarkup(<MemoryRouter><TooltipProvider>{node}</TooltipProvider></MemoryRouter>);
const rowCount = (html: string) => html.match(/<tr data-service=/g)?.length ?? 0;

describe("ServicesTab", () => {
  it("renders one row per container and no service header rows", () => {
    const containers = [ctr("web-1", "web"), ctr("db-1", "db"), ctr("job-1", "job", { state: "exited", exitCode: 0, restartPolicy: "no" })];
    const html = wrap(<ServicesTab connectionId="c1" project={project(true, ["web", "db", "job"])} containers={containers} />);
    expect(rowCount(html)).toBe(3);
    expect(html).not.toContain("service ·");
    expect(html).toContain("Completed");
    expect(html).toContain("2 KB");
  });

  it("renders two rows for a service with two replicas, the first with the service-wide menu", () => {
    const containers = [ctr("web-2", "web"), ctr("web-1", "web")];
    const html = wrap(<ServicesTab connectionId="c1" project={project(true, ["web"])} containers={containers} />);
    expect(rowCount(html)).toBe(2);
    expect(html.match(/data-service="web"/g)).toHaveLength(2);
    expect(html).toContain("1 of 2");
    expect(html).toContain("2 of 2");
    expect(html.match(/All containers of service web/g)).toHaveLength(1);
  });

  it("shows Not created and an Up button for a service without container", () => {
    const html = wrap(<ServicesTab connectionId="c1" project={project(true, ["web", "job"])} containers={[ctr("web-1", "web")]} />);
    expect(html).toContain("Not created");
    expect(html).toContain("Compose Up service job");
    expect(html).not.toContain("Compose Up service web");
  });

  it("keeps Up disabled and offers Start when Compose cannot run", () => {
    const html = wrap(<ServicesTab connectionId="c1" project={project(false, ["web", "job"])} containers={[ctr("web-1", "web", { state: "exited" })]} />);
    expect(html).toMatch(/<button[^>]*disabled[^>]*aria-label="Compose Up service job"/);
    expect(html).not.toContain("Compose Up service web");
    expect(html).toContain('aria-label="Start web-1"');
  });
});

describe("ServicesView filter", () => {
  const rows = buildRows(["web", "db", "job"], [ctr("web-1", "web"), ctr("db-1", "db", { state: "exited", exitCode: 1 })]);
  const view = (filter: "all" | "running" | "stopped", list = rows) =>
    wrap(<ServicesView rows={list} filter={filter} onFilter={() => {}} images={[]} busy={false} canRun upReason={undefined} onAct={() => {}} onUp={() => {}} />);

  it("shows only the stopped rows, and counts that match", () => {
    const html = view("stopped");
    expect(rowCount(html)).toBe(2);
    expect(html).not.toContain('data-service="web"');
    expect(html).toMatch(/aria-pressed="true"[^>]*>Stopped<span[^>]*>2</);
    expect(html).toMatch(/>Running<span[^>]*>1</);
  });

  it("says so in one line when a filter has no rows", () => {
    const html = view("running", buildRows(["db"], [ctr("db-1", "db", { state: "exited" })]));
    expect(rowCount(html)).toBe(0);
    expect(html).toContain("No running services.");
  });
});
