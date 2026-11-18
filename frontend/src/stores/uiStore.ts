import { create } from "zustand";
import { persist } from "zustand/middleware";
import type { ConnectionProfile } from "@bindings/model";
import { DEFAULT_TEXT_SIZE, normalizeTextSize, type TextSize } from "@/lib/textSize";

export type ContainerViewMode = "grouped" | "flat";
export type CopyAsShell = "bash" | "powershell";
export type ThemePreference = "dark" | "light" | "system";
/** Colour palette: the Docklet navy/violet design palette or the stock shadcn/ui neutral base. */
export type ThemePalette = "docklet" | "shadcn";

/** Sort state of one table (see lib/sorting). Absent = the table's default order. */
export interface TableSortState {
  id: string;
  desc: boolean;
}

/** Terminal used for "Open in external terminal": auto-detect, or a custom template containing {cmd}. */
export interface ExternalTerminalSetting {
  mode: "auto" | "custom";
  template: string;
}

/** Editor used for "Open in editor": auto-detect, a named one, or a custom template containing {path}. */
export interface EditorSetting {
  mode: "auto" | "vscode" | "cursor" | "custom";
  template: string;
}

/** Add/Edit connection dialog request (not persisted). */
export interface ConnectionDialogRequest {
  mode: "add" | "edit";
  /** Profile id when editing. */
  id?: string;
  /** Called with the saved profile. When set, the dialog does not select or connect it itself (the setup wizard decides). */
  onSaved?: (profile: ConnectionProfile) => void;
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

  /** Whole-window zoom in percent (see lib/textSize); applied natively by lib/textZoom. */
  textSize: TextSize;
  setTextSize: (n: number) => void;

  /** Terminal font size in px (read by terminal views). */
  terminalFontSize: number;
  setTerminalFontSize: (n: number) => void;
  /** Default number of log lines loaded when opening logs. */
  logTail: number;
  setLogTail: (n: number) => void;
  /** Reconnect the last used engine when the app starts. */
  reconnectOnStartup: boolean;
  setReconnectOnStartup: (v: boolean) => void;

  /** Tab shown in the activity center (not persisted); the tray can ask for "alerts". */
  activityTab: "operations" | "alerts";
  setActivityTab: (t: "operations" | "alerts") => void;

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

  /** Per-table sort, keyed by table id (e.g. "containers", "images"). */
  tableSort: Record<string, TableSortState>;
  setTableSort: (table: string, sort: TableSortState) => void;
  externalTerminal: ExternalTerminalSetting;
  setExternalTerminal: (v: ExternalTerminalSetting) => void;
  editor: EditorSetting;
  setEditor: (v: EditorSetting) => void;
  /** Toolbox image of "Debug shell" for containers without a shell. */
  debugShellImage: string;
  setDebugShellImage: (v: string) => void;
  /** Skip the confirm panel before opening a debug shell. */
  /** Shell flavour of the generated "Copy as docker run command" text. */
  copyAsShell: CopyAsShell;
  setCopyAsShell: (v: CopyAsShell) => void;
  debugShellSkipConfirm: boolean;
  setDebugShellSkipConfirm: (v: boolean) => void;
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
      textSize: DEFAULT_TEXT_SIZE,
      setTextSize: (n) => set({ textSize: normalizeTextSize(n) }),
      terminalFontSize: 13,
      setTerminalFontSize: (n) => set({ terminalFontSize: Math.min(24, Math.max(10, Math.round(n))) }),
      logTail: 500,
      setLogTail: (logTail) => set({ logTail }),
      reconnectOnStartup: true,
      setReconnectOnStartup: (reconnectOnStartup) => set({ reconnectOnStartup }),
      activityTab: "operations",
      setActivityTab: (activityTab) => set({ activityTab }),
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
      tableSort: {},
      setTableSort: (table, sort) => set((s) => ({ tableSort: { ...s.tableSort, [table]: sort } })),
      externalTerminal: { mode: "auto", template: "" },
      setExternalTerminal: (externalTerminal) => set({ externalTerminal }),
      editor: { mode: "auto", template: "" },
      setEditor: (editor) => set({ editor }),
      debugShellImage: "busybox:latest",
      setDebugShellImage: (debugShellImage) => set({ debugShellImage }),
      copyAsShell: typeof navigator !== "undefined" && /Windows/i.test(navigator.userAgent) ? "powershell" : "bash",
      setCopyAsShell: (copyAsShell) => set({ copyAsShell }),
      debugShellSkipConfirm: false,
      setDebugShellSkipConfirm: (debugShellSkipConfirm) => set({ debugShellSkipConfirm }),
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
        textSize: s.textSize,
        terminalFontSize: s.terminalFontSize,
        logTail: s.logTail,
        reconnectOnStartup: s.reconnectOnStartup,
        containerView: s.containerView,
        collapsedGroups: s.collapsedGroups,
        inspectorWidth: s.inspectorWidth,
        tableSort: s.tableSort,
        externalTerminal: s.externalTerminal,
        editor: s.editor,
        debugShellImage: s.debugShellImage,
        copyAsShell: s.copyAsShell,
        debugShellSkipConfirm: s.debugShellSkipConfirm,
      }),
    },
  ),
);
