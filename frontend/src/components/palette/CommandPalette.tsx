import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import {
  Box,
  Container,
  Database,
  Layers3,
  LayoutDashboard,
  Network,
  Plug,
  ServerCog,
  Settings,
  type LucideIcon,
} from "lucide-react";
import { ContainerService, ImageService, NetworkService, VolumeService } from "@bindings/services";
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandShortcut,
} from "@/components/ui/command";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { ConnectionDot } from "@/components/connections/ConnectionStatus";
import { stateText } from "@/components/connections/connectionMeta";
import { containersKey } from "@/hooks/useContainers";
import { useActiveConnection, useConnections, useConnectionStatus, useSelectConnection } from "@/hooks/useActiveConnection";
import { useUI } from "@/stores/uiStore";

const PAGES: { to: string; label: string; icon: LucideIcon }[] = [
  { to: "/dashboard", label: "Dashboard", icon: LayoutDashboard },
  { to: "/containers", label: "Containers", icon: Container },
  { to: "/images", label: "Images", icon: Box },
  { to: "/compose", label: "Compose", icon: Layers3 },
  { to: "/volumes", label: "Volumes", icon: Database },
  { to: "/networks", label: "Networks", icon: Network },
  { to: "/system", label: "System", icon: ServerCog },
  { to: "/connections", label: "Connections", icon: Plug },
  { to: "/settings", label: "Settings", icon: Settings },
];

const MAX_PER_GROUP = 8;
const norm = (s: string) => s.toLowerCase();
const matches = (q: string, ...fields: (string | undefined)[]) => !q || fields.some((f) => f && norm(f).includes(q));

function ConnectionItem({ id, name, onPick }: { id: string; name: string; onPick: () => void }) {
  const { data: status } = useConnectionStatus(id);
  return (
    <CommandItem value={`conn-${id}`} onSelect={onPick}>
      <ConnectionDot state={status?.state} />
      <span className="truncate">Switch to {name}</span>
      <CommandShortcut>{stateText(status?.state)}</CommandShortcut>
    </CommandItem>
  );
}

function PaletteBody({ close }: { close: () => void }) {
  const navigate = useNavigate();
  const select = useSelectConnection();
  const { data: conns } = useConnections();
  const { connectionId, profile, isConnected } = useActiveConnection();
  const [query, setQuery] = useState("");
  const q = norm(query.trim());

  // Resource search is scoped to the ACTIVE engine and only runs while the palette is open.
  const enabled = !!connectionId && isConnected;
  const containers = useQuery({
    queryKey: containersKey(connectionId ?? "none"),
    queryFn: () => ContainerService.List(connectionId as string),
    enabled,
    retry: false,
  });
  const images = useQuery({
    queryKey: ["palette", "images", connectionId],
    queryFn: () => ImageService.List(connectionId as string),
    enabled,
    retry: false,
    staleTime: 10_000,
  });
  const volumes = useQuery({
    queryKey: ["palette", "volumes", connectionId],
    queryFn: () => VolumeService.List(connectionId as string),
    enabled,
    retry: false,
    staleTime: 10_000,
  });
  const networks = useQuery({
    queryKey: ["palette", "networks", connectionId],
    queryFn: () => NetworkService.List(connectionId as string),
    enabled,
    retry: false,
    staleTime: 10_000,
  });

  const go = (to: string) => {
    close();
    navigate(to);
  };

  const pageHits = PAGES.filter((p) => matches(q, p.label));
  const connHits = (conns ?? []).filter((c) => matches(q, c.name, "switch connection engine"));
  const containerHits = useMemo(
    () => (q ? (containers.data ?? []).filter((c) => matches(q, c.name, c.id, c.image)) : []).slice(0, MAX_PER_GROUP),
    [containers.data, q],
  );
  const imageHits = useMemo(
    () => (q ? (images.data ?? []).filter((i) => matches(q, i.ref, i.repository, i.tag, i.id)) : []).slice(0, MAX_PER_GROUP),
    [images.data, q],
  );
  const volumeHits = useMemo(
    () => (q ? (volumes.data ?? []).filter((v) => matches(q, v.name)) : []).slice(0, MAX_PER_GROUP),
    [volumes.data, q],
  );
  const networkHits = useMemo(
    () => (q ? (networks.data ?? []).filter((n) => matches(q, n.name, n.id)) : []).slice(0, MAX_PER_GROUP),
    [networks.data, q],
  );
  const loadingRes = enabled && !!q && (containers.isLoading || images.isLoading || volumes.isLoading || networks.isLoading);
  const none =
    !pageHits.length && !connHits.length && !containerHits.length && !imageHits.length && !volumeHits.length && !networkHits.length;

  return (
    <Command shouldFilter={false} className="[&_[cmdk-item]]:px-2 [&_[cmdk-item]]:py-1.5">
      <CommandInput value={query} onValueChange={setQuery} placeholder="Search pages, connections and resources…" />
      <p className="border-b px-3 py-1.5 text-xs text-muted-foreground">
        {enabled
          ? `Resources are searched in ${profile?.name ?? "the active engine"} only.`
          : "Connect to an engine to search its containers, images, volumes and networks."}
      </p>
      <CommandList className="max-h-[360px]">
        {none && <CommandEmpty>{loadingRes ? "Searching…" : "No results."}</CommandEmpty>}
        {pageHits.length > 0 && (
          <CommandGroup heading="Go to">
            {pageHits.map(({ to, label, icon: Icon }) => (
              <CommandItem key={to} value={`page-${to}`} onSelect={() => go(to)}>
                <Icon />
                {label}
              </CommandItem>
            ))}
          </CommandGroup>
        )}
        {connHits.length > 0 && (
          <CommandGroup heading="Connections">
            {connHits.map((c) => (
              <ConnectionItem
                key={c.id}
                id={c.id}
                name={c.name}
                onPick={() => {
                  close();
                  void select(c.id);
                }}
              />
            ))}
          </CommandGroup>
        )}
        {containerHits.length > 0 && (
          <CommandGroup heading="Containers">
            {containerHits.map((c) => (
              <CommandItem key={c.id} value={`container-${c.id}`} onSelect={() => go(`/containers/${c.id}`)}>
                <Container />
                <span className="truncate">{c.name}</span>
                <CommandShortcut className="max-w-[45%] truncate tracking-normal">{c.image}</CommandShortcut>
              </CommandItem>
            ))}
          </CommandGroup>
        )}
        {imageHits.length > 0 && (
          <CommandGroup heading="Images">
            {imageHits.map((i) => (
              <CommandItem key={i.key} value={`image-${i.key}`} onSelect={() => go("/images")}>
                <Box />
                <span className="truncate">{i.dangling ? i.id.replace("sha256:", "").slice(0, 12) : i.ref}</span>
                {i.dangling && <CommandShortcut className="tracking-normal">untagged</CommandShortcut>}
              </CommandItem>
            ))}
          </CommandGroup>
        )}
        {volumeHits.length > 0 && (
          <CommandGroup heading="Volumes">
            {volumeHits.map((v) => (
              <CommandItem key={v.name} value={`volume-${v.name}`} onSelect={() => go("/volumes")}>
                <Database />
                <span className="truncate">{v.name}</span>
              </CommandItem>
            ))}
          </CommandGroup>
        )}
        {networkHits.length > 0 && (
          <CommandGroup heading="Networks">
            {networkHits.map((n) => (
              <CommandItem key={n.id} value={`network-${n.id}`} onSelect={() => go("/networks")}>
                <Network />
                <span className="truncate">{n.name}</span>
              </CommandItem>
            ))}
          </CommandGroup>
        )}
      </CommandList>
    </Command>
  );
}

/** Global command palette (Ctrl/Cmd+K). Mounted once in AppShell; state lives in useUI. */
export function CommandPalette() {
  const open = useUI((s) => s.paletteOpen);
  const setOpen = useUI((s) => s.setPaletteOpen);
  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogHeader className="sr-only">
        <DialogTitle>Command palette</DialogTitle>
        <DialogDescription>Search pages, connections and resources of the active engine.</DialogDescription>
      </DialogHeader>
      <DialogContent showCloseButton={false} className="top-[22%] translate-y-0 gap-0 overflow-hidden p-0 sm:max-w-xl">
        {/* Mounted only while open so queries and the search text reset every time. */}
        {open && <PaletteBody close={() => setOpen(false)} />}
      </DialogContent>
    </Dialog>
  );
}
