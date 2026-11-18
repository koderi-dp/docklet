import { useEffect, useRef } from "react";
import { Outlet } from "react-router-dom";
import SidebarNav from "./SidebarNav";
import TopBar from "./TopBar";
import StatusBar from "./StatusBar";
import { useConnections, useConnectionStatusEvents, useSelectConnection } from "@/hooks/useActiveConnection";
import { useUI } from "@/stores/uiStore";
import { ConnectionDialogHost } from "@/components/connections/ConnectionDialog";
import { RemoveConnectionDialogHost } from "@/components/connections/RemoveConnectionDialog";
import { PortGuardHost } from "@/components/ports/portGuard";
import { CommandPalette } from "@/components/palette/CommandPalette";
import { useTrayEvents } from "@/hooks/useApp";
import { useGlobalShortcuts } from "@/components/palette/useGlobalShortcuts";
import { useSystemTheme } from "@/hooks/useSystemTheme";

export default function AppShell() {
  useConnectionStatusEvents();
  useSystemTheme();
  useGlobalShortcuts();
  useTrayEvents();
  const { data: conns } = useConnections();
  const select = useSelectConnection();
  const setActive = useUI((s) => s.setActiveConnection);
  const booted = useRef(false);

  // Startup: resume the remembered engine (when "reconnect on startup" is on) or, on first run,
  // the discovered default. Runs once; later list changes must never reconnect anything.
  useEffect(() => {
    if (!conns || booted.current) return;
    booted.current = true;
    const { activeConnectionId: active, reconnectOnStartup } = useUI.getState();
    const known = !!active && conns.some((c) => c.id === active);
    if (!reconnectOnStartup) {
      // Keep the remembered selection visible but do not connect; an unknown one is cleared.
      if (active && !known) setActive(null);
      return;
    }
    if (known) void select(active as string);
    else if (conns.length) void select(conns[0].id);
    else setActive(null);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [conns]);

  // A removed/renamed-away active connection must not linger as a phantom selection.
  const active = useUI((s) => s.activeConnectionId);
  useEffect(() => {
    if (conns && booted.current && active && !conns.some((c) => c.id === active)) setActive(null);
  }, [conns, active, setActive]);

  return (
    <div className="flex h-screen w-screen overflow-hidden bg-background">
      <SidebarNav />
      <div className="flex min-w-0 flex-1 flex-col">
        <TopBar />
        <main className="min-h-0 min-w-0 flex-1 overflow-auto">
          <Outlet />
        </main>
        <StatusBar />
      </div>
      <ConnectionDialogHost />
      <RemoveConnectionDialogHost />
      <PortGuardHost />
      <CommandPalette />
    </div>
  );
}
