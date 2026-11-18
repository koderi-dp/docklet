import { Browser } from "@wailsio/runtime";
import { Loader2 } from "lucide-react";
import type { UpdateStatus } from "@bindings/model";
import { Button } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { SettingRow } from "@/components/settings/SettingRow";
import { useAppSettings, useAppVersion, useUpdateStatus } from "@/hooks/useApp";
import { toastError } from "@/lib/errors";

const SCHEDULES = [
  { value: "never", label: "Never" },
  { value: "startup", label: "On startup" },
  { value: "weekly", label: "Weekly" },
];

function openReleasePage(url: string) {
  Browser.OpenURL(url).catch((e) => toastError("Could not open the browser", e));
}

function resultText(st: UpdateStatus | undefined): { text: string; tone: "muted" | "warning" } | null {
  if (!st || !st.checkedAt) return null;
  if (st.error) return { text: st.error, tone: "warning" };
  if (st.message) return { text: st.message, tone: "muted" };
  if (st.updateAvailable) return { text: `Version ${st.latestVersion} is available.`, tone: "muted" };
  if (st.latestVersion) return { text: `You are up to date (latest release: ${st.latestVersion}).`, tone: "muted" };
  return null;
}

/** Settings > About and updates. Checking only reports; Docklet never downloads or installs anything. */
export function AboutSection() {
  const { data: version } = useAppVersion();
  const { query: settings, save } = useAppSettings();
  const { query, checkNow, dismiss } = useUpdateStatus();
  const st = checkNow.data ?? query.data;
  const result = resultText(st);
  const checked = st?.checkedAt ? new Date(st.checkedAt).toLocaleString() : "never";
  return (
    <section className="rounded-lg border bg-card">
      <div className="border-b px-4 py-3">
        <h2 className="text-sm font-semibold">About and updates</h2>
        <p className="text-xs text-muted-foreground">Desktop client for existing Docker Engines.</p>
      </div>
      <div className="divide-y">
        <SettingRow label="Version" hint="Update checks only look for a newer release. Docklet never downloads or installs anything by itself.">
          <span className="text-sm tabular-nums text-muted-foreground">{version ?? "—"}</span>
        </SettingRow>
        <SettingRow label="Check for updates" htmlFor="update-schedule" hint="Contacts GitHub (api.github.com) without sending any personal data. Off by default.">
          <Select
            value={settings.data?.checkForUpdates ?? "never"}
            disabled={!settings.data || save.isPending}
            onValueChange={(checkForUpdates) => settings.data && save.mutate({ ...settings.data, checkForUpdates })}
          >
            <SelectTrigger id="update-schedule" className="w-32" size="sm">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {SCHEDULES.map((o) => (
                <SelectItem key={o.value} value={o.value}>
                  {o.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </SettingRow>
        <SettingRow label="Check now" hint={`Last checked: ${checked}`}>
          <Button variant="outline" size="sm" disabled={checkNow.isPending} onClick={() => checkNow.mutate()}>
            {checkNow.isPending && <Loader2 className="size-3.5 animate-spin" />}
            Check now
          </Button>
        </SettingRow>
        {result && (
          <div className="space-y-2 px-4 py-3 text-sm" role="status">
            <p className={result.tone === "warning" ? "text-warning" : "text-muted-foreground"}>{result.text}</p>
            {st?.updateAvailable && (
              <div className="flex flex-wrap gap-2">
                {st.releaseUrl && (
                  <Button variant="outline" size="sm" onClick={() => openReleasePage(st.releaseUrl)}>
                    Release notes
                  </Button>
                )}
                {!st.dismissed && (
                  <Button variant="ghost" size="sm" disabled={dismiss.isPending} onClick={() => dismiss.mutate(st.latestVersion)}>
                    Dismiss this version
                  </Button>
                )}
              </div>
            )}
            {st?.updateAvailable && st.releaseNotes && <p className="max-h-40 overflow-auto whitespace-pre-wrap text-xs text-muted-foreground">{st.releaseNotes}</p>}
          </div>
        )}
      </div>
    </section>
  );
}
