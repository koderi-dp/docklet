import { useRef, useState, type FormEvent } from "react";
import { Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { cn } from "@/lib/utils";
import { LOG_LEVELS } from "@/lib/logLevel";
import { HIGHLIGHT_COLOURS, ruleError, type HighlightColour, type Rule, type RuleKind, type RuleTarget } from "@/lib/logFilter/rules";
import { HIGHLIGHT_LABEL, HIGHLIGHT_SWATCH, LEVEL_LABEL } from "./logColours";
import { KIND_LABEL, TARGET_LABEL } from "./RuleChip";

const LEVELS = LOG_LEVELS.filter((l) => l !== "other");
const HINT: Record<RuleTarget, string> = {
  message: "text, GET /api/*, /timeout|refused/",
  container: "name, web-*, db-?",
  level: "",
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

interface Props {
  rules: readonly Rule[];
  addRule: (r: Omit<Rule, "id" | "enabled">) => void;
  /** Containers whose lines are in the view: offered as suggestions for the Container target. Empty: no such target. */
  containerNames?: readonly string[];
}

/** "+ Filter": builds one explicit filter rule (kind, what it applies to, pattern) and adds it as a chip. Stays open for the next. */
const KIND_HELP: Record<RuleKind, string> = {
  include: "Show only lines that match.",
  exclude: "Hide lines that match. Always wins over Include.",
  highlight: "Colour lines that match. Hides nothing.",
};

export function AddFilterPopover({ rules, addRule, containerNames = [] }: Props) {
  const [open, setOpen] = useState(false);
  const [kind, setKind] = useState<RuleKind>("include");
  const [target, setTarget] = useState<RuleTarget>("message");
  const [colour, setColour] = useState<HighlightColour>("warning");
  const [pattern, setPattern] = useState("");
  const [level, setLevel] = useState("error");
  const [note, setNote] = useState<string | null>(null);
  const input = useRef<HTMLInputElement>(null);

  const hasContainers = containerNames.length > 0;
  const targets: Partial<Record<RuleTarget, string>> = hasContainers ? TARGET_LABEL : { message: TARGET_LABEL.message, level: TARGET_LABEL.level };
  const effTarget: RuleTarget = target === "container" && !hasContainers ? "message" : target;
  const value = effTarget === "level" ? level : pattern;
  const trimmed = value.trim();
  const error = trimmed === "" ? null : ruleError({ pattern: value, target: effTarget });
  const duplicate = rules.some((r) => r.kind === kind && r.target === effTarget && r.pattern === value);
  const reason = trimmed === "" ? "Type a pattern first" : error ? `Invalid pattern: ${error}` : duplicate ? "This filter already exists" : null;

  const submit = (e: FormEvent) => {
    e.preventDefault();
    if (reason) return;
    addRule({ kind, target: effTarget, pattern: value, colour: kind === "highlight" ? colour : undefined });
    setNote(`Added: ${KIND_LABEL[kind]} ${TARGET_LABEL[effTarget].toLowerCase()} ${value}`);
    setPattern("");
    input.current?.focus();
  };

  return (
    <Popover
      open={open}
      onOpenChange={(o) => {
        setOpen(o);
        if (!o) setNote(null);
      }}
    >
      <PopoverTrigger asChild>
        <Button variant="outline" size="sm" className="shrink-0 gap-1.5" title="Add a filter that hides or highlights lines">
          <Plus className="size-3.5" /> Filter
        </Button>
      </PopoverTrigger>
      <PopoverContent
        align="end"
        className="w-[340px] p-3"
        onOpenAutoFocus={(e) => {
          e.preventDefault();
          input.current?.focus();
        }}
      >
        <form onSubmit={submit} className="space-y-3">
          <div className="space-y-1">
            <Segmented label="Kind" value={kind} options={KIND_LABEL} onChange={setKind} />
            <p className="text-xs text-muted-foreground">{KIND_HELP[kind]}</p>
          </div>
          <Segmented label="Applies to" value={effTarget} options={targets} onChange={setTarget} />
          <div className="space-y-1">
            <label htmlFor="add-filter-pattern" className="text-xs text-muted-foreground">
              {effTarget === "level" ? "Level" : "Pattern"}
            </label>
            {effTarget === "level" ? (
              <Select value={level} onValueChange={setLevel}>
                <SelectTrigger id="add-filter-pattern" size="sm" className="h-8 w-full text-[13px]">
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
                  id="add-filter-pattern"
                  ref={input}
                  value={pattern}
                  onChange={(e) => setPattern(e.target.value)}
                  list={effTarget === "container" ? "add-filter-containers" : undefined}
                  placeholder={effTarget === "container" ? "Container name or web-*" : "Pattern"}
                  aria-invalid={!!error}
                  autoComplete="off"
                  className={cn("h-8 text-[13px]", error && "border-destructive")}
                />
                {effTarget === "container" && (
                  <datalist id="add-filter-containers">
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
                Add filter
              </Button>
            </span>
          </div>
        </form>
      </PopoverContent>
    </Popover>
  );
}
