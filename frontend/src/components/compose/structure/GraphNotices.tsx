import { Network } from "lucide-react";
import type { ComposeGraph } from "@bindings/model";
import { EmptyState } from "@/components/shared/EmptyState";

/** Shown instead of the canvas when the graph has no services. */
export function EmptyGraph({ graph }: { graph: ComposeGraph }) {
  if (graph.source === "containers") {
    return (
      <EmptyState icon={<Network />} title="Nothing to draw yet">
        <p>{graph.note}</p>
      </EmptyState>
    );
  }
  return (
    <EmptyState icon={<Network />} title="No services to draw">
      <p>This project defines no services.</p>
    </EmptyState>
  );
}

/** Warning line above the canvas when the graph was drawn from containers instead of the config. */
export function FallbackNote({ note }: { note: string }) {
  return (
    <p className="border-b border-border px-3 py-1.5 text-xs text-warning">
      Drawn from this project&apos;s containers: {note.replace(/\.+$/, "")}. Dependencies may be incomplete.
    </p>
  );
}
