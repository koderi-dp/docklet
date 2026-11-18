import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Loader2 } from "lucide-react";
import { AppService } from "@bindings/services";
import { Button } from "@/components/ui/button";
import { SettingRow } from "@/components/settings/SettingRow";
import { appSettingsKey } from "@/hooks/useApp";
import { toastError } from "@/lib/errors";

/** Reopens the first-run wizard. Saved connections and settings are kept; the wizard asks before it changes anything. */
export function RunSetupRow() {
  const qc = useQueryClient();
  const reopen = useMutation({
    mutationFn: () => AppService.SetOnboarding("pending"),
    onSuccess: (s) => qc.setQueryData(appSettingsKey, s),
    onError: (e) => toastError("Could not start setup", e),
  });
  return (
    <SettingRow label="Setup" hint="Look for engines again and walk through choosing and checking one. Your connections and settings are kept.">
      <Button variant="outline" size="sm" disabled={reopen.isPending} onClick={() => reopen.mutate()}>
        {reopen.isPending && <Loader2 className="animate-spin" />}
        Run setup again
      </Button>
    </SettingRow>
  );
}
