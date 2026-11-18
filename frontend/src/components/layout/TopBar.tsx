import { Check, ChevronDown, Moon, Plus, Search, Settings2, Sun } from "lucide-react";
import { shortcutLabel } from "@/lib/platform";
import { useNavigate } from "react-router-dom";
import { useUI } from "@/stores/uiStore";
import { useActiveConnection, useConnections, useConnectionStatus, useSelectConnection } from "@/hooks/useActiveConnection";
import { ConnectionDot } from "@/components/connections/ConnectionStatus";
import { stateText } from "@/components/connections/connectionMeta";
import { Button } from "@/components/ui/button";
import OperationCenter from "@/components/activity/OperationCenter";
import { RegistrySignInHost } from "@/components/registries/RegistrySignInDialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import type { ConnectionProfile } from "@bindings/model";

function PickerItem({ c, active, onPick }: { c: ConnectionProfile; active: boolean; onPick: () => void }) {
  const { data: status } = useConnectionStatus(c.id);
  return (
    <DropdownMenuItem onSelect={onPick} className="gap-2">
      <ConnectionDot state={status?.state} />
      <span className="truncate">{c.name}</span>
      <span className="ml-auto flex items-center gap-1.5 text-xs text-muted-foreground">
        {stateText(status?.state)}
        {active && <Check className="size-3.5 text-primary" />}
      </span>
    </DropdownMenuItem>
  );
}

export default function TopBar() {
  const { profile, status, connectionId } = useActiveConnection();
  const { data: conns } = useConnections();
  const select = useSelectConnection();
  const navigate = useNavigate();
  const theme = useUI((s) => s.theme);
  const setTheme = useUI((s) => s.setTheme);
  const setPaletteOpen = useUI((s) => s.setPaletteOpen);
  const openDialog = useUI((s) => s.openConnectionDialog);
  return (
    <header className="flex h-[60px] shrink-0 items-center gap-3 border-b border-border px-4 lg:px-5">
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="outline" className="h-9 w-48 shrink-0 justify-between gap-2 bg-card xl:w-60" aria-label="Active engine">
            <span className="flex min-w-0 items-center gap-2">
              <ConnectionDot state={status?.state} />
              <span className="truncate">{profile?.name ?? "No engine selected"}</span>
              {profile && <span className="hidden text-xs text-muted-foreground xl:inline">{stateText(status?.state)}</span>}
            </span>
            <ChevronDown className="size-4 shrink-0 opacity-60" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="w-72">
          {conns?.map((c) => (
            <PickerItem key={c.id} c={c} active={c.id === connectionId} onPick={() => void select(c.id)} />
          ))}
          {!conns?.length && <div className="px-2 py-1.5 text-xs text-muted-foreground">No connections</div>}
          <DropdownMenuSeparator />
          <DropdownMenuItem onSelect={() => openDialog({ mode: "add" })}>
            <Plus />
            Add connection…
          </DropdownMenuItem>
          <DropdownMenuItem onSelect={() => navigate("/connections")}>
            <Settings2 />
            Manage connections
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      <button
        type="button"
        onClick={() => setPaletteOpen(true)}
        className="mx-auto flex h-9 min-w-0 max-w-md flex-1 items-center gap-2 rounded-md border border-input bg-input/60 px-3 text-left text-muted-foreground hover:bg-input"
        aria-label="Open command palette"
      >
        <Search className="size-4 shrink-0" />
        <span className="min-w-0 flex-1 truncate text-sm">Search resources…</span>
        <kbd className="hidden text-xs lg:inline">{shortcutLabel("K")}</kbd>
      </button>
      <OperationCenter />
      <RegistrySignInHost />
      <Button variant="ghost" size="icon" onClick={() => setTheme(theme === "dark" ? "light" : "dark")} aria-label="Toggle theme">
        {theme === "dark" ? <Sun className="size-4" /> : <Moon className="size-4" />}
      </Button>
    </header>
  );
}
