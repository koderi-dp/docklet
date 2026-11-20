import { Bell } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { useAlertEvents, useAlerts } from "@/hooks/useNotifications";
import { useUI } from "@/stores/uiStore";
import { AlertsPanel } from "./AlertsPanel";

/** Top-bar bell with the unread-alert count; opens the alerts popover. */
export default function AlertsButton() {
  const { unreadCount } = useAlerts();
  const open = useUI((s) => s.alertsOpen);
  const setOpen = useUI((s) => s.setAlertsOpen);
  useAlertEvents();
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button variant="ghost" size="icon" className="relative" title="Alerts" aria-label={unreadCount > 0 ? `Alerts: ${unreadCount} unread` : "Alerts"}>
          <Bell className="size-4" />
          {unreadCount > 0 && (
            <Badge className="absolute -right-0.5 -top-0.5 h-4 min-w-4 justify-center rounded-full bg-destructive px-1 text-[10px] leading-none text-white">
              {unreadCount > 9 ? "9+" : unreadCount}
            </Badge>
          )}
        </Button>
      </PopoverTrigger>
      <PopoverContent align="end" className="w-[420px] p-0">
        <AlertsPanel />
      </PopoverContent>
    </Popover>
  );
}
