import { useEffect, useState } from "react";
import { FolderOpen, KeyRound } from "lucide-react";
import { toast } from "sonner";
import { ConnectionService, DialogService } from "@bindings/services";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import type { FormState } from "./profileForm";
import { errorMessage } from "@/lib/errors";

export const SECRET_PASSWORD = "ssh-password";
export const SECRET_PASSPHRASE = "ssh-passphrase";

/** Secrets typed in the dialog. Held only in dialog state until sent once to SetSecret. */
export interface TypedSecrets {
  password: string;
  passphrase: string;
}

function SecretRow({
  id,
  label,
  kind,
  connectionId,
  value,
  onChange,
}: {
  id: string;
  label: string;
  kind: string;
  connectionId?: string;
  value: string;
  onChange: (v: string) => void;
}) {
  const [stored, setStored] = useState(false);
  useEffect(() => {
    if (!connectionId) return;
    let live = true;
    ConnectionService.HasSecret(connectionId, kind)
      .then((v) => live && setStored(v))
      .catch(() => live && setStored(false));
    return () => {
      live = false;
    };
  }, [connectionId, kind]);

  const remove = async () => {
    if (!connectionId) return;
    try {
      await ConnectionService.DeleteSecret(connectionId, kind);
      setStored(false);
      toast.success(`Removed the stored ${label.toLowerCase()}`);
    } catch (e) {
      toast.error("Could not remove the secret", { description: errorMessage(e), duration: Infinity, closeButton: true });
    }
  };

  return (
    <div className="space-y-1.5">
      <Label htmlFor={id} className="text-xs font-medium text-muted-foreground">
        {label} (optional)
      </Label>
      {stored && !value ? (
        <div className="flex items-center justify-between rounded-md border bg-muted/40 px-3 py-1.5 text-xs text-muted-foreground">
          <span className="flex items-center gap-2">
            <KeyRound className="size-[15px]" />
            Stored in the OS credential store
          </span>
          <Button type="button" variant="ghost" size="sm" onClick={remove}>
            Remove
          </Button>
        </div>
      ) : null}
      <Input
        id={id}
        type="password"
        autoComplete="off"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={stored ? "Enter a new value to replace the stored one" : "Not stored"}
      />
    </div>
  );
}

/** SSH identity, ProxyJump and write-only password/passphrase fields. */
export function SshAuthFields({
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
  const pick = async () => {
    try {
      const path = await DialogService.PickFile("Select an SSH private key", "All files", "*");
      if (path) set({ sshIdentityFile: path });
    } catch (e) {
      toast.error("Could not open the file dialog", { description: errorMessage(e) });
    }
  };
  return (
    <div className="space-y-3">
      <div className="space-y-1.5">
        <Label htmlFor="ssh-identity" className="text-xs font-medium text-muted-foreground">
          Identity file (optional)
        </Label>
        <div className="flex gap-2">
          <Input
            id="ssh-identity"
            value={f.sshIdentityFile}
            onChange={(e) => set({ sshIdentityFile: e.target.value })}
            placeholder="from ssh config / agent"
            className="font-mono text-xs"
          />
          <Button type="button" variant="outline" size="sm" onClick={pick}>
            <FolderOpen />
            Browse
          </Button>
        </div>
        {f.sshIdentityFile.trim() && (
          <label className="flex items-center gap-2 text-xs text-muted-foreground">
            <Checkbox checked={f.sshUseAgent} onCheckedChange={(v) => set({ sshUseAgent: v === true })} />
            Also try keys from the SSH agent
          </label>
        )}
      </div>
      <div className="space-y-1.5">
        <Label htmlFor="ssh-jump" className="text-xs font-medium text-muted-foreground">
          ProxyJump (optional)
        </Label>
        <Input id="ssh-jump" value={f.sshProxyJump} onChange={(e) => set({ sshProxyJump: e.target.value })} placeholder="user@jump-host:22" className="font-mono text-xs" />
      </div>
      <div className="grid grid-cols-2 gap-3">
        <SecretRow
          id="ssh-password"
          label="Password"
          kind={SECRET_PASSWORD}
          connectionId={connectionId}
          value={secrets.password}
          onChange={(password) => onSecrets({ ...secrets, password })}
        />
        <SecretRow
          id="ssh-passphrase"
          label="Key passphrase"
          kind={SECRET_PASSPHRASE}
          connectionId={connectionId}
          value={secrets.passphrase}
          onChange={(passphrase) => onSecrets({ ...secrets, passphrase })}
        />
      </div>
    </div>
  );
}
