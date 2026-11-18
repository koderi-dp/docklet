import { Link } from "react-router-dom";
import { Copy, Layers, X } from "lucide-react";
import type { ImageRow } from "@bindings/model";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { StatusBadge } from "@/components/shared/StatusBadge";
import { formatBytes, formatCreated } from "@/components/containers/format";
import { useImageDetail, useImageHistory } from "@/hooks/useImages";
import { InUseBadge, shortImageId } from "./ImagesTable";
import { copyToClipboard } from "@/lib/clipboard";
import { UpdateInspectorRow } from "./update/UpdateInspectorRow";

interface Props {
  connectionId: string;
  row: ImageRow;
  width: number;
  onResize: (w: number) => void;
  onClose: () => void;
}

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="grid grid-cols-[96px_1fr] gap-3 border-b border-border/50 py-1.5 text-[13px]">
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="min-w-0 break-words">{children}</dd>
    </div>
  );
}

const none = <span className="text-muted-foreground">None</span>;

function Mono({ items }: { items: readonly string[] }) {
  if (items.length === 0) return none;
  return (
    <div className="flex flex-col gap-0.5 font-mono text-xs">
      {items.map((i) => (
        <span key={i} className="break-all">
          {i}
        </span>
      ))}
    </div>
  );
}

function cmdLine(argv: readonly string[]): string {
  return argv.join(" ");
}

export function ImageInspector({ connectionId, row, width, onResize, onClose }: Props) {
  // Dangling rows are addressed by ID; tagged rows by their reference.
  const { data: d, error, isLoading } = useImageDetail(connectionId, row.ref);
  const hist = useImageHistory(connectionId, row.ref);

  const startDrag = (e: React.PointerEvent) => {
    e.preventDefault();
    const startX = e.clientX;
    const startW = width;
    const move = (ev: PointerEvent) => onResize(startW + (startX - ev.clientX));
    const up = () => {
      window.removeEventListener("pointermove", move);
      window.removeEventListener("pointerup", up);
    };
    window.addEventListener("pointermove", move);
    window.addEventListener("pointerup", up);
  };

  const layers = hist.data ?? [];
  const sized = layers.filter((l) => l.size > 0).length;

  return (
    <aside style={{ width }} className="relative flex shrink-0 flex-col border-l border-border bg-card max-[1199px]:absolute max-[1199px]:inset-y-0 max-[1199px]:right-0 max-[1199px]:z-30 max-[1199px]:max-w-[92%] max-[1199px]:shadow-2xl" aria-label={`Inspector for ${row.ref}`}>
      <div
        role="separator"
        aria-orientation="vertical"
        aria-label="Resize inspector"
        onPointerDown={startDrag}
        className="absolute inset-y-0 -left-1 z-10 w-2 cursor-col-resize hover:bg-primary/30"
      />
      <div className="flex items-start gap-3 border-b border-border p-4">
        <Layers className="mt-0.5 size-6 shrink-0 text-primary" />
        <div className="min-w-0 flex-1">
          <h2 className="truncate text-lg font-semibold" title={row.ref}>
            {row.dangling ? `${row.repository || "<none>"}:<none>` : row.ref}
          </h2>
          <div className="mt-1 flex items-center gap-2">
            <InUseBadge row={row} />
            <span className="font-mono text-xs text-muted-foreground">{shortImageId(row.id)}</span>
          </div>
        </div>
        <Button variant="ghost" size="icon" className="size-7" onClick={onClose} aria-label="Close inspector" title="Close inspector">
          <X className="size-4" />
        </Button>
      </div>
      <div className="min-h-0 flex-1 overflow-auto px-4 pb-4 pt-2">
        {error && <p className="py-2 text-xs text-destructive">Could not load details: {String(error)}</p>}
        {isLoading && <Skeleton className="mt-2 h-40 w-full" />}
        <dl>
          <Row label="Image ID">
            <span className="inline-flex items-center gap-1 font-mono text-xs">
              <span className="break-all">{row.id}</span>
              <button aria-label="Copy image ID" title="Copy image ID" onClick={() => copyToClipboard(row.id, "Image ID")} className="shrink-0 text-muted-foreground hover:text-foreground">
                <Copy className="size-3.5" />
              </button>
            </span>
          </Row>
          <Row label="Size">{formatBytes(row.size)}</Row>
          <Row label="Created">{formatCreated(row.created)}</Row>
          <UpdateInspectorRow row={row} />
          {d && (
            <>
              <Row label="Tags">
                <Mono items={d.tags ?? []} />
              </Row>
              <Row label="Digests">
                <Mono items={d.digests ?? []} />
              </Row>
              <Row label="Platform">
                {d.os || d.architecture ? `${d.os}/${d.architecture}${d.variant ? `/${d.variant}` : ""}` : <span className="text-muted-foreground">—</span>}
              </Row>
              <Row label="Env">
                {d.envCount} variable{d.envCount === 1 ? "" : "s"} <span className="text-muted-foreground">(values hidden)</span>
              </Row>
              <Row label="Entrypoint">{d.entrypoint?.length ? <code className="font-mono text-xs">{cmdLine(d.entrypoint)}</code> : none}</Row>
              <Row label="Cmd">{d.cmd?.length ? <code className="font-mono text-xs">{cmdLine(d.cmd)}</code> : none}</Row>
              {d.workingDir && <Row label="Working dir"><code className="font-mono text-xs">{d.workingDir}</code></Row>}
              {d.user && <Row label="User"><code className="font-mono text-xs">{d.user}</code></Row>}
              <Row label="Ports">
                <Mono items={d.exposedPorts ?? []} />
              </Row>
              <Row label="Labels">
                {Object.keys(d.labels ?? {}).length === 0 ? (
                  none
                ) : (
                  <div className="flex flex-col gap-0.5 font-mono text-xs">
                    {Object.entries(d.labels ?? {}).map(([k, v]) => (
                      <span key={k} className="break-all">
                        {k}=<span className="text-muted-foreground">{v}</span>
                      </span>
                    ))}
                  </div>
                )}
              </Row>
            </>
          )}
        </dl>

        <h3 className="mb-1 mt-4 text-sm font-semibold">History</h3>
        {hist.isLoading && <Skeleton className="h-16 w-full" />}
        {hist.error && <p className="text-xs text-destructive">Could not load history: {String(hist.error)}</p>}
        {hist.data && (
          <>
            <p className="mb-2 text-xs text-muted-foreground">
              {layers.length} step{layers.length === 1 ? "" : "s"}, {sized} adding data{d ? `, ${d.layerCount} filesystem layer${d.layerCount === 1 ? "" : "s"}` : ""}
            </p>
            <ol className="space-y-1.5">
              {layers.map((l, i) => (
                <li key={`${l.id}-${i}`} className="rounded-md border border-border bg-muted/30 p-2">
                  <div className="flex items-center justify-between gap-2 text-xs text-muted-foreground">
                    <span>{formatCreated(l.created)}</span>
                    <span className="tabular-nums">{l.size > 0 ? formatBytes(l.size) : "0 B"}</span>
                  </div>
                  <code className="mt-1 block max-h-16 overflow-hidden break-all font-mono text-[11px]" title={l.createdBy}>
                    {l.createdBy || "—"}
                  </code>
                </li>
              ))}
            </ol>
          </>
        )}

        <h3 className="mb-1 mt-4 text-sm font-semibold">Containers</h3>
        {d ? (
          d.containers?.length ? (
            <ul className="space-y-1">
              {d.containers.map((c) => (
                <li key={c.id} className="flex items-center justify-between gap-2 text-[13px]">
                  <Link to={`/containers/${c.id}`} className="truncate text-primary hover:underline" title={c.name}>
                    {c.name}
                  </Link>
                  <StatusBadge state={c.state} />
                </li>
              ))}
            </ul>
          ) : (
            <p className="text-xs text-muted-foreground">No container uses this image.</p>
          )
        ) : (
          !error && <Skeleton className="h-8 w-full" />
        )}
      </div>
    </aside>
  );
}
