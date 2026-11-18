import type { ReactNode } from "react";

/** One-row action strip for inspectors: compact text buttons first, icon buttons (with tooltips) after. Never wraps. */
export function InspectorActions({ children }: { children: ReactNode }) {
  return <div className="flex flex-nowrap items-center gap-1.5 px-4 pt-3">{children}</div>;
}
