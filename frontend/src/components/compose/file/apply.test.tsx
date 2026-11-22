import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { ApplyPanel } from "./ApplyPanel";

const opts = { removeOrphans: false, profiles: [] as string[] };
const render = (plan: { items: { service: string; action: string; detail: string }[]; error: string }, dirty = false) =>
  renderToStaticMarkup(<ApplyPanel plan={plan} loading={false} dirty={dirty} options={opts} onOptions={() => {}} />);

describe("ApplyPanel", () => {
  it("renders one row per item with its label and detail", () => {
    const html = render({
      error: "",
      items: [
        { service: "web", action: "recreateImage", detail: "image changed" },
        { service: "db", action: "unchanged", detail: "" },
      ],
    });
    expect(html).toMatch(/web.*Recreate \(new image\).*image changed/s);
    expect(html).toMatch(/db.*Unchanged/s);
    expect(html).toContain("Saved, not applied");
  });
  it("shows the error instead of rows", () => {
    const html = render({ error: "service web has no image", items: [{ service: "web", action: "new", detail: "" }] });
    expect(html).toContain("docker compose config failed");
    expect(html).toContain("service web has no image");
    expect(html).toContain("Apply is off until this is fixed.");
    expect(html).not.toContain(">web<");
  });
  it("says nothing to do for an unchanged plan", () => {
    const html = render({ error: "", items: [{ service: "web", action: "unchanged", detail: "" }] });
    expect(html).toContain("Nothing to do");
    expect(html).toContain("Up to date");
  });
  it("does not say Up to date for a configuration error", () => {
    const html = render({ error: "bad", items: [] });
    expect(html).toContain("Saved, with configuration errors");
    expect(html).not.toContain("Up to date");
  });
  it("shows a failed plan query in place of the rows", () => {
    const html = renderToStaticMarkup(<ApplyPanel plan={undefined} loading={false} dirty={false} options={opts} onOptions={() => {}} queryError="engine down" />);
    expect(html).toContain("engine down");
    expect(html).toContain("text-destructive");
    expect(html).not.toContain("Up to date");
    expect(html).not.toContain("Nothing to do");
  });
  it("shows the capability reason when the CLI cannot run", () => {
    const html = renderToStaticMarkup(<ApplyPanel plan={undefined} loading={false} dirty={false} options={opts} onOptions={() => {}} canRun={false} capReason="No docker CLI" />);
    expect(html).toContain("No docker CLI");
    expect(html).not.toContain("Up to date");
    expect(html).not.toContain("Nothing to do");
  });
  it("shows a one-shot as Runs again and still reaches Up to date", () => {
    const html = render({ error: "", items: [{ service: "migrate", action: "start", detail: "", oneShot: true } as never] });
    expect(html).toContain("Runs again");
    expect(html).toContain("Up to date");
    expect(html).toContain("Nothing to do");
  });
  it("says the rows describe the saved file while dirty", () => {
    expect(render({ error: "", items: [] }, true)).toContain("Unsaved changes: save to see the effect");
  });
});
