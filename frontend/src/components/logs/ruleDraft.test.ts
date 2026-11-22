import { describe, expect, it } from "vitest";
import { draftValues, levelOptions } from "./ruleDraft";

describe("draftValues", () => {
  it("defaults for add mode", () => {
    expect(draftValues()).toEqual({ kind: "include", target: "message", colour: "warning", pattern: "", level: "error" });
  });

  it("puts a level rule's pattern in level, not pattern", () => {
    const v = draftValues({ kind: "exclude", target: "level", pattern: "warn*" });
    expect(v.level).toBe("warn*");
    expect(v.pattern).toBe("");
  });
});

describe("levelOptions", () => {
  it("lists the known levels only when the value is one of them", () => {
    expect(levelOptions("error")).not.toContain("");
    expect(levelOptions("error")).toEqual(levelOptions("warn"));
  });

  it("adds a value that is not a level so the select is not blank", () => {
    expect(levelOptions("warn*")).toContain("warn*");
  });
});
