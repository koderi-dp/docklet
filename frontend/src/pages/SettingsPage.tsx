import type { ReactNode } from "react";
import { Link } from "react-router-dom";
import { Monitor, Moon, Sun } from "lucide-react";
import pkg from "../../package.json";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { PageHeader } from "@/components/shared/PageHeader";
import { cn } from "@/lib/utils";
import { useUI, type ContainerViewMode, type ThemePalette, type ThemePreference } from "@/stores/uiStore";

function Section({ title, description, children }: { title: string; description?: string; children: ReactNode }) {
  return (
    <section className="rounded-lg border bg-card">
      <div className="border-b px-4 py-3">
        <h2 className="text-sm font-semibold">{title}</h2>
        {description && <p className="text-xs text-muted-foreground">{description}</p>}
      </div>
      <div className="divide-y">{children}</div>
    </section>
  );
}

function Row({ label, htmlFor, hint, children }: { label: string; htmlFor?: string; hint?: ReactNode; children: ReactNode }) {
  return (
    <div className="flex flex-wrap items-center justify-between gap-3 px-4 py-3">
      <div className="min-w-0 max-w-md">
        <Label htmlFor={htmlFor} className="text-sm font-medium">
          {label}
        </Label>
        {hint && <p className="text-xs text-muted-foreground">{hint}</p>}
      </div>
      <div className="shrink-0">{children}</div>
    </div>
  );
}

function Segmented<T extends string>({
  value,
  onChange,
  options,
  label,
}: {
  value: T;
  onChange: (v: T) => void;
  options: { value: T; label: string; icon?: ReactNode }[];
  label: string;
}) {
  return (
    <div role="radiogroup" aria-label={label} className="inline-flex rounded-md border bg-background p-0.5">
      {options.map((o) => (
        <Button
          key={o.value}
          type="button"
          role="radio"
          aria-checked={value === o.value}
          variant="ghost"
          size="sm"
          onClick={() => onChange(o.value)}
          className={cn("h-7 gap-1.5 px-3", value === o.value && "bg-accent text-accent-foreground")}
        >
          {o.icon}
          {o.label}
        </Button>
      ))}
    </div>
  );
}

const LOG_TAILS = [100, 200, 500, 1000, 5000];

export default function SettingsPage() {
  const themePreference = useUI((s) => s.themePreference);
  const setThemePreference = useUI((s) => s.setThemePreference);
  const themePalette = useUI((s) => s.themePalette);
  const setThemePalette = useUI((s) => s.setThemePalette);
  const containerView = useUI((s) => s.containerView);
  const setContainerView = useUI((s) => s.setContainerView);
  const terminalFontSize = useUI((s) => s.terminalFontSize);
  const setTerminalFontSize = useUI((s) => s.setTerminalFontSize);
  const logTail = useUI((s) => s.logTail);
  const setLogTail = useUI((s) => s.setLogTail);
  const reconnect = useUI((s) => s.reconnectOnStartup);
  const setReconnect = useUI((s) => s.setReconnectOnStartup);

  return (
    <div className="max-w-3xl space-y-4 p-5">
      <PageHeader title="Settings" subtitle="Preferences are stored on this computer." />

      <Section title="Appearance">
        <Row label="Theme" hint="System follows your operating system and updates when it changes.">
          <Segmented<ThemePreference>
            label="Theme"
            value={themePreference}
            onChange={setThemePreference}
            options={[
              { value: "dark", label: "Dark", icon: <Moon className="size-3.5" /> },
              { value: "light", label: "Light", icon: <Sun className="size-3.5" /> },
              { value: "system", label: "System", icon: <Monitor className="size-3.5" /> },
            ]}
          />
        </Row>
        <Row label="Palette" hint="Docklet is the navy and violet design palette. shadcn is the stock shadcn/ui neutral base theme.">
          <Segmented<ThemePalette>
            label="Palette"
            value={themePalette}
            onChange={setThemePalette}
            options={[
              { value: "docklet", label: "Docklet" },
              { value: "shadcn", label: "shadcn" },
            ]}
          />
        </Row>
      </Section>

      <Section title="Behavior">
        <Row label="Default containers view" hint="Grouped shows Compose projects as collapsible groups.">
          <Segmented<ContainerViewMode>
            label="Default containers view"
            value={containerView}
            onChange={setContainerView}
            options={[
              { value: "grouped", label: "Grouped" },
              { value: "flat", label: "Flat" },
            ]}
          />
        </Row>
        <div className="px-4 py-3 text-xs text-muted-foreground">
          Deleting containers, images, volumes and networks always asks for confirmation. This cannot be turned off.
        </div>
      </Section>

      <Section title="Terminal & Logs">
        <Row label="Terminal font size" htmlFor="term-font" hint="In pixels, 10 to 24.">
          <div className="flex items-center gap-2">
            <Button variant="outline" size="icon-sm" aria-label="Decrease font size" title="Decrease font size" onClick={() => setTerminalFontSize(terminalFontSize - 1)} disabled={terminalFontSize <= 10}>
              -
            </Button>
            <output id="term-font" className="w-8 text-center text-sm tabular-nums">
              {terminalFontSize}
            </output>
            <Button variant="outline" size="icon-sm" aria-label="Increase font size" title="Increase font size" onClick={() => setTerminalFontSize(terminalFontSize + 1)} disabled={terminalFontSize >= 24}>
              +
            </Button>
          </div>
        </Row>
        <Row label="Default log lines" htmlFor="log-tail" hint="How many recent lines are loaded when logs open.">
          <Select value={String(logTail)} onValueChange={(v) => setLogTail(Number(v))}>
            <SelectTrigger id="log-tail" className="w-28" size="sm">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {LOG_TAILS.map((n) => (
                <SelectItem key={n} value={String(n)}>
                  {n}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </Row>
      </Section>

      <Section title="Connections">
        <Row label="Reconnect last engine on startup" htmlFor="reconnect" hint="When off, the app starts without connecting; use the connection picker to connect.">
          <Switch id="reconnect" checked={reconnect} onCheckedChange={setReconnect} />
        </Row>
        <Row label="Manage connections" hint="Add, edit, test and remove Docker Engine connections.">
          <Button variant="outline" size="sm" asChild>
            <Link to="/connections">Open connections</Link>
          </Button>
        </Row>
      </Section>

      <Section title="About">
        <Row label="Docklet" hint="Desktop client for existing Docker Engines.">
          <span className="text-sm text-muted-foreground tabular-nums">Version {pkg.version}</span>
        </Row>
      </Section>
    </div>
  );
}
