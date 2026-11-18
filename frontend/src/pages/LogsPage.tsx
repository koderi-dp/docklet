import { ConnectionGate } from "@/components/shared/ConnectionGate";
import { LogsScreen } from "@/components/logs/LogsScreen";

export default function LogsPage() {
  return <ConnectionGate>{({ connectionId }) => <LogsScreen connectionId={connectionId} />}</ConnectionGate>;
}
