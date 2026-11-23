import { describe, expect, it, vi } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import type { ComposeLocation, Container } from "@bindings/model";
import { orderContainers, presentIds, previewWanted, type NameState } from "@/lib/newComposeFile";
import { NewComposeBody, type NewComposeBodyProps } from "./NewComposeDialog";

vi.mock("@bindings/services", () => ({ ComposeService: {}, DialogService: {} }));

const containers = [
  { id: "c3", name: "zeta", image: "busybox", state: "exited" },
  { id: "c1", name: "web", image: "nginx:1.27", state: "running", composeProject: "shop" },
  { id: "c2", name: "db", image: "postgres:16", state: "running" },
] as Container[];

const base: NewComposeBodyProps = {
  location: { available: true, managedAvailable: true, executionLocation: "This machine" } as ComposeLocation,
  dir: "C:\\apps\\shop",
  onDir: () => {},
  name: "shop",
  onName: () => {},
  nameState: { kind: "free" },
  source: "minimal",
  onSource: () => {},
  containers,
  ticked: new Set<string>(),
  onToggle: () => {},
  includeSecrets: false,
  onIncludeSecrets: () => {},
  templates: [],
  template: "",
  onTemplate: () => {},
  preview: { text: "name: shop\nservices: {}\n", notIncluded: [], loading: false },
  result: {},
  blocked: undefined,
  pending: false,
  onCancel: () => {},
  onCreate: () => {},
  onOpenExisting: () => {},
};

const render = (p: Partial<NewComposeBodyProps>) => renderToStaticMarkup(<NewComposeBody {...base} {...p} />);
/** The opening tag of the button whose text is `label`. */
const buttonTag = (html: string, label: string) => html.match(new RegExp(`<button[^>]*>(?:(?!</button>).)*${label}`, "s"))?.[0] ?? "";

/** The disabled attribute itself; class names also contain the word. */
const DISABLED = / disabled=""/;

describe("NewComposeBody", () => {
  it("offers Create and open for a free name, without a warning", () => {
    const html = render({});
    expect(html).toContain("Create and open");
    expect(html).not.toContain("already running");
    expect(buttonTag(html, "Create and open")).not.toMatch(DISABLED);
    expect(html).toContain("compose.yaml will be created here.");
  });

  it("shows the takeover warning and button for a running project without a file", () => {
    const nameState: NameState = { kind: "takeover", containers: 3, containerIds: ["a", "b", "c"] };
    const html = render({ nameState });
    expect(html).toContain("A project named &quot;shop&quot; is already running (3 containers) without a Compose file in Docklet.");
    expect(html).toContain("This file will become its Compose file: Up and Apply will then change those containers.");
    expect(html).toContain("Create and take over");
    expect(html).not.toContain("Create and open");
  });

  it("disables Create and shows the reason", () => {
    const html = render({ source: "containers", blocked: "Tick at least one container" });
    expect(html).toContain("Tick at least one container");
    expect(buttonTag(html, "Create and open")).toMatch(DISABLED);
  });

  it("lists running containers first with image and Compose project, only for that source", () => {
    expect(render({})).not.toContain("postgres:16");
    const html = render({ source: "containers", ticked: new Set(["c2"]) });
    expect(html.indexOf("db")).toBeLessThan(html.indexOf("zeta"));
    expect(html.indexOf("web")).toBeLessThan(html.indexOf("zeta"));
    expect(html).toContain("postgres:16");
    expect(html).toMatch(/web.*nginx:1\.27.*shop/s);
    expect(html).toContain("Include secret values");
    expect(html).toContain("The original containers keep running. Up will clash with their names and ports until they are stopped or removed.");
  });

  it("shows the preview text and what is not included", () => {
    const html = render({ preview: { text: "name: shop\nservices:\n  web: {}\n", notIncluded: ["web: auto-remove (--rm)"], loading: false } });
    expect(html).toContain("name: shop");
    expect(html).toContain("Not included: web: auto-remove (--rm)");
  });

  it("shows a preview error in place of the editor and disables Create", () => {
    const html = render({ preview: { error: "container gone999: No such container", loading: false }, blocked: "The preview could not be made" });
    expect(html).toContain("container gone999: No such container");
    expect(html).not.toContain("name: shop\nservices");
    expect(buttonTag(html, "Create and open")).toMatch(DISABLED);
  });

  it("shows Open it instead when the folder already has a Compose file", () => {
    const html = render({ result: { existingFile: "/p/compose.yaml" } });
    expect(html).toContain("This folder already has a Compose file.");
    expect(html).toContain("/p/compose.yaml");
    expect(html).toContain("Open it instead");
  });

  it("shows a creation error inline", () => {
    expect(render({ result: { error: "the Compose files are not valid" } })).toMatch(/role="alert"[^>]*>[^<]*the Compose files are not valid/);
  });

  it("hides the Template choice until there are templates", () => {
    expect(render({})).not.toContain("Template");
    const html = render({ templates: [{ id: "web-postgres", title: "Web app + PostgreSQL" }], source: "template", template: "web-postgres" });
    expect(html).toContain("Template");
    expect(html).toContain("Web app + PostgreSQL");
  });
});

describe("previewWanted", () => {
  const ok = { name: "shop", source: "minimal" as const, ticked: 0 };
  it("asks for a preview when the name is valid", () => expect(previewWanted(ok)).toBe(true));
  it("makes no request while the name is empty or invalid", () => {
    expect(previewWanted({ ...ok, name: "" })).toBe(false);
    expect(previewWanted({ ...ok, name: "My Shop" })).toBe(false);
  });
  it("makes no request while no container is ticked", () => {
    expect(previewWanted({ ...ok, source: "containers", ticked: 0 })).toBe(false);
    expect(previewWanted({ ...ok, source: "containers", ticked: 2 })).toBe(true);
  });
});

describe("orderContainers", () => {
  it("puts running containers first, each group by name", () => {
    expect(orderContainers(containers).map((c) => c.name)).toEqual(["db", "web", "zeta"]);
  });
  it("does not change the list it was given", () => {
    orderContainers(containers);
    expect(containers[0].name).toBe("zeta");
  });
});

describe("presentIds", () => {
  it("drops ticked containers that no longer exist", () => expect([...presentIds(new Set(["c1", "gone"]), containers)]).toEqual(["c1"]));
  it("keeps the selection while the container list is still loading", () => expect([...presentIds(new Set(["c1", "gone"]), undefined)]).toEqual(["c1", "gone"]));
  it("keeps the order in which containers were ticked", () => expect([...presentIds(new Set(["c2", "c1"]), containers)]).toEqual(["c2", "c1"]));
});
