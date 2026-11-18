import { Logo } from "./Logo";
import { NavLink } from "react-router-dom";
import { Box, Container, Database, Layers3, LayoutDashboard, Network, PanelLeft, ScrollText, ServerCog, Settings } from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
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

export default function SidebarNav() {
  const { collapsed, setCollapsed } = useSidebarCollapsed();
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
      {/* Engines are switched in the top bar and managed on the Connections page. */}
      <div className="min-h-0 flex-1" />
      <div className="p-3">
        <NavLink to="/settings" className={({ isActive }) => routeClass(isActive, collapsed)}>
          <Settings className="size-[18px]" />
          {!collapsed && "Settings"}
        </NavLink>
      </div>
    </aside>
  );
}
