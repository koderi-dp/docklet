import { useMemo, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { ChevronDown, ChevronUp, Copy, Download, Search } from "lucide-react";
import { toast } from "sonner";
import { ContainerService } from "@bindings/services";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";

/** Pretty JSON of the Engine inspect (secret env values masked server-side). */
export function InspectPanel({ connectionId, containerId, name }: { connectionId: string; containerId: string; name: string }) {
  const q = useQuery({
    queryKey: ["container-inspect-raw", connectionId, containerId],
    queryFn: () => ContainerService.InspectRaw(connectionId, containerId),
    retry: false,
  });
  const [search, setSearch] = useState("");
  const [cursor, setCursor] = useState(0);
  const box = useRef<HTMLDivElement>(null);

  const lines = useMemo(() => (q.data ?? "").split("\n"), [q.data]);
  const needle = search.trim().toLowerCase();
  const matches = useMemo(() => {
    if (!needle) return [] as number[];
    const out: number[] = [];
    lines.forEach((l, i) => {
      if (l.toLowerCase().includes(needle)) out.push(i);
    });
    return out;
  }, [lines, needle]);
  const norm = (n: number) => (matches.length ? ((n % matches.length) + matches.length) % matches.length : 0);
  const active = matches.length ? matches[norm(cursor)] : -1;

  const go = (delta: number) => {
    if (!matches.length) return;
    const next = norm(cursor + delta);
    setCursor(next);
    requestAnimationFrame(() => box.current?.querySelector(`[data-line="${matches[next]}"]`)?.scrollIntoView({ block: "center" }));
  };

  const copy = () =>
    navigator.clipboard
      .writeText(q.data ?? "")
      .then(() => toast.success("Inspect JSON copied"))
      .catch((e: unknown) => toast.error("Could not copy", { description: String(e) }));
  const download = () => {
    const url = URL.createObjectURL(new Blob([q.data ?? ""], { type: "application/json" }));
    const a = document.createElement("a");
    a.href = url;
    a.download = `${name}-inspect.json`;
    document.body.appendChild(a);
    a.click();
    a.remove();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  };

  if (q.isLoading) return <Skeleton className="h-64 w-full" />;
  if (q.error) return <p className="text-sm text-destructive">Could not load inspect data: {String(q.error)}</p>;

  return (
    <div className="flex h-full min-h-0 flex-col gap-2">
      <div className="flex flex-wrap items-center gap-2">
        <div className="relative min-w-[200px] max-w-sm flex-1">
          <Search className="pointer-events-none absolute left-2 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={search}
            onChange={(e) => {
              setSearch(e.target.value);
              setCursor(0);
            }}
            onKeyDown={(e) => {
              if (e.key === "Enter") go(e.shiftKey ? -1 : 1);
            }}
            placeholder="Search JSON"
            aria-label="Search inspect JSON"
            className="h-8 pl-7 text-[13px]"
          />
        </div>
        {needle && (
          <>
            <span className="text-xs text-muted-foreground">{matches.length ? `${norm(cursor) + 1} / ${matches.length}` : "No matches"}</span>
            <Button size="icon" variant="outline" className="size-8" onClick={() => go(-1)} disabled={!matches.length} aria-label="Previous match">
              <ChevronUp className="size-4" />
            </Button>
            <Button size="icon" variant="outline" className="size-8" onClick={() => go(1)} disabled={!matches.length} aria-label="Next match">
              <ChevronDown className="size-4" />
            </Button>
          </>
        )}
        <Button size="sm" variant="outline" onClick={copy}>
          <Copy className="size-3.5" /> Copy
        </Button>
        <Button size="sm" variant="outline" onClick={download}>
          <Download className="size-3.5" /> Download
        </Button>
      </div>
      <p className="text-xs text-muted-foreground">Values of secret-like environment variables are masked.</p>
      <div ref={box} className="min-h-0 flex-1 overflow-auto rounded-md border border-border bg-muted/30 p-2 font-mono text-[11.5px] leading-[1.45]">
        {lines.map((l, i) => {
          const isMatch = !!needle && l.toLowerCase().includes(needle);
          return (
            <div key={i} data-line={i} className={cn("whitespace-pre-wrap break-all", isMatch && "bg-warning/15", i === active && "bg-warning/40")}>
              {l || " "}
            </div>
          );
        })}
      </div>
    </div>
  );
}
