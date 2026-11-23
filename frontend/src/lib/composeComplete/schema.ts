import composeSpec from "./compose-spec.json";

/** One entry of a completion list. */
export interface Suggestion {
  label: string;
  /** Short text beside the label, e.g. the value type. */
  detail?: string;
  /** Longer text shown next to the list, e.g. the schema's description. */
  info?: string;
}

/** A JSON Schema node; only the keywords the Compose schema uses are read. */
type Node = Record<string, unknown>;

const ROOT = composeSpec as unknown as Node;

const isNode = (v: unknown): v is Node => typeof v === "object" && v !== null && !Array.isArray(v);

function pointer(ref: string): Node | null {
  if (!ref.startsWith("#/")) return null;
  let cur: unknown = ROOT;
  for (const part of ref.slice(2).split("/")) {
    if (!isNode(cur)) return null;
    cur = cur[part.replace(/~1/g, "/").replace(/~0/g, "~")];
  }
  return isNode(cur) ? cur : null;
}

/** The node itself plus everything it refers to or is combined from; a reference is followed once. */
function expand(node: Node, seen = new Set<Node>(), out: Node[] = []): Node[] {
  if (seen.has(node)) return out;
  seen.add(node);
  out.push(node);
  if (typeof node.$ref === "string") {
    const target = pointer(node.$ref);
    if (target) expand(target, seen, out);
  }
  for (const kw of ["oneOf", "anyOf", "allOf"]) {
    const list = node[kw];
    if (Array.isArray(list)) for (const alt of list) if (isNode(alt)) expand(alt, seen, out);
  }
  return out;
}

function matches(pattern: string, key: string): boolean {
  try {
    return new RegExp(pattern).test(key);
  } catch {
    return true;
  }
}

/** The nodes one step below `nodes`: a key of a map, or "[]" for an item of a list. */
function step(nodes: Node[], seg: string): Node[] {
  const next: Node[] = [];
  for (const n of nodes) {
    if (seg === "[]") {
      if (isNode(n.items)) next.push(n.items);
      continue;
    }
    const props = n.properties;
    if (isNode(props) && isNode(props[seg])) {
      next.push(props[seg] as Node);
      continue;
    }
    let matched = false;
    if (isNode(n.patternProperties)) {
      for (const [pattern, child] of Object.entries(n.patternProperties)) {
        if (isNode(child) && matches(pattern, seg)) {
          next.push(child);
          matched = true;
        }
      }
    }
    if (!matched && isNode(n.additionalProperties)) next.push(n.additionalProperties);
  }
  return next.flatMap((n) => expand(n));
}

function resolve(path: readonly string[]): Node[] {
  let nodes = expand(ROOT);
  for (const seg of path) {
    nodes = step(nodes, seg);
    if (nodes.length === 0) break;
  }
  return nodes;
}

function describe(node: Node): { detail?: string; info?: string } {
  const all = expand(node);
  const info = all.map((n) => n.description).find((d): d is string => typeof d === "string" && d !== "");
  const types = new Set<string>();
  for (const n of all) {
    const t = n.type;
    if (typeof t === "string") types.add(t);
    else if (Array.isArray(t)) for (const x of t) if (typeof x === "string" && x !== "null") types.add(x);
  }
  return { detail: types.size === 1 ? [...types][0] : undefined, info };
}

const byLabel = (a: Suggestion, b: Suggestion) => (a.label < b.label ? -1 : a.label > b.label ? 1 : 0);

/** Keys valid in the map at `path` (see CursorContext.path), sorted by label. */
export function keysAt(path: readonly string[]): Suggestion[] {
  const found = new Map<string, Suggestion>();
  for (const n of resolve(path)) {
    if (!isNode(n.properties)) continue;
    for (const [label, child] of Object.entries(n.properties)) {
      if (found.has(label) || !isNode(child) || child.deprecated === true) continue;
      found.set(label, { label, ...describe(child) });
    }
  }
  return [...found.values()].sort(byLabel);
}

/**
 * Values the Compose schema types as free text although only a few are meaningful. Keyed by the
 * service key; merged with whatever the schema itself lists.
 */
const SERVICE_VALUES: Record<string, string[]> = {
  restart: ["no", "always", "on-failure", "unless-stopped"],
  pull_policy: ["always", "never", "missing", "build"],
  network_mode: ["bridge", "host", "none"],
};

/** Fixed values for `key` in the map at `path`, or for an item of the list at `path` when key is undefined. */
export function valuesAt(path: readonly string[], key: string | undefined): Suggestion[] {
  const at = resolve(path);
  if (at.length === 0) return [];
  const labels = new Set<string>();
  for (const n of step(at, key ?? "[]")) {
    if (Array.isArray(n.enum)) for (const v of n.enum) if (typeof v === "string" || typeof v === "number") labels.add(String(v));
    if (typeof n.const === "string") labels.add(n.const);
    if (n.type === "boolean" || (Array.isArray(n.type) && n.type.includes("boolean"))) {
      labels.add("true");
      labels.add("false");
    }
  }
  if (key !== undefined && path.length === 2 && path[0] === "services") for (const v of SERVICE_VALUES[key] ?? []) labels.add(v);
  return [...labels].map((label) => ({ label })).sort(byLabel);
}
