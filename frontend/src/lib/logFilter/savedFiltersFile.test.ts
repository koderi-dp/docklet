import { describe, expect, it } from "vitest";
import type { SavedFilter } from "@/stores/logFilterStore";
import { mergeImported, parseSavedFilters, serializeSavedFilters } from "./savedFiltersFile";

const saved = (id: string, name: string): SavedFilter => ({
  id,
  name,
  includeMode: "all",
  rules: [
    { id: "x1", kind: "include", target: "message", pattern: "GET /api/*", enabled: true },
    { id: "x2", kind: "highlight", target: "level", pattern: "error", enabled: false, colour: "destructive" },
  ],
});

const file = (filters: unknown, extra: Record<string, unknown> = {}) => JSON.stringify({ format: "docklet-log-filters", version: 1, filters, ...extra });
const okFilter = { name: "Errors", includeMode: "any", rules: [{ kind: "exclude", target: "message", pattern: "/health/", enabled: true }] };

describe("export and import round trip", () => {
  it("keeps names, modes and rules and drops ids", () => {
    const text = serializeSavedFilters([saved("a", "One")]);
    expect(text).not.toContain('"id"');
    const r = parseSavedFilters(text);
    expect(r.ok).toBe(true);
    if (!r.ok) return;
    expect(r.filters).toHaveLength(1);
    const f = r.filters[0];
    expect(f).toMatchObject({ name: "One", includeMode: "all" });
    expect(f.id).not.toBe("a");
    expect(f.rules.map(({ id: _id, ...rest }) => rest)).toEqual([
      { kind: "include", target: "message", pattern: "GET /api/*", enabled: true },
      { kind: "highlight", target: "level", pattern: "error", enabled: false, colour: "destructive" },
    ]);
    expect(new Set(f.rules.map((x) => x.id)).size).toBe(2);
  });

  it("keeps an invalid regex as it is (it is only shown as invalid later)", () => {
    const r = parseSavedFilters(file([{ ...okFilter, rules: [{ kind: "include", target: "message", pattern: "/(unclosed/", enabled: true }] }]));
    expect(r.ok && r.filters[0].rules[0].pattern).toBe("/(unclosed/");
  });

  it("defaults a highlight colour and drops a colour on other kinds", () => {
    const r = parseSavedFilters(
      file([
        {
          ...okFilter,
          rules: [
            { kind: "highlight", target: "message", pattern: "x", enabled: true },
            { kind: "include", target: "message", pattern: "y", enabled: true, colour: "info" },
          ],
        },
      ]),
    );
    expect(r.ok && r.filters[0].rules.map((x) => x.colour)).toEqual(["warning", undefined]);
  });
});

describe("parseSavedFilters rejects", () => {
  const bad = (text: string, message: RegExp) => {
    const r = parseSavedFilters(text);
    expect(r.ok).toBe(false);
    if (!r.ok) expect(r.error).toMatch(message);
  };
  it("text that is not JSON", () => bad("not json {", /not a JSON file/));
  it("JSON of another shape", () => {
    bad("[]", /not a Docklet/);
    bad(JSON.stringify({ filters: [] }), /not a Docklet/);
    bad("null", /not a Docklet/);
  });
  it("a missing, wrong or newer version", () => {
    bad(JSON.stringify({ format: "docklet-log-filters", filters: [okFilter] }), /version/);
    bad(file([okFilter], { version: "1" }), /version/);
    bad(file([okFilter], { version: 2 }), /newer Docklet/);
  });
  it("an empty or missing filter list", () => {
    bad(file([]), /no filters/);
    bad(file("nope"), /"filters"/);
  });
  it("a filter with a bad name, mode or rules", () => {
    bad(file([{ ...okFilter, name: "  " }]), /Filter 1: "name"/);
    bad(file([{ ...okFilter, name: 5 }]), /"name"/);
    bad(file([{ ...okFilter, includeMode: "some" }]), /includeMode/);
    bad(file([{ ...okFilter, rules: "x" }]), /"rules"/);
    bad(file([{ ...okFilter, rules: [] }]), /no rules/);
    bad(file([okFilter, 7]), /Filter 2 is not an object/);
  });
  it("a rule with a bad field, naming where it is", () => {
    const rule = { kind: "include", target: "message", pattern: "x", enabled: true };
    bad(file([{ ...okFilter, rules: [rule, { ...rule, kind: "run" }] }]), /Filter 1 \(“Errors”\), rule 2: "kind"/);
    bad(file([{ ...okFilter, rules: [{ ...rule, target: "file" }] }]), /"target"/);
    bad(file([{ ...okFilter, rules: [{ ...rule, pattern: "" }] }]), /"pattern"/);
    bad(file([{ ...okFilter, rules: [{ ...rule, pattern: 1 }] }]), /"pattern"/);
    bad(file([{ ...okFilter, rules: [{ ...rule, enabled: "yes" }] }]), /"enabled"/);
    bad(file([{ ...okFilter, rules: [{ ...rule, kind: "highlight", colour: "pink" }] }]), /"colour"/);
    bad(file([{ ...okFilter, rules: [{ ...rule, pattern: "x".repeat(2001) }] }]), /longer than/);
    bad(file([{ ...okFilter, rules: [null] }]), /not an object/);
  });
  it("too many filters", () => bad(file(Array.from({ length: 201 }, () => okFilter)), /more than 200/));
});

describe("mergeImported", () => {
  const imp = (name: string) => saved(`i-${name}`, name);
  it("appends filters with new names unchanged", () => {
    const r = mergeImported([saved("a", "One")], [imp("Two")]);
    expect(r).toMatchObject({ added: 1, renamed: 0 });
    expect(r.merged.map((s) => s.name)).toEqual(["One", "Two"]);
  });
  it("keeps both on a name collision, ignoring case", () => {
    const r = mergeImported([saved("a", "Errors")], [imp("ERRORS")]);
    expect(r.renamed).toBe(1);
    expect(r.merged.map((s) => s.name)).toEqual(["Errors", "ERRORS (imported)"]);
  });
  it("numbers repeated collisions, also between imported filters", () => {
    const r = mergeImported([saved("a", "A"), saved("b", "A (imported)")], [imp("A"), imp("A")]);
    expect(r.merged.map((s) => s.name)).toEqual(["A", "A (imported)", "A (imported 2)", "A (imported 3)"]);
    expect(r.renamed).toBe(2);
  });
  it("never changes the existing filters", () => {
    const existing = [saved("a", "A")];
    mergeImported(existing, [imp("A")]);
    expect(existing[0].name).toBe("A");
  });
});
