import { Layers, List, Search } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { FilterTabs } from "@/components/shared/FilterTabs";
import { FilterGroup, FilterPopover } from "@/components/shared/FilterPopover";
import { type ContainerFilters, type HealthFilter, type StatusTab } from "@/lib/composeGrouping";
import { STATE_FILTER_OPTIONS } from "./format";

interface Props {
  filters: ContainerFilters;
  onChange: (f: ContainerFilters) => void;
  counts: { all: number; running: number; stopped: number };
  view: "grouped" | "flat";
  onViewChange: (v: "grouped" | "flat") => void;
}

const HEALTH_OPTIONS: { value: HealthFilter; label: string }[] = [
  { value: "all", label: "Any" },
  { value: "healthy", label: "Healthy" },
  { value: "unhealthy", label: "Unhealthy" },
  { value: "starting", label: "Starting" },
  { value: "none", label: "No healthcheck" },
];

const STATUS_OPTIONS = [{ value: "all", label: "Any" }, ...STATE_FILTER_OPTIONS.map((s) => ({ value: s as string, label: s }))];

/** Content of the toolbar row (render inside <TableToolbar/>): one line at every width. */
export function ContainersToolbar({ filters, onChange, counts, view, onViewChange }: Props) {
  const secondaryCount = (filters.status !== "all" ? 1 : 0) + (filters.health !== "all" ? 1 : 0);

  const viewBtn = (mode: "grouped" | "flat", label: string, icon: React.ReactNode) => (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          size="sm"
          variant={view === mode ? "default" : "ghost"}
          aria-pressed={view === mode}
          aria-label={label}
          className="h-7 gap-1.5 px-2"
          onClick={() => onViewChange(mode)}
        >
          {icon}
          <span className="hidden @[860px]:inline">{label}</span>
        </Button>
      </TooltipTrigger>
      <TooltipContent>{label}</TooltipContent>
    </Tooltip>
  );

  return (
    <>
      <FilterTabs
        label="Filter by state"
        value={filters.tab}
        onChange={(tab: StatusTab) => onChange({ ...filters, tab })}
        tabs={[
          { value: "all", label: "All", count: counts.all },
          { value: "running", label: "Running", count: counts.running, dot: "bg-success" },
          { value: "stopped", label: "Stopped", count: counts.stopped, dot: "bg-muted-foreground/60" },
        ]}
      />

      <div className="relative min-w-24 max-w-72 flex-1">
        <Search aria-hidden className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          value={filters.search}
          onChange={(e) => onChange({ ...filters, search: e.target.value })}
          placeholder="Search containers…"
          aria-label="Search containers by name, ID, image, service or project"
          className="h-8 pl-8 text-sm"
        />
      </div>

      <div className="ml-auto flex shrink-0 items-center gap-2">
        <div className="flex items-center rounded-md border border-border p-0.5" role="group" aria-label="View mode">
          {viewBtn("flat", "Flat", <List className="size-3.5" />)}
          {viewBtn("grouped", "Compose Groups", <Layers className="size-3.5" />)}
        </div>

        <FilterPopover activeCount={secondaryCount} onClear={() => onChange({ ...filters, status: "all", health: "all" })}>
          <FilterGroup label="Status" value={filters.status} options={STATUS_OPTIONS} onChange={(status) => onChange({ ...filters, status })} />
          <FilterGroup label="Health" value={filters.health} options={HEALTH_OPTIONS} onChange={(health) => onChange({ ...filters, health })} />
        </FilterPopover>
      </div>
    </>
  );
}
