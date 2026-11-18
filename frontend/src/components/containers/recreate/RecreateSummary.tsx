import { AlertTriangle } from "lucide-react";
import { Checkbox } from "@/components/ui/checkbox";
import type { ContainerCreateSpec, ContainerRecreateInfo } from "@bindings/model";
import { diffSpecs, isTagChange, type SpecChange } from "./specDiff";

interface Props {
  info: ContainerRecreateInfo;
  edited: ContainerCreateSpec;
  copy: boolean;
  pullFirst: boolean;
  onPullFirst: (v: boolean) => void;
}

/** The confirmation step: what changes, what is lost, and what the operation will do. */
export function RecreateSummary({ info, edited, copy, pullFirst, onPullFirst }: Props) {
  const changes: SpecChange[] = diffSpecs(info.spec, edited);
  const tagChange = isTagChange(info.spec.image, edited.image.trim());
  return (
    <div className="space-y-3 text-[13px]">
      <section>
        <h3 className="mb-1 text-xs font-medium text-muted-foreground">Changes</h3>
        {changes.length === 0 ? (
          <p className="text-muted-foreground">No fields changed. The container is recreated with the same settings{pullFirst ? " and the freshly pulled image" : ""}.</p>
        ) : (
          <ul className="divide-y divide-border/60 rounded-md border border-border">
            {changes.map((c, i) => (
              <li key={i} className="grid grid-cols-[150px_1fr] gap-2 px-3 py-1.5">
                <span className="text-muted-foreground">{c.label}</span>
                <span className="min-w-0 break-words font-mono text-xs">
                  <span className="text-muted-foreground line-through">{c.from}</span> <span aria-hidden>→</span> <span>{c.to}</span>
                </span>
              </li>
            ))}
          </ul>
        )}
        {tagChange && <p className="mt-1 text-xs text-muted-foreground">The image tag changes: the new image must exist on this Engine, or tick “Pull image first”.</p>}
      </section>

      {(info.warnings ?? []).length > 0 && (
        <section className="space-y-1.5" aria-label="Warnings">
          {(info.warnings ?? []).map((w, i) => (
            <p key={i} className="flex gap-2 rounded-md border border-warning/40 bg-warning/10 px-3 py-2">
              <AlertTriangle className="mt-0.5 size-4 shrink-0 text-warning" />
              <span>{w}</span>
            </p>
          ))}
        </section>
      )}

      <label className="flex items-center gap-2">
        <Checkbox checked={pullFirst} onCheckedChange={(c) => onPullFirst(c === true)} />
        <span>
          Pull image first (<span className="font-mono text-xs">{edited.image.trim()}</span>)
        </span>
      </label>

      <p className="rounded-md border border-border bg-muted/40 px-3 py-2">
        {copy ? (
          <>
            A new container <span className="font-mono">{edited.name.trim()}</span> is created from this configuration. <span className="font-medium">{info.name}</span> is not touched.
          </>
        ) : (
          <>
            {info.running ? "The container will be stopped, " : "The container will be "}replaced by a new one with this configuration, and the old container removed.{" "}
            <span className="font-medium">Volumes are kept.</span> If the new container cannot start, the original is restored.
          </>
        )}
      </p>
    </div>
  );
}
