import { Plus, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { emptyPort, type FormState, type MountRow } from "./createContainerForm";

const selectCls = "h-8 rounded-md border border-border bg-card px-2 text-[13px] text-foreground";

function Field({ label, children, className }: { label: string; children: React.ReactNode; className?: string }) {
  return (
    <div className={className}>
      <label className="mb-1 block text-xs font-medium text-muted-foreground">{label}</label>
      {children}
    </div>
  );
}

function SectionHeader({ title, onAdd }: { title: string; onAdd: () => void }) {
  return (
    <div className="mb-1 flex items-center justify-between">
      <h3 className="text-xs font-medium text-muted-foreground">{title}</h3>
      <Button type="button" size="sm" variant="ghost" onClick={onAdd}>
        <Plus className="size-4" /> Add
      </Button>
    </div>
  );
}

interface Props {
  form: FormState;
  onChange: (patch: Partial<FormState>) => void;
  /** Edit mode (recreate): secret env values stay on the server and show as "unchanged". */
  edit?: boolean;
  /** Name is required in edit mode. */
  nameLabel?: string;
}

/** The container form body shared by "New container" and "Edit & recreate". */
export function ContainerFormFields({ form, onChange, edit, nameLabel }: Props) {
  const { env, ports, mounts } = form;
  const upd = <K extends "env" | "ports" | "mounts">(key: K, i: number, patch: Partial<FormState[K][number]>) =>
    onChange({ [key]: form[key].map((r, j) => (j === i ? { ...r, ...patch } : r)) } as Partial<FormState>);
  const del = (key: "env" | "ports" | "mounts", i: number) => onChange({ [key]: form[key].filter((_, j) => j !== i) } as Partial<FormState>);

  return (
    <>
      <div className="grid grid-cols-2 gap-3">
        <Field label="Image *">
          <Input value={form.image} onChange={(e) => onChange({ image: e.target.value })} placeholder="nginx:latest" className="font-mono" autoFocus={!edit} />
        </Field>
        <Field label={nameLabel ?? "Name (optional)"}>
          <Input value={form.name} onChange={(e) => onChange({ name: e.target.value })} placeholder="my-container" />
        </Field>
      </div>
      <Field label={edit ? "Command (empty = the image default)" : "Command (optional, overrides the image default)"}>
        <Input value={form.command} onChange={(e) => onChange({ command: e.target.value })} placeholder={'sh -c "echo hello"'} className="font-mono" />
      </Field>

      <section>
        <SectionHeader title="Environment variables" onAdd={() => onChange({ env: [...env, { key: "", value: "" }] })} />
        {env.map((r, i) => (
          <div key={i} className="mb-1.5 grid grid-cols-[1fr_1.5fr_auto] gap-2">
            <Input value={r.key} onChange={(e) => upd("env", i, { key: e.target.value })} placeholder="KEY" className="font-mono" aria-label="Variable name" disabled={r.keep} />
            {r.keep ? (
              <div className="flex items-center gap-2">
                <Input value="" placeholder="unchanged (hidden)" disabled className="font-mono" aria-label="Variable value (unchanged)" />
                <Button type="button" size="sm" variant="outline" onClick={() => upd("env", i, { keep: false, value: "" })} title="Replace the stored secret with a new value">
                  Change
                </Button>
              </div>
            ) : (
              <Input value={r.value} onChange={(e) => upd("env", i, { value: e.target.value })} placeholder="value" className="font-mono" aria-label="Variable value" />
            )}
            <Button type="button" size="icon" variant="ghost" aria-label="Remove variable" title="Remove variable" onClick={() => del("env", i)}>
              <X className="size-4" />
            </Button>
          </div>
        ))}
      </section>

      <section>
        <SectionHeader title="Port mappings" onAdd={() => onChange({ ports: [...ports, emptyPort()] })} />
        {ports.map((r, i) => (
          <div key={i} className="mb-1.5 grid grid-cols-[1.2fr_0.8fr_0.8fr_auto_auto] gap-2">
            <Input value={r.hostIp} onChange={(e) => upd("ports", i, { hostIp: e.target.value })} placeholder="Host IP (all)" className="font-mono" aria-label="Host IP" />
            <Input value={r.hostPort} onChange={(e) => upd("ports", i, { hostPort: e.target.value })} placeholder="Host port" className="font-mono" inputMode="numeric" aria-label="Host port" />
            <Input value={r.containerPort} onChange={(e) => upd("ports", i, { containerPort: e.target.value })} placeholder="Container" className="font-mono" inputMode="numeric" aria-label="Container port" />
            <select value={r.protocol} onChange={(e) => upd("ports", i, { protocol: e.target.value })} className={selectCls} aria-label="Protocol">
              <option value="tcp">tcp</option>
              <option value="udp">udp</option>
              <option value="sctp">sctp</option>
            </select>
            <Button type="button" size="icon" variant="ghost" aria-label="Remove port mapping" title="Remove port mapping" onClick={() => del("ports", i)}>
              <X className="size-4" />
            </Button>
          </div>
        ))}
      </section>

      <section>
        <SectionHeader title="Mounts" onAdd={() => onChange({ mounts: [...mounts, { type: "volume", source: "", target: "", readOnly: false }] })} />
        {mounts.map((r, i) => (
          <div key={i} className="mb-1.5 grid grid-cols-[auto_1.3fr_1.3fr_auto_auto] items-center gap-2">
            <select value={r.type} onChange={(e) => upd("mounts", i, { type: e.target.value as MountRow["type"] })} className={selectCls} aria-label="Mount type">
              <option value="volume">Volume</option>
              <option value="bind">Bind</option>
            </select>
            <Input value={r.source} onChange={(e) => upd("mounts", i, { source: e.target.value })} placeholder={r.type === "bind" ? "Host path" : "Volume name"} className="font-mono" aria-label="Mount source" />
            <Input value={r.target} onChange={(e) => upd("mounts", i, { target: e.target.value })} placeholder="/container/path" className="font-mono" aria-label="Mount target" />
            <label className="flex items-center gap-1 text-xs text-muted-foreground">
              <Checkbox checked={r.readOnly} onCheckedChange={(c) => upd("mounts", i, { readOnly: c === true })} /> RO
            </label>
            <Button type="button" size="icon" variant="ghost" aria-label="Remove mount" title="Remove mount" onClick={() => del("mounts", i)}>
              <X className="size-4" />
            </Button>
          </div>
        ))}
      </section>

      <div className="grid grid-cols-3 gap-3">
        <Field label="Restart policy">
          <select value={form.restart} onChange={(e) => onChange({ restart: e.target.value })} className={`${selectCls} w-full`}>
            <option value="no">No</option>
            <option value="always">Always</option>
            <option value="unless-stopped">Unless stopped</option>
            <option value="on-failure">On failure</option>
          </select>
        </Field>
        <Field label="Memory limit (MiB)">
          <Input value={form.memory} onChange={(e) => onChange({ memory: e.target.value })} placeholder="Unlimited" inputMode="numeric" />
        </Field>
        <Field label="CPU limit (cores)">
          <Input value={form.cpus} onChange={(e) => onChange({ cpus: e.target.value })} placeholder="Unlimited" inputMode="decimal" />
        </Field>
      </div>
      {form.restart === "on-failure" && (
        <Field label="Maximum retries (empty = unlimited)" className="max-w-[220px]">
          <Input value={form.maxRetries} onChange={(e) => onChange({ maxRetries: e.target.value })} inputMode="numeric" />
        </Field>
      )}
      <label className="flex items-center gap-2 text-[13px]">
        <Checkbox checked={form.autoStart} onCheckedChange={(c) => onChange({ autoStart: c === true })} />
        {edit ? "Start the new container after recreating" : "Start after create"}
      </label>
    </>
  );
}
