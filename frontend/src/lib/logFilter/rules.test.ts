import { describe, expect, it } from "vitest";
import type { LogLevel } from "@/lib/logLevel";
import { evaluate, type EvalResult, type FilterableLine, type Rule } from "./rules";

const line = (container: string, text: string, level: LogLevel = "info", time?: number): FilterableLine => ({ container, text, level, time });

const LINES: FilterableLine[] = [
  line("web", "GET /api/users 200"),
  line("web", "GET /health 200"),
  line("db", "checkpoint complete"),
  line("db", "ERROR connection refused", "error"),
  line("worker", "job 7 timeout after 30s", "warn"),
];

let n = 0;
const rule = (kind: Rule["kind"], pattern: string, extra: Partial<Rule> = {}): Rule => ({
  id: `r${++n}`,
  kind,
  target: "message",
  pattern,
  enabled: true,
  ...extra,
});
const texts = (r: EvalResult<FilterableLine>) => r.visible.map((l) => l.text);

describe("evaluate", () => {
  it("shows everything without rules", () => {
    const r = evaluate(LINES, [], { includeMode: "any" });
    expect(r.visible).toHaveLength(5);
    expect(r.counts).toEqual({});
  });

  it("several includes mean any of them by default", () => {
    const a = rule("include", "api");
    const b = rule("include", "/timeout|refused/");
    const r = evaluate(LINES, [a, b], { includeMode: "any" });
    expect(texts(r)).toEqual(["GET /api/users 200", "ERROR connection refused", "job 7 timeout after 30s"]);
    expect(r.counts[a.id]).toBe(1);
    expect(r.counts[b.id]).toBe(2);
  });

  it("include mode all requires every include rule", () => {
    const r = evaluate(LINES, [rule("include", "get"), rule("include", "*200")], { includeMode: "all" });
    expect(r.visible).toHaveLength(2);
    expect(evaluate(LINES, [rule("include", "get"), rule("include", "checkpoint")], { includeMode: "all" }).visible).toHaveLength(0);
  });

  it("exclude always wins over include", () => {
    const inc = rule("include", "GET *");
    const exc = rule("exclude", "/health");
    const r = evaluate(LINES, [inc, exc], { includeMode: "any" });
    expect(texts(r)).toEqual(["GET /api/users 200"]);
    expect(r.counts[inc.id]).toBe(2);
    expect(r.counts[exc.id]).toBe(1);
  });

  it("disabled rules do nothing and have no count", () => {
    const off = rule("exclude", "api", { enabled: false });
    const r = evaluate(LINES, [off], { includeMode: "any" });
    expect(r.visible).toHaveLength(5);
    expect(off.id in r.counts).toBe(false);
  });

  it("an invalid regex rule is ignored instead of hiding everything", () => {
    const bad = rule("include", "/(oops/");
    const r = evaluate(LINES, [bad], { includeMode: "any" });
    expect(r.visible).toHaveLength(5);
    expect(bad.id in r.counts).toBe(false);
  });

  it("targets the container name as a whole, with wildcards", () => {
    const only = evaluate(LINES, [rule("include", "db", { target: "container" })], { includeMode: "any" });
    expect(only.visible.map((l) => l.container)).toEqual(["db", "db"]);
    const hidden = evaluate(LINES, [rule("exclude", "w*", { target: "container" })], { includeMode: "any" });
    expect(hidden.visible.map((l) => l.container)).toEqual(["db", "db"]);
    expect(evaluate(LINES, [rule("include", "d", { target: "container" })], { includeMode: "any" }).visible).toHaveLength(0);
  });

  it("targets the level", () => {
    const r = evaluate(LINES, [rule("include", "warn", { target: "level" })], { includeMode: "any" });
    expect(texts(r)).toEqual(["job 7 timeout after 30s"]);
  });

  it("highlight colours lines without filtering; the first matching rule wins", () => {
    const h1 = rule("highlight", "200", { colour: "success" });
    const h2 = rule("highlight", "health", { colour: "warning" });
    const r = evaluate(LINES, [h1, h2], { includeMode: "any" });
    expect(r.visible).toHaveLength(5);
    expect(r.highlights).toEqual(["success", "success", undefined, undefined, undefined]);
    expect(r.counts[h1.id]).toBe(2);
    expect(r.counts[h2.id]).toBe(1);
  });

  it("level filter narrows the view, and level counts ignore it", () => {
    const r = evaluate(LINES, [], { includeMode: "any", levels: new Set<LogLevel>(["error"]) });
    expect(texts(r)).toEqual(["ERROR connection refused"]);
    expect(r.levelCounts).toMatchObject({ info: 3, error: 1, warn: 1 });
  });

  it("since drops older lines and keeps lines without a time", () => {
    const lines = [line("a", "old", "info", 1000), line("a", "new", "info", 5000), line("a", "unknown")];
    const r = evaluate(lines, [], { includeMode: "any", since: 3000 });
    expect(texts(r)).toEqual(["new", "unknown"]);
  });

  it("counts do not depend on other rules", () => {
    const exc = rule("exclude", "api");
    const hil = rule("highlight", "api", { colour: "info" });
    const r = evaluate(LINES, [exc, hil], { includeMode: "any" });
    expect(r.counts[exc.id]).toBe(1);
    expect(r.counts[hil.id]).toBe(1);
  });
});
