import type { ReactNode } from "react";
import { AlertTriangle, FolderOpen } from "lucide-react";
import { toast } from "sonner";
import { DialogService } from "@bindings/services";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { errorMessage } from "@/lib/errors";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { isLoopbackHost, type FormState } from "./profileForm";
import { SshAuthFields, type TypedSecrets } from "./SshAuthFields";

export function Field({ label, htmlFor, hint, children }: { label: string; htmlFor?: string; hint?: ReactNode; children: ReactNode }) {
  return (
    <div className="space-y-1.5">
      <Label htmlFor={htmlFor} className="text-xs font-medium text-muted-foreground">
        {label}
      </Label>
      {children}
      {hint && <p className="text-xs text-muted-foreground">{hint}</p>}
    </div>
  );
}

export function TcpForm({ f, set }: { f: FormState; set: (p: Partial<FormState>) => void }) {
  const loopback = isLoopbackHost(f.host);
  return (
    <div className="space-y-3">
      <div className="grid grid-cols-[1fr_96px] gap-3">
        <Field label="Host" htmlFor="tcp-host">
          <Input id="tcp-host" value={f.host} onChange={(e) => set({ host: e.target.value })} placeholder="docker.example.com" autoFocus />
        </Field>
        <Field label="Port" htmlFor="tcp-port">
          <Input id="tcp-port" inputMode="numeric" value={f.port} onChange={(e) => set({ port: e.target.value })} />
        </Field>
      </div>
      <div className="flex items-center justify-between rounded-md border bg-card px-3 py-2">
        <div>
          <Label htmlFor="tcp-tls" className="text-sm font-medium">
            Secure with TLS
          </Label>
          <p className="text-xs text-muted-foreground">Verifies the server certificate. Recommended; on by default.</p>
        </div>
        <Switch
          id="tcp-tls"
          checked={f.tls}
          onCheckedChange={(tls) =>
            set({ tls, port: f.port === (tls ? "2375" : "2376") ? (tls ? "2376" : "2375") : f.port, allowInsecure: tls ? false : f.allowInsecure })
          }
        />
      </div>
      {f.tls ? (
        <Field
          label="Certificate directory (optional)"
          htmlFor="tcp-certs"
          hint="Docker-style folder containing ca.pem, cert.pem and key.pem for mutual TLS. Leave empty to verify against the system certificate store."
        >
          <div className="flex gap-2">
            <Input id="tcp-certs" value={f.certDir} onChange={(e) => set({ certDir: e.target.value })} placeholder="C:\Users\you\.docker\certs" className="font-mono text-xs" />
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={async () => {
                try {
                  const dir = await DialogService.PickDirectory("Select the TLS certificate directory");
                  if (dir) set({ certDir: dir });
                } catch (e) {
                  toast.error("Could not open the folder dialog", { description: errorMessage(e) });
                }
              }}
            >
              <FolderOpen />
              Browse
            </Button>
          </div>
        </Field>
      ) : loopback ? (
        <p className="text-xs text-muted-foreground">Plain TCP to this machine (loopback) does not need an opt-in.</p>
      ) : (
        <div className="space-y-2 rounded-md border border-warning/40 bg-warning/10 p-3 text-sm">
          <p className="flex items-start gap-2 text-warning">
            <AlertTriangle className="mt-0.5 size-4 shrink-0" />
            Plain TCP is unencrypted and unauthenticated. Anyone on the network path can control this Docker Engine.
          </p>
          <label className="flex items-center gap-2 text-sm">
            <Checkbox checked={f.allowInsecure} onCheckedChange={(v) => set({ allowInsecure: v === true })} />
            Allow insecure plain TCP to {f.host.trim() || "this host"}
          </label>
        </div>
      )}
    </div>
  );
}

export function SshForm({
  f,
  set,
  connectionId,
  secrets,
  onSecrets,
}: {
  f: FormState;
  set: (p: Partial<FormState>) => void;
  connectionId?: string;
  secrets: TypedSecrets;
  onSecrets: (s: TypedSecrets) => void;
}) {
  return (
    <div className="space-y-3">
      <div className="grid grid-cols-[1fr_1fr_88px] gap-3">
        <Field label="Host" htmlFor="ssh-host">
          <Input id="ssh-host" value={f.sshHost} onChange={(e) => set({ sshHost: e.target.value })} placeholder="192.168.1.100" autoFocus />
        </Field>
        <Field label="User" htmlFor="ssh-user">
          <Input id="ssh-user" value={f.sshUser} onChange={(e) => set({ sshUser: e.target.value })} placeholder="from ssh config" />
        </Field>
        <Field label="Port" htmlFor="ssh-port">
          <Input id="ssh-port" inputMode="numeric" value={f.sshPort} onChange={(e) => set({ sshPort: e.target.value })} />
        </Field>
      </div>
      <SshAuthFields f={f} set={set} connectionId={connectionId} secrets={secrets} onSecrets={onSecrets} />
      <p className="rounded-md border bg-muted/40 p-3 text-xs text-muted-foreground">
        Uses the system OpenSSH client with your ssh config. Host keys are always verified and never accepted
        automatically: trust the host once from a terminal (ssh user@host) before connecting. Passwords and key
        passphrases are kept only in the OS credential store.
      </p>
    </div>
  );
}
