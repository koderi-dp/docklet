import { describe, expect, it, vi } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { TooltipProvider } from "@/components/ui/tooltip";
import type { ComposeProject } from "@bindings/model";
import type { ProjectTotals } from "@/lib/projectTotals";
import type { DiskState } from "./ProjectsTable";
import { ProjectResourcesCell } from "./ProjectResourcesCell";

vi.mock("@bindings/services", () => ({ ComposeService: {}, ImageService: {}, ContainerService: {} }));
vi.mock("@/hooks/useCompose", () => ({
  useProjectEngineBatch: () => ({ pending: false, run: vi.fn() }),
  useComposeVerb: () => ({ run: vi.fn(), isBusy: () => false }),
}));

const project = { name: "shop", notCreated: false } as unknown as ComposeProject;
const disk = (over: Partial<DiskState>): DiskState => ({ data: undefined, loading: false, requested: false, onCalculate: () => {}, ...over });
const running = {
  project: "shop",
  running: 2,
  cpu: { value: 2.4, partial: false },
  memory: { value: 10_000_000, partial: false },
  runningKey: "a,b",
} as unknown as ProjectTotals;

const render = (totals: ProjectTotals | undefined, d: DiskState) =>
  renderToStaticMarkup(
    <TooltipProvider>
      <ProjectResourcesCell project={project} totals={totals} idle={undefined} disk={d} diskEntry={undefined} />
    </TooltipProvider>,
  );

describe("ProjectResourcesCell disk part", () => {
  it("offers Calculate again after a failed calculation (requested, no data, not loading)", () => {
    expect(render(undefined, disk({ requested: true }))).toContain("Calculate");
  });

  it("does not offer Calculate while the calculation is loading", () => {
    expect(render(undefined, disk({ requested: true, loading: true }))).not.toContain("Calculate");
  });

  it("puts the disk part on its own line below CPU and Mem while running", () => {
    const html = render(running, disk({}));
    expect(html).toContain("flex-col");
    expect(html.indexOf("Mem")).toBeLessThan(html.indexOf("Disk"));
    expect(html).not.toMatch(/·<\/span>\s*<span[^>]*><span[^>]*>Disk/);
    expect(html.match(/·/g)?.length).toBe(1);
  });
});
