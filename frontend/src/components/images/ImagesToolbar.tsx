import { RefreshCw, Search, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { FilterTabs } from "@/components/shared/FilterTabs";
import { IconButton } from "@/components/shared/IconButton";
import { cn } from "@/lib/utils";
import type { ImageFilter } from "./imageFilters";

const FILTERS: { id: ImageFilter; label: string }[] = [
  { id: "all", label: "All" },
  { id: "inuse", label: "In use" },
  { id: "unused", label: "Unused" },
  { id: "dangling", label: "Dangling" },
];

interface Props {
  filter: ImageFilter;
  onFilter: (f: ImageFilter) => void;
  counts: Record<ImageFilter, number>;
  query: string;
  onQuery: (q: string) => void;
  danglingCount: number;
  pruning: boolean;
  onPrune: () => void;
  refreshing: boolean;
  onRefresh: () => void;
}

/** One-line toolbar content: filter tabs, flexible search, Remove dangling, refresh. */
export function ImagesToolbar(p: Props) {
  return (
    <>
      <FilterTabs
        label="Filter images"
        value={p.filter}
        onChange={p.onFilter}
        tabs={FILTERS.map((f) => ({ value: f.id, label: f.label, count: p.counts[f.id] }))}
      />
      <div className="relative min-w-24 max-w-72 flex-1">
        <Search aria-hidden className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          value={p.query}
          onChange={(e) => p.onQuery(e.target.value)}
          placeholder="Search repository, tag or ID"
          className="h-8 pl-8"
          aria-label="Search images"
        />
      </div>
      <div className="ml-auto flex shrink-0 items-center gap-1.5">
        {p.danglingCount > 0 && (
          <Button size="sm" variant="outline" disabled={p.pruning} onClick={p.onPrune} aria-label="Remove dangling images" title="Remove dangling images">
            <Trash2 className="size-3.5" />
            <span className="hidden @[900px]:inline">Remove dangling</span> ({p.danglingCount})
          </Button>
        )}
        <IconButton label="Refresh" onClick={p.onRefresh}>
          <RefreshCw className={cn("size-4", p.refreshing && "animate-spin")} />
        </IconButton>
      </div>
    </>
  );
}
