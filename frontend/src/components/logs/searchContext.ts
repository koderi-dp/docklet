import { createContext, useContext } from "react";
import type { Search } from "@/lib/logFilter/search";
import { SearchRegistry } from "./searchRegistry";

export interface SearchContextValue {
  /** The active search (null: nothing to mark). */
  search: Search | null;
  /** Sequence number of the current match. */
  current: number | null;
  registry: SearchRegistry;
}

const idle: SearchContextValue = { search: null, current: null, registry: new SearchRegistry() };

/** Lets the lists below a LogView mark matches and be scrolled to one, without prop drilling through the split panes. */
export const LogSearchContext = createContext<SearchContextValue>(idle);

export const useLogSearchContext = () => useContext(LogSearchContext);
