import { isValidElement } from "react";
import { describe, expect, it, vi } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ComposeFileList, ComposeFileRef, ComposeOutlineItem, ComposeProject } from "@bindings/model";
import { composeFilesKey } from "@/hooks/useComposeFiles";
import { ComposeFileTab } from "./ComposeFileTab";
import { FileSwitcher } from "./FileSwitcher";
import { ServiceOutline } from "./ServiceOutline";

vi.mock("@bindings/services", () => ({ ComposeService: {} }));

const items = [
  { name: "web", line: 3 },
  { name: "db", line: 12 },
] as ComposeOutlineItem[];

describe("ServiceOutline", () => {
  it("lists service names with their line numbers", () => {
    const html = renderToStaticMarkup(<ServiceOutline services={items} onJump={() => {}} />);
    expect(html).toMatch(/web.*3.*db.*12/s);
  });
  it("calls onJump with the line of the clicked service", () => {
    const onJump = vi.fn();
    const clicks: (() => void)[] = [];
    const walk = (n: unknown): void => {
      if (Array.isArray(n)) return n.forEach(walk);
      if (!isValidElement<{ onClick?: () => void; children?: unknown }>(n)) return;
      if (n.props.onClick) clicks.push(n.props.onClick);
      walk(n.props.children);
    };
    walk(ServiceOutline({ services: items, onJump }));
    clicks[1]();
    expect(onJump).toHaveBeenCalledWith(12);
  });
});

describe("FileSwitcher", () => {
  const files = [
    { path: "/p/compose.yaml", kind: "config" },
    { path: "/p/.env", kind: "env" },
  ] as ComposeFileRef[];
  it("marks only env files read-only", () => {
    const html = renderToStaticMarkup(<FileSwitcher files={files} value="/p/compose.yaml" onChange={() => {}} />);
    expect(html.match(/read-only/g)).toHaveLength(1);
    expect(html.indexOf("read-only")).toBeGreaterThan(html.indexOf(".env"));
  });
});

describe("ComposeFileTab", () => {
  const project = { name: "shop" } as ComposeProject;
  const render = (list: Partial<ComposeFileList>) => {
    const qc = new QueryClient();
    qc.setQueryData(composeFilesKey("c1", "shop"), { files: [], canEdit: false, reason: "", ...list });
    return renderToStaticMarkup(
      <QueryClientProvider client={qc}>
        <ComposeFileTab connectionId="c1" project={project} containers={[]} active onDirtyChange={() => {}} panel={<i>side panel</i>} />
      </QueryClientProvider>,
    );
  };
  it("shows the reason and no editor when the files cannot be edited", () => {
    const html = render({ canEdit: false, reason: "This project runs on a remote Engine." });
    expect(html).toContain("This project runs on a remote Engine.");
    expect(html).not.toContain("Save");
    expect(html).not.toContain("side panel");
  });
});
