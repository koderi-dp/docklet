import { describe, expect, it, vi } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import type { ComposeWSLPathIssue, ComposeWSLPathReport } from "@bindings/model";
import { WSLPathBody } from "./wslPathGuard";

vi.mock("@bindings/services", () => ({ ComposeService: {} }));

const fixable: ComposeWSLPathIssue = {
  service: "devrun",
  source: "C:/Source/amagi/studio-e",
  target: "/studio",
  linuxPath: "/mnt/c/Source/amagi/studio-e",
  problem: "",
  file: "/mnt/c/Source/dv/compose.yaml",
  line: 13,
  oldLine: "      - C:/Source/amagi/studio-e:/studio:ro",
  newLine: "      - ${STUDIO_DIR:-C:/Source/amagi/studio-e}:/studio:ro",
  variable: "STUDIO_DIR",
};

const report = (issues: ComposeWSLPathIssue[], fixableAny = true): ComposeWSLPathReport => ({ issues, envFile: "/mnt/c/Source/dv/.env.wsl", fixable: fixableAny });

const render = (r: ComposeWSLPathReport) => renderToStaticMarkup(<WSLPathBody project="dv" report={r} />);

describe("WSLPathBody", () => {
  it("shows the Windows path, the Linux path and the exact line change", () => {
    const html = render(report([fixable]));
    expect(html).toContain("C:/Source/amagi/studio-e");
    expect(html).toContain("/mnt/c/Source/amagi/studio-e");
    expect(html).toContain("- C:/Source/amagi/studio-e:/studio:ro");
    expect(html).toContain("- ${STUDIO_DIR:-C:/Source/amagi/studio-e}:/studio:ro");
    expect(html).toContain("STUDIO_DIR");
  });

  it("says where the Linux path goes and that the file keeps working elsewhere", () => {
    const html = render(report([fixable]));
    expect(html).toContain("/mnt/c/Source/dv/.env.wsl");
    expect(html).toContain("works as before on Docker Desktop");
  });

  it("gives the reason instead of a change when a mount cannot be fixed", () => {
    const html = render(report([{ ...fixable, linuxPath: "", problem: "The WSL distribution cannot reach Z:/far.", oldLine: "", newLine: "", variable: "" }], false));
    expect(html).toContain("cannot reach Z:/far");
    expect(html).not.toContain("line-through");
    expect(html).not.toContain(".env.wsl");
  });

  it("says so when the file already uses the variable", () => {
    const line = "      - ${STUDIO_DIR:-C:/Source/amagi/studio-e}:/studio:ro";
    const html = render(report([{ ...fixable, oldLine: line, newLine: line }]));
    expect(html).toContain("already uses this variable");
    expect(html).not.toContain("line-through");
  });

  it("uses the plural for several mounts", () => expect(render(report([fixable, { ...fixable, target: "/other" }]))).toContain("these mounts"));
});
