import { useState } from "react";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { parseProfiles } from "@/components/compose/UpOptionsPopover";
import { actionLabel, pendingChanges, summary, visibleItems, type ApplyOptions, type PlanItem } from "@/lib/applyPlanView";
import { cn } from "@/lib/utils";

interface Props {
  plan: { items: readonly PlanItem[] | null; error: string } | undefined;
  loading: boolean;
  dirty: boolean;
  options: ApplyOptions;
  onOptions: (o: ApplyOptions) => void;
}

const TONE: Record<string, string> = {
  recreate: "text-warning",
  recreateImage: "text-warning",
  remove: "text-warning",
  new: "text-success",
  start: "text-success",
};

/** What Apply (compose up -d) would do to each service, and its options. */
export function ApplyPanel({ plan, loading, dirty, options, onOptions }: Props) {
  const [profileText, setProfileText] = useState(options.profiles.join(", "));
  const parsed = parseProfiles(profileText);
  const items = plan?.items ?? [];
  const state = dirty ? "Unsaved changes: save to see the effect" : pendingChanges(items, options.removeOrphans) ? "Saved, not applied" : "Up to date";

  let body;
  if (loading && !plan) body = <Skeleton className="h-16 w-full" />;
  else if (plan?.error)
    body = (
      <div className="space-y-2 text-xs">
        <p className="font-medium text-destructive">docker compose config failed</p>
        <pre className="whitespace-pre-wrap break-words rounded-md bg-muted p-2 font-mono text-[11px]">{plan.error}</pre>
        <p className="text-muted-foreground">The file is saved. Apply is off until this is fixed.</p>
      </div>
    );
  else
    body = (
      <div className={cn("space-y-2", dirty && "opacity-50")}>
        <p className="text-xs font-medium">{summary(items, options.removeOrphans)}</p>
        <ul className="space-y-2">
          {visibleItems(items, options.removeOrphans).map((i) => (
            <li key={i.service} className="text-xs">
              <div className="flex items-baseline justify-between gap-2">
                <span className="min-w-0 truncate font-mono">{i.service}</span>
                <span className={cn("shrink-0 text-[10px] font-medium", i.skipped || i.action === "unchanged" ? "text-muted-foreground" : TONE[i.action])}>
                  {actionLabel(i.action)}
                </span>
              </div>
              {i.detail && <p className="text-muted-foreground">{i.detail}</p>}
            </li>
          ))}
        </ul>
      </div>
    );

  return (
    <section aria-label="What Apply does" className="space-y-3 rounded-[10px] border border-border bg-card p-3">
      <h3 className="text-[10px] font-semibold uppercase tracking-wide text-muted-foreground">What Apply does</h3>
      <p className={cn("text-xs", dirty ? "text-warning" : "text-muted-foreground")}>{state}</p>
      {body}
      <div className="space-y-2 border-t border-border pt-3 text-xs">
        <label className="flex items-center gap-2">
          <Checkbox checked={options.removeOrphans} onCheckedChange={(v) => onOptions({ ...options, removeOrphans: v === true })} />
          Remove orphans
        </label>
        <div className="space-y-1">
          <Label htmlFor="apply-profiles" className="text-xs">
            Profiles
          </Label>
          <Input
            id="apply-profiles"
            value={profileText}
            onChange={(e) => {
              setProfileText(e.target.value);
              const p = parseProfiles(e.target.value);
              if (p) onOptions({ ...options, profiles: p });
            }}
            placeholder="debug, worker"
            spellCheck={false}
            aria-invalid={parsed === null}
            className="h-8"
          />
          {parsed === null && <p className="text-destructive">Profile names may contain letters, digits, _ . and -.</p>}
        </div>
      </div>
    </section>
  );
}
