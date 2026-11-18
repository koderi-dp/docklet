import type { BuildRecord, BuildSpec } from "@bindings/model";

export type BuilderChoice = "auto" | "buildkit" | "classic";

export interface ArgRow {
  id: number;
  key: string;
  value: string;
}

/** State of the Build dialog. */
export interface BuildForm {
  contextDir: string;
  dockerfile: string;
  tags: string[];
  args: ArgRow[];
  target: string;
  platform: string; // "" = Engine default
  noCache: boolean;
  pull: boolean;
  builder: BuilderChoice;
}

let nextArgId = 1;
export const newArgRow = (key = ""): ArgRow => ({ id: nextArgId++, key, value: "" });

export function emptyForm(): BuildForm {
  return { contextDir: "", dockerfile: "Dockerfile", tags: [""], args: [], target: "", platform: "", noCache: false, pull: false, builder: "auto" };
}

/**
 * Form for "Build again". History never keeps argument values, so each argument comes back with its name
 * and an empty value that has to be typed again.
 */
export function formFromRecord(r: BuildRecord): BuildForm {
  return {
    contextDir: r.contextDir,
    dockerfile: r.dockerfile || "Dockerfile",
    tags: r.tags?.length ? [...r.tags] : [""],
    args: (r.argNames ?? []).map((n) => newArgRow(n)),
    target: r.target,
    platform: r.platform,
    noCache: r.noCache,
    pull: r.pull,
    builder: "auto",
  };
}

export function toSpec(f: BuildForm): BuildSpec {
  const buildArgs: Record<string, string> = {};
  for (const a of f.args) if (a.key.trim()) buildArgs[a.key.trim()] = a.value;
  return {
    contextDir: f.contextDir,
    dockerfile: f.dockerfile.trim(),
    tags: f.tags.map((t) => t.trim()).filter(Boolean),
    buildArgs,
    target: f.target.trim(),
    platform: f.platform.trim(),
    noCache: f.noCache,
    pull: f.pull,
    builder: f.builder,
  };
}
