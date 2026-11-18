import { useState } from "react";
import { ChevronRight } from "lucide-react";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import { breadcrumbs } from "./paths";

interface Props {
  path: string;
  /** Shown in the breadcrumb root, e.g. the volume name. */
  rootLabel?: string;
  input: string;
  onInputChange: (value: string) => void;
  onSubmit: () => void;
  onGo: (path: string) => void;
}

/** Clickable breadcrumbs that turn into a path input when the empty area is clicked. */
export function FilePathBar({ path, rootLabel, input, onInputChange, onSubmit, onGo }: Props) {
  const [editing, setEditing] = useState(false);
  if (editing) {
    return (
      <form
        className="flex min-w-0 flex-1 items-center"
        onSubmit={(e) => {
          e.preventDefault();
          onSubmit();
          setEditing(false);
        }}
      >
        <Input
          autoFocus
          value={input}
          onChange={(e) => onInputChange(e.target.value)}
          onBlur={() => setEditing(false)}
          onKeyDown={(e) => e.key === "Escape" && setEditing(false)}
          onFocus={(e) => e.currentTarget.select()}
          aria-label="Path"
          spellCheck={false}
          className="h-8 font-mono text-xs"
        />
      </form>
    );
  }
  const crumbs = breadcrumbs(path, rootLabel ?? "/");
  return (
    <nav
      aria-label="Path"
      className="flex h-8 min-w-0 flex-1 cursor-text items-center gap-0.5 overflow-hidden rounded-md border border-input bg-input/30 px-1.5 font-mono text-xs text-muted-foreground transition-colors duration-100 hover:bg-input/50"
      onClick={() => setEditing(true)}
      title="Click the empty area to type a path"
    >
      {crumbs.map((c, i) => (
        <span key={c.path} className="flex shrink-0 items-center gap-0.5">
          {i > (rootLabel ? 0 : 1) && <ChevronRight aria-hidden className="size-3" />}
          <button
            type="button"
            className={cn("rounded px-1 outline-none transition-colors duration-100 hover:bg-accent hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/60", c.path === path && "text-foreground")}
            onClick={(e) => {
              e.stopPropagation();
              onGo(c.path);
            }}
          >
            {c.label}
          </button>
        </span>
      ))}
    </nav>
  );
}
