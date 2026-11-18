import { useEffect, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { ConnectionGate } from "@/components/shared/ConnectionGate";
import { PageHeader } from "@/components/shared/PageHeader";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { PortsTab } from "@/components/ports/PortsTab";
import { CleanupTab } from "@/components/system/CleanupTab";
import { DiskUsageTab } from "@/components/system/DiskUsageTab";
import { EventsTab } from "@/components/system/EventsTab";
import { OverviewTab } from "@/components/system/OverviewTab";

const TABS = ["overview", "disk", "events", "ports", "cleanup"];

export default function SystemPage() {
  // ?tab=ports&q=8080 opens the Ports tab with a search (links from port conflict messages).
  const [params] = useSearchParams();
  const wanted = params.get("tab") ?? "overview";
  const [tab, setTab] = useState(TABS.includes(wanted) ? wanted : "overview");
  useEffect(() => {
    if (TABS.includes(wanted)) setTab(wanted);
  }, [wanted, params]);
  return (
    <ConnectionGate>
      {({ connectionId }) => (
        <div className="h-full overflow-auto p-5">
          <PageHeader title="System" subtitle="Docker Engine information, disk usage, events, ports and cleanup" />
          <Tabs value={tab} onValueChange={setTab} className="gap-4">
            <TabsList className="w-fit">
              <TabsTrigger value="overview">Overview</TabsTrigger>
              <TabsTrigger value="disk">Disk Usage</TabsTrigger>
              <TabsTrigger value="events">Events</TabsTrigger>
              <TabsTrigger value="ports">Ports</TabsTrigger>
              <TabsTrigger value="cleanup">Cleanup</TabsTrigger>
            </TabsList>
            {/* Radix unmounts inactive tabs, so events polling stops when the tab is left. */}
            <TabsContent value="overview"><OverviewTab connectionId={connectionId} /></TabsContent>
            <TabsContent value="disk"><DiskUsageTab connectionId={connectionId} /></TabsContent>
            <TabsContent value="events"><EventsTab connectionId={connectionId} /></TabsContent>
            <TabsContent value="ports"><PortsTab key={params.get("q") ?? ""} connectionId={connectionId} /></TabsContent>
            <TabsContent value="cleanup"><CleanupTab connectionId={connectionId} /></TabsContent>
          </Tabs>
        </div>
      )}
    </ConnectionGate>
  );
}
