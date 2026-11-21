import { describe, expect, it } from "vitest";
import { filterComposeLogs, parseComposeLogs, type RawComposeLogLine } from "./composeLogs";

const line = (service: string, text: string): RawComposeLogLine => ({ service, container: `${service}-1`, timestamp: "", stream: "stdout", text });

describe("parseComposeLogs", () => {
  it("strips escape codes into spans and numbers rows by their place in the snapshot", () => {
    const rows = parseComposeLogs([line("web", "\u001b[31mred\u001b[0m"), line("db", "plain")]);
    expect(rows.map((r) => [r.seq, r.text])).toEqual([
      [0, "red"],
      [1, "plain"],
    ]);
    expect(rows[0].spans?.[0]).toMatchObject({ start: 0, end: 3, style: { fg: 1 } });
    expect(rows[1].spans).toBeUndefined();
  });
  it("treats a missing snapshot as empty", () => {
    expect(parseComposeLogs(undefined)).toEqual([]);
    expect(parseComposeLogs(null)).toEqual([]);
  });
});

describe("filterComposeLogs", () => {
  const rows = parseComposeLogs([line("web", "a"), line("db", "b"), line("web", "c")]);
  it("keeps the parsed rows and their keys, without parsing again", () => {
    const web = filterComposeLogs(rows, "web");
    expect(web.map((r) => r.seq)).toEqual([0, 2]);
    expect(web[0]).toBe(rows[0]);
  });
  it("returns the snapshot itself for all services", () => {
    expect(filterComposeLogs(rows, "all")).toBe(rows);
  });
});
