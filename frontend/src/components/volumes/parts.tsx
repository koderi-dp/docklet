import type { ReactNode } from "react";
import { Link } from "react-router-dom";
import { Copy, Search, X } from "lucide-react";
import type { ResourceUser } from "@bindings/model";
import { IconButton } from "@/components/shared/IconButton";
import { InspectorActions } from "@/components/shared/InspectorActions";
import { Input } from "@/components/ui/input";
import { StatusBadge } from "@/components/shared/StatusBadge";
import { useUI } from "@/stores/uiStore";
import { FilterTabs } from "@/components/shared/FilterTabs";
import { formatDateTime } from "@/components/containers/format";
import { copyToClipboard } from "@/lib/clipboard";

/** Shared building blocks of the Volumes and Networks pages. */

export type UsageFilter = "all" | "inuse" | "unused";

/** RFC3339 string from the Engine -> local text; "—" when the Engine did not provide one. */
export function formatCreatedIso(iso?: string): string {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return formatDateTime(d);
}

export function UsageToolbar({
  search,
  onSearch,
  filter,
  onFilter,
  counts,
  placeholder,
}: {
  search: string;
  onSearch: (s: string) => void;
  filter: UsageFilter;
  onFilter: (f: UsageFilter) => void;
  counts: { all: number; inuse: number; unused: number };
  placeholder: string;
}) {
  return (
    <>
      <FilterTabs
        label="Filter by usage"
        value={filter}
        onChange={onFilter}
        tabs={[
          { value: "all", label: "All", count: counts.all },
          { value: "inuse", label: "In use", count: counts.inuse },
          { value: "unused", label: "Unused", count: counts.unused },
        ]}
      />
      <div className="relative min-w-24 max-w-72 flex-1">
        <Search aria-hidden className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
        <Input value={search} onChange={(e) => onSearch(e.target.value)} placeholder={placeholder} aria-label={placeholder} className="h-8 pl-8" />
      </div>
    </>
  );
}

export function InUseBadge({ n }: { n: number }) {
  return n > 0 ? (
    <span className="inline-flex items-center whitespace-nowrap rounded-md border border-success/30 bg-success/10 px-2 py-0.5 text-xs font-medium text-success">
      In use · {n}
    </span>
  ) : (
    <span className="inline-flex items-center whitespace-nowrap rounded-md border border-border bg-muted px-2 py-0.5 text-xs font-medium text-muted-foreground">Unused</span>
  );
}

export function Row({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="grid grid-cols-[96px_1fr] gap-3 border-b border-border/50 py-1.5 text-[13px]">
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="min-w-0 [overflow-wrap:anywhere]">{children}</dd>
    </div>
  );
}

export function KeyValues({ data, empty }: { data?: Record<string, string | undefined> | null; empty: string }) {
  const entries = Object.entries(data ?? {});
  if (entries.length === 0) return <p className="text-[13px] text-muted-foreground">{empty}</p>;
  return (
    <ul className="space-y-1 font-mono text-xs">
      {entries.map(([k, v]) => (
        <li key={k} className="break-all">
          <span className="text-muted-foreground">{k}</span>={v}
        </li>
      ))}
    </ul>
  );
}

/** Containers linked to the resource; each links to its detail page. */
export function UserList({ users, empty, extra }: { users: ResourceUser[]; empty: string; extra?: (u: ResourceUser) => ReactNode }) {
  if (users.length === 0) return <p className="text-[13px] text-muted-foreground">{empty}</p>;
  return (
    <ul className="space-y-1.5">
      {users.map((u) => (
        <li key={`${u.id}-${u.destination ?? ""}`} className="flex items-center gap-2 text-[13px]">
          <Link to={`/containers/${u.id}`} className="min-w-0 truncate font-medium text-primary hover:underline" title={u.name}>
            {u.name}
          </Link>
          {u.state && <StatusBadge state={u.state} />}
          <span className="min-w-0 flex-1 truncate font-mono text-xs text-muted-foreground">
            {u.destination ? `${u.destination}${u.readOnly ? " (ro)" : ""}` : [u.ipv4, u.ipv6].filter(Boolean).join(" · ")}
          </span>
          {extra?.(u)}
        </li>
      ))}
    </ul>
  );
}

export function SectionTitle({ children }: { children: ReactNode }) {
  return <h3 className="mt-4 mb-1.5 text-xs font-semibold uppercase tracking-wide text-muted-foreground">{children}</h3>;
}

/** Resizable right-hand inspector shell shared by volumes and networks. */
export function InspectorShell({
  icon,
  title,
  subtitle,
  onClose,
  actions,
  children,
}: {
  icon: ReactNode;
  title: string;
  subtitle?: ReactNode;
  onClose: () => void;
  actions?: ReactNode;
  children: ReactNode;
}) {
  const width = useUI((s) => s.inspectorWidth);
  const setWidth = useUI((s) => s.setInspectorWidth);
  const startDrag = (e: React.PointerEvent) => {
    e.preventDefault();
    const startX = e.clientX;
    const startW = width;
    const move = (ev: PointerEvent) => setWidth(startW + (startX - ev.clientX));
    const up = () => {
      window.removeEventListener("pointermove", move);
      window.removeEventListener("pointerup", up);
    };
    window.addEventListener("pointermove", move);
    window.addEventListener("pointerup", up);
  };
  return (
    <aside style={{ width }} className="relative flex shrink-0 flex-col border-l border-border bg-card max-[1199px]:absolute max-[1199px]:inset-y-0 max-[1199px]:right-0 max-[1199px]:z-30 max-[1199px]:max-w-[92%] max-[1199px]:shadow-2xl" aria-label={`Inspector for ${title}`}>
      <div
        role="separator"
        aria-orientation="vertical"
        aria-label="Resize inspector"
        onPointerDown={startDrag}
        className="absolute inset-y-0 -left-1 z-10 w-2 cursor-col-resize hover:bg-primary/30"
      />
      <div className="flex items-start gap-3 border-b border-border p-4">
        <div className="mt-0.5 shrink-0 text-primary [&_svg]:size-6">{icon}</div>
        <div className="min-w-0 flex-1">
          <h2 className="flex items-center gap-1 text-lg font-semibold">
            <span className="truncate" title={title}>{title}</span>
            <IconButton label="Copy name" className="size-6 shrink-0" onClick={() => copyToClipboard(title, "Name")}>
              <Copy className="size-3.5" />
            </IconButton>
          </h2>
          {subtitle && <div className="mt-1 text-xs text-muted-foreground">{subtitle}</div>}
        </div>
        <IconButton label="Close inspector" className="size-7" onClick={onClose}>
          <X className="size-4" />
        </IconButton>
      </div>
      {actions && <InspectorActions>{actions}</InspectorActions>}
      <div className="min-h-0 flex-1 overflow-auto px-4 pb-4 pt-3">{children}</div>
    </aside>
  );
}
