import { describe, expect, it, vi } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router-dom";
import { TooltipProvider } from "@/components/ui/tooltip";
import { buildRows } from "@/lib/serviceRows";
import { formatDateTime } from "@/components/containers/format";
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
const F = { filter: "all" as const, onFilterChange: () => {} };

describe("ServicesTab", () => {
  it("renders one row per container and no service header rows", () => {
    const containers = [ctr("web-1", "web"), ctr("db-1", "db"), ctr("job-1", "job", { state: "exited", exitCode: 0, restartPolicy: "no" })];
    const html = wrap(<ServicesTab connectionId="c1" project={project(true, ["web", "db", "job"])} containers={containers} {...F} />);
    expect(rowCount(html)).toBe(3);
    expect(html).not.toContain("service ·");
    expect(html).toContain("Completed");
    expect(html).toContain("2 KB");
  });

  it("renders two rows for a service with two replicas, the first with the service-wide menu", () => {
    const containers = [ctr("web-2", "web"), ctr("web-1", "web")];
    const html = wrap(<ServicesTab connectionId="c1" project={project(true, ["web"])} containers={containers} {...F} />);
    expect(rowCount(html)).toBe(2);
    expect(html.match(/data-service="web"/g)).toHaveLength(2);
    expect(html).toContain("1 of 2");
    expect(html).toContain("2 of 2");
    expect(html.match(/All containers of service web/g)).toHaveLength(1);
  });

  it("shows Not created and an Up button for a service without container", () => {
    const html = wrap(<ServicesTab connectionId="c1" project={project(true, ["web", "job"])} containers={[ctr("web-1", "web")]} {...F} />);
    expect(html).toContain("Not created");
    expect(html).toContain("Compose Up service job");
    expect(html).not.toContain("Compose Up service web");
  });

  it("keeps Up disabled and offers Start when Compose cannot run", () => {
    const html = wrap(<ServicesTab connectionId="c1" project={project(false, ["web", "job"])} containers={[ctr("web-1", "web", { state: "exited" })]} {...F} />);
    expect(html).toMatch(/<button[^>]* disabled=""[^>]*aria-label="Compose Up service job"/);
    expect(html).not.toContain("Compose Up service web");
    expect(html).toContain('aria-label="Start web-1"');
  });

  it("enables Up when Compose can run", () => {
    const html = wrap(<ServicesTab connectionId="c1" project={project(true, ["web", "job"])} containers={[ctr("web-1", "web")]} {...F} />);
    const tag = html.match(/<button[^>]*aria-label="Compose Up service job"[^>]*>/)?.[0] ?? "";
    expect(tag).not.toBe("");
    expect(tag).not.toContain(' disabled=""');
  });

  it("offers the container's own Start beside Up when Compose can run", () => {
    const html = wrap(<ServicesTab connectionId="c1" project={project(true, ["web"])} containers={[ctr("web-1", "web", { state: "exited" })]} {...F} />);
    expect(html).toContain('aria-label="Start web-1"');
    expect(html).toContain("Compose Up service web");
    expect(html.indexOf("Compose Up service web")).toBeLessThan(html.indexOf('aria-label="Start web-1"'));
  });

  it("links the container name, and shows dashes for a row without container", () => {
    const linked = wrap(<ServicesTab connectionId="c1" project={project(true, ["web"])} containers={[ctr("web-1", "web")]} {...F} />);
    expect(linked).toContain('href="/containers/id-web-1"');
    const bare = wrap(<ServicesTab connectionId="c1" project={project(true, ["job"])} containers={[]} {...F} />);
    expect(bare.match(/<td[^>]*><span class="text-muted-foreground">—<\/span><\/td>/g)).toHaveLength(4); // Image, Ports, Image size, Created
  });

  it("puts the exact date and time of creation in the Created title", () => {
    const html = wrap(<ServicesTab connectionId="c1" project={project(true, ["web"])} containers={[ctr("web-1", "web")]} {...F} />);
    expect(html).toContain(`title="${formatDateTime(new Date(1_700_000_000 * 1000))}"`);
  });

  it("applies the filter it is given", () => {
    const containers = [ctr("web-1", "web"), ctr("db-1", "db", { state: "exited" })];
    const html = wrap(<ServicesTab connectionId="c1" project={project(true, ["web", "db"])} containers={containers} filter="stopped" onFilterChange={() => {}} />);
    expect(rowCount(html)).toBe(1);
    expect(html).toContain('data-service="db"');
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

  it("moves the service-wide menu to the first visible replica", () => {
    const two = buildRows(["web"], [ctr("web-1", "web"), ctr("web-2", "web", { state: "exited" })]);
    const html = view("stopped", two);
    expect(rowCount(html)).toBe(1);
    expect(html.match(/All containers of service web/g)).toHaveLength(1);
  });

  it("disables the service-wide menu while busy", () => {
    const two = buildRows(["web"], [ctr("web-1", "web"), ctr("web-2", "web")]);
    const html = wrap(<ServicesView rows={two} filter="all" onFilter={() => {}} images={[]} busy canRun upReason={undefined} onAct={() => {}} onUp={() => {}} />);
    expect(html).toMatch(/<button[^>]* disabled=""[^>]*aria-label="All containers of service web"/);
  });

  it("enables the service-wide menu when not busy", () => {
    const two = buildRows(["web"], [ctr("web-1", "web"), ctr("web-2", "web")]);
    const html = view("all", two);
    const tag = html.match(/<button[^>]*aria-label="All containers of service web"[^>]*>/)?.[0] ?? "";
    expect(tag).not.toBe("");
    expect(tag).not.toContain(' disabled=""');
  });

  it("says so in one line when a filter has no rows", () => {
    const html = view("running", buildRows(["db"], [ctr("db-1", "db", { state: "exited" })]));
    expect(rowCount(html)).toBe(0);
    expect(html).toContain("No running services.");
  });
});
