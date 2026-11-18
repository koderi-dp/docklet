import { Check, Loader2 } from "lucide-react";
import { SettingRow } from "@/components/settings/SettingRow";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { useActiveConnection } from "@/hooks/useActiveConnection";
import { engineVersionLabel } from "@/lib/engineKind";
import { useAppSettings } from "@/hooks/useApp";
import { useNotificationSettings } from "@/hooks/useNotifications";
import { Panel, StepFrame } from "./StepFrame";
import type { Onboarding } from "./useOnboarding";

/** The four optional preferences. Each is the real setting from Settings, off until the user turns it on here. */
function Preferences() {
  const { query: app, save: saveApp } = useAppSettings();
  const { query: alerts, save: saveAlerts } = useNotificationSettings();
  const a = app.data;
  const n = alerts.data;
  const busy = saveApp.isPending || saveAlerts.isPending;
  return (
    <Panel>
      <div className="divide-y">
        <SettingRow htmlFor="pref-tray" label="Show a tray icon" hint="Adds a Docklet icon to the system tray.">
          <Switch id="pref-tray" checked={!!a?.showTrayIcon} disabled={!a || busy} onCheckedChange={(showTrayIcon) => a && saveApp.mutate({ ...a, showTrayIcon })} />
        </SettingRow>
        <SettingRow
          htmlFor="pref-close"
          label="Keep running in the tray when the window is closed"
          hint={a?.showTrayIcon ? "The close button hides the window instead of quitting." : "Needs the tray icon."}
        >
          <Switch id="pref-close" checked={!!a?.closeToTray} disabled={!a?.showTrayIcon || busy} onCheckedChange={(closeToTray) => a && saveApp.mutate({ ...a, closeToTray })} />
        </SettingRow>
        <SettingRow htmlFor="pref-alerts" label="Alert me when a container crashes or turns unhealthy" hint="Works only while Docklet is running.">
          <Switch
            id="pref-alerts"
            checked={!!n?.enabled}
            disabled={!n || busy}
            onCheckedChange={(enabled) => n && saveAlerts.mutate({ ...n, enabled, rules: enabled ? { ...n.rules, crash: true, unhealthy: true } : n.rules })}
          />
        </SettingRow>
        <SettingRow
          htmlFor="pref-updates"
          label="Check for updates weekly"
          hint={a?.checkForUpdates === "startup" ? "Currently set to check on startup in Settings. It only looks for a newer release." : "Contacts GitHub without sending personal data. It only looks for a newer release."}
        >
          <Switch id="pref-updates" checked={a?.checkForUpdates === "weekly"} disabled={!a || busy} onCheckedChange={(on) => a && saveApp.mutate({ ...a, checkForUpdates: on ? "weekly" : "never" })} />
        </SettingRow>
      </div>
    </Panel>
  );
}

/** Step 5: a summary of what is connected, optional preferences (all off), and the way into the app. */
export function ReadyStep({ ob, finishing, onFinish }: { ob: Onboarding; finishing: boolean; onFinish: () => void }) {
  const { profile, status } = useActiveConnection();
  const name = profile?.name ?? ob.selected?.name ?? "your engine";
  return (
    <StepFrame
      title="You are ready"
      lede="Optional preferences. They are off until you turn them on, and all of them are in Settings later."
      secondary={
        <Button variant="ghost" onClick={() => ob.dispatch({ type: "back" })} disabled={finishing}>
          Back
        </Button>
      }
      actions={
        <Button data-primary onClick={onFinish} disabled={finishing}>
          {finishing && <Loader2 className="animate-spin" />}
          Open Docklet
        </Button>
      }
    >
      <div className="space-y-4">
        <Panel className="flex items-center gap-3 border-success/30 bg-success/10 p-4">
          <span className="flex size-8 shrink-0 items-center justify-center rounded-full bg-success/20 text-success">
            <Check className="size-4" strokeWidth={3} />
          </span>
          <p className="min-w-0 text-sm font-medium break-words">
            Connected to {name}
            {status?.serverVersion ? ` · ${engineVersionLabel(status.kind, status.serverVersion)}` : ""}
          </p>
        </Panel>
        <Preferences />
      </div>
    </StepFrame>
  );
}
