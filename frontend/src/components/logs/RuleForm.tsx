import { useId, useRef, useState, type FormEvent, type RefObject } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { cn } from "@/lib/utils";
import { LOG_LEVELS } from "@/lib/logLevel";
import { HIGHLIGHT_COLOURS, ruleError, type HighlightColour, type Rule, type RuleKind, type RuleTarget } from "@/lib/logFilter/rules";
import { HIGHLIGHT_LABEL, HIGHLIGHT_SWATCH, LEVEL_LABEL } from "./logColours";
import { KIND_LABEL, TARGET_LABEL } from "./ruleLabels";

const LEVELS = LOG_LEVELS.filter((l) => l !== "other");
const HINT: Record<RuleTarget, string> = {
  message: "text, GET /api/*, /timeout|refused/",
  container: "name, web-*, db-?",
  level: "",
};
const KIND_HELP: Record<RuleKind, string> = {
  include: "Show only lines that match.",
  exclude: "Hide lines that match. Always wins over Include.",
  highlight: "Colour lines that match. Hides nothing.",
};

function Segmented<T extends string>({ label, value, options, onChange }: { label: string; value: T; options: Partial<Record<T, string>>; onChange: (v: T) => void }) {
  return (
    <div className="space-y-1">
      <div className="text-xs text-muted-foreground">{label}</div>
      <div role="radiogroup" aria-label={label} className="flex overflow-hidden rounded-md border border-border">
        {(Object.keys(options) as T[]).map((k) => (
          <button
            key={k}
            type="button"
            role="radio"
            aria-checked={value === k}
            onClick={() => onChange(k)}
            className={cn(
              "h-7 flex-1 border-r border-border text-xs last:border-r-0 focus-visible:ring-2 focus-visible:ring-ring/60 focus-visible:outline-none focus-visible:ring-inset",
              value === k ? "bg-primary/10 font-medium text-foreground" : "text-muted-foreground hover:bg-muted/60",
            )}
          >
            {options[k]}
          </button>
        ))}
      </div>
    </div>
  );
}

export interface RuleDraft {
  kind: RuleKind;
  target: RuleTarget;
  pattern: string;
  colour?: HighlightColour;
}

interface Props {
  initial?: RuleDraft;
  rules: readonly Rule[];
  /** The rule being edited: it does not count as its own duplicate. */
  ignoreId?: string;
  /** Containers whose lines are in the view: offered as suggestions for the Container target. */
  containerNames?: readonly string[];
  submitLabel: string;
  onSubmit: (r: RuleDraft) => void;
  /** Add mode: after submit the pattern is cleared and the input refocused. */
  keepOpen?: boolean;
  note?: string | null;
  selectOnFocus?: boolean;
  inputRef?: RefObject<HTMLInputElement | null>;
}

/** The kind / applies-to / pattern / colour form shared by "+ Filter" and the chip's edit popover. */
export function RuleForm({ initial, rules, ignoreId, containerNames = [], submitLabel, onSubmit, keepOpen, note, selectOnFocus, inputRef }: Props) {
  const [kind, setKind] = useState<RuleKind>(initial?.kind ?? "include");
  const [target, setTarget] = useState<RuleTarget>(initial?.target ?? "message");
  const [colour, setColour] = useState<HighlightColour>(initial?.colour ?? "warning");
  const [pattern, setPattern] = useState(initial && initial.target !== "level" ? initial.pattern : "");
  const [level, setLevel] = useState(initial?.target === "level" ? initial.pattern : "error");
  const ownRef = useRef<HTMLInputElement>(null);
  const input = inputRef ?? ownRef;
  const id = useId();
  const patternId = `${id}-pattern`;
  const listId = `${id}-containers`;

  const hasContainers = containerNames.length > 0 || initial?.target === "container";
  const targets: Partial<Record<RuleTarget, string>> = hasContainers ? TARGET_LABEL : { message: TARGET_LABEL.message, level: TARGET_LABEL.level };
  const effTarget: RuleTarget = target === "container" && !hasContainers ? "message" : target;
  const value = effTarget === "level" ? level : pattern;
  const trimmed = value.trim();
  const error = trimmed === "" ? null : ruleError({ pattern: value, target: effTarget });
  const duplicate = rules.some((r) => r.id !== ignoreId && r.kind === kind && r.target === effTarget && r.pattern === value);
  const reason = trimmed === "" ? "Type a pattern first" : error ? `Invalid pattern: ${error}` : duplicate ? "This filter already exists" : null;

  const submit = (e: FormEvent) => {
    e.preventDefault();
    if (reason) return;
    onSubmit({ kind, target: effTarget, pattern: value, colour: kind === "highlight" ? colour : undefined });
    if (keepOpen) {
      setPattern("");
      input.current?.focus();
    }
  };

  return (
    <form onSubmit={submit} className="space-y-3">
      <div className="space-y-1">
        <Segmented label="Kind" value={kind} options={KIND_LABEL} onChange={setKind} />
        <p className="text-xs text-muted-foreground">{KIND_HELP[kind]}</p>
      </div>
      <Segmented label="Applies to" value={effTarget} options={targets} onChange={setTarget} />
      <div className="space-y-1">
        <label htmlFor={patternId} className="text-xs text-muted-foreground">
          {effTarget === "level" ? "Level" : "Pattern"}
        </label>
        {effTarget === "level" ? (
          <Select value={level} onValueChange={setLevel}>
            <SelectTrigger id={patternId} size="sm" className="h-8 w-full text-[13px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {LEVELS.map((l) => (
                <SelectItem key={l} value={l}>
                  {LEVEL_LABEL[l]}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        ) : (
          <>
            <Input
              id={patternId}
              ref={input}
              value={pattern}
              onChange={(e) => setPattern(e.target.value)}
              onFocus={selectOnFocus ? (e) => e.currentTarget.select() : undefined}
              list={effTarget === "container" ? listId : undefined}
              placeholder={effTarget === "container" ? "Container name or web-*" : "Pattern"}
              aria-invalid={!!error}
              autoComplete="off"
              className={cn("h-8 text-[13px]", error && "border-destructive")}
            />
            {effTarget === "container" && (
              <datalist id={listId}>
                {containerNames.map((n) => (
                  <option key={n} value={n} />
                ))}
              </datalist>
            )}
            <p className="text-[11px] text-muted-foreground">{HINT[effTarget]}</p>
          </>
        )}
      </div>
      {kind === "highlight" && (
        <div className="space-y-1">
          <div className="text-xs text-muted-foreground">Colour</div>
          <div role="radiogroup" aria-label="Highlight colour" className="flex gap-1.5">
            {HIGHLIGHT_COLOURS.map((c) => (
              <button
                key={c}
                type="button"
                role="radio"
                aria-checked={colour === c}
                aria-label={HIGHLIGHT_LABEL[c]}
                title={HIGHLIGHT_LABEL[c]}
                onClick={() => setColour(c)}
                className={cn("size-5 rounded-full ring-offset-2 ring-offset-popover focus-visible:ring-2 focus-visible:ring-ring/60 focus-visible:outline-none", HIGHLIGHT_SWATCH[c], colour === c && "ring-2 ring-foreground")}
              />
            ))}
          </div>
        </div>
      )}
      <div className="flex items-center justify-between gap-2">
        <p className={cn("min-w-0 truncate text-[11px]", error ? "text-destructive" : "text-muted-foreground")} role={error ? "alert" : "status"} title={note ?? undefined}>
          {error ? `Invalid pattern: ${error}` : (note ?? "")}
        </p>
        <span title={reason ?? undefined} className="shrink-0">
          <Button type="submit" size="sm" disabled={!!reason}>
            {submitLabel}
          </Button>
        </span>
      </div>
    </form>
  );
}
