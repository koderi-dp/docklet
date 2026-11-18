import { ChevronDown } from "lucide-react";
import { Checkbox } from "@/components/ui/checkbox";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { cn } from "@/lib/utils";
import { describePane, groupItems, type PaneSource, type TickedItem } from "@/lib/logPanes";
import { containerTone } from "../logColours";

interface Props {
  source: PaneSource;
  items: readonly TickedItem[];
  onAll: (on: boolean) => void;
  onNames: (names: readonly string[], on: boolean) => void;
  /** Draws attention while the pane has nothing to show. */
  prominent?: boolean;
}

/** The pane's "Showing" dropdown: which of the ticked containers this pane displays. */
export function PaneSourceMenu({ source, items, onAll, onNames, prominent }: Props) {
  const summary = describePane(source, items);
  const groups = groupItems(items);
  const shown = new Set(summary.names);
  return (
    <Popover>
      <PopoverTrigger asChild>
        <button
          type="button"
          aria-label={`Showing: ${summary.label}. Change what this pane shows`}
          title="Choose which ticked containers this pane shows"
          className={cn(
            "flex h-6 min-w-0 items-center gap-1.5 rounded-md border px-1.5 text-xs transition-colors focus-visible:ring-2 focus-visible:ring-ring/60 focus-visible:outline-none",
            prominent ? "border-primary/50 bg-primary/10 text-foreground" : "border-transparent hover:border-border hover:bg-muted/60",
          )}
        >
          <span className="shrink-0 text-muted-foreground">Showing</span>
          {summary.single && <span aria-hidden className={cn("size-2 shrink-0 rounded-full", containerTone(summary.single).dot)} />}
          <span className={cn("min-w-0 truncate font-medium", summary.single && containerTone(summary.single).text)}>{summary.label}</span>
          <ChevronDown className="size-3 shrink-0 text-muted-foreground" />
        </button>
      </PopoverTrigger>
      <PopoverContent align="start" className="w-72 gap-0 p-1.5">
        <label className="flex h-7 cursor-pointer items-center gap-2 rounded-md px-1.5 text-[13px] font-medium hover:bg-muted/60">
          <Checkbox checked={source.all} onCheckedChange={(v) => onAll(v === true)} aria-label="Show all ticked containers" />
          All ticked containers
          <span className="ml-auto text-xs font-normal text-muted-foreground tabular-nums">{items.length}</span>
        </label>
        <div className="my-1 border-t border-border" />
        <div className="max-h-64 overflow-y-auto">
          {groups.map((g) => {
            const names = g.items.map((i) => i.name);
            const n = names.filter((x) => source.all || shown.has(x)).length;
            const state = n === 0 ? false : n === names.length ? true : "indeterminate";
            return (
              <div key={g.project ?? "__standalone__"} className="mb-0.5">
                <label className="flex h-7 cursor-pointer items-center gap-2 rounded-md px-1.5 text-[13px] font-medium hover:bg-muted/60">
                  <Checkbox checked={state} onCheckedChange={() => onNames(names, n !== names.length)} aria-label={`Show all of ${g.project ?? "standalone containers"}`} />
                  <span className="truncate">{g.project ?? "Standalone"}</span>
                  <span className="ml-auto text-xs font-normal text-muted-foreground tabular-nums">
                    {n}/{names.length}
                  </span>
                </label>
                {g.items.map((it) => (
                  <label key={it.name} className="flex h-7 cursor-pointer items-center gap-2 rounded-md pr-1.5 pl-6 text-[13px] hover:bg-muted/60">
                    <Checkbox checked={source.all || shown.has(it.name)} onCheckedChange={(v) => onNames([it.name], v === true)} aria-label={`Show ${it.name}`} />
                    <span aria-hidden className={cn("size-2 shrink-0 rounded-full", containerTone(it.name).dot)} />
                    <span className="min-w-0 truncate" title={it.name}>
                      {it.name}
                    </span>
                  </label>
                ))}
              </div>
            );
          })}
        </div>
      </PopoverContent>
    </Popover>
  );
}
