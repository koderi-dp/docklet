import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router-dom";
import type { ComposeGraph } from "@bindings/model";
import { EmptyGraph, FallbackNote } from "./GraphNotices";
import { ServiceCard, type ServiceCardProps } from "./ServiceCard";

const base: ServiceCardProps = {
  name: "web", image: "nginx", build: false, kind: "app" as ServiceCardProps["kind"], state: "running", ports: "", volume: "",
  moreVolumes: 0, profile: "", containerId: "", dimmed: false, onShowService: () => {}, onHover: () => {},
};
const card = (p: Partial<ServiceCardProps>) =>
  renderToStaticMarkup(
    <MemoryRouter>
      <ServiceCard {...base} {...p} />
    </MemoryRouter>,
  );

describe("ServiceCard", () => {
  it("is a button when it has containers", () => {
    expect(card({ state: "running", containerId: "c1" })).toMatch(/^<button/);
  });
  it("is an inert div when the service has no container", () => {
    const html = card({ state: "notCreated", profile: "debug" });
    expect(html).toMatch(/^<div/);
    expect(html).toContain("cursor-default");
    expect(html).toContain('aria-label="web, Not created"');
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
