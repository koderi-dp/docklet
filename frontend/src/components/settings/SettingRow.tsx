import type { ReactNode } from "react";
import { Label } from "@/components/ui/label";

/** One label + hint on the left, its control on the right (Settings sections). */
export function SettingRow({ label, htmlFor, hint, children }: { label: string; htmlFor?: string; hint?: ReactNode; children: ReactNode }) {
  return (
    <div className="flex flex-wrap items-center justify-between gap-3 px-4 py-3">
      <div className="min-w-0 max-w-md">
        <Label htmlFor={htmlFor} className="text-sm font-medium">
          {label}
        </Label>
        {hint && <p className="text-xs text-muted-foreground">{hint}</p>}
      </div>
      <div className="ml-auto max-w-full shrink-0">{children}</div>
    </div>
  );
}
