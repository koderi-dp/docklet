import { Minus, Plus, Search } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

interface Props {
  find: string;
  onFind: (v: string) => void;
  count: number;
  scale: number;
  onFit: () => void;
  onZoomIn: () => void;
  onZoomOut: () => void;
  selection: { name: string; needs: number; neededBy: number; containerId: string; hasContainers: boolean } | null;
  onClear: () => void;
  onOpenContainer: () => void;
  onShowInServices: () => void;
  onShowInFile?: () => void;
}

export function GraphToolbar({
  find, onFind, count, scale, onFit, onZoomIn, onZoomOut, selection, onClear, onOpenContainer, onShowInServices, onShowInFile,
}: Props) {
  return (
    <div className="flex items-center gap-3 border-b border-border px-3 py-2">
      <div className="relative w-56">
        <Search aria-hidden className="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
        <Input value={find} onChange={(e) => onFind(e.target.value)} placeholder="Find a service" aria-label="Find a service" className="h-8 pl-8 text-[13px]" />
      </div>
      {selection ? (
        <div className="flex min-w-0 items-center gap-2">
          <span className="min-w-0 truncate text-xs">
            <span className="font-semibold">{selection.name}</span>
            <span className="text-muted-foreground">{` · needs ${selection.needs} · ${selection.neededBy} needs it`}</span>
          </span>
          {selection.containerId !== "" && (
            <Button variant="outline" size="sm" className="h-7 px-2 text-xs" onClick={onOpenContainer}>
              Open container
            </Button>
          )}
          {selection.containerId === "" && selection.hasContainers && (
            <Button variant="outline" size="sm" className="h-7 px-2 text-xs" onClick={onShowInServices}>
              Show in Services
            </Button>
          )}
          {onShowInFile && (
            <Button variant="outline" size="sm" className="h-7 px-2 text-xs" onClick={onShowInFile}>
              Show in file
            </Button>
          )}
          <Button variant="ghost" size="sm" className="h-7 px-2 text-xs" onClick={onClear}>
            Clear
          </Button>
        </div>
      ) : (
        <span className="text-xs text-muted-foreground">
          {count} {count === 1 ? "service" : "services"}
        </span>
      )}
      <div className="ml-auto flex items-center overflow-hidden rounded-md border border-border">
        <Button variant="ghost" size="icon" className="size-8 rounded-none" aria-label="Zoom out" onClick={onZoomOut}>
          <Minus className="size-3.5" />
        </Button>
        <Button variant="ghost" size="sm" className="h-8 rounded-none border-x border-border px-3 text-xs" onClick={onFit}>
          Fit
        </Button>
        <span className="w-12 text-center text-xs tabular-nums text-muted-foreground" aria-label="Zoom level">
          {Math.round(scale * 100)}%
        </span>
        <Button variant="ghost" size="icon" className="size-8 rounded-none border-l border-border" aria-label="Zoom in" onClick={onZoomIn}>
          <Plus className="size-3.5" />
        </Button>
      </div>
    </div>
  );
}
