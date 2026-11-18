import { useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { useCreateVolume } from "@/hooks/useVolumes";
import { errorMessage } from "@/lib/errors";

/** Parses "key=value" lines; returns an error message for malformed lines. */
export function parseLabels(text: string): { labels: Record<string, string>; error?: string } {
  const labels: Record<string, string> = {};
  for (const raw of text.split("\n")) {
    const line = raw.trim();
    if (!line) continue;
    const i = line.indexOf("=");
    if (i <= 0) return { labels, error: `Label "${line}" must look like key=value` };
    labels[line.slice(0, i).trim()] = line.slice(i + 1).trim();
  }
  return { labels };
}

export function LabelsField({ id, value, onChange }: { id: string; value: string; onChange: (v: string) => void }) {
  return (
    <div className="space-y-1.5">
      <Label htmlFor={id}>Labels (optional)</Label>
      <textarea
        id={id}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        rows={3}
        placeholder={"one per line, e.g.\nenv=dev"}
        className="w-full rounded-md border border-input bg-transparent px-3 py-2 font-mono text-xs outline-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
      />
    </div>
  );
}

export function CreateVolumeDialog({ connectionId, open, onOpenChange }: { connectionId: string; open: boolean; onOpenChange: (o: boolean) => void }) {
  const [name, setName] = useState("");
  const [driver, setDriver] = useState("local");
  const [labelsText, setLabelsText] = useState("");
  const [error, setError] = useState<string | null>(null);
  const create = useCreateVolume(connectionId);

  const reset = () => {
    setName("");
    setDriver("local");
    setLabelsText("");
    setError(null);
  };

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    const parsed = parseLabels(labelsText);
    if (parsed.error) return setError(parsed.error);
    setError(null);
    try {
      const v = await create.mutateAsync({ name: name.trim(), driver: driver.trim(), labels: parsed.labels });
      toast.success(`Volume ${v.name} created`);
      reset();
      onOpenChange(false);
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        if (!o) reset();
        onOpenChange(o);
      }}
    >
      <DialogContent className="sm:max-w-md">
        <form onSubmit={submit} className="space-y-4">
          <DialogHeader>
            <DialogTitle>Create volume</DialogTitle>
            <DialogDescription>Creates a named volume on this Engine.</DialogDescription>
          </DialogHeader>
          <div className="space-y-1.5">
            <Label htmlFor="vol-name">Name</Label>
            <Input id="vol-name" value={name} onChange={(e) => setName(e.target.value)} autoFocus placeholder="my-data" className="h-8" />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="vol-driver">Driver</Label>
            <Input id="vol-driver" value={driver} onChange={(e) => setDriver(e.target.value)} placeholder="local" className="h-8" />
          </div>
          <LabelsField id="vol-labels" value={labelsText} onChange={setLabelsText} />
          {error && <p className="text-sm text-destructive" role="alert">{error}</p>}
          <DialogFooter>
            <Button type="button" variant="outline" size="sm" onClick={() => onOpenChange(false)}>Cancel</Button>
            <Button type="submit" size="sm" disabled={!name.trim() || create.isPending}>
              {create.isPending ? "Creating…" : "Create volume"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
