import { useEffect, useMemo, useState } from "react";
import { Loader2, Plus, X } from "lucide-react";
import { toast } from "sonner";
import { useQueryClient } from "@tanstack/react-query";
import { ContainerService } from "@bindings/services";
import type { ContainerCreateSpec } from "@bindings/model";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { useConnections } from "@/hooks/useActiveConnection";
import { containersKey } from "@/hooks/useContainers";
import { errorMessage } from "@/lib/errors";
import { emptyPort, validateForm, type EnvRow, type MountRow, type PortRow } from "./createContainerForm";

export interface CreateContainerDialogProps {
  connectionId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Optional image reference to prefill. */
  image?: string;
}

const selectCls = "h-8 rounded-md border border-border bg-card px-2 text-[13px] text-foreground";

function Field({ label, children, className }: { label: string; children: React.ReactNode; className?: string }) {
  return (
    <div className={className}>
      <label className="mb-1 block text-xs font-medium text-muted-foreground">{label}</label>
      {children}
    </div>
  );
}

export default function CreateContainerDialog({ connectionId, open, onOpenChange, image: imageProp }: CreateContainerDialogProps) {
  const qc = useQueryClient();
  const { data: profiles } = useConnections();
  const engineName = profiles?.find((p) => p.id === connectionId)?.name ?? connectionId;

  const [image, setImage] = useState(imageProp ?? "");
  const [name, setName] = useState("");
  const [command, setCommand] = useState("");
  const [env, setEnv] = useState<EnvRow[]>([]);
  const [ports, setPorts] = useState<PortRow[]>([]);
  const [mounts, setMounts] = useState<MountRow[]>([]);
  const [restart, setRestart] = useState("no");
  const [maxRetries, setMaxRetries] = useState("");
  const [memory, setMemory] = useState("");
  const [cpus, setCpus] = useState("");
  const [autoStart, setAutoStart] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Fresh form each time the dialog opens.
  useEffect(() => {
    if (!open) return;
    setImage(imageProp ?? "");
    setName("");
    setCommand("");
    setEnv([]);
    setPorts([]);
    setMounts([]);
    setRestart("no");
    setMaxRetries("");
    setMemory("");
    setCpus("");
    setAutoStart(true);
    setError(null);
    setBusy(false);
  }, [open, imageProp]);

  const clientError = useMemo(
    () => validateForm({ image, name, env, ports, mounts, memory, cpus, maxRetries }),
    [image, name, env, ports, mounts, memory, cpus, maxRetries],
  );

  const submit = async () => {
    const v = clientError;
    if (v) {
      setError(v);
      return;
    }
    const spec: ContainerCreateSpec = {
      name: name.trim(),
      image: image.trim(),
      command: command.trim(),
      env: env.filter((e) => e.key.trim()).map((e) => ({ key: e.key.trim(), value: e.value })),
      ports: ports
        .filter((p) => p.containerPort.trim())
        .map((p) => ({
          hostIp: p.hostIp.trim(),
          hostPort: p.hostPort.trim() ? Number(p.hostPort) : 0,
          containerPort: Number(p.containerPort),
          protocol: p.protocol,
        })),
      mounts: mounts
        .filter((m) => m.target.trim())
        .map((m) => ({ type: m.type, source: m.source.trim(), target: m.target.trim(), readOnly: m.readOnly })),
      restartPolicy: restart,
      maxRetries: restart === "on-failure" && maxRetries.trim() ? Number(maxRetries) : 0,
      memoryMiB: memory.trim() ? Math.round(Number(memory)) : 0,
      cpus: cpus.trim() ? Number(cpus) : 0,
      autoStart,
    };
    setBusy(true);
    setError(null);
    try {
      const res = await ContainerService.Create(connectionId, spec);
      void qc.invalidateQueries({ queryKey: containersKey(connectionId) });
      if (res.error) {
        toast.warning(`Container ${res.name} created`, { description: res.error, duration: Infinity, closeButton: true });
      } else {
        toast.success(`Container ${res.name} ${autoStart ? "created and started" : "created"}`);
      }
      onOpenChange(false);
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setBusy(false);
    }
  };

  const upd = <T,>(set: React.Dispatch<React.SetStateAction<T[]>>, i: number, patch: Partial<T>) =>
    set((rows) => rows.map((r, j) => (j === i ? { ...r, ...patch } : r)));
  const del = <T,>(set: React.Dispatch<React.SetStateAction<T[]>>, i: number) => set((rows) => rows.filter((_, j) => j !== i));

  return (
    <Dialog open={open} onOpenChange={(o) => !busy && onOpenChange(o)}>
      <DialogContent className="flex max-h-[88vh] flex-col gap-3 sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>New container</DialogTitle>
          <DialogDescription>
            Creates a container on <span className="font-medium text-foreground">{engineName}</span>. The image must already exist on
            this Engine; it is not pulled automatically.
          </DialogDescription>
        </DialogHeader>

        <form
          className="min-h-0 flex-1 space-y-3 overflow-y-auto pr-1"
          onSubmit={(e) => {
            e.preventDefault();
            void submit();
          }}
        >
          <div className="grid grid-cols-2 gap-3">
            <Field label="Image *">
              <Input value={image} onChange={(e) => setImage(e.target.value)} placeholder="nginx:latest" className="font-mono" autoFocus />
            </Field>
            <Field label="Name (optional)">
              <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="my-container" />
            </Field>
          </div>
          <Field label="Command (optional, overrides the image default)">
            <Input
              value={command}
              onChange={(e) => setCommand(e.target.value)}
              placeholder={'sh -c "echo hello"'}
              className="font-mono"
            />
          </Field>

          <section>
            <div className="mb-1 flex items-center justify-between">
              <h3 className="text-xs font-medium text-muted-foreground">Environment variables</h3>
              <Button type="button" size="sm" variant="ghost" onClick={() => setEnv((r) => [...r, { key: "", value: "" }])}>
                <Plus className="size-4" /> Add
              </Button>
            </div>
            {env.map((r, i) => (
              <div key={i} className="mb-1.5 grid grid-cols-[1fr_1.5fr_auto] gap-2">
                <Input value={r.key} onChange={(e) => upd(setEnv, i, { key: e.target.value })} placeholder="KEY" className="font-mono" aria-label="Variable name" />
                <Input value={r.value} onChange={(e) => upd(setEnv, i, { value: e.target.value })} placeholder="value" className="font-mono" aria-label="Variable value" />
                <Button type="button" size="icon" variant="ghost" aria-label="Remove variable" title="Remove variable" onClick={() => del(setEnv, i)}>
                  <X className="size-4" />
                </Button>
              </div>
            ))}
          </section>

          <section>
            <div className="mb-1 flex items-center justify-between">
              <h3 className="text-xs font-medium text-muted-foreground">Port mappings</h3>
              <Button type="button" size="sm" variant="ghost" onClick={() => setPorts((r) => [...r, emptyPort()])}>
                <Plus className="size-4" /> Add
              </Button>
            </div>
            {ports.map((r, i) => (
              <div key={i} className="mb-1.5 grid grid-cols-[1.2fr_0.8fr_0.8fr_auto_auto] gap-2">
                <Input value={r.hostIp} onChange={(e) => upd(setPorts, i, { hostIp: e.target.value })} placeholder="Host IP (all)" className="font-mono" aria-label="Host IP" />
                <Input value={r.hostPort} onChange={(e) => upd(setPorts, i, { hostPort: e.target.value })} placeholder="Host port" className="font-mono" inputMode="numeric" aria-label="Host port" />
                <Input value={r.containerPort} onChange={(e) => upd(setPorts, i, { containerPort: e.target.value })} placeholder="Container" className="font-mono" inputMode="numeric" aria-label="Container port" />
                <select value={r.protocol} onChange={(e) => upd(setPorts, i, { protocol: e.target.value })} className={selectCls} aria-label="Protocol">
                  <option value="tcp">tcp</option>
                  <option value="udp">udp</option>
                  <option value="sctp">sctp</option>
                </select>
                <Button type="button" size="icon" variant="ghost" aria-label="Remove port mapping" title="Remove port mapping" onClick={() => del(setPorts, i)}>
                  <X className="size-4" />
                </Button>
              </div>
            ))}
          </section>

          <section>
            <div className="mb-1 flex items-center justify-between">
              <h3 className="text-xs font-medium text-muted-foreground">Mounts</h3>
              <Button type="button" size="sm" variant="ghost" onClick={() => setMounts((r) => [...r, { type: "volume", source: "", target: "", readOnly: false }])}>
                <Plus className="size-4" /> Add
              </Button>
            </div>
            {mounts.map((r, i) => (
              <div key={i} className="mb-1.5 grid grid-cols-[auto_1.3fr_1.3fr_auto_auto] items-center gap-2">
                <select value={r.type} onChange={(e) => upd(setMounts, i, { type: e.target.value as MountRow["type"] })} className={selectCls} aria-label="Mount type">
                  <option value="volume">Volume</option>
                  <option value="bind">Bind</option>
                </select>
                <Input value={r.source} onChange={(e) => upd(setMounts, i, { source: e.target.value })} placeholder={r.type === "bind" ? "Host path" : "Volume name"} className="font-mono" aria-label="Mount source" />
                <Input value={r.target} onChange={(e) => upd(setMounts, i, { target: e.target.value })} placeholder="/container/path" className="font-mono" aria-label="Mount target" />
                <label className="flex items-center gap-1 text-xs text-muted-foreground">
                  <input type="checkbox" checked={r.readOnly} onChange={(e) => upd(setMounts, i, { readOnly: e.target.checked })} /> RO
                </label>
                <Button type="button" size="icon" variant="ghost" aria-label="Remove mount" title="Remove mount" onClick={() => del(setMounts, i)}>
                  <X className="size-4" />
                </Button>
              </div>
            ))}
          </section>

          <div className="grid grid-cols-3 gap-3">
            <Field label="Restart policy">
              <select value={restart} onChange={(e) => setRestart(e.target.value)} className={`${selectCls} w-full`}>
                <option value="no">No</option>
                <option value="always">Always</option>
                <option value="unless-stopped">Unless stopped</option>
                <option value="on-failure">On failure</option>
              </select>
            </Field>
            <Field label="Memory limit (MiB)">
              <Input value={memory} onChange={(e) => setMemory(e.target.value)} placeholder="Unlimited" inputMode="numeric" />
            </Field>
            <Field label="CPU limit (cores)">
              <Input value={cpus} onChange={(e) => setCpus(e.target.value)} placeholder="Unlimited" inputMode="decimal" />
            </Field>
          </div>
          {restart === "on-failure" && (
            <Field label="Maximum retries (empty = unlimited)" className="max-w-[220px]">
              <Input value={maxRetries} onChange={(e) => setMaxRetries(e.target.value)} inputMode="numeric" />
            </Field>
          )}
          <label className="flex items-center gap-2 text-[13px]">
            <input type="checkbox" checked={autoStart} onChange={(e) => setAutoStart(e.target.checked)} />
            Start after create
          </label>

          {error && (
            <p role="alert" className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-[13px] text-destructive">
              {error}
            </p>
          )}
          <button type="submit" className="hidden" aria-hidden tabIndex={-1} />
        </form>

        <DialogFooter>
          <Button variant="outline" size="sm" onClick={() => onOpenChange(false)} disabled={busy}>
            Cancel
          </Button>
          <Button size="sm" onClick={() => void submit()} disabled={busy || !image.trim()}>
            {busy && <Loader2 className="size-4 animate-spin" />}
            {busy ? "Creating…" : "Create container"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
