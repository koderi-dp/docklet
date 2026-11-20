import { useEffect } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Events } from "@wailsio/runtime";
import { useNavigate } from "react-router-dom";
import { AppService } from "@bindings/services";
import type { AppSettings, TrayOpen, TraySelectConnection, UpdateStatus } from "@bindings/model";
import { toastError } from "@/lib/errors";
import { useSelectConnection } from "@/hooks/useActiveConnection";
import { useUI } from "@/stores/uiStore";

export const appSettingsKey = ["app-settings"] as const;
export const updateStatusKey = ["update-status"] as const;
const versionKey = ["app-version"] as const;

export function useAppVersion() {
  return useQuery({ queryKey: versionKey, queryFn: () => AppService.GetVersion(), staleTime: Infinity, retry: false });
}

/** Tray and update preferences (all off until the user opts in). Saving applies the tray at once. */
export function useAppSettings() {
  const qc = useQueryClient();
  const query = useQuery({ queryKey: appSettingsKey, queryFn: () => AppService.GetAppSettings(), retry: false });
  const save = useMutation({
    mutationFn: (s: AppSettings) => AppService.SaveAppSettings(s),
    onSuccess: (s) => qc.setQueryData(appSettingsKey, s),
    onError: (e) => toastError("Could not save settings", e),
  });
  return { query, save };
}

/** Last known update status; refreshed when the backend announces a new release. */
export function useUpdateStatus() {
  const qc = useQueryClient();
  const query = useQuery({ queryKey: updateStatusKey, queryFn: () => AppService.GetUpdateStatus(), retry: false });
  useEffect(() => {
    const off = Events.On("app:update", () => void qc.invalidateQueries({ queryKey: updateStatusKey }));
    return () => off();
  }, [qc]);
  const checkNow = useMutation({
    mutationFn: () => AppService.CheckForUpdate(),
    onSuccess: (st: UpdateStatus) => qc.setQueryData(updateStatusKey, st),
    onError: (e) => toastError("Could not check for updates", e),
  });
  const dismiss = useMutation({
    mutationFn: (version: string) => AppService.DismissUpdate(version),
    onSuccess: () => qc.invalidateQueries({ queryKey: updateStatusKey }),
    onError: (e) => toastError("Could not dismiss the update", e),
  });
  return { query, checkNow, dismiss };
}

/** Handles the tray's requests (select an engine, open alerts or a container) and reports the active engine. Mount once. */
export function useTrayEvents() {
  const select = useSelectConnection();
  const navigate = useNavigate();
  const active = useUI((s) => s.activeConnectionId);
  useEffect(() => {
    void AppService.SetActiveConnection(active ?? "").catch(() => undefined);
  }, [active]);
  useEffect(() => {
    const data = <T,>(ev: { data?: unknown }) => (Array.isArray(ev.data) ? ev.data[0] : ev.data) as T | undefined;
    const offSelect = Events.On("tray:select-connection", (ev: { data?: unknown }) => {
      const d = data<TraySelectConnection>(ev);
      if (d?.connectionId) void select(d.connectionId);
    });
    const offOpen = Events.On("tray:open", (ev: { data?: unknown }) => {
      const d = data<TrayOpen>(ev);
      if (d?.target === "alerts") {
        useUI.getState().setAlertsOpen(true);
      } else if (d?.target === "container" && d.containerId) {
        void (async () => {
          if (useUI.getState().activeConnectionId !== d.connectionId) await select(d.connectionId);
          navigate(`/containers/${d.containerId}`);
        })();
      }
    });
    return () => {
      offSelect();
      offOpen();
    };
  }, [select, navigate]);
}
