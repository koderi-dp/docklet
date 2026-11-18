// Export and import of saved log filters as a JSON file. Pure: no React, no DOM.
// A file is data only: it is validated strictly, ids are always generated here, and patterns are never
// compiled or run during import (invalid regular expressions are kept and shown as invalid like any other rule).
import type { SavedFilter } from "@/stores/logFilterStore";
import { HIGHLIGHT_COLOURS, newRuleId, type HighlightColour, type IncludeMode, type Rule, type RuleKind, type RuleTarget } from "./rules";

export const FILE_FORMAT = "docklet-log-filters";
export const FILE_VERSION = 1;

/** Largest file the importer reads, and bounds that keep a hostile file from filling the store. */
export const MAX_FILE_BYTES = 1024 * 1024;
const MAX_FILTERS = 200;
const MAX_RULES = 200;
const MAX_PATTERN = 2000;
const MAX_NAME = 100;

const KINDS: readonly RuleKind[] = ["include", "exclude", "highlight"];
const TARGETS: readonly RuleTarget[] = ["message", "container", "level"];
const MODES: readonly IncludeMode[] = ["any", "all"];

/** The JSON text of a file holding every saved filter (ids are not exported). */
export function serializeSavedFilters(saved: readonly SavedFilter[]): string {
  const filters = saved.map((s) => ({
    name: s.name,
    includeMode: s.includeMode,
    rules: s.rules.map((r) => ({ kind: r.kind, target: r.target, pattern: r.pattern, enabled: r.enabled, ...(r.kind === "highlight" && r.colour ? { colour: r.colour } : {}) })),
  }));
  return JSON.stringify({ format: FILE_FORMAT, version: FILE_VERSION, filters }, null, 2) + "\n";
}

export type ParseResult = { ok: true; filters: SavedFilter[] } | { ok: false; error: string };

const isObject = (v: unknown): v is Record<string, unknown> => typeof v === "object" && v !== null && !Array.isArray(v);
const oneOf = <T extends string>(list: readonly T[], v: unknown): v is T => typeof v === "string" && (list as readonly string[]).includes(v);

function parseRule(raw: unknown, where: string): Rule | string {
  if (!isObject(raw)) return `${where} is not an object.`;
  if (!oneOf(KINDS, raw.kind)) return `${where}: "kind" must be include, exclude or highlight.`;
  if (!oneOf(TARGETS, raw.target)) return `${where}: "target" must be message, container or level.`;
  if (typeof raw.pattern !== "string" || raw.pattern === "") return `${where}: "pattern" must be a non-empty text.`;
  if (raw.pattern.length > MAX_PATTERN) return `${where}: "pattern" is longer than ${MAX_PATTERN} characters.`;
  if (typeof raw.enabled !== "boolean") return `${where}: "enabled" must be true or false.`;
  const rule: Rule = { id: newRuleId(), kind: raw.kind, target: raw.target, pattern: raw.pattern, enabled: raw.enabled };
  if (raw.kind === "highlight") {
    if (raw.colour !== undefined && !oneOf<HighlightColour>(HIGHLIGHT_COLOURS, raw.colour)) return `${where}: "colour" must be one of ${HIGHLIGHT_COLOURS.join(", ")}.`;
    rule.colour = (raw.colour as HighlightColour | undefined) ?? "warning";
  }
  return rule;
}

/** Reads and validates the text of a saved-filters file. Any problem rejects the whole file with a message. */
export function parseSavedFilters(text: string): ParseResult {
  let data: unknown;
  try {
    data = JSON.parse(text);
  } catch {
    return { ok: false, error: "This is not a JSON file." };
  }
  if (!isObject(data) || data.format !== FILE_FORMAT) return { ok: false, error: "This is not a Docklet saved-filters file." };
  if (typeof data.version !== "number" || !Number.isInteger(data.version)) return { ok: false, error: 'The file has no valid "version".' };
  if (data.version > FILE_VERSION) return { ok: false, error: `The file was made by a newer Docklet (file version ${data.version}); this one reads version ${FILE_VERSION}.` };
  if (data.version < 1) return { ok: false, error: `Unsupported file version ${data.version}.` };
  if (!Array.isArray(data.filters)) return { ok: false, error: 'The file has no "filters" list.' };
  if (data.filters.length === 0) return { ok: false, error: "The file contains no filters." };
  if (data.filters.length > MAX_FILTERS) return { ok: false, error: `The file has more than ${MAX_FILTERS} filters.` };

  const out: SavedFilter[] = [];
  for (let i = 0; i < data.filters.length; i++) {
    const f: unknown = data.filters[i];
    const at = `Filter ${i + 1}`;
    if (!isObject(f)) return { ok: false, error: `${at} is not an object.` };
    if (typeof f.name !== "string" || f.name.trim() === "") return { ok: false, error: `${at}: "name" must be a non-empty text.` };
    if (f.name.trim().length > MAX_NAME) return { ok: false, error: `${at}: "name" is longer than ${MAX_NAME} characters.` };
    const label = `${at} (“${f.name.trim()}”)`;
    if (!oneOf(MODES, f.includeMode)) return { ok: false, error: `${label}: "includeMode" must be any or all.` };
    if (!Array.isArray(f.rules)) return { ok: false, error: `${label}: "rules" must be a list.` };
    if (f.rules.length === 0) return { ok: false, error: `${label} has no rules.` };
    if (f.rules.length > MAX_RULES) return { ok: false, error: `${label} has more than ${MAX_RULES} rules.` };
    const rules: Rule[] = [];
    for (let j = 0; j < f.rules.length; j++) {
      const r = parseRule(f.rules[j], `${label}, rule ${j + 1}`);
      if (typeof r === "string") return { ok: false, error: r };
      rules.push(r);
    }
    out.push({ id: newRuleId(), name: f.name.trim(), includeMode: f.includeMode, rules });
  }
  return { ok: true, filters: out };
}

export interface MergeResult {
  merged: SavedFilter[];
  added: number;
  /** How many imported filters got " (imported)" because their name was taken. */
  renamed: number;
}

const key = (name: string) => name.trim().toLowerCase();

/** Adds imported filters after the existing ones. A taken name keeps both: the imported one gets " (imported)". */
export function mergeImported(existing: readonly SavedFilter[], incoming: readonly SavedFilter[]): MergeResult {
  const taken = new Set(existing.map((s) => key(s.name)));
  let renamed = 0;
  const added = incoming.map((s) => {
    let name = s.name;
    if (taken.has(key(name))) {
      renamed++;
      name = `${s.name} (imported)`;
      for (let n = 2; taken.has(key(name)); n++) name = `${s.name} (imported ${n})`;
    }
    taken.add(key(name));
    return { ...s, name };
  });
  return { merged: [...existing, ...added], added: added.length, renamed };
}
