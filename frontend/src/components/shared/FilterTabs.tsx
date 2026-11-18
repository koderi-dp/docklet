import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

export interface FilterTab<T extends string> {
  value: T;
  label: string;
  count: number;
  /** Optional status dot colour class, e.g. "bg-success". */
  dot?: string;
}

/** Segmented count filters shared by every resource toolbar (All 4 · Running 3 · Stopped 1). */
export function FilterTabs<T extends string>({
  tabs,
  value,
  onChange,
  label,
}: {
  tabs: FilterTab<T>[];
  value: T;
  onChange: (v: T) => void;
  label: string;
}) {
  return (
    <div className="flex shrink-0 items-center gap-1" role="group" aria-label={label}>
      {tabs.map((t) => {
        const active = value === t.value;
        return (
          <Button
            key={t.value}
            size="sm"
            variant={active ? "default" : "ghost"}
            aria-pressed={active}
            onClick={() => onChange(t.value)}
            className="h-8 gap-2"
          >
            {t.dot && <span aria-hidden className={cn("size-2 rounded-full", t.dot)} />}
            {t.label}
            <span
              className={cn(
                "rounded-md px-1.5 py-px text-xs tabular-nums",
                active ? "bg-primary-foreground/20" : "bg-muted text-muted-foreground",
              )}
            >
              {t.count}
            </span>
          </Button>
        );
      })}
    </div>
  );
}
