import type { ReactNode } from "react";

/** Page title (24px) + subtitle on the left; contextual actions right-aligned on the SAME row (never wrap below). */
export function PageHeader({
  title,
  subtitle,
  actions,
  breadcrumb,
}: {
  title: ReactNode;
  subtitle?: ReactNode;
  actions?: ReactNode;
  breadcrumb?: ReactNode;
}) {
  return (
    <div className="mb-4 flex min-h-[52px] flex-nowrap items-start justify-between gap-3">
      <div className="min-w-0">
        {breadcrumb && <div className="mb-1 text-xs text-muted-foreground">{breadcrumb}</div>}
        <h1 className="truncate text-2xl font-semibold leading-8">{title}</h1>
        {subtitle && <p className="truncate text-muted-foreground">{subtitle}</p>}
      </div>
      {actions && <div className="flex shrink-0 items-center gap-2 pt-0.5">{actions}</div>}
    </div>
  );
}
