import { useQuery } from "@tanstack/react-query";
import { ConnectionService } from "@bindings/services";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { typeLabel } from "./connectionMeta";

/** Docker CLI contexts (read-only discovery) with a read-only endpoint preview. */
export function ContextPicker({ value, onChange }: { value: string; onChange: (name: string) => void }) {
  const q = useQuery({ queryKey: ["docker-contexts"], queryFn: () => ConnectionService.DockerContexts(), retry: false, staleTime: 0 });
  const list = q.data ?? [];
  const sel = list.find((c) => c.name === value);
  if (q.isLoading) return <Skeleton className="h-9 w-full" />;
  if (q.error)
    return (
      <p className="text-sm text-destructive" role="alert">
        Could not read Docker contexts: {String(q.error)}
      </p>
    );
  if (list.length === 0)
    return (
      <p className="rounded-md border border-dashed p-3 text-sm text-muted-foreground">
        No Docker contexts were found. Contexts are read from the Docker CLI configuration; it is never modified.
      </p>
    );
  return (
    <div className="space-y-2">
      <Select value={value} onValueChange={onChange}>
        <SelectTrigger className="w-full" aria-label="Docker context">
          <SelectValue placeholder="Select a context" />
        </SelectTrigger>
        <SelectContent>
          {list.map((c) => (
            <SelectItem key={c.name} value={c.name} disabled={!c.type}>
              {c.name}
              {c.current ? " (current)" : ""}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      {sel && (
        <div className="space-y-1.5">
          <Input readOnly value={sel.endpoint || "(no endpoint)"} className="font-mono text-xs" aria-label="Endpoint" />
          <p className="text-xs text-muted-foreground">
            {sel.type ? `Imported as ${typeLabel(sel.type)}. ` : ""}
            {sel.description}
            {sel.reason && <span className="text-warning"> {sel.reason}</span>}
          </p>
        </div>
      )}
      <p className="text-xs text-muted-foreground">
        Importing creates an app connection; your global Docker context is not changed.
      </p>
    </div>
  );
}
