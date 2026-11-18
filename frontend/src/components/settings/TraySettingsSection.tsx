import { Switch } from "@/components/ui/switch";
import { SettingRow } from "@/components/settings/SettingRow";
import { useAppSettings } from "@/hooks/useApp";

/** Settings > Tray and background: everything is off by default and applies immediately. */
export function TraySettingsSection() {
  const { query, save } = useAppSettings();
  const s = query.data;
  const update = (patch: Partial<NonNullable<typeof s>>) => s && save.mutate({ ...s, ...patch });
  return (
    <section className="rounded-lg border bg-card">
      <div className="border-b px-4 py-3">
        <h2 className="text-sm font-semibold">Tray and background</h2>
        <p className="text-xs text-muted-foreground">Keep Docklet running out of the way. Off by default; changes apply immediately.</p>
      </div>
      {query.isError ? (
        <p className="px-4 py-3 text-sm text-destructive">Could not load settings: {String(query.error)}</p>
      ) : !s ? (
        <p className="px-4 py-3 text-sm text-muted-foreground">Loading…</p>
      ) : (
        <div className="divide-y">
          <SettingRow htmlFor="tray-show" label="Show tray icon" hint="Adds a Docklet icon to the system tray. Click it to show or hide the window; right-click for engines, running containers and Quit.">
            <Switch id="tray-show" checked={s.showTrayIcon} disabled={save.isPending} onCheckedChange={(showTrayIcon) => update({ showTrayIcon })} />
          </SettingRow>
          <SettingRow
            htmlFor="tray-close"
            label="Closing the window keeps Docklet running"
            hint={s.showTrayIcon ? "The close button hides the window to the tray, so alerts and background operations continue. Use Quit in the tray menu to exit." : "Needs the tray icon. Without it, closing the window quits Docklet."}
          >
            <Switch id="tray-close" checked={s.closeToTray} disabled={!s.showTrayIcon || save.isPending} onCheckedChange={(closeToTray) => update({ closeToTray })} />
          </SettingRow>
          <SettingRow
            htmlFor="tray-start"
            label="Start hidden in the tray"
            hint={s.showTrayIcon ? "Docklet starts without opening its window. Takes effect the next time Docklet starts." : "Needs the tray icon."}
          >
            <Switch id="tray-start" checked={s.startMinimizedToTray} disabled={!s.showTrayIcon || save.isPending} onCheckedChange={(startMinimizedToTray) => update({ startMinimizedToTray })} />
          </SettingRow>
        </div>
      )}
    </section>
  );
}
