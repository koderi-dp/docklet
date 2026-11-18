import { useState } from "react";
import { KeyRound, Loader2, ShieldAlert } from "lucide-react";
import type { RegistryEntry } from "@bindings/model";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import { useRegistries, useSignInDialog, useSignOut } from "@/hooks/useRegistries";

function displayUser(u: string): string {
  return u === "<token>" ? "identity token" : u || "—";
}

/** Settings > Registries: sign-ins Docklet uses for pulls and pushes. Secrets never reach this component. */
export function RegistriesSection() {
  const list = useRegistries();
  const signOut = useSignOut();
  const openSignIn = useSignInDialog((s) => s.open);
  const [confirm, setConfirm] = useState<RegistryEntry | null>(null);
  const entries = list.data?.entries ?? [];
  const warnings = list.data?.warnings ?? [];

  return (
    <section className="rounded-lg border bg-card">
      <div className="flex items-center justify-between gap-3 border-b px-4 py-3">
        <div className="min-w-0">
          <h2 className="text-sm font-semibold">Registries</h2>
          <p className="text-xs text-muted-foreground">Sign in for private pulls and pushes. Stored on this computer and sent only with pulls and pushes to that registry.</p>
        </div>
        <Button variant="outline" size="sm" className="shrink-0" onClick={() => openSignIn()}>
          <KeyRound />
          Sign in
        </Button>
      </div>
      {list.isError ? (
        <p className="px-4 py-3 text-sm text-destructive">Could not load registries: {String(list.error)}</p>
      ) : list.isLoading ? (
        <p className="px-4 py-3 text-sm text-muted-foreground">Loading…</p>
      ) : entries.length === 0 ? (
        <p className="px-4 py-3 text-sm text-muted-foreground">Not signed in to any registry. Public images can be pulled without signing in.</p>
      ) : (
        <ul className="divide-y">
          {entries.map((e) => (
            <li key={e.host} className="flex items-center justify-between gap-3 px-4 py-2.5">
              <div className="min-w-0">
                <div className="flex items-baseline gap-2">
                  <span className="truncate font-mono text-[13px] font-medium">{e.host}</span>
                  <span className="truncate text-xs text-muted-foreground">{displayUser(e.username)}</span>
                </div>
                <p className="flex items-center gap-1 truncate text-xs text-muted-foreground">
                  {e.store === "config" && <ShieldAlert className="size-3.5 shrink-0 text-warning" />}
                  {e.storeLabel}
                </p>
              </div>
              <Button variant="outline" size="sm" className="shrink-0" disabled={signOut.isPending} onClick={() => setConfirm(e)}>
                Sign out
              </Button>
            </li>
          ))}
        </ul>
      )}
      {warnings.length > 0 && (
        <ul className="space-y-0.5 border-t px-4 py-2 text-xs text-warning">
          {warnings.map((w) => (
            <li key={w}>{w}</li>
          ))}
        </ul>
      )}
      <AlertDialog open={!!confirm} onOpenChange={(o) => !o && setConfirm(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Sign out of {confirm?.host}?</AlertDialogTitle>
            <AlertDialogDescription>
              Removes the saved credentials for <span className="font-medium text-foreground">{confirm && displayUser(confirm.username)}</span> from {confirm?.storeLabel}.
              {confirm?.store === "helper" && " The docker CLI shares this entry, so it is signed out there too."} Pulls and pushes to{" "}
              <span className="font-mono">{confirm?.host}</span> will be anonymous until you sign in again.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              onClick={() => {
                if (confirm) signOut.mutate(confirm);
                setConfirm(null);
              }}
            >
              {signOut.isPending && <Loader2 className="animate-spin" />}
              Sign out
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </section>
  );
}
