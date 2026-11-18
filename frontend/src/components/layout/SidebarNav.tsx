import { Logo } from "./Logo";
import { NavLink, useNavigate } from "react-router-dom";
import {
  Box,
  Container,
  Copy,
  Database,
  Layers3,
  LayoutDashboard,
  Network,
  PanelLeft,
  Pencil,
  Plug,
  Plus,
  ScrollText,
  ServerCog,
  Settings,
  Trash2,
  Unplug,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { useUI } from "@/stores/uiStore";
import { useActiveConnection, useConnections, useConnectionStatus } from "@/hooks/useActiveConnection";
import { ConnectionDot } from "@/components/connections/ConnectionStatus";
import { TypeIcon, stateText } from "@/components/connections/connectionMeta";
import { useConnectionActions } from "@/components/connections/useConnectionActions";
import { Button } from "@/components/ui/button";
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuTrigger,
} from "@/components/ui/context-menu";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { ConnectionState, type ConnectionProfile } from "@bindings/model";
import { useSidebarCollapsed } from "./useSidebarState";

const NAV = [
  { to: "/dashboard", label: "Dashboard", icon: LayoutDashboard },
  { to: "/containers", label: "Containers", icon: Container },
  { to: "/images", label: "Images", icon: Box },
  { to: "/compose", label: "Compose", icon: Layers3 },
  { to: "/logs", label: "Logs", icon: ScrollText },
  { to: "/volumes", label: "Volumes", icon: Database },
  { to: "/networks", label: "Networks", icon: Network },
  { to: "/system", label: "System", icon: ServerCog },
];

/** Route selection: filled background plus an inline accent bar. */
const routeClass = (isActive: boolean, collapsed: boolean) =>
  cn(
    "relative flex items-center gap-3 rounded-md px-3 py-2 text-sm text-muted-foreground hover:bg-muted hover:text-foreground",
    isActive &&
      "bg-sidebar-accent font-medium text-foreground before:absolute before:inset-y-1.5 before:left-0 before:w-0.5 before:rounded-full before:bg-primary [&_svg]:text-primary",
    collapsed && "mx-auto size-10 justify-center px-0 py-0",
  );

function ConnectionRow({ c }: { c: ConnectionProfile }) {
  const active = useUI((s) => s.activeConnectionId) === c.id;
  const openDialog = useUI((s) => s.openConnectionDialog);
  const setRemove = useUI((s) => s.setRemoveConnectionId);
  const { data: status } = useConnectionStatus(c.id);
  const { connect, disconnect, duplicate } = useConnectionActions();
  const state = status?.state;
  const connected = state === ConnectionState.StateConnected;
  const connecting = state === ConnectionState.StateConnecting;
  const row = (
    <button
      onClick={() => void connect(c.id)}
      aria-current={active ? "true" : undefined}
      title={`${c.name}: ${stateText(state)}`}
      className={cn(
        "flex w-full items-center gap-2 rounded-md px-3 py-1.5 text-left text-[13px] text-muted-foreground hover:bg-muted hover:text-foreground",
        active && "text-foreground ring-1 ring-primary/50 ring-inset",
      )}
    >
      <ConnectionDot state={state} />
      <span className={cn("truncate", active && "font-medium")}>{c.name}</span>
    </button>
  );
  return (
    <ContextMenu>
      <ContextMenuTrigger asChild>{row}</ContextMenuTrigger>
      <ContextMenuContent className="w-48">
        {connected ? (
          <ContextMenuItem onSelect={() => void disconnect(c.id, c.name)}>
            <Unplug />
            Disconnect
          </ContextMenuItem>
        ) : (
          <ContextMenuItem disabled={connecting} onSelect={() => void connect(c.id)}>
            <Plug />
            {state === ConnectionState.StateError ? "Reconnect" : "Connect"}
          </ContextMenuItem>
        )}
        <ContextMenuItem
          disabled={c.discovered}
          title={c.discovered ? "Auto-detected connections cannot be edited" : undefined}
          onSelect={() => openDialog({ mode: "edit", id: c.id })}
        >
          <Pencil />
          Edit…
        </ContextMenuItem>
        <ContextMenuItem onSelect={() => void duplicate(c.id)}>
          <Copy />
          Duplicate
        </ContextMenuItem>
        <ContextMenuSeparator />
        <ContextMenuItem
          variant="destructive"
          disabled={c.discovered}
          title={c.discovered ? "Auto-detected connections cannot be removed" : undefined}
          onSelect={() => setRemove(c.id)}
        >
          <Trash2 />
          Remove…
        </ContextMenuItem>
      </ContextMenuContent>
    </ContextMenu>
  );
}

function ConnectionSelector() {
  const { data: conns } = useConnections();
  const { profile, status } = useActiveConnection();
  const { connect } = useConnectionActions();
  const openDialog = useUI((s) => s.openConnectionDialog);
  const navigate = useNavigate();
  return (
    <Popover>
      <Tooltip>
        <TooltipTrigger asChild>
          <PopoverTrigger asChild>
            <Button variant="ghost" size="icon" className="mx-auto" aria-label="Choose connection" title="Choose connection">
              <span className="relative">
                <Plug className="size-[18px]" />
                <ConnectionDot state={status?.state} className="absolute -right-1 -bottom-1 ring-2 ring-sidebar" />
              </span>
            </Button>
          </PopoverTrigger>
        </TooltipTrigger>
        <TooltipContent side="right">{profile ? `Connection: ${profile.name}` : "Choose a connection"}</TooltipContent>
      </Tooltip>
      <PopoverContent side="right" align="end" className="w-64 p-1.5">
        <div className="px-2 py-1 text-xs font-medium text-muted-foreground">Connections</div>
        {conns?.map((c) => (
          <PopoverRow key={c.id} c={c} onPick={() => void connect(c.id)} />
        ))}
        <div className="my-1 border-t" />
        <button
          className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-sm hover:bg-muted"
          onClick={() => openDialog({ mode: "add" })}
        >
          <Plus className="size-4" />
          Add connection…
        </button>
        <button className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-sm hover:bg-muted" onClick={() => navigate("/connections")}>
          <Plug className="size-4" />
          Manage connections
        </button>
      </PopoverContent>
    </Popover>
  );
}

function PopoverRow({ c, onPick }: { c: ConnectionProfile; onPick: () => void }) {
  const active = useUI((s) => s.activeConnectionId) === c.id;
  const { data: status } = useConnectionStatus(c.id);
  return (
    <button
      onClick={onPick}
      className={cn("flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-sm hover:bg-muted", active && "bg-muted font-medium")}
    >
      <ConnectionDot state={status?.state} />
      <TypeIcon type={c.type} className="size-3.5 text-muted-foreground" />
      <span className="truncate">{c.name}</span>
      <span className="ml-auto text-xs text-muted-foreground">{stateText(status?.state)}</span>
    </button>
  );
}

export default function SidebarNav() {
  const { collapsed, setCollapsed } = useSidebarCollapsed();
  const { data: conns } = useConnections();
  const openDialog = useUI((s) => s.openConnectionDialog);
  return (
    <aside
      className={cn(
        "flex shrink-0 flex-col border-r border-sidebar-border bg-sidebar transition-[width] duration-150",
        collapsed ? "w-16" : "w-[232px]",
      )}
    >
      <div className={cn("flex h-[60px] items-center gap-2 px-4", collapsed && "justify-center px-0")}>
        <Logo className="size-7" />
        {!collapsed && <span className="text-lg font-semibold">Docklet</span>}
        {!collapsed && (
          <Button variant="ghost" size="icon" className="ml-auto size-7" onClick={() => setCollapsed(true)} aria-label="Collapse sidebar">
            <PanelLeft className="size-4" />
          </Button>
        )}
      </div>
      {collapsed && (
        <Tooltip>
          <TooltipTrigger asChild>
            <Button variant="ghost" size="icon" className="mx-auto mb-1 size-8" onClick={() => setCollapsed(false)} aria-label="Expand sidebar">
              <PanelLeft className="size-4" />
            </Button>
          </TooltipTrigger>
          <TooltipContent side="right">Expand sidebar</TooltipContent>
        </Tooltip>
      )}
      <nav className={cn("flex flex-col px-3 pt-2", collapsed ? "gap-1.5" : "gap-0.5")}>
        {NAV.map(({ to, label, icon: Icon }) => {
          const link = (
            <NavLink to={to} className={({ isActive }) => routeClass(isActive, collapsed)}>
              <Icon className="size-[18px]" />
              {!collapsed && label}
            </NavLink>
          );
          return collapsed ? (
            <Tooltip key={to}>
              <TooltipTrigger asChild>
                <div>{link}</div>
              </TooltipTrigger>
              <TooltipContent side="right">{label}</TooltipContent>
            </Tooltip>
          ) : (
            <div key={to}>{link}</div>
          );
        })}
      </nav>
      <div className="mx-3 my-3 border-t border-sidebar-border" />
      <div className="flex min-h-0 flex-1 flex-col px-3">
        {collapsed ? (
          <ConnectionSelector />
        ) : (
          <>
            <div className="mb-1 flex items-center justify-between px-3 text-xs font-medium text-foreground/90">
              Connections
              <button
                onClick={() => openDialog({ mode: "add" })}
                aria-label="Add connection"
                title="Add connection"
                className="rounded p-1 hover:bg-muted"
              >
                <Plus className="size-4" />
              </button>
            </div>
            <div className="flex min-h-0 flex-col gap-0.5 overflow-auto">
              {conns?.map((c) => <ConnectionRow key={c.id} c={c} />)}
              {conns && conns.length === 0 && <p className="px-3 py-1 text-xs text-muted-foreground">No connections</p>}
            </div>
          </>
        )}
      </div>
      <div className="p-3">
        <NavLink to="/settings" className={({ isActive }) => routeClass(isActive, collapsed)}>
          <Settings className="size-[18px]" />
          {!collapsed && "Settings"}
        </NavLink>
      </div>
    </aside>
  );
}
