import { Copy, MoreHorizontal, Trash2 } from "lucide-react";
import type { Network } from "@bindings/model";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { SortTh } from "@/components/tables/SortTh";
import { cn } from "@/lib/utils";
import { InUseBadge } from "@/components/volumes/parts";
import { networkAttached } from "./useNetworksPage";
import { copyToClipboard } from "@/lib/clipboard";

interface Props {
  rows: Network[];
  selected: ReadonlySet<string>;
  inspected: string | null;
  allVisibleSelected: boolean;
  onToggleAll: (on: boolean) => void;
  onToggle: (id: string) => void;
  onInspect: (id: string) => void;
  onRemove: (n: Network) => void;
}

const joined = (xs?: string[] | null) => (xs && xs.length > 0 ? xs.join(", ") : "—");

function Truncated({ text, mono, className }: { text: string; mono?: boolean; className?: string }) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span className={cn("block truncate", mono && "font-mono text-xs", className)}>{text}</span>
      </TooltipTrigger>
      <TooltipContent className="max-w-lg break-all">{text}</TooltipContent>
    </Tooltip>
  );
}

export function NetworksTable(p: Props) {
  return (
    <table className="w-full table-fixed border-collapse text-[13px]">
      <colgroup>
        <col className="w-10" />
        <col className="w-[26%]" />
        <col className="w-[90px]" />
        <col className="hidden w-[80px] @[900px]:table-column" />
        <col />
        <col className="hidden w-[150px] @[1000px]:table-column" />
        <col className="w-[110px]" />
        <col className="w-12" />
      </colgroup>
      <thead>
        <tr className="border-b border-border text-left text-xs font-medium text-muted-foreground">
          <th className="h-10 px-3">
            <Checkbox aria-label="Select all visible networks" checked={p.allVisibleSelected} onCheckedChange={(c) => p.onToggleAll(c === true)} />
          </th>
          <SortTh table="networks" id="name">Name</SortTh>
          <SortTh table="networks" id="driver">Driver</SortTh>
          <th className="hidden px-3 font-medium @[900px]:table-cell">Scope</th>
          <th className="px-3 font-medium">Subnet</th>
          <th className="hidden px-3 font-medium @[1000px]:table-cell">Gateway</th>
          <SortTh table="networks" id="attached">Attached</SortTh>
          <th className="px-3"><span className="sr-only">Actions</span></th>
        </tr>
      </thead>
      <tbody>
        {p.rows.map((n) => {
          const sel = p.selected.has(n.id);
          const isInspected = p.inspected === n.id;
          return (
            <tr
              key={n.id}
              tabIndex={0}
              aria-selected={isInspected}
              onClick={() => p.onInspect(n.id)}
              onKeyDown={(e) => {
                if (e.target === e.currentTarget && (e.key === "Enter" || e.key === " ")) {
                  e.preventDefault();
                  p.onInspect(n.id);
                }
              }}
              className={cn(
                "h-[42px] cursor-pointer border-b border-border/60 outline-none transition-colors duration-100 hover:bg-muted/50 focus-visible:bg-muted/60 focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring/60",
                sel && "bg-primary/[0.07] hover:bg-primary/10",
                isInspected && "bg-accent hover:bg-accent [&>td:first-child]:shadow-[inset_2px_0_0_0_var(--primary)]",
              )}
            >
              <td className="px-3" onClick={(e) => e.stopPropagation()}>
                <Checkbox aria-label={`Select ${n.name}`} checked={sel} onCheckedChange={() => p.onToggle(n.id)} />
              </td>
              <td className="px-3">
                <span className="flex min-w-0 items-center gap-2">
                  <Truncated text={n.name} className="font-medium" />
                  {n.predefined && <span className="shrink-0 rounded-md bg-muted px-1.5 py-px text-[11px] text-muted-foreground">predefined</span>}
                </span>
              </td>
              <td className="truncate px-3 text-muted-foreground">{n.driver || "—"}</td>
              <td className="hidden truncate px-3 text-muted-foreground @[900px]:table-cell">{n.scope || "—"}</td>
              <td className="px-3"><Truncated text={joined(n.subnets)} mono /></td>
              <td className="hidden px-3 @[1000px]:table-cell"><Truncated text={joined(n.gateways)} mono /></td>
              <td className="px-3"><InUseBadge n={networkAttached(n)} /></td>
              <td className="px-3" onClick={(e) => e.stopPropagation()}>
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button size="icon" variant="ghost" className="size-7" aria-label={`Actions for ${n.name}`} title="Actions">
                      <MoreHorizontal className="size-4" />
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end">
                    <DropdownMenuItem onSelect={() => p.onInspect(n.id)}>Inspect</DropdownMenuItem>
                    <DropdownMenuItem onSelect={() => copyToClipboard(n.name, "Network name")}><Copy className="size-4" /> Copy name</DropdownMenuItem>
                    <DropdownMenuSeparator />
                    <DropdownMenuItem variant="destructive" disabled={n.predefined} onSelect={() => p.onRemove(n)}>
                      <Trash2 className="size-4" /> {n.predefined ? "Remove (predefined)" : "Remove"}
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
