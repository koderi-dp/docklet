import { Fragment } from "react";
import { cn } from "@/lib/utils";
import type { TextRange } from "@/lib/logFilter/search";

/** A message with the search matches marked. The current match is drawn stronger. */
export function MarkedText({ text, ranges, current }: { text: string; ranges: readonly TextRange[]; current: boolean }) {
  const parts: React.ReactNode[] = [];
  let at = 0;
  ranges.forEach((r, i) => {
    if (r.start > at) parts.push(<Fragment key={`t${i}`}>{text.slice(at, r.start)}</Fragment>);
    parts.push(
      <mark
        key={`m${i}`}
        className={cn("rounded-[2px] px-px", current ? "bg-search-current text-search-current-foreground ring-1 ring-foreground/60" : "bg-search-match text-foreground")}
      >
        {text.slice(r.start, r.end)}
      </mark>,
    );
    at = r.end;
  });
  if (at < text.length) parts.push(<Fragment key="tail">{text.slice(at)}</Fragment>);
  return <>{parts}</>;
}
