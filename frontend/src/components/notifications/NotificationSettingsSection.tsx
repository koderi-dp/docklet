import { BellOff } from "lucide-react";
import type { AlertRules, NotificationSettings } from "@bindings/model";
import { Button } from "@/components/ui/button";
import { SettingRow } from "@/components/settings/SettingRow";
import { Switch } from "@/components/ui/switch";
import { useConnections } from "@/hooks/useActiveConnection";
import { useNotificationSettings } from "@/hooks/useNotifications";
import { useUI } from "@/stores/uiStore";

const RULES: { key: keyof AlertRules; label: string; hint: string }[] = [
  { key: "crash", label: "Container crashed", hint: "Exited with a non-zero code. Stops you start yourself are ignored." },
  { key: "oom", label: "Out of memory", hint: "The container was killed for exceeding its memory limit." },
  { key: "unhealthy", label: "Became unhealthy", hint: "Its health check started failing." },
  { key: "restartLoop", label: "Restart loop", hint: "Started 3 or more times within 2 minutes." },
];

/** Settings > Notifications: opt-in container health alerts. */
export function NotificationSettingsSection() {
  const { query, save, unmute, test } = useNotificationSettings();
  const { data: conns } = useConnections();
  const activeId = useUI((s) => s.activeConnectionId);
  const s = query.data;
  const nameOf = (id: string) => conns?.find((c) => c.id === id)?.name ?? id;
  const update = (patch: Partial<NotificationSettings>) => s && save.mutate({ ...s, ...patch });
  const muted = s?.mutedContainers ?? [];

  return (
    <section className="rounded-lg border bg-card">
      <div className="border-b px-4 py-3">
        <h2 className="text-sm font-semibold">Notifications</h2>
        <p className="text-xs text-muted-foreground">Alerts for containers on connected engines. Off by default.</p>
      </div>
      {query.isError ? (
        <p className="px-4 py-3 text-sm text-destructive">Could not load notification settings: {String(query.error)}</p>
      ) : !s ? (
        <p className="px-4 py-3 text-sm text-muted-foreground">Loading…</p>
      ) : (
        <div className="divide-y">
          <SettingRow htmlFor="alerts-on" label="Container alerts" hint="Tell me when a container crashes, runs out of memory, turns unhealthy or keeps restarting. Works only while Docklet is running.">
            <Switch id="alerts-on" checked={s.enabled} disabled={save.isPending} onCheckedChange={(enabled) => update({ enabled })} />
          </SettingRow>
          {s.enabled &&
            RULES.map((r) => (
              <SettingRow key={r.key} htmlFor={`rule-${r.key}`} label={r.label} hint={r.hint}>
                <Switch
                  id={`rule-${r.key}`}
                  checked={s.rules[r.key]}
                  disabled={save.isPending}
                  onCheckedChange={(v) => update({ rules: { ...s.rules, [r.key]: v } })}
                />
              </SettingRow>
            ))}
          {s.enabled && (
            <SettingRow
              htmlFor="alerts-os"
              label="Also show OS notifications"
              hint={s.osAvailable ? "Shows a desktop notification in addition to the in-app alert." : "Not available: this build has no operating system notification support."}
            >
              <Switch
                id="alerts-os"
                checked={s.osNotifications && s.osAvailable}
                disabled={!s.osAvailable || save.isPending}
                title={s.osAvailable ? undefined : "No operating system notification support is wired in"}
                onCheckedChange={(osNotifications) => update({ osNotifications })}
              />
            </SettingRow>
          )}
          {s.enabled && (
            <SettingRow label="Test" hint="Records a sample alert and sends a sample OS notification when enabled.">
              <Button variant="outline" size="sm" disabled={test.isPending || !activeId} title={activeId ? undefined : "Select a connection first"} onClick={() => activeId && test.mutate(activeId)}>
                Send test notification
              </Button>
            </SettingRow>
          )}
          {muted.length > 0 && (
            <div className="px-4 py-3">
              <h3 className="text-sm font-medium">Muted containers</h3>
              <ul className="mt-2 space-y-1">
                {muted.map((m) => (
                  <li key={`${m.connectionId}/${m.containerName}`} className="flex items-center justify-between gap-2 text-sm">
                    <span className="flex min-w-0 items-center gap-2">
                      <BellOff className="size-3.5 shrink-0 text-muted-foreground" aria-hidden />
                      <span className="truncate">{m.containerName}</span>
                      <span className="truncate text-xs text-muted-foreground">{nameOf(m.connectionId)}</span>
                    </span>
                    <Button size="sm" variant="ghost" disabled={unmute.isPending} onClick={() => unmute.mutate(m)}>
                      Unmute
                    </Button>
                  </li>
                ))}
              </ul>
            </div>
          )}
        </div>
      )}
    </section>
  );
}
