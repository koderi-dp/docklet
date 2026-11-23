import type { CursorContext } from "./cursorPath";
import type { Suggestion } from "./schema";

/** Names defined in the open Compose file, in document order. */
export interface DocumentNames {
  services: string[];
  volumes: string[];
  networks: string[];
}

const SECTION = /^(services|volumes|networks):\s*(#.*)?$/;
const CHILD_KEY = /^([A-Za-z0-9_.-]+)\s*:(\s|$)/;

/**
 * Child keys of the top-level services, volumes and networks maps. It reads indentation, not YAML:
 * the file is usually half-typed when names are wanted.
 */
export function documentNames(text: string): DocumentNames {
  const names: DocumentNames = { services: [], volumes: [], networks: [] };
  let section: keyof DocumentNames | null = null;
  let childIndent = -1;
  for (const raw of text.split("\n")) {
    const line = raw.endsWith("\r") ? raw.slice(0, -1) : raw;
    const body = line.trimStart();
    if (body === "" || body.startsWith("#")) continue;
    const indent = line.length - body.length;
    if (indent === 0) {
      const m = SECTION.exec(body);
      section = m ? (m[1] as keyof DocumentNames) : null;
      childIndent = -1;
      continue;
    }
    if (!section || line.slice(0, indent).includes("\t")) continue;
    if (childIndent === -1) childIndent = indent;
    if (indent !== childIndent) continue;
    const m = CHILD_KEY.exec(body);
    if (m && !names[section].includes(m[1])) names[section].push(m[1]);
  }
  return names;
}

/** Image references usable after `image:`: every repo:tag once, untagged entries left out, sorted. */
export function imageNames(repoTags: readonly (readonly string[] | null | undefined)[]): string[] {
  const out = new Set<string>();
  for (const tags of repoTags) for (const t of tags ?? []) if (t && !t.includes("<none>")) out.add(t);
  return [...out].sort();
}

const SERVICE_LISTS = new Set(["depends_on", "links", "volumes_from"]);

const asSuggestions = (labels: readonly string[], detail: string): Suggestion[] => labels.map((label) => ({ label, detail }));

/** Suggestions that come from the project for this position; empty when the position has none. */
export function projectValues(ctx: CursorContext, names: DocumentNames, images: readonly string[]): Suggestion[] {
  const p = ctx.path;
  if (p[0] !== "services" || p.length < 2) return [];
  const others = names.services.filter((s) => s !== p[1]);
  const item = ctx.kind === "value" && ctx.key === undefined && p.length === 4 && p[3] === "[]";
  const mapKey = ctx.kind === "key" && p.length === 3;
  const field = p[2];

  if (ctx.kind === "value" && p.length === 2) {
    if (ctx.key === "image") return asSuggestions(images, "image");
    if (ctx.key === "network_mode") return asSuggestions(others.map((s) => `service:${s}`), "service");
    return [];
  }
  if (ctx.kind === "value" && p.length === 3 && field === "extends" && ctx.key === "service") return asSuggestions(others, "service");
  if ((item && SERVICE_LISTS.has(field)) || (mapKey && field === "depends_on")) return asSuggestions(others, "service");
  if ((item || mapKey) && field === "networks") return asSuggestions(names.networks, "network");
  if (item && field === "volumes") return asSuggestions(names.volumes.map((v) => `${v}:`), "volume");
  return [];
}
