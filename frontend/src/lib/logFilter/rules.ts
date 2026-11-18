// Rule model and evaluation of the log filter. Pure: no React, no DOM.
//   include    keep lines that match (several = any of them, or all of them)
//   exclude    drop lines that match; always wins over includes
//   highlight  never filters, only colours matching lines
import type { LogLevel } from "@/lib/logLevel";
import { getPattern, type CompiledPattern } from "./pattern";

export type RuleKind = "include" | "exclude" | "highlight";
export type RuleTarget = "message" | "container" | "level";
export type IncludeMode = "any" | "all";
/** Semantic colour name of a highlight (mapped to theme tokens by the UI). */
export type HighlightColour = "warning" | "success" | "info" | "primary" | "destructive";

export const HIGHLIGHT_COLOURS: readonly HighlightColour[] = ["warning", "success", "info", "primary", "destructive"];

export interface Rule {
  id: string;
  kind: RuleKind;
  target: RuleTarget;
  pattern: string;
  enabled: boolean;
  /** Highlight rules only. */
  colour?: HighlightColour;
}

/** What a rule needs to know about a log line. */
export interface FilterableLine {
  container: string;
  level: LogLevel;
  /** Message text without terminal escape codes. */
  text: string;
  /** Epoch ms of the Engine timestamp; undefined when the line has none. */
  time?: number;
}

export interface EvalOptions {
  includeMode: IncludeMode;
  /** Visible levels; empty or absent = all levels. */
  levels?: ReadonlySet<LogLevel>;
  /** Epoch ms lower bound; lines without a time are kept (their age is unknown). */
  since?: number;
}

export interface EvalResult<L> {
  visible: L[];
  /** Highlight colour of each visible line (same index as `visible`). */
  highlights: Array<HighlightColour | undefined>;
  /** Per rule id: lines the rule matches (inside the time range), independent of the other rules. Disabled or invalid rules are absent. */
  counts: Record<string, number>;
  /** Per level, among lines that pass the time range and the include/exclude rules (the level filter itself is ignored, so the chips can show what they would add). */
  levelCounts: Record<LogLevel, number>;
}

/** Container and level values are matched as a whole (`web-*`, `error`); messages by substring. */
const WHOLE_TARGETS: ReadonlySet<RuleTarget> = new Set(["container", "level"]);

export function compileRule(rule: Pick<Rule, "pattern" | "target">): CompiledPattern {
  return getPattern(rule.pattern, { whole: WHOLE_TARGETS.has(rule.target) });
}

function valueOf(line: FilterableLine, target: RuleTarget): string {
  return target === "message" ? line.text : target === "container" ? line.container : line.level;
}

interface Active {
  id: string;
  target: RuleTarget;
  test: (v: string) => boolean;
  colour?: HighlightColour;
}

function activeRules(rules: readonly Rule[], kind: RuleKind): Active[] {
  const out: Active[] = [];
  for (const r of rules) {
    if (!r.enabled || r.kind !== kind) continue;
    const p = compileRule(r);
    if (p.error) continue; // an invalid rule filters nothing
    out.push({ id: r.id, target: r.target, test: p.test, colour: r.colour });
  }
  return out;
}

export function evaluate<L extends FilterableLine>(lines: readonly L[], rules: readonly Rule[], opts: EvalOptions): EvalResult<L> {
  const inc = activeRules(rules, "include");
  const exc = activeRules(rules, "exclude");
  const hil = activeRules(rules, "highlight");
  const counts: Record<string, number> = {};
  for (const r of [...inc, ...exc, ...hil]) counts[r.id] = 0;
  const levelCounts: Record<LogLevel, number> = { error: 0, warn: 0, info: 0, debug: 0, other: 0 };
  const visible: L[] = [];
  const highlights: Array<HighlightColour | undefined> = [];
  const levels = opts.levels && opts.levels.size > 0 ? opts.levels : null;
  const all = opts.includeMode === "all";

  for (const line of lines) {
    if (opts.since !== undefined && line.time !== undefined && line.time < opts.since) continue;

    let includeHits = 0;
    for (const r of inc) {
      if (r.test(valueOf(line, r.target))) {
        counts[r.id]++;
        includeHits++;
      }
    }
    let excluded = false;
    for (const r of exc) {
      if (r.test(valueOf(line, r.target))) {
        counts[r.id]++;
        excluded = true;
      }
    }
    let colour: HighlightColour | undefined;
    for (const r of hil) {
      if (r.test(valueOf(line, r.target))) {
        counts[r.id]++;
        colour ??= r.colour;
      }
    }

    const included = inc.length === 0 || (all ? includeHits === inc.length : includeHits > 0);
    if (excluded || !included) continue;
    levelCounts[line.level]++;
    if (levels && !levels.has(line.level)) continue;
    visible.push(line);
    highlights.push(colour);
  }
  return { visible, highlights, counts, levelCounts };
}

/** Reason a rule cannot be applied (invalid regex), or null. */
export function ruleError(rule: Pick<Rule, "pattern" | "target">): string | null {
  return compileRule(rule).error;
}

let counter = 0;
export function newRuleId(): string {
  counter++;
  return `r${Date.now().toString(36)}${counter.toString(36)}${Math.random().toString(36).slice(2, 6)}`;
}
