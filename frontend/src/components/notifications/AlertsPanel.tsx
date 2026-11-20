import { AlertTriangle, BellOff, XCircle } from "lucide-react";
import { useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { NotificationService } from "@bindings/services";
import type { Alert } from "@bindings/model";
import { Button } from "@/components/ui/button";
import { notificationSettingsKey, useAlertActions, useAlerts, useOpenAlert } from "@/hooks/useNotifications";
import { toastError } from "@/lib/errors";
import { cn } from "@/lib/utils";

function ago(at: number): string {
  const s = Math.max(0, Math.floor((Date.now() - at) / 1000));
  if (s < 60) return "just now";
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ago`;
  const h = Math.floor(m / 60);
  return h < 24 ? `${h}h ago` : `${Math.floor(h / 24)}d ago`;
}

function AlertRow({ alert, onOpen, onMute }: { alert: Alert; onOpen: () => void; onMute: () => void }) {
  const Icon = alert.severity === "error" ? XCircle : AlertTriangle;
  const canOpen = !!alert.containerId;
  return (
    <li className="flex items-start gap-2 border-b border-border/60 px-3 py-2.5 last:border-b-0">
      <Icon className={cn("mt-0.5 size-4 shrink-0", alert.severity === "error" ? "text-destructive" : "text-warning")} aria-hidden />
      <button
        type="button"
        onClick={onOpen}
        disabled={!canOpen}
        className="min-w-0 flex-1 text-left disabled:cursor-default"
        title={canOpen ? "Open the container logs" : undefined}
      >
        <span className="flex items-center gap-2">
          <span className={cn("truncate text-[13px]", !alert.read && "font-semibold")}>
            {alert.title}
            {alert.containerName ? `: ${alert.containerName}` : ""}
          </span>
          {!alert.read && <span className="size-1.5 shrink-0 rounded-full bg-primary" aria-label="Unread" />}
        </span>
        <span className="block break-words text-xs text-muted-foreground">{alert.detail}</span>
        <span className="block truncate text-xs text-muted-foreground">
          {[alert.connectionName, alert.composeProject, ago(alert.at)].filter(Boolean).join(" · ")}
        </span>
      </button>
      {canOpen && (
        <Button size="icon-sm" variant="ghost" title="Mute alerts for this container" aria-label={`Mute alerts for ${alert.containerName}`} onClick={onMute}>
          <BellOff className="size-3.5" />
        </Button>
      )}
    </li>
  );
}

/** The alerts list shown in the top-bar Alerts popover. */
export function AlertsPanel() {
  const { alerts, isLoading, error, unreadCount } = useAlerts();
  const { markAllRead, clear } = useAlertActions();
  const open = useOpenAlert();
  const qc = useQueryClient();

  const mute = async (a: Alert) => {
    try {
      await NotificationService.MuteContainer(a.connectionId, a.containerName);
      void qc.invalidateQueries({ queryKey: notificationSettingsKey });
      toast.success(`Muted alerts for ${a.containerName}`, { description: "Unmute it in Settings > Notifications." });
    } catch (e) {
      toastError("Could not mute the container", e);
    }
  };

  return (
    <div>
      <div className="flex items-center justify-between border-b border-border px-3 py-1.5">
        <span className="text-xs text-muted-foreground">{unreadCount > 0 ? `${unreadCount} unread` : "No unread alerts"}</span>
        <div className="flex gap-1">
          <Button size="sm" variant="ghost" className="h-7 text-xs" disabled={unreadCount === 0} onClick={() => void markAllRead()}>
            Mark all read
          </Button>
          <Button size="sm" variant="ghost" className="h-7 text-xs" disabled={alerts.length === 0} onClick={() => void clear()}>
            Clear
          </Button>
        </div>
      </div>
      <div className="max-h-[360px] overflow-auto">
        {error ? (
          <p className="p-4 text-sm text-destructive">Could not load alerts: {String(error)}</p>
        ) : isLoading ? (
          <p className="p-4 text-sm text-muted-foreground">Loading…</p>
        ) : alerts.length === 0 ? (
          <p className="p-4 text-sm text-muted-foreground">
            No alerts. Crashes, out-of-memory kills, unhealthy containers and restart loops appear here once container alerts are turned on in Settings.
          </p>
        ) : (
          <ul>
            {alerts.map((a) => (
              <AlertRow key={a.id} alert={a} onOpen={() => void open(a)} onMute={() => void mute(a)} />
            ))}
          </ul>
        )}
      </div>
    </div>
  );
}

