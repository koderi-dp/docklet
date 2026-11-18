import { useMemo } from "react";
import { columnVisibilityFeature, createColumnHelper, tableFeatures, useTable } from "@tanstack/react-table";
import { Copy, Download, Eye, MoreHorizontal, Play, Tag, Trash2, Upload } from "lucide-react";
import type { ImageRow } from "@bindings/model";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { formatBytes, formatCreated } from "@/components/containers/format";
import { cn } from "@/lib/utils";
import { copyToClipboard } from "@/lib/clipboard";

const features = tableFeatures({ columnVisibilityFeature });
const helper = createColumnHelper<typeof features, ImageRow>();

export function shortImageId(id: string): string {
  return id.replace(/^sha256:/, "").slice(0, 12);
}

export interface ImagesTableProps {
  rows: ImageRow[];
  selected: ReadonlySet<string>;
  inspectedKey: string | null;
  onToggleSelect: (key: string) => void;
  onToggleSelectAll: () => void;
  onInspect: (r: ImageRow) => void;
  onCreateContainer: (r: ImageRow) => void;
  onTag: (r: ImageRow) => void;
  onRemove: (r: ImageRow) => void;
  onPush: (r: ImageRow) => void;
  onSave: (r: ImageRow) => void;
}

export function RowActions({
  row,
  onInspect,
  onCreateContainer,
  onTag,
  onRemove,
  onPush,
  onSave,
}: { row: ImageRow } & Pick<ImagesTableProps, "onInspect" | "onCreateContainer" | "onTag" | "onRemove" | "onPush" | "onSave">) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" className="size-7" aria-label={`Actions for ${row.ref}`} title={`Actions for ${row.ref}`} onClick={(e) => e.stopPropagation()}>
          <MoreHorizontal className="size-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" onClick={(e) => e.stopPropagation()}>
        <DropdownMenuItem onSelect={() => onInspect(row)}>
          <Eye /> Inspect
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={() => onCreateContainer(row)}>
          <Play /> Create container
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={() => onTag(row)}>
          <Tag /> Tag…
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={() => copyToClipboard(row.id, "Image ID")}>
          <Copy /> Copy ID
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={() => onSave(row)}>
          <Download /> Save…
        </DropdownMenuItem>
        <DropdownMenuItem
          disabled={row.dangling}
          title={row.dangling ? "Untagged image: tag it with a registry name first" : undefined}
          onSelect={() => onPush(row)}
        >
          <Upload /> Push…
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem variant="destructive" onSelect={() => onRemove(row)}>
          <Trash2 /> Remove
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

export function InUseBadge({ row }: { row: ImageRow }) {
  if (!row.inUse) return <span className="text-muted-foreground">Unused</span>;
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Badge variant="secondary" className="bg-success/15 font-normal text-success">
          In use
        </Badge>
      </TooltipTrigger>
      <TooltipContent>
        Used by {row.containerCount} container{row.containerCount === 1 ? "" : "s"} (running or stopped)
      </TooltipContent>
    </Tooltip>
  );
}

export function ImagesTable(p: ImagesTableProps) {
  const allSelected = p.rows.length > 0 && p.rows.every((r) => p.selected.has(r.key));
  const someSelected = p.rows.some((r) => p.selected.has(r.key));

  const columns = useMemo(
    () =>
      helper.columns([
        helper.display({
          id: "select",
          header: () => (
            <Checkbox
              checked={allSelected ? true : someSelected ? "indeterminate" : false}
              onCheckedChange={() => p.onToggleSelectAll()}
              aria-label="Select all visible images"
            />
          ),
          cell: (ctx) => {
            const r = ctx.row.original;
            return (
              <span onClick={(e) => e.stopPropagation()}>
                <Checkbox checked={p.selected.has(r.key)} onCheckedChange={() => p.onToggleSelect(r.key)} aria-label={`Select ${r.ref}`} />
              </span>
            );
          },
        }),
        helper.display({
          id: "name",
          header: "Repository : Tag",
          cell: (ctx) => {
            const r = ctx.row.original;
            if (r.dangling) {
              return (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <span className="text-muted-foreground">{r.repository || "<none>"}:&lt;none&gt;</span>
                  </TooltipTrigger>
                  <TooltipContent>Dangling: this image has no tag</TooltipContent>
                </Tooltip>
              );
            }
            return (
              <span className="block max-w-[420px] truncate font-medium" title={r.ref}>
                {r.repository}
                <span className="font-normal text-muted-foreground">:{r.tag}</span>
              </span>
            );
          },
        }),
        helper.display({
          id: "id",
          header: "Image ID",
          cell: (ctx) => {
            const r = ctx.row.original;
            return (
              <span className="inline-flex items-center gap-1 font-mono text-xs">
                {shortImageId(r.id)}
                <button
                  aria-label={`Copy image ID ${shortImageId(r.id)}`}
                  className="text-muted-foreground hover:text-foreground"
                  onClick={(e) => {
                    e.stopPropagation();
                    copyToClipboard(r.id, "Image ID");
                  }}
                >
                  <Copy className="size-3.5" />
                </button>
              </span>
            );
          },
        }),
        helper.display({
          id: "created",
          header: "Created",
          cell: (ctx) => <span className="whitespace-nowrap">{formatCreated(ctx.row.original.created)}</span>,
        }),
        helper.display({
          id: "size",
          header: "Size",
          cell: (ctx) => <span className="whitespace-nowrap tabular-nums">{formatBytes(ctx.row.original.size)}</span>,
        }),
        helper.display({
          id: "inuse",
          header: "Usage",
          cell: (ctx) => <InUseBadge row={ctx.row.original} />,
        }),
        helper.display({
          id: "actions",
          header: () => <span className="sr-only">Actions</span>,
          cell: (ctx) => (
            <div className="flex justify-end">
              <RowActions
                row={ctx.row.original}
                onInspect={p.onInspect}
                onCreateContainer={p.onCreateContainer}
                onTag={p.onTag}
                onRemove={p.onRemove}
                onPush={p.onPush}
                onSave={p.onSave}
              />
            </div>
          ),
        }),
      ]),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [p.selected, allSelected, someSelected, p.onToggleSelect, p.onToggleSelectAll, p.onInspect, p.onCreateContainer, p.onTag, p.onRemove, p.onPush, p.onSave],
  );

  const table = useTable({ features, columns, data: p.rows, getRowId: (r) => r.key });

  return (
    <div className="overflow-x-auto">
      <table className="w-full min-w-[640px] border-collapse text-[13px]">
        <thead>
          {table.getHeaderGroups().map((hg) => (
            <tr key={hg.id} className="border-b border-border text-left text-xs font-medium text-muted-foreground">
              {hg.headers.map((h) => (
                <th key={h.id} scope="col" className={cn("h-10 whitespace-nowrap px-3 font-medium", h.column.id === "select" && "w-10")}>
                  {h.isPlaceholder ? null : <table.FlexRender header={h} />}
                </th>
              ))}
            </tr>
          ))}
        </thead>
        <tbody>
          {table.getRowModel().rows.map((row) => {
            const r = row.original;
            const checked = p.selected.has(r.key);
            return (
              <tr
                key={row.id}
                tabIndex={0}
                aria-selected={p.inspectedKey === r.key}
                onClick={() => p.onInspect(r)}
                onKeyDown={(e) => {
                  if (e.target === e.currentTarget && (e.key === "Enter" || e.key === " ")) {
                    e.preventDefault();
                    p.onInspect(r);
                  }
                }}
                className={cn(
                  "h-[42px] cursor-pointer border-b border-border/60 outline-none transition-colors hover:bg-muted/50 focus-visible:bg-muted/60 focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring/60",
                  checked && "bg-primary/[0.07] hover:bg-primary/10",
                  p.inspectedKey === r.key && "bg-accent hover:bg-accent [&>td:first-child]:shadow-[inset_2px_0_0_0_var(--primary)]",
                )}
              >
                {row.getVisibleCells().map((cell) => (
                  <td key={cell.id} className="px-3">
                    <table.FlexRender cell={cell} />
                  </td>
                ))}
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
