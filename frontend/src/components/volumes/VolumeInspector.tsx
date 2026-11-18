import { Database, Trash2 } from "lucide-react";
import type { Volume } from "@bindings/model";
import { Button } from "@/components/ui/button";
import { PathText } from "@/components/shared/CopyValue";
import { VolumeFileActions } from "@/components/files/VolumeFiles";
import { InspectorShell, KeyValues, Row, SectionTitle, UserList, formatCreatedIso } from "./parts";

export function VolumeInspector({ volume, busy, onClose, onRemove }: { volume: Volume; busy: boolean; onClose: () => void; onRemove: (v: Volume) => void }) {
  const users = volume.inUseBy ?? [];
  return (
    <InspectorShell
      icon={<Database />}
      title={volume.name}
      subtitle={users.length > 0 ? `In use by ${users.length} container${users.length === 1 ? "" : "s"}` : "Unused"}
      onClose={onClose}
      actions={
        <Button size="sm" variant="outline" disabled={busy} onClick={() => onRemove(volume)} className="text-destructive hover:text-destructive">
          <Trash2 className="size-3.5" /> Remove
        </Button>
      }
    >
      <dl>
        <Row label="Driver">{volume.driver || "—"}</Row>
        <Row label="Scope">{volume.scope || "—"}</Row>
        <Row label="Mountpoint"><span className="font-mono text-xs"><PathText value={volume.mountpoint || "—"} /></span></Row>
        <Row label="Created">{formatCreatedIso(volume.created)}</Row>
      </dl>
      <SectionTitle>Attached containers</SectionTitle>
      <UserList users={users} empty="No container mounts this volume." />
      <SectionTitle>Labels</SectionTitle>
      <KeyValues data={volume.labels} empty="No labels." />
      <SectionTitle>Driver options</SectionTitle>
      <KeyValues data={volume.options} empty="No driver options." />
      <SectionTitle>Contents</SectionTitle>
      <VolumeFileActions connectionId={volume.connectionId} volume={volume} />
    </InspectorShell>
  );
}
