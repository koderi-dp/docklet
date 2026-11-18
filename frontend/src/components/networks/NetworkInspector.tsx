import { useMemo, useState } from "react";
import { Network as NetworkIcon, Trash2, Unplug } from "lucide-react";
import { toast } from "sonner";
import type { Network, ResourceUser } from "@bindings/model";
import { Button } from "@/components/ui/button";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { InspectorShell, KeyValues, Row, SectionTitle, UserList, formatCreatedIso } from "@/components/volumes/parts";
import { useConnectContainer, useNetworkDetail } from "@/hooks/useNetworks";
import { useContainersQuery } from "@/hooks/useContainers";
import { errorMessage } from "@/lib/errors";

interface Props {
  connectionId: string;
  network: Network;
  busy: boolean;
  onClose: () => void;
  onRemove: (n: Network) => void;
}

export function NetworkInspector({ connectionId, network, busy, onClose, onRemove }: Props) {
  const detail = useNetworkDetail(connectionId, network.id);
  const containers = useContainersQuery(connectionId, true);
  const connect = useConnectContainer(connectionId);
  const [pick, setPick] = useState<string>("");
  const [confirm, setConfirm] = useState<{ connect: boolean; container: { id: string; name: string } } | null>(null);

  const d = detail.data;
  // Engine inspect is authoritative for endpoints; fall back to the list while it loads.
  const attached: ResourceUser[] = d ? (d.attached ?? []) : (network.attached ?? []);
  const attachedIds = useMemo(() => new Set(attached.map((a) => a.id)), [attached]);
  const candidates = useMemo(
    () => (containers.data ?? []).filter((c) => c.connectionId === connectionId && !attachedIds.has(c.id)),
    [containers.data, connectionId, attachedIds],
  );
  const connectable = !network.predefined || network.name === "bridge";
  const disabledReason = network.driver === "host" || network.driver === "null" ? `Containers cannot be attached to the ${network.name} network` : undefined;

  const run = async () => {
    if (!confirm) return;
    const { connect: doConnect, container } = confirm;
    setConfirm(null);
    try {
      await connect.mutateAsync({ networkId: network.id, containerId: container.id, connect: doConnect });
      toast.success(`${container.name} ${doConnect ? "connected to" : "disconnected from"} ${network.name}`);
      setPick("");
    } catch (e) {
      toast.error(`Could not ${doConnect ? "connect" : "disconnect"} ${container.name}`, {
        description: errorMessage(e),
        duration: Infinity,
        closeButton: true,
      });
    }
  };

  return (
    <InspectorShell
      icon={<NetworkIcon />}
      title={network.name}
      subtitle={network.predefined ? "Predefined network" : `${attached.length} attached container${attached.length === 1 ? "" : "s"}`}
      onClose={onClose}
      actions={
        <Button
          size="sm"
          variant="outline"
          disabled={busy || network.predefined}
          title={network.predefined ? "Predefined networks cannot be removed" : undefined}
          onClick={() => onRemove(network)}
          className="text-destructive hover:text-destructive"
        >
          <Trash2 className="size-3.5" /> Remove
        </Button>
      }
    >
      <dl>
        <Row label="Network ID"><span className="font-mono text-xs">{network.id.slice(0, 12)}</span></Row>
        <Row label="Driver">{network.driver || "—"}</Row>
        <Row label="Scope">{network.scope || "—"}</Row>
        <Row label="Internal">{network.internal ? "Yes" : "No"}</Row>
        <Row label="Attachable">{network.attachable ? "Yes" : "No"}</Row>
        <Row label="IPv6">{network.enableIPv6 ? "Enabled" : "Disabled"}</Row>
        <Row label="Created">{formatCreatedIso(network.created)}</Row>
      </dl>

      <SectionTitle>IPAM</SectionTitle>
      {detail.isLoading ? (
        <Skeleton className="h-10 w-full" />
      ) : detail.error ? (
        <p className="text-[13px] text-destructive">Could not inspect network: {String(detail.error)}</p>
      ) : (d?.ipam ?? []).length === 0 ? (
        <p className="text-[13px] text-muted-foreground">The Engine reports no IPAM configuration.</p>
      ) : (
        <ul className="space-y-1 font-mono text-xs">
          {(d?.ipam ?? []).map((c, i) => (
            <li key={i}>
              {c.subnet ? `subnet ${c.subnet}` : "no subnet"}
              {c.gateway ? ` · gateway ${c.gateway}` : ""}
              {c.ipRange ? ` · range ${c.ipRange}` : ""}
            </li>
          ))}
        </ul>
      )}
      {d?.ipamDriver && <p className="mt-1 text-xs text-muted-foreground">IPAM driver: {d.ipamDriver}</p>}

      <SectionTitle>Attached containers</SectionTitle>
      <UserList
        users={attached}
        empty="No container is attached."
        extra={(u) => (
          <Button
            size="icon"
            variant="ghost"
            className="size-6 shrink-0"
            aria-label={`Disconnect ${u.name}`}
            title={`Disconnect ${u.name}`}
            disabled={connect.isPending || !!disabledReason}
            onClick={() => setConfirm({ connect: false, container: { id: u.id, name: u.name } })}
          >
            <Unplug className="size-3.5" />
          </Button>
        )}
      />

      <div className="mt-3 flex items-center gap-2">
        <Select value={pick} onValueChange={setPick} disabled={!connectable || !!disabledReason || candidates.length === 0}>
          <SelectTrigger size="sm" className="h-8 min-w-0 flex-1" aria-label="Container to connect">
            <SelectValue placeholder={candidates.length === 0 ? "No other containers" : "Connect a container…"} />
          </SelectTrigger>
          <SelectContent>
            {candidates.map((c) => (
              <SelectItem key={c.id} value={c.id}>
                {c.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Button
          size="sm"
          variant="outline"
          disabled={!pick || connect.isPending || !!disabledReason}
          title={disabledReason}
          onClick={() => {
            const c = candidates.find((x) => x.id === pick);
            if (c) setConfirm({ connect: true, container: { id: c.id, name: c.name } });
          }}
        >
          Connect
        </Button>
      </div>
      {disabledReason && <p className="mt-1 text-xs text-muted-foreground">{disabledReason}.</p>}

      <SectionTitle>Labels</SectionTitle>
      <KeyValues data={network.labels} empty="No labels." />
      <SectionTitle>Options</SectionTitle>
      <KeyValues data={d?.options} empty="No driver options." />

      <AlertDialog open={!!confirm} onOpenChange={(o) => !o && setConfirm(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {confirm?.connect ? "Connect" : "Disconnect"} {confirm?.container.name}?
            </AlertDialogTitle>
            <AlertDialogDescription>
              {confirm?.connect
                ? `${confirm?.container.name} will be attached to ${network.name} and receive an address on it.`
                : `${confirm?.container.name} will lose connectivity through ${network.name}. Services reaching it over this network will be interrupted.`}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={() => void run()}>{confirm?.connect ? "Connect" : "Disconnect"}</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </InspectorShell>
  );
}
