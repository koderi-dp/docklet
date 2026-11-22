import { useState } from "react";
import { LOG_LEVELS } from "@/lib/logLevel";
import type { HighlightColour, RuleKind, RuleTarget } from "@/lib/logFilter/rules";

export const LEVELS = LOG_LEVELS.filter((l) => l !== "other");

export interface RuleDraftInit {
  kind: RuleKind;
  target: RuleTarget;
  pattern: string;
  colour?: HighlightColour;
}

export interface DraftValues {
  kind: RuleKind;
  target: RuleTarget;
  colour: HighlightColour;
  pattern: string;
  level: string;
}

/** The form's starting values: from the rule being edited, or the add-mode defaults. */
export function draftValues(initial?: RuleDraftInit): DraftValues {
  return {
    kind: initial?.kind ?? "include",
    target: initial?.target ?? "message",
    colour: initial?.colour ?? "warning",
    pattern: initial && initial.target !== "level" ? initial.pattern : "",
    level: initial?.target === "level" ? initial.pattern : "error",
  };
}

/** The Level select's items: the known levels, plus the current value when it is a pattern such as `warn*`. */
export function levelOptions(current: string): string[] {
  return LEVELS.includes(current as (typeof LEVELS)[number]) ? [...LEVELS] : [...LEVELS, current];
}

/** The form's state. A caller that owns it (+ Filter) keeps the draft across popover closes. */
export function useRuleDraft(initial?: RuleDraftInit) {
  const [start] = useState(() => draftValues(initial));
  const [kind, setKind] = useState(start.kind);
  const [target, setTarget] = useState(start.target);
  const [colour, setColour] = useState(start.colour);
  const [pattern, setPattern] = useState(start.pattern);
  const [level, setLevel] = useState(start.level);
  return { kind, setKind, target, setTarget, colour, setColour, pattern, setPattern, level, setLevel };
}

export type RuleDraftState = ReturnType<typeof useRuleDraft>;
