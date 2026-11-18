import { Copy, MoreHorizontal, Trash2 } from "lucide-react";
import type { Volume } from "@bindings/model";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { SortTh } from "@/components/tables/SortTh";
import { cn } from "@/lib/utils";
import { InUseBadge, formatCreatedIso } from "./parts";
import { volumeUsers } from "./useVolumesPage";
import { copyToClipboard } from "@/lib/clipboard";

interface Props {
  rows: Volume[];
  selected: ReadonlySet<string>;
  inspected: string | null;
  allVisibleSelected: boolean;
  onToggleAll: (on: boolean) => void;
  onToggle: (name: string) => void;
  onInspect: (name: string) => void;
  onRemove: (v: Volume) => void;
}

function Truncated({ text, mono }: { text: string; mono?: boolean }) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span className={cn("block truncate", mono && "font-mono text-xs text-muted-foreground")}>{text}</span>
      </TooltipTrigger>
      <TooltipContent className="max-w-lg break-all">{text}</TooltipContent>
    </Tooltip>
  );
}

export function VolumesTable(p: Props) {
  return (
    <table className="w-full table-fixed border-collapse text-[13px]">
      <colgroup>
        <col className="w-10" />
        <col className="w-[26%]" />
        <col className="w-[90px]" />
        <col />
        <col className="hidden w-[80px] @[900px]:table-column" />
        <col className="w-[110px]" />
        <col className="hidden w-[150px] @[1000px]:table-column" />
        <col className="w-12" />
      </colgroup>
      <thead>
        <tr className="border-b border-border text-left text-xs font-medium text-muted-foreground">
          <th className="h-10 px-3">
            <Checkbox aria-label="Select all visible volumes" checked={p.allVisibleSelected} onCheckedChange={(c) => p.onToggleAll(c === true)} />
          </th>
          <SortTh table="volumes" id="name">Name</SortTh>
          <SortTh table="volumes" id="driver">Driver</SortTh>
          <th className="px-3 font-medium">Mountpoint</th>
          <th className="hidden px-3 font-medium @[900px]:table-cell">Scope</th>
          <SortTh table="volumes" id="inuse">In use</SortTh>
          <SortTh table="volumes" id="created" className="hidden @[1000px]:table-cell">Created</SortTh>
          <th className="px-3"><span className="sr-only">Actions</span></th>
        </tr>
      </thead>
      <tbody>
        {p.rows.map((v) => {
          const sel = p.selected.has(v.name);
          const isInspected = p.inspected === v.name;
          return (
            <tr
              key={v.name}
              tabIndex={0}
              aria-selected={isInspected}
              onClick={() => p.onInspect(v.name)}
              onKeyDown={(e) => {
                if (e.target === e.currentTarget && (e.key === "Enter" || e.key === " ")) {
                  e.preventDefault();
                  p.onInspect(v.name);
                }
              }}
              className={cn(
                "h-[42px] cursor-pointer border-b border-border/60 outline-none transition-colors duration-100 hover:bg-muted/50 focus-visible:bg-muted/60 focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring/60",
                sel && "bg-primary/[0.07] hover:bg-primary/10",
                isInspected && "bg-accent hover:bg-accent [&>td:first-child]:shadow-[inset_2px_0_0_0_var(--primary)]",
              )}
            >
              <td className="px-3" onClick={(e) => e.stopPropagation()}>
                <Checkbox aria-label={`Select ${v.name}`} checked={sel} onCheckedChange={() => p.onToggle(v.name)} />
              </td>
              <td className="px-3 font-medium"><Truncated text={v.name} /></td>
              <td className="truncate px-3 text-muted-foreground">{v.driver || "—"}</td>
              <td className="px-3"><Truncated text={v.mountpoint || "—"} mono /></td>
              <td className="hidden truncate px-3 text-muted-foreground @[900px]:table-cell">{v.scope || "—"}</td>
              <td className="px-3"><InUseBadge n={volumeUsers(v)} /></td>
              <td className="hidden truncate px-3 text-muted-foreground @[1000px]:table-cell">{formatCreatedIso(v.created)}</td>
              <td className="px-3" onClick={(e) => e.stopPropagation()}>
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button size="icon" variant="ghost" className="size-7" aria-label={`Actions for ${v.name}`} title="Actions">
                      <MoreHorizontal className="size-4" />
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end">
                    <DropdownMenuItem onSelect={() => p.onInspect(v.name)}>Inspect</DropdownMenuItem>
                    <DropdownMenuItem onSelect={() => copyToClipboard(v.name, "Volume name")}><Copy className="size-4" /> Copy name</DropdownMenuItem>
                    <DropdownMenuSeparator />
                    <DropdownMenuItem variant="destructive" onSelect={() => p.onRemove(v)}>
                      <Trash2 className="size-4" /> Remove
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}
