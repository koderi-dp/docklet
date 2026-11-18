import { ConnectionGate } from "@/components/shared/ConnectionGate";
import { PageHeader } from "@/components/shared/PageHeader";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { CleanupTab } from "@/components/system/CleanupTab";
import { DiskUsageTab } from "@/components/system/DiskUsageTab";
import { EventsTab } from "@/components/system/EventsTab";
import { OverviewTab } from "@/components/system/OverviewTab";

export default function SystemPage() {
  return (
    <ConnectionGate>
      {({ connectionId }) => (
        <div className="h-full overflow-auto p-5">
          <PageHeader title="System" subtitle="Docker Engine information, disk usage, events and cleanup" />
          <Tabs defaultValue="overview" className="gap-4">
            <TabsList className="w-fit">
              <TabsTrigger value="overview">Overview</TabsTrigger>
              <TabsTrigger value="disk">Disk Usage</TabsTrigger>
              <TabsTrigger value="events">Events</TabsTrigger>
              <TabsTrigger value="cleanup">Cleanup</TabsTrigger>
            </TabsList>
            {/* Radix unmounts inactive tabs, so events polling stops when the tab is left. */}
            <TabsContent value="overview"><OverviewTab connectionId={connectionId} /></TabsContent>
            <TabsContent value="disk"><DiskUsageTab connectionId={connectionId} /></TabsContent>
            <TabsContent value="events"><EventsTab connectionId={connectionId} /></TabsContent>
            <TabsContent value="cleanup"><CleanupTab connectionId={connectionId} /></TabsContent>
          </Tabs>
        </div>
      )}
    </ConnectionGate>
  );
}
