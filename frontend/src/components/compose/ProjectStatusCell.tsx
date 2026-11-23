import type { ComposeProject, Container } from "@bindings/model";
import { cn } from "@/lib/utils";
import { projectBar, projectHealthLine, type BarKind } from "@/lib/projectRowModel";
import { ProjectStateBadge } from "./ProjectStateBadge";

const SEGMENT: Record<BarKind, string> = {
  running: "bg-success",
  failed: "bg-destructive",
  completed: "bg-primary/60",
  stopped: "bg-muted-foreground/40",
  notCreated: "border border-dashed border-muted-foreground/50 bg-transparent",
};

/** State badge and health text, a segmented bar of the containers, and the "N of M running" summary. */
export function ProjectStatusCell({ project, containers, known }: { project: ComposeProject; containers: readonly Container[]; known: boolean }) {
  if (!known && !project.notCreated) return <span className="text-xs text-muted-foreground">Loading…</span>;
  const bar = projectBar(project, containers);
  const health = project.notCreated ? undefined : projectHealthLine(containers);
  return (
    <div className="flex min-w-0 flex-col gap-1">
      <div className="flex flex-wrap items-center gap-x-2 gap-y-0.5">
        <ProjectStateBadge containers={project.notCreated ? [] : containers} notCreated={project.notCreated} hideNotes />
        {health && <span className={cn("text-xs", health.tone === "danger" ? "text-destructive" : "text-muted-foreground")}>{health.text}</span>}
      </div>
      {bar.total > 0 && (
        <div className="flex h-1.5 w-full max-w-[220px] gap-px overflow-hidden rounded-full" role="img" aria-label={bar.label}>
          {bar.segments.map((k, i) => (
            <span key={i} className={cn("flex-1", SEGMENT[k])} />
          ))}
        </div>
      )}
      <span className="text-xs tabular-nums text-muted-foreground">{bar.summary}</span>
    </div>
  );
}
