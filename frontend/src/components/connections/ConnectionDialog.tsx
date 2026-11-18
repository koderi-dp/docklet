import { useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { CheckCircle2, Loader2 } from "lucide-react";
import { toast } from "sonner";
import { ConnectionService } from "@bindings/services";
import { ConnectionState, ConnectionType, type ConnectionProfile, type ConnectionStatus } from "@bindings/model";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import { connectionsKey, useConnections, useSelectConnection } from "@/hooks/useActiveConnection";
import { useUI } from "@/stores/uiStore";
import { ContextPicker } from "./ContextPicker";
import { ErrorWithDiagnostics } from "./DiagnosticsBlock";
import { TYPE_META, TypeIcon, typeLabel } from "./connectionMeta";
import { buildProfile, defaultForm, defaultName, formFromProfile, type FormState } from "./profileForm";
import { Field, SshForm, TcpForm } from "./TransportForms";
import { WslPicker } from "./WslPicker";
import { SECRET_PASSPHRASE, SECRET_PASSWORD, type TypedSecrets } from "./SshAuthFields";
import { errorMessage } from "@/lib/errors";
import { isWindows } from "@/lib/platform";

const CARD_ORDER = [
  ConnectionType.ConnWSL,
  ConnectionType.ConnNamedPipe,
  ConnectionType.ConnUnix,
  ConnectionType.ConnTCP,
  ConnectionType.ConnSSH,
  ConnectionType.ConnContext,
];

type Result = { kind: "ok"; status: ConnectionStatus } | { kind: "error"; message: string; diagnostics?: string };

function ConnectionForm({ editId, onClose }: { editId?: string; onClose: () => void }) {
  const qc = useQueryClient();
  const { data: conns } = useConnections();
  const existing = editId ? conns?.find((c) => c.id === editId) : undefined;
  const activeId = useUI((s) => s.activeConnectionId);
  const select = useSelectConnection();
  const [f, setF] = useState<FormState>(() =>
    existing ? formFromProfile(existing) : defaultForm(isWindows ? ConnectionType.ConnWSL : ConnectionType.ConnUnix),
  );
  const [busy, setBusy] = useState<null | "test" | "testsave" | "save">(null);
  const [result, setResult] = useState<Result | null>(null);
  const [secrets, setSecrets] = useState<TypedSecrets>({ password: "", passphrase: "" });
  const hasTyped = f.type === ConnectionType.ConnSSH && (secrets.password !== "" || secrets.passphrase !== "");
  const editing = !!editId;
  const set = (p: Partial<FormState>) => {
    setF((s) => ({ ...s, ...p }));
    setResult(null);
  };
  const pickType = (type: ConnectionType) => setF((s) => ({ ...defaultForm(type), name: s.name }));

  const resolve = async (): Promise<ConnectionProfile | null> => {
    if (f.type === ConnectionType.ConnContext) {
      if (!f.contextName) {
        setResult({ kind: "error", message: "Select a Docker context" });
        return null;
      }
      const p = await ConnectionService.ContextProfile(f.contextName);
      p.id = editId ?? "";
      p.name = f.name.trim() || p.name;
      return p;
    }
    const b = buildProfile(f, editId ?? "");
    if (!b.profile) {
      setResult({ kind: "error", message: b.error ?? "Check the form" });
      return null;
    }
    return b.profile;
  };

  const run = async (mode: "test" | "testsave" | "save") => {
    setBusy(mode);
    setResult(null);
    try {
      const profile = await resolve();
      if (!profile) return;
      if (mode !== "save" && !hasTyped) {
        const st = await ConnectionService.Test(profile);
        if (st.state !== ConnectionState.StateConnected) {
          setResult({ kind: "error", message: st.error || "The connection test failed.", diagnostics: st.diagnostics });
          return;
        }
        setResult({ kind: "ok", status: st });
        if (mode === "test") return;
      }
      const saved = await ConnectionService.Save(profile);
      if (hasTyped) {
        // Secrets are sent once to the OS credential store; they never enter the profile or any cache.
        if (secrets.password) await ConnectionService.SetSecret(saved.id, SECRET_PASSWORD, secrets.password);
        if (secrets.passphrase) await ConnectionService.SetSecret(saved.id, SECRET_PASSPHRASE, secrets.passphrase);
        setSecrets({ password: "", passphrase: "" });
        if (mode === "testsave") {
          const st = await ConnectionService.Test(saved);
          if (st.state !== ConnectionState.StateConnected) {
            toast.error(`Saved ${saved.name}, but the connection test failed`, { description: st.error || undefined, duration: Infinity, closeButton: true });
          }
        }
      }
      await qc.invalidateQueries({ queryKey: connectionsKey });
      toast.success(`${editing ? "Saved" : "Added"} ${saved.name}`, {
        description: editing && saved.id === activeId ? "Reconnect to apply the changes." : undefined,
      });
      if (!editing && !activeId) void select(saved.id);
      onClose();
    } catch (e) {
      setResult({ kind: "error", message: errorMessage(e) });
    } finally {
      setBusy(null);
    }
  };

  const blockedReason = f.type === ConnectionType.ConnContext ? (f.contextName ? undefined : "Select a Docker context") : buildProfile(f).error;
  const disabled = busy !== null || !!blockedReason;

  return (
    <>
      <DialogHeader>
        <DialogTitle>{editing ? `Edit ${existing?.name ?? "connection"}` : "Add connection"}</DialogTitle>
        <DialogDescription>
          {editing ? `${typeLabel(f.type)} connection. Changes apply the next time it connects.` : "Choose how Docklet reaches an existing Docker Engine."}
        </DialogDescription>
      </DialogHeader>

      {!editing && (
        <div className="grid grid-cols-3 gap-2" role="radiogroup" aria-label="Connection type">
          {CARD_ORDER.filter((t) => isWindows || (t !== ConnectionType.ConnWSL && t !== ConnectionType.ConnNamedPipe)).map((t) => {
            const m = TYPE_META[t];
            const on = f.type === t;
            return (
              <button
                key={t}
                type="button"
                role="radio"
                aria-checked={on}
                onClick={() => pickType(t)}
                className={cn(
                  "flex flex-col items-start gap-1 rounded-md border bg-card p-2.5 text-left transition-colors hover:bg-muted",
                  on && "border-primary bg-accent/40",
                )}
              >
                <TypeIcon type={t} className={cn("size-[18px] text-muted-foreground", on && "text-primary")} />
                <span className="text-sm font-medium">{m.short}</span>
                <span className="line-clamp-2 text-[11px] leading-tight text-muted-foreground">{m.hint}</span>
              </button>
            );
          })}
        </div>
      )}

      <div className="space-y-3">
        <Field label="Name" htmlFor="conn-name">
          <Input
            id="conn-name"
            value={f.name}
            onChange={(e) => set({ name: e.target.value })}
            placeholder={defaultName(f) || "My Docker Engine"}
            maxLength={100}
          />
        </Field>
        {f.type === ConnectionType.ConnWSL && (
          <WslPicker value={f.distro} onChange={(distro) => set({ distro })} successAction={{ label: "Test & Save", run: () => void run("testsave") }} />
        )}
        {f.type === ConnectionType.ConnNamedPipe && (
          <Field label="Pipe path" htmlFor="pipe" hint="Windows named pipe of the Docker Engine, for example npipe:////./pipe/docker_engine.">
            <Input id="pipe" value={f.pipe} onChange={(e) => set({ pipe: e.target.value })} className="font-mono text-xs" />
          </Field>
        )}
        {f.type === ConnectionType.ConnUnix && (
          <Field label="Socket path" htmlFor="sock" hint="For example unix:///var/run/docker.sock.">
            <Input id="sock" value={f.socket} onChange={(e) => set({ socket: e.target.value })} className="font-mono text-xs" />
          </Field>
        )}
        {f.type === ConnectionType.ConnTCP && <TcpForm f={f} set={set} />}
        {f.type === ConnectionType.ConnSSH && <SshForm f={f} set={set} connectionId={editId} secrets={secrets} onSecrets={setSecrets} />}
        {f.type === ConnectionType.ConnContext && (
          <ContextPicker value={f.contextName} onChange={(contextName) => set({ contextName })} />
        )}
      </div>

      {result?.kind === "ok" && (
        <p className="flex items-center gap-2 rounded-md border border-success/30 bg-success/10 p-3 text-sm text-success" role="status">
          <CheckCircle2 className="size-4 shrink-0" />
          Connection works
          {result.status.serverVersion ? `: Docker ${result.status.serverVersion}` : ""}
          {result.status.apiVersion ? `, API ${result.status.apiVersion}` : ""}
          {result.status.os ? `, ${result.status.os}` : ""}.
        </p>
      )}
      {result?.kind === "error" && <ErrorWithDiagnostics message={result.message} diagnostics={result.diagnostics} />}

      <DialogFooter className="gap-2 sm:justify-between">
        <Button variant="ghost" onClick={onClose} disabled={busy === "testsave" || busy === "save"}>
          Cancel
        </Button>
        <div className="flex flex-wrap justify-end gap-2">
          <Button variant="outline" onClick={() => run("test")} disabled={disabled || hasTyped} title={blockedReason ?? (hasTyped ? "A new password is stored with the saved connection: use Test & Save" : "Connects with a temporary client; nothing is saved")}>
            {busy === "test" && <Loader2 className="animate-spin" />}
            Test
          </Button>
          <Button variant="outline" onClick={() => run("save")} disabled={disabled} title={disabled ? blockedReason : undefined}>
            {busy === "save" && <Loader2 className="animate-spin" />}
            Save without testing
          </Button>
          <Button onClick={() => run("testsave")} disabled={disabled} title={disabled ? blockedReason : undefined}>
            {busy === "testsave" && <Loader2 className="animate-spin" />}
            Test &amp; Save
          </Button>
        </div>
      </DialogFooter>
    </>
  );
}

/** Mounted once in AppShell; opened via useUI().openConnectionDialog. */
export function ConnectionDialogHost() {
  const req = useUI((s) => s.connectionDialog);
  const open = useUI((s) => s.openConnectionDialog);
  return (
    <Dialog open={!!req} onOpenChange={(o) => !o && open(null)}>
      <DialogContent className="max-h-[90vh] gap-4 overflow-y-auto sm:max-w-[640px]">
        {req && <ConnectionForm key={`${req.mode}:${req.id ?? ""}`} editId={req.mode === "edit" ? req.id : undefined} onClose={() => open(null)} />}
      </DialogContent>
    </Dialog>
  );
}
