import { describe, expect, it } from "vitest";
import { compilePattern, getPattern, isRegexPattern, literalPattern } from "./pattern";

describe("compilePattern", () => {
  it("plain text is a case-insensitive substring and treats regex characters literally", () => {
    const p = compilePattern("a.b(");
    expect(p.kind).toBe("text");
    expect(p.error).toBeNull();
    expect(p.test("xx A.B( yy")).toBe(true);
    expect(p.test("axb(")).toBe(false);
  });

  it("* and ? work as wildcards", () => {
    const star = compilePattern("GET /api/*");
    expect(star.kind).toBe("wildcard");
    expect(star.test("req: get /api/users 200")).toBe(true);
    expect(star.test("get /web/users")).toBe(false);
    expect(compilePattern("*bootstrap*").test("Bootstrapping done")).toBe(true);
    const one = compilePattern("job-?");
    expect(one.test("job-7 started")).toBe(true);
    expect(one.test("job-")).toBe(false);
  });

  it("an escaped wildcard is literal", () => {
    const p = compilePattern("what\\?");
    expect(p.kind).toBe("text");
    expect(p.test("what? no")).toBe(true);
    expect(p.test("whatx")).toBe(false);
  });

  it("/regex/ is case-insensitive by default, explicit flags win", () => {
    expect(compilePattern("/timeout|refused/").test("Connection REFUSED")).toBe(true);
    const sensitive = compilePattern("/Err/s");
    expect(sensitive.test("an Err here")).toBe(true);
    expect(sensitive.test("an err here")).toBe(false);
  });

  it("g and y flags are dropped so repeated tests stay stable", () => {
    const p = compilePattern("/x/g");
    expect(p.test("x")).toBe(true);
    expect(p.test("x")).toBe(true);
  });

  it("an invalid regex reports an error and never throws or matches", () => {
    const p = compilePattern("/(unclosed/");
    expect(p.kind).toBe("regex");
    expect(p.error).toBeTruthy();
    expect(p.test("(unclosed")).toBe(false);
  });

  it("an empty pattern is an error", () => {
    expect(compilePattern("").error).toBeTruthy();
  });

  it("whole matching anchors text and wildcards, not regexes", () => {
    const text = compilePattern("web", { whole: true });
    expect(text.test("web")).toBe(true);
    expect(text.test("web-db")).toBe(false);
    expect(compilePattern("health*", { whole: true }).test("healthcheck-1")).toBe(true);
    expect(compilePattern("/web/", { whole: true }).test("my-web-1")).toBe(true);
  });

  it("memoises compiled patterns", () => {
    expect(getPattern("abc")).toBe(getPattern("abc"));
    expect(getPattern("abc", { whole: true })).not.toBe(getPattern("abc"));
  });

  it("literalPattern matches the original text exactly", () => {
    for (const text of ["took 5* ms?", "/not a regex/", "C:\\dir\\*", "plain"]) {
      const p = compilePattern(literalPattern(text));
      expect(p.error).toBeNull();
      expect(p.test(`prefix ${text} suffix`)).toBe(true);
    }
    expect(isRegexPattern(literalPattern("/x/"))).toBe(false);
    expect(compilePattern(literalPattern("a*b")).test("aXb")).toBe(false);
  });
});
