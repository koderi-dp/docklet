import { describe, expect, it, vi } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import type { ComposeClonePreview, ComposeLocation } from "@bindings/model";
import { CloneBody, type CloneBodyProps } from "./CloneProjectDialog";

vi.mock("@bindings/services", () => ({ ComposeService: {}, DialogService: {} }));

const preview: ComposeClonePreview = {
  sourceDir: "/home/dev/shop",
  suggestedName: "shop-copy",
  suggestedDir: "/home/dev/shop-copy",
  sizeBytes: 1536,
  files: 3,
  sizeKnown: true,
  running: false,
  outside: [],
  collisions: [],
  blocked: "",
};

const base: CloneBodyProps = {
  connectionId: "c1",
  project: "shop",
  location: { available: true, managedAvailable: true, kind: "wsl", executionLocation: "WSL: Ubuntu" } as ComposeLocation,
  preview,
  name: "shop-copy",
  onName: () => {},
  dir: "/home/dev/shop-copy",
  onDir: () => {},
  startAfter: false,
  onStartAfter: () => {},
};

const render = (p: Partial<CloneBodyProps>) => renderToStaticMarkup(<CloneBody {...base} {...p} />);

describe("CloneBody", () => {
  it("shows the folder, its size and where the copy runs", () => {
    const html = render({});
    expect(html).toContain("/home/dev/shop");
    expect(html).toContain("3 files, 1.5 KB");
    expect(html).toContain("WSL: Ubuntu");
    expect(html).toContain("Named volumes are not copied");
  });

  it("has no warning box for a plain, stopped project", () => expect(render({})).not.toContain("Before you clone"));

  it("lists what the copy shares or cannot hold, and the files it does not copy", () => {
    const html = render({
      preview: {
        ...preview,
        running: true,
        outside: ["/srv/shared.env"],
        collisions: ["Service web publishes host port 8080: only one of the two projects can hold it at a time."],
      },
    });
    expect(html).toContain("Before you clone");
    expect(html).toContain("The original is running");
    expect(html).toContain("/srv/shared.env is outside the folder and is not copied");
    expect(html).toContain("host port 8080");
  });

  it("points out a large folder", () => expect(render({ preview: { ...preview, sizeBytes: 2 * 1024 ** 3 } })).toContain("The folder is large"));

  it("shows why a project cannot be cloned instead of its size", () => {
    const html = render({ preview: { ...preview, blocked: "The project has no folder to copy." } });
    expect(html).toContain("The project has no folder to copy.");
    expect(html).not.toContain("3 files");
    expect(html).not.toContain("Before you clone");
  });

  it("says a path on a Windows drive is slower inside WSL", () => expect(render({ dir: "C:\\Source\\shop-copy" })).toContain("Windows drive"));

  it("shows the reason a copy failed", () => expect(render({ error: "could not copy the folder: disk full" })).toContain("disk full"));
});
