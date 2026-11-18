import { describe, expect, it } from "vitest";
import type { BuildRecord } from "@bindings/model";
import { formFromRecord, toSpec } from "./buildForm";
import { formatDuration, isSecretName, relativeInside, summaryLine } from "./buildFormat";

describe("build form", () => {
  it("Build again keeps argument names but never values", () => {
    const rec = { contextDir: "/w/app", dockerfile: "docker/Dockerfile", tags: ["app:1"], argNames: ["API_TOKEN", "GREETING"], target: "final", platform: "linux/arm64", noCache: true, pull: false } as BuildRecord;
    const form = formFromRecord(rec);
    expect(form.args.map((a) => [a.key, a.value])).toEqual([["API_TOKEN", ""], ["GREETING", ""]]);
    expect(form.builder).toBe("auto");
    const spec = toSpec({ ...form, args: form.args.map((a) => ({ ...a, value: a.key === "GREETING" ? "hi" : "" })) });
    expect(spec.tags).toEqual(["app:1"]);
    expect(spec.buildArgs).toEqual({ API_TOKEN: "", GREETING: "hi" });
    expect(spec.platform).toBe("linux/arm64");
  });

  it("drops blank tags and unnamed arguments", () => {
    const base = formFromRecord({ contextDir: "/w", dockerfile: "", tags: [], argNames: [], target: "", platform: "", noCache: false, pull: false } as unknown as BuildRecord);
    const spec = toSpec({ ...base, tags: ["a:1", " "], args: [{ id: 1, key: " ", value: "x" }] });
    expect(spec.tags).toEqual(["a:1"]);
    expect(spec.buildArgs).toEqual({});
  });
});

describe("build format", () => {
  it("formats durations and the summary line", () => {
    expect(formatDuration(400)).toBe("0.4 s");
    expect(formatDuration(41_000)).toBe("41 s");
    expect(formatDuration(125_000)).toBe("2 m 05 s");
    expect(summaryLine(5, 9, 2, 41_000)).toBe("5 of 9 steps · 2 cached · 41 s");
    expect(summaryLine(1, 1, 0)).toBe("1 of 1 step");
  });

  it("spots secret-looking argument names", () => {
    expect(["API_TOKEN", "db_password", "SSH_PRIVATE_KEY", "NPM_AUTH"].every(isSecretName)).toBe(true);
    expect(["GREETING", "NODE_VERSION", "KEYBOARD"].some(isSecretName)).toBe(false);
  });

  it("finds the Dockerfile relative to the context", () => {
    expect(relativeInside("C:\\w\\app", "c:\\w\\app\\docker\\Dockerfile")).toBe("docker/Dockerfile");
    expect(relativeInside("/w/app", "/w/other/Dockerfile")).toBeNull();
  });
});
