import { Construction } from "lucide-react";

/** Honest placeholder for screens that are not built yet (DESIGN section 20: no dead UI). */
export default function Unavailable({ title }: { title: string }) {
  return (
    <div className="flex h-full flex-col items-center justify-center gap-2 text-center text-muted-foreground">
      <Construction className="size-8" />
      <h1 className="text-lg font-semibold text-foreground">{title}</h1>
      <p>This screen is not implemented yet.</p>
    </div>
  );
}
