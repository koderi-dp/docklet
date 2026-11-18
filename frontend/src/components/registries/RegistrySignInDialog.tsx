import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";
import { RegistryService } from "@bindings/services";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useActiveConnection } from "@/hooks/useActiveConnection";
import { useInvalidateRegistries, useSignInDialog } from "@/hooks/useRegistries";
import { errorMessage } from "@/lib/errors";
import { CUSTOM_PRESET_ID, REGISTRY_PRESETS, presetForHost } from "@/lib/registryPresets";

/** The one sign-in dialog; mounted once in the top bar and opened through useSignInDialog. */
export function RegistrySignInHost() {
  const request = useSignInDialog((s) => s.request);
  const close = useSignInDialog((s) => s.close);
  const { connectionId, isConnected, profile } = useActiveConnection();
  const invalidate = useInvalidateRegistries();

  const [presetId, setPresetId] = useState(REGISTRY_PRESETS[0].id);
  const [customHost, setCustomHost] = useState("");
  const [username, setUsername] = useState("");
  const [secret, setSecret] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const open = request !== null;

  useEffect(() => {
    if (!request) return;
    const preset = request.registry ? presetForHost(request.registry) : REGISTRY_PRESETS[0];
    setPresetId(preset.id);
    setCustomHost(preset.id === CUSTOM_PRESET_ID ? (request.registry ?? "") : "");
    setUsername("");
    setSecret("");
    setError(null);
    setBusy(false);
  }, [request]);

  const preset = REGISTRY_PRESETS.find((p) => p.id === presetId) ?? REGISTRY_PRESETS[0];
  const host = preset.id === CUSTOM_PRESET_ID ? customHost.trim() : preset.host;
  const storage = useQuery({
    queryKey: ["registry-storage", host],
    queryFn: () => RegistryService.StorageFor(host),
    enabled: open && host !== "",
    retry: false,
    staleTime: 30_000,
  });
  const canSubmit = isConnected && !!connectionId && host !== "" && username.trim() !== "" && secret !== "" && !busy;

  const submit = async () => {
    if (!canSubmit || !connectionId) return;
    // The secret lives in component state only until the request is sent.
    const pw = secret;
    setSecret("");
    setError(null);
    setBusy(true);
    try {
      const entry = await RegistryService.SignIn(connectionId, host, username.trim(), pw);
      invalidate();
      const done = request?.onSignedIn;
      close();
      toast.success(`Signed in to ${entry.host}`, { description: `Saved in ${entry.storeLabel}.` });
      done?.(entry);
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setBusy(false);
    }
  };

  const submitButton = (
    <Button type="submit" disabled={!canSubmit}>
      {busy && <Loader2 className="size-4 animate-spin" />}
      Sign in
    </Button>
  );

  return (
    <Dialog open={open} onOpenChange={(o) => !o && !busy && close()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Sign in to a registry</DialogTitle>
          <DialogDescription>
            {isConnected ? (
              <>
                The credentials are checked with <span className="font-medium text-foreground">{profile?.name ?? "the connected engine"}</span> before they are saved.
              </>
            ) : (
              "Connect to an engine first: Docklet checks the credentials through it before saving them."
            )}
          </DialogDescription>
        </DialogHeader>
        <form
          autoComplete="off"
          onSubmit={(e) => {
            e.preventDefault();
            void submit();
          }}
          className="space-y-3"
        >
          <div className="space-y-1.5">
            <label htmlFor="reg-preset" className="text-xs font-medium text-muted-foreground">
              Registry
            </label>
            <Select value={presetId} onValueChange={setPresetId}>
              <SelectTrigger id="reg-preset" className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {REGISTRY_PRESETS.map((p) => (
                  <SelectItem key={p.id} value={p.id}>
                    {p.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            {preset.id === CUSTOM_PRESET_ID && (
              <Input
                id="reg-host"
                aria-label="Registry host"
                value={customHost}
                onChange={(e) => setCustomHost(e.target.value)}
                placeholder="registry.example.com:5000"
                className="font-mono"
                spellCheck={false}
                autoComplete="off"
              />
            )}
            <p className="text-xs text-muted-foreground">{preset.hint}</p>
          </div>
          <div className="space-y-1.5">
            <label htmlFor="reg-user" className="text-xs font-medium text-muted-foreground">
              User name
            </label>
            <Input id="reg-user" value={username} onChange={(e) => setUsername(e.target.value)} spellCheck={false} autoComplete="off" autoFocus />
          </div>
          <div className="space-y-1.5">
            <label htmlFor="reg-secret" className="text-xs font-medium text-muted-foreground">
              Password or access token
            </label>
            <Input id="reg-secret" type="password" value={secret} onChange={(e) => setSecret(e.target.value)} autoComplete="new-password" aria-invalid={!!error} />
          </div>
          {error && (
            <p role="alert" className="whitespace-pre-wrap break-words text-xs text-destructive">
              {error}
            </p>
          )}
          <p className="text-xs text-muted-foreground">
            Stored on this computer and sent only with pulls and pushes to that registry.
            {storage.data && <> Saved in {storage.data}.</>}
          </p>
          <DialogFooter className="pt-1">
            <Button type="button" variant="outline" disabled={busy} onClick={close}>
              Cancel
            </Button>
            {isConnected ? (
              submitButton
            ) : (
              <Tooltip>
                <TooltipTrigger asChild>
                  <span>{submitButton}</span>
                </TooltipTrigger>
                <TooltipContent>Connect to an engine first</TooltipContent>
              </Tooltip>
            )}
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
