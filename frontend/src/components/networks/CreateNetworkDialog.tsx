import { useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { LabelsField, parseLabels } from "@/components/volumes/CreateVolumeDialog";
import { useCreateNetwork } from "@/hooks/useNetworks";
import { errorMessage } from "@/lib/errors";

export function CreateNetworkDialog({ connectionId, open, onOpenChange }: { connectionId: string; open: boolean; onOpenChange: (o: boolean) => void }) {
  const [name, setName] = useState("");
  const [driver, setDriver] = useState("bridge");
  const [subnet, setSubnet] = useState("");
  const [gateway, setGateway] = useState("");
  const [internal, setInternal] = useState(false);
  const [attachable, setAttachable] = useState(false);
  const [labelsText, setLabelsText] = useState("");
  const [error, setError] = useState<string | null>(null);
  const create = useCreateNetwork(connectionId);

  const reset = () => {
    setName("");
    setDriver("bridge");
    setSubnet("");
    setGateway("");
    setInternal(false);
    setAttachable(false);
    setLabelsText("");
    setError(null);
  };

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    const parsed = parseLabels(labelsText);
    if (parsed.error) return setError(parsed.error);
    setError(null);
    try {
      const n = await create.mutateAsync({
        name: name.trim(),
        driver: driver.trim(),
        subnet: subnet.trim(),
        gateway: gateway.trim(),
        internal,
        attachable,
        labels: parsed.labels,
      });
      toast.success(`Network ${n.name} created`);
      reset();
      onOpenChange(false);
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        if (!o) reset();
        onOpenChange(o);
      }}
    >
      <DialogContent className="sm:max-w-md">
        <form onSubmit={submit} className="space-y-4">
          <DialogHeader>
            <DialogTitle>Create network</DialogTitle>
            <DialogDescription>Creates a network on this Engine. Subnet and gateway are optional; the Engine assigns them when left empty.</DialogDescription>
          </DialogHeader>
          <div className="space-y-1.5">
            <Label htmlFor="net-name">Name</Label>
            <Input id="net-name" value={name} onChange={(e) => setName(e.target.value)} autoFocus placeholder="my-network" className="h-8" />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="net-driver">Driver</Label>
            <Input id="net-driver" value={driver} onChange={(e) => setDriver(e.target.value)} placeholder="bridge" className="h-8" />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label htmlFor="net-subnet">Subnet (CIDR)</Label>
              <Input id="net-subnet" value={subnet} onChange={(e) => setSubnet(e.target.value)} placeholder="172.30.0.0/16" className="h-8 font-mono" />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="net-gw">Gateway</Label>
              <Input id="net-gw" value={gateway} onChange={(e) => setGateway(e.target.value)} placeholder="172.30.0.1" className="h-8 font-mono" />
            </div>
          </div>
          <div className="flex flex-col gap-2 text-[13px]">
            <label className="flex items-center gap-2">
              <Checkbox checked={internal} onCheckedChange={(c) => setInternal(c === true)} /> Internal (no external connectivity)
            </label>
            <label className="flex items-center gap-2">
              <Checkbox checked={attachable} onCheckedChange={(c) => setAttachable(c === true)} /> Attachable
            </label>
          </div>
          <LabelsField id="net-labels" value={labelsText} onChange={setLabelsText} />
          {error && <p className="text-sm text-destructive" role="alert">{error}</p>}
          <DialogFooter>
            <Button type="button" variant="outline" size="sm" onClick={() => onOpenChange(false)}>Cancel</Button>
            <Button type="submit" size="sm" disabled={!name.trim() || create.isPending}>
              {create.isPending ? "Creating…" : "Create network"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
