import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

/** Centered empty / error / not-available state. Put actions in children. */
export function EmptyState({
  icon,
  title,
  children,
  className,
}: {
  icon?: ReactNode;
  title: string;
  children?: ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("flex flex-col items-center gap-2 px-6 py-16 text-center text-muted-foreground", className)}>
      {icon && <div className="[&_svg]:size-8">{icon}</div>}
      <h2 className="text-base font-semibold text-foreground">{title}</h2>
      {children && <div className="max-w-md text-sm">{children}</div>}
    </div>
  );
}
