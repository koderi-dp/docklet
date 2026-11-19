import { describe, expect, it } from "vitest";
import { fromBase64, toBase64 } from "./util";

const roundTrip = (s: string) => new TextDecoder().decode(fromBase64(toBase64(s)));

describe("toBase64", () => {
  it("encodes the UTF-8 bytes, not UTF-16 code units", () => {
    expect(toBase64("hi")).toBe("aGk=");
    expect(toBase64("é")).toBe("w6k=");
    expect(toBase64("€")).toBe("4oKs");
  });
  it("encodes empty input as empty output", () => {
    expect(toBase64("")).toBe("");
  });
  it("encodes control characters sent by a terminal", () => {
    expect(toBase64("\r")).toBe("DQ==");
    expect(toBase64("\x03")).toBe("Aw==");
  });
});

describe("fromBase64", () => {
  it("returns the raw bytes", () => {
    expect([...fromBase64("aGk=")]).toEqual([104, 105]);
    expect([...fromBase64("w6k=")]).toEqual([0xc3, 0xa9]);
    expect(fromBase64("").length).toBe(0);
  });
  it("keeps bytes above 0x7f intact (no sign or charset mangling)", () => {
    expect([...fromBase64("//4A")]).toEqual([0xff, 0xfe, 0x00]);
  });
});

describe("round trip", () => {
  it("preserves ASCII, accents, CJK, emoji (surrogate pairs) and newlines", () => {
    for (const s of ["", "plain", "héllo wörld", "日本語のテキスト", "😀👍🏽 mixed", "a\r\nb\tc\u0000d"]) expect(roundTrip(s), s).toBe(s);
  });
  it("handles input larger than the 32 KiB encoding chunk, including a multi-byte char on the boundary", () => {
    const big = "é".repeat(40_000) + "😀".repeat(5_000) + "x".repeat(70_000);
    expect(roundTrip(big)).toBe(big);
  });
});
