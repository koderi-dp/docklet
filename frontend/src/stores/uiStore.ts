import { create } from "zustand";
import { persist } from "zustand/middleware";

export type ContainerViewMode = "grouped" | "flat";
export type ThemePreference = "dark" | "light" | "system";
/** Colour palette: the Docklet navy/violet design palette or the stock shadcn/ui neutral base. */
export type ThemePalette = "docklet" | "shadcn";

/** Add/Edit connection dialog request (not persisted). */
export interface ConnectionDialogRequest {
  mode: "add" | "edit";
  /** Profile id when editing. */
  id?: string;
}

interface UIState {
  /** Explicit active Docker connection; null = none selected. Never inferred. */
  activeConnectionId: string | null;
  /** Bumped on every engine switch; use in query keys / effects to drop stale work. */
  engineEpoch: number;
  setActiveConnection: (id: string | null) => void;

  sidebarCollapsed: boolean;
  toggleSidebar: () => void;
  /** True once the user explicitly collapsed/expanded the sidebar; stops width-based auto-collapse. */
  sidebarPinned: boolean;
  /** Explicit user choice: stores the collapsed state and pins it. */
  setSidebarCollapsed: (collapsed: boolean) => void;

  /** Resolved theme ("system" already resolved); App.tsx toggles the `dark` class from this. */
  theme: "dark" | "light";
  setTheme: (t: "dark" | "light") => void;
  /** What the user chose; "system" follows the OS (resolved into `theme` by AppShell). */
  themePreference: ThemePreference;
  setThemePreference: (p: ThemePreference) => void;
  themePalette: ThemePalette;
  setThemePalette: (p: ThemePalette) => void;

  /** Terminal font size in px (read by terminal views). */
  terminalFontSize: number;
  setTerminalFontSize: (n: number) => void;
  /** Default number of log lines loaded when opening logs. */
  logTail: number;
  setLogTail: (n: number) => void;
  /** Reconnect the last used engine when the app starts. */
  reconnectOnStartup: boolean;
  setReconnectOnStartup: (v: boolean) => void;

  /** Global command palette (not persisted). */
  paletteOpen: boolean;
  setPaletteOpen: (open: boolean) => void;
  /** Add/Edit connection dialog (not persisted). */
  connectionDialog: ConnectionDialogRequest | null;
  openConnectionDialog: (r: ConnectionDialogRequest | null) => void;
  /** Connection id awaiting remove confirmation (not persisted). */
  removeConnectionId: string | null;
  setRemoveConnectionId: (id: string | null) => void;

  containerView: ContainerViewMode;
  setContainerView: (v: ContainerViewMode) => void;
  /** Collapsed Compose groups, keyed `${connectionId}::${project}`. */
  collapsedGroups: Record<string, boolean>;
  toggleGroup: (key: string) => void;
  setAllGroups: (keys: string[], collapsed: boolean) => void;
  inspectorWidth: number;
  setInspectorWidth: (w: number) => void;
}

export const useUI = create<UIState>()(
  persist(
    (set) => ({
      activeConnectionId: null,
      engineEpoch: 0,
      setActiveConnection: (id) =>
        set((s) =>
          s.activeConnectionId === id
            ? s
            : { activeConnectionId: id, engineEpoch: s.engineEpoch + 1 },
        ),
      sidebarCollapsed: false,
      toggleSidebar: () => set((s) => ({ sidebarCollapsed: !s.sidebarCollapsed, sidebarPinned: true })),
      sidebarPinned: false,
      setSidebarCollapsed: (sidebarCollapsed) => set({ sidebarCollapsed, sidebarPinned: true }),
      theme: "dark",
      setTheme: (theme) => set({ theme, themePreference: theme }),
      themePreference: "dark",
      setThemePreference: (themePreference) =>
        set(themePreference === "system" ? { themePreference } : { themePreference, theme: themePreference }),
      themePalette: "docklet",
      setThemePalette: (themePalette) => set({ themePalette }),
      terminalFontSize: 13,
      setTerminalFontSize: (n) => set({ terminalFontSize: Math.min(24, Math.max(10, Math.round(n))) }),
      logTail: 500,
      setLogTail: (logTail) => set({ logTail }),
      reconnectOnStartup: true,
      setReconnectOnStartup: (reconnectOnStartup) => set({ reconnectOnStartup }),
      paletteOpen: false,
      setPaletteOpen: (paletteOpen) => set({ paletteOpen }),
      connectionDialog: null,
      openConnectionDialog: (connectionDialog) => set({ connectionDialog }),
      removeConnectionId: null,
      setRemoveConnectionId: (removeConnectionId) => set({ removeConnectionId }),
      containerView: "grouped",
      setContainerView: (containerView) => set({ containerView }),
      collapsedGroups: {},
      toggleGroup: (key) =>
        set((s) => ({ collapsedGroups: { ...s.collapsedGroups, [key]: !s.collapsedGroups[key] } })),
      setAllGroups: (keys, collapsed) =>
        set((s) => ({
          collapsedGroups: { ...s.collapsedGroups, ...Object.fromEntries(keys.map((k) => [k, collapsed])) },
        })),
      inspectorWidth: 380,
      setInspectorWidth: (inspectorWidth) => set({ inspectorWidth: Math.min(500, Math.max(320, inspectorWidth)) }),
    }),
    {
      name: "docklet-ui",
      partialize: (s) => ({
        activeConnectionId: s.activeConnectionId,
        sidebarCollapsed: s.sidebarCollapsed,
        sidebarPinned: s.sidebarPinned,
        theme: s.theme,
        themePreference: s.themePreference,
        themePalette: s.themePalette,
        terminalFontSize: s.terminalFontSize,
        logTail: s.logTail,
        reconnectOnStartup: s.reconnectOnStartup,
        containerView: s.containerView,
        collapsedGroups: s.collapsedGroups,
        inspectorWidth: s.inspectorWidth,
      }),
    },
  ),
);
