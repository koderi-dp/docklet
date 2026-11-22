import { ChevronDown, EyeOff, Filter, X } from "lucide-react";
import { cn } from "@/lib/utils";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { HIGHLIGHT_COLOURS, ruleError, type HighlightColour, type Rule, type RuleKind, type RuleTarget } from "@/lib/logFilter/rules";
import { HIGHLIGHT_LABEL, HIGHLIGHT_SWATCH } from "./logColours";
import { KIND_LABEL, TARGET_LABEL } from "./ruleLabels";

const KIND_STYLE: Record<RuleKind, string> = {
  include: "border-primary/40 bg-primary/10",
  exclude: "border-destructive/40 bg-destructive/10",
  highlight: "border-border bg-muted",
};

function KindMark({ rule }: { rule: Rule }) {
  if (rule.kind === "include") return <Filter className="size-3.5 shrink-0 text-primary" />;
  if (rule.kind === "exclude") return <EyeOff className="size-3.5 shrink-0 text-destructive" />;
  return <span aria-hidden className={cn("size-2.5 shrink-0 rounded-full", HIGHLIGHT_SWATCH[rule.colour ?? "warning"])} />;
}

interface Props {
  rule: Rule;
  /** Lines this rule matches; undefined while the rule is off or invalid. */
  count?: number;
  onChange: (patch: Partial<Omit<Rule, "id">>) => void;
  onRemove: () => void;
}

/** One filter rule: click toggles it, the arrow edits kind / target / colour, x removes it. */
export function RuleChip({ rule, count, onChange, onRemove }: Props) {
  const error = ruleError(rule);
  const label = `${KIND_LABEL[rule.kind]} ${TARGET_LABEL[rule.target].toLowerCase()} ${rule.pattern}`;
  return (
    <div
      className={cn(
        "inline-flex h-7 max-w-full items-center rounded-md border text-xs",
        error ? "border-destructive bg-destructive/10 text-destructive" : KIND_STYLE[rule.kind],
        !rule.enabled && "border-dashed bg-transparent text-muted-foreground opacity-70",
      )}
    >
      <button
        type="button"
        aria-pressed={rule.enabled}
        title={error ? `Invalid pattern: ${error}` : `${label} (click to turn ${rule.enabled ? "off" : "on"})`}
        onClick={() => onChange({ enabled: !rule.enabled })}
        className="flex h-full min-w-0 items-center gap-1.5 rounded-l-md pr-1 pl-2 hover:bg-foreground/5 focus-visible:ring-2 focus-visible:ring-ring/60 focus-visible:outline-none"
      >
        <KindMark rule={rule} />
        {rule.target !== "message" && <span className="shrink-0 text-muted-foreground">{TARGET_LABEL[rule.target].toLowerCase()}:</span>}
        <span className="max-w-[240px] truncate font-mono">{rule.pattern}</span>
        {count !== undefined && <span className="shrink-0 text-muted-foreground tabular-nums">{count.toLocaleString()}</span>}
      </button>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <button type="button" aria-label={`Edit rule ${rule.pattern}`} className="flex h-full w-5 shrink-0 items-center justify-center hover:bg-foreground/5 focus-visible:ring-2 focus-visible:ring-ring/60 focus-visible:outline-none">
            <ChevronDown className="size-3" />
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="w-44">
          <DropdownMenuLabel className="text-xs text-muted-foreground">Kind</DropdownMenuLabel>
          <DropdownMenuRadioGroup value={rule.kind} onValueChange={(v) => onChange({ kind: v as RuleKind })}>
            {(Object.keys(KIND_LABEL) as RuleKind[]).map((k) => (
              <DropdownMenuRadioItem key={k} value={k}>
                {KIND_LABEL[k]}
              </DropdownMenuRadioItem>
            ))}
          </DropdownMenuRadioGroup>
          <DropdownMenuSeparator />
          <DropdownMenuLabel className="text-xs text-muted-foreground">Applies to</DropdownMenuLabel>
          <DropdownMenuRadioGroup value={rule.target} onValueChange={(v) => onChange({ target: v as RuleTarget })}>
            {(Object.keys(TARGET_LABEL) as RuleTarget[]).map((t) => (
              <DropdownMenuRadioItem key={t} value={t}>
                {TARGET_LABEL[t]}
              </DropdownMenuRadioItem>
            ))}
          </DropdownMenuRadioGroup>
          {rule.kind === "highlight" && (
            <>
              <DropdownMenuSeparator />
              <DropdownMenuLabel className="text-xs text-muted-foreground">Colour</DropdownMenuLabel>
              <DropdownMenuRadioGroup value={rule.colour ?? "warning"} onValueChange={(v) => onChange({ colour: v as HighlightColour })}>
                {HIGHLIGHT_COLOURS.map((c) => (
                  <DropdownMenuRadioItem key={c} value={c}>
                    <span aria-hidden className={cn("size-2.5 rounded-full", HIGHLIGHT_SWATCH[c])} />
                    {HIGHLIGHT_LABEL[c]}
                  </DropdownMenuRadioItem>
                ))}
              </DropdownMenuRadioGroup>
            </>
          )}
        </DropdownMenuContent>
      </DropdownMenu>
      <button
        type="button"
        aria-label={`Remove rule ${rule.pattern}`}
        title="Remove rule"
        onClick={onRemove}
        className="flex h-full w-6 shrink-0 items-center justify-center rounded-r-md hover:bg-foreground/5 focus-visible:ring-2 focus-visible:ring-ring/60 focus-visible:outline-none"
      >
        <X className="size-3" />
      </button>
    </div>
  );
}
