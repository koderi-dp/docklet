import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router-dom";
import type { ComposeGraph } from "@bindings/model";
import { EmptyGraph, FallbackNote } from "./GraphNotices";
import { ServiceCard, type ServiceCardProps } from "./ServiceCard";
import { GraphToolbar } from "./GraphToolbar";

const base: ServiceCardProps = {
  name: "web", image: "nginx", build: false, kind: "app" as ServiceCardProps["kind"], state: "running", ports: "", volume: "",
  moreVolumes: 0, profile: "", dimmed: false, selected: false, onSelect: () => {}, onHover: () => {},
};
const card = (p: Partial<ServiceCardProps>) =>
  renderToStaticMarkup(
    <MemoryRouter>
      <ServiceCard {...base} {...p} />
    </MemoryRouter>,
  );

describe("ServiceCard", () => {
  it("is a button that reflects whether it is selected", () => {
    expect(card({})).toMatch(/^<button/);
    expect(card({})).toContain('aria-pressed="false"');
    const html = card({ selected: true });
    expect(html).toContain('aria-pressed="true"');
    expect(html).toContain("ring-primary");
  });
  it("stays a button for a service with no container", () => {
    const html = card({ state: "notCreated", profile: "debug" });
    expect(html).toMatch(/^<button/);
    expect(html).toContain('aria-label="web, Not created"');
    expect(html).toContain("profile: debug");
  });
});

describe("GraphToolbar selection", () => {
  const toolbar = (selection: { containerId: string; hasContainers: boolean } | null, onShowInFile?: () => void) =>
    renderToStaticMarkup(
      <GraphToolbar
        find="" onFind={() => {}} count={3} scale={1} onFit={() => {}} onZoomIn={() => {}} onZoomOut={() => {}}
        selection={selection && { name: "web", needs: 2, neededBy: 1, ...selection }}
        onClear={() => {}} onOpenContainer={() => {}} onShowInServices={() => {}} onShowInFile={onShowInFile}
      />,
    );
  it("shows the service count when nothing is selected", () => {
    expect(toolbar(null)).toContain("3");
    expect(toolbar(null)).not.toContain("Clear");
  });
  it("offers Open container for a single container", () => {
    const html = toolbar({ containerId: "c1", hasContainers: true });
    expect(html).toContain("needs 2 · 1 needs it");
    expect(html).toContain("Open container");
    expect(html).not.toContain("Show in Services");
  });
  it("offers Show in Services for several containers", () => {
    const html = toolbar({ containerId: "", hasContainers: true });
    expect(html).toContain("Show in Services");
    expect(html).not.toContain("Open container");
  });
  it("offers neither without containers, and Show in file only when supplied", () => {
    const html = toolbar({ containerId: "", hasContainers: false });
    expect(html).not.toContain("Open container");
    expect(html).not.toContain("Show in Services");
    expect(html).not.toContain("Show in file");
    expect(toolbar({ containerId: "", hasContainers: false }, () => {})).toContain("Show in file");
  });
});

describe("graph notices", () => {
  const graph = (source: string, note: string) => ({ source, note, services: [] }) as unknown as ComposeGraph;
  it("explains why a container-sourced graph is empty", () => {
    const html = renderToStaticMarkup(<EmptyGraph graph={graph("containers", "The env file is missing.")} />);
    expect(html).toContain("Nothing to draw yet");
    expect(html).toContain("The env file is missing.");
    expect(html).not.toContain("defines no services");
  });
  it("says the project defines no services only for a config graph", () => {
    expect(renderToStaticMarkup(<EmptyGraph graph={graph("config", "")} />)).toContain("defines no services");
  });
  it("does not double the dot in the fallback line", () => {
    const html = renderToStaticMarkup(<FallbackNote note="Config file unknown." />);
    expect(html).toContain("Drawn from this project&#x27;s containers: Config file unknown. Dependencies");
  });
});
