import type { WSLInfo } from "@bindings/model";
import type { ReactNode } from "react";
import { Card } from "@/components/system/parts";
import { StatusDot } from "./StatusDot";

const DOCKER_LABEL: Record<string, { text: string; tone: "success" | "warning" | "muted" }> = {
  running: { text: "Running", tone: "success" },
  stopped: { text: "Stopped", tone: "warning" },
  "not-installed": { text: "Not installed", tone: "muted" },
  "docker-desktop": { text: "Docker Desktop integration", tone: "muted" },
};

/** One column: these cards are half the page wide, two columns would wrap every value. */
function Rows({ items }: { items: { label: string; value: ReactNode }[] }) {
  return (
    <dl>
      {items.map((it) => (
        <div key={it.label} className="grid grid-cols-[150px_1fr] gap-3 border-b border-border/50 py-2 text-[13px] last:border-b-0">
          <dt className="text-muted-foreground">{it.label}</dt>
          <dd className="min-w-0 break-words">{it.value === "" || it.value == null ? "—" : it.value}</dd>
        </div>
      ))}
    </dl>
  );
}

const yesNo = (b: boolean | null) => (b == null ? "" : b ? "Yes" : "No");

function systemd(i: WSLInfo): string {
  if (i.systemdEnabled == null && i.systemdRunning == null) return "";
  const conf = i.systemdEnabled == null ? "" : `${i.systemdEnabled ? "enabled" : "not enabled"} in wsl.conf`;
  const run = i.systemdRunning == null ? "" : i.systemdRunning ? "running" : "not running";
  return [conf, run].filter(Boolean).join(" · ");
}

export function WslDetails({ info }: { info: WSLInfo }) {
  const d = info.docker ? DOCKER_LABEL[info.docker] : undefined;
  return (
    <Card title="Distribution" subtitle="Read through wsl.exe and commands inside the distribution">
      <Rows
        items={[
          { label: "Name", value: info.distro },
          { label: "State", value: <StatusDot tone={info.running ? "success" : "muted"} text={info.running ? "Running" : "Stopped"} /> },
          { label: "WSL version", value: info.version ? String(info.version) : "" },
          { label: "Default distribution", value: yesNo(info.isDefault) },
          { label: "Operating system", value: info.osName },
          { label: "systemd", value: systemd(info) },
          { label: "Docker service", value: d ? <StatusDot tone={d.tone} text={d.text} /> : "" },
          { label: "Other running", value: info.running || (info.otherRunning ?? []).length > 0 ? ((info.otherRunning ?? []).join(", ") || "None") : "" },
        ]}
      />
    </Card>
  );
}

export function WslDiskNote({ info }: { info: WSLInfo }) {
  return (
    <Card title="Virtual disk" subtitle="The file Windows keeps for this distribution">
      <Rows
        items={[
          { label: "File", value: info.vhdxPath ? <span className="break-all font-mono text-xs">{info.vhdxPath}</span> : "" },
        ]}
      />
      <p className="mt-3 text-[13px] text-muted-foreground">
        The virtual disk grows as the distribution fills up and does not shrink when files are deleted. Making it smaller needs an elevated
        terminal and diskpart, so Docklet does not do it.
      </p>
    </Card>
  );
}
