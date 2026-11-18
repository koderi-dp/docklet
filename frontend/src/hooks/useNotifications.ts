import { useEffect } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Events } from "@wailsio/runtime";
import { useNavigate } from "react-router-dom";
import { toast } from "sonner";
import { NotificationService } from "@bindings/services";
import type { Alert, NotificationSettings } from "@bindings/model";
import { useUI } from "@/stores/uiStore";
import { toastError } from "@/lib/errors";
import { useSelectConnection } from "@/hooks/useActiveConnection";

export const alertsKey = ["alerts"] as const;
export const notificationSettingsKey = ["notification-settings"] as const;

export function useAlerts() {
  const query = useQuery({ queryKey: alertsKey, queryFn: async () => (await NotificationService.List(200)) ?? [], retry: false });
  const alerts = query.data ?? [];
  return { ...query, alerts, unreadCount: alerts.filter((a) => !a.read).length };
}

/** Opens the container behind an alert on its Logs tab, switching the active connection first. */
export function useOpenAlert() {
  const navigate = useNavigate();
  const select = useSelectConnection();
  const qc = useQueryClient();
  return async (a: Alert) => {
    if (!a.read) {
      void NotificationService.MarkRead([a.id]).then(() => qc.invalidateQueries({ queryKey: alertsKey }));
    }
    if (!a.containerId) return;
    if (useUI.getState().activeConnectionId !== a.connectionId) await select(a.connectionId);
    navigate(`/containers/${a.containerId}?tab=logs`);
  };
}

/** Refreshes the alert list on new alerts and toasts those of the active connection. Mount once. */
export function useAlertEvents() {
  const qc = useQueryClient();
  const open = useOpenAlert();
  useEffect(() => {
    const off = Events.On("alerts:new", (ev: { data?: unknown }) => {
      const raw = Array.isArray(ev.data) ? ev.data[0] : ev.data;
      const alert = (raw as { alert?: Alert } | undefined)?.alert;
      void qc.invalidateQueries({ queryKey: alertsKey });
      if (!alert || alert.connectionId !== useUI.getState().activeConnectionId) return;
      toast.warning(`${alert.title}${alert.containerName ? `: ${alert.containerName}` : ""}`, {
        description: alert.detail,
        action: alert.containerId ? { label: "View logs", onClick: () => void open(alert) } : undefined,
      });
    });
    return () => off();
  }, [qc, open]);
}

export function useAlertActions() {
  const qc = useQueryClient();
  const refresh = () => qc.invalidateQueries({ queryKey: alertsKey });
  const run = (fn: () => Promise<unknown>, title: string) => fn().then(refresh, (e) => toastError(title, e));
  return {
    markAllRead: () => run(() => NotificationService.MarkAllRead(), "Could not mark alerts as read"),
    clear: () => run(() => NotificationService.Clear(), "Could not clear alerts"),
  };
}

export function useNotificationSettings() {
  const qc = useQueryClient();
  const query = useQuery({ queryKey: notificationSettingsKey, queryFn: () => NotificationService.GetSettings(), retry: false });
  const apply = (s: NotificationSettings) => qc.setQueryData(notificationSettingsKey, s);
  const save = useMutation({
    mutationFn: (s: NotificationSettings) => NotificationService.SaveSettings(s),
    onSuccess: apply,
    onError: (e) => toastError("Could not save notification settings", e),
  });
  const unmute = useMutation({
    mutationFn: (v: { connectionId: string; containerName: string }) => NotificationService.UnmuteContainer(v.connectionId, v.containerName),
    onSuccess: apply,
    onError: (e) => toastError("Could not unmute the container", e),
  });
  const test = useMutation({
    mutationFn: (connectionId: string) => NotificationService.TestNotification(connectionId),
    onError: (e) => toastError("Test notification failed", e),
  });
  return { query, save, unmute, test };
}
